package main

import (
	dcnaquestions "dcna-questions"
	"dcna-questions/pkg/counter"
	"dcna-questions/pkg/session"

	"log"
	"net/http"
)

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
func WithSessionId(h http.Handler, sm *session.OnMemorySessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			id   string
			sess *session.Session
		)

		if c, err := r.Cookie(session.CookieName); err == nil && c.Value != "" {
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

func main() {
	sm := session.NewOnMemorySessionManager()

	mux := http.NewServeMux()
	mux.Handle("/api/counter", counter.NewHandler(sm))
	mux.Handle("/", dcnaquestions.Handler())

	const addr = ":8080"
	log.Printf("listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, WithSessionId(mux, sm)); err != nil {
		log.Fatal(err)
	}
}
