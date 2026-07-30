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

// Session holds the server-side state for a single anonymous session. Both the
// counter and the expiry are atomics so they can be read and mutated from
// concurrent requests without a mutex.
type Session struct {
	counter   atomic.Int64
	expiresAt atomic.Int64 // unix nanos
}

// Counter returns the current counter value.
func (s *Session) Counter() int64 { return s.counter.Load() }

// SetCounter replaces the counter value.
func (s *Session) SetCounter(v int64) { s.counter.Store(v) }

// IncrCounter increments the counter by one and returns the new value.
func (s *Session) IncrCounter() int64 { return s.counter.Add(1) }

// expiry returns the time at which the session is no longer valid.
func (s *Session) expiry() time.Time { return time.Unix(0, s.expiresAt.Load()) }

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
	if time.Now().After(sess.expiry()) {
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
	sess := &Session{}
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
