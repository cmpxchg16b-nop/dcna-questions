// Package session provides anonymous, in-memory session management for HTTP
// handlers.
//
// A Session carries per-visitor state (such as a counter) and is identified by
// a random UUID stored in a cookie. Sessions expire after a sliding TTL that is
// renewed on each use. SessionManager abstracts how sessions are looked up and
// plumbed through request contexts; OnMemorySessionManager is the provided
// in-memory implementation.
package session

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	// CookieName is the name of the cookie carrying the session id.
	CookieName = "session_id"
	// DefaultTTL is how long a session stays valid after its last use.
	DefaultTTL = 7 * 24 * time.Hour
)

type ExamSession struct {
	// Point to an on-going exam
	ExamId string

	// TableVersion, increment at client-side before every update
	TblVer int

	ExamAnswersXML string
}

// Session holds the server-side state for a single anonymous session. It carries
// its own id; the counter and expiry are atomics so they can be read and mutated
// from concurrent requests without a mutex.
type Session struct {
	id           string
	counter      atomic.Int64
	examSessions sync.Map     // map[string]ExamSession
	expiresAt    atomic.Int64 // unix nanos
}

// Id returns the session's identifier.
func (s *Session) Id() string { return s.id }

// Counter returns the current counter value.
func (s *Session) Counter() int64 { return s.counter.Load() }

// SetCounter replaces the counter value.
func (s *Session) SetCounter(v int64) { s.counter.Store(v) }

// IncrCounter increments the counter by one and returns the new value.
func (s *Session) IncrCounter() int64 { return s.counter.Add(1) }

// ListExams returns the ids of every exam currently tracked by the session. The
// ids are the keys of the underlying concurrent map. The returned slice should
// be treated as a snapshot; concurrent updates are not reflected in it.
func (s *Session) ListExams() []string {
	var ids []string
	s.examSessions.Range(func(key, _ any) bool {
		ids = append(ids, key.(string))
		return true
	})
	return ids
}

// GetExamById returns the ExamSession for the given exam id. If no such exam is
// tracked by this session yet, a fresh ExamSession seeded with the id is stored
// and returned (LoadOrStore semantics); the result is therefore never nil.
// Because ExamSession values are immutable, a returned pointer is a stable
// snapshot that never races with later updates.
func (s *Session) GetExamById(id string) *ExamSession {
	v, _ := s.examSessions.LoadOrStore(id, ExamSession{ExamId: id})
	sess := v.(ExamSession)
	return &sess
}

// UpdateExam atomically replaces the ExamSession for id with new only if the
// current value equals old. It reports whether the swap succeeded. On failure
// the caller is responsible for resolving the conflict (e.g. re-reading and
// retrying), since ExamSession values are immutable and updated via copy.
func (s *Session) UpdateExam(id string, old, new ExamSession) bool {
	return s.examSessions.CompareAndSwap(id, old, new)
}

// Expiry returns the time at which the session is no longer valid.
func (s *Session) Expiry() time.Time { return time.Unix(0, s.expiresAt.Load()) }

// SessionManager abstracts session storage and the wiring of a Session into a
// request context.
type SessionManager interface {
	// GetSessionById returns the live session for id. A missing or expired
	// session reports ok == false.
	GetSessionById(id string) (*Session, bool)
	// GetSessionFromContext returns the Session attached to ctx, if any.
	GetSessionFromContext(ctx context.Context) (*Session, bool)
	// WithSession returns ctx with sess attached so downstream handlers can
	// retrieve it via GetSessionFromContext.
	WithSession(ctx context.Context, sess *Session) context.Context
}

// sessionContextKey is the context key used to stash a *Session.
type sessionContextKey struct{}

// OnMemorySessionManager is a SessionManager backed by an in-memory map guarded
// by a sync.Map. Sessions do not survive process restarts and are not shared
// across instances.
type OnMemorySessionManager struct {
	sessions sync.Map // map[string]*Session
	ttl      time.Duration
}

// NewOnMemorySessionManager returns an in-memory SessionManager using the
// default 7d TTL and "session_id" cookie name.
func NewOnMemorySessionManager() *OnMemorySessionManager {
	return &OnMemorySessionManager{ttl: DefaultTTL}
}

// TTL returns the sliding expiration window applied to sessions.
func (m *OnMemorySessionManager) TTL() time.Duration { return m.ttl }

// GetSessionById returns the live session for id. Expired entries are evicted
// lazily.
func (m *OnMemorySessionManager) GetSessionById(id string) (*Session, bool) {
	v, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	sess := v.(*Session)
	if time.Now().After(sess.Expiry()) {
		m.sessions.Delete(id)
		return nil, false
	}
	return sess, true
}

// GetSessionFromContext returns the Session attached to ctx by WithSession.
func (m *OnMemorySessionManager) GetSessionFromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(sessionContextKey{}).(*Session)
	return sess, ok
}

// WithSession returns ctx with sess attached.
func (m *OnMemorySessionManager) WithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, sess)
}

// CreateSession allocates a fresh anonymous session with a random UUID id and a
// full TTL, stores it, and returns the id and session.
func (m *OnMemorySessionManager) CreateSession() (string, *Session) {
	id := uuid.NewString()
	sess := &Session{id: id}
	sess.expiresAt.Store(time.Now().Add(m.ttl).UnixNano())
	m.sessions.Store(id, sess)
	return id, sess
}

// RenewSession extends the session's expiry back to a full TTL from now.
func (m *OnMemorySessionManager) RenewSession(sess *Session) {
	sess.expiresAt.Store(time.Now().Add(m.ttl).UnixNano())
}

// SetCookie writes the session id into the response cookie with a Max-Age
// matching the TTL.
func (m *OnMemorySessionManager) SetCookie(w http.ResponseWriter, id string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    id,
		Path:     "/",
		MaxAge:   int(m.ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// WithSessionId ensures every request carries a valid anonymous session.
//
//   - If no session_id cookie is present, or it refers to an unknown/expired
//     session, a new anonymous session is created with a random UUID id and the
//     default 7d TTL.
//   - If a valid session_id cookie is present, the session's TTL is renewed to
//     the full window.
//
// In both cases the (possibly new) id is written back to the response cookie and
// the resolved Session is attached to the request context via the SessionManager.
func WithSessionId(h http.Handler, sm *OnMemorySessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			id   string
			sess *Session
		)

		if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
			if s, ok := sm.GetSessionById(c.Value); ok {
				// Existing, valid session: slide its expiration forward.
				id, sess = c.Value, s
				sm.RenewSession(sess)
			}
		}
		if sess == nil {
			// No cookie, unknown id, or expired: start fresh.
			id, sess = sm.CreateSession()
		}

		sm.SetCookie(w, id)
		h.ServeHTTP(w, r.WithContext(sm.WithSession(r.Context(), sess)))
	})
}
