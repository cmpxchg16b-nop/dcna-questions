package session

import (
	"context"
	pkgutils "dcna-questions/pkg/utils"
	"net/http"
	"time"
)


type Session struct {
	id           string
	subjectId    string
	username     string
	email        string
	expiresAtSec    int64
}

func (s *Session) Id() string { return s.id }

func (s *Session) SubjectId() string { return s.subjectId }

func (s *Session) Username() string { return s.username }

func (s *Session) Email() string { return s.email }

func (s *Session) Expiry() time.Time { return time.Unix(s.expiresAtSec, 0) }

type SessionManager interface {
	// GetSessionFromContext returns the Session attached to ctx, if any.
	GetSessionFromContext(ctx context.Context) (*Session, bool)

	// WithSession returns ctx with sess attached so downstream handlers can
	// retrieve it via GetSessionFromContext.
	WithSession(ctx context.Context, sess *Session) context.Context
}

// A completely stateless session manager that out-sourced everything to context
type OnMemorySessionManager struct { }

func NewOnMemorySessionManager() *OnMemorySessionManager {
	return &OnMemorySessionManager{}
}

func (m *OnMemorySessionManager) GetSessionFromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(pkgutils.CtxKeySessionObject).(*Session)
	return sess, ok
}

// WithSession returns ctx with sess attached.
func (m *OnMemorySessionManager) WithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, pkgutils.CtxKeySessionObject, sess)
}

// This middleware is expected the be chained before jwt middleware, in another word,
// it is expected that the request is flowed through the jwt middleware before it hits this.
func WithSessionId(h http.Handler, sm *OnMemorySessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sess Session

		ctx := r.Context()

		if jwtIdAny := ctx.Value(pkgutils.CtxKeySessionId); jwtIdAny != nil {
			sess.id = jwtIdAny.(string)
		}

		if subjectIdAny := ctx.Value(pkgutils.CtxKeySubjectId); subjectIdAny != nil {
			sess.subjectId = subjectIdAny.(string)
		}

		if usernameAny := ctx.Value(pkgutils.CtxKeyUsername); usernameAny != nil {
			sess.username = usernameAny.(string)
		}

		if emailAny := ctx.Value(pkgutils.CtxKeyEmail); emailAny != nil {
			sess.email = emailAny.(string)
		}

		if expAny := ctx.Value(pkgutils.CtxKeySessionTTLSecs); expAny != nil {
			sess.expiresAtSec = expAny.(int64)
		}

		h.ServeHTTP(w, r.WithContext(sm.WithSession(ctx, &sess)))
	})
}
