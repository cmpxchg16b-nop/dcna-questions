package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionAccessors(t *testing.T) {
	exp := time.Now().Add(time.Hour)
	s := &Session{id: "abc"}
	s.expiresAt.Store(exp.UnixNano())

	if got := s.Id(); got != "abc" {
		t.Fatalf("Id: got %q, want %q", got, "abc")
	}
	if got, want := s.Expiry(), exp; !got.Equal(want) {
		t.Fatalf("Expiry: got %v, want %v", got, want)
	}
}

func TestNewOnMemorySessionManagerDefaults(t *testing.T) {
	m := NewOnMemorySessionManager()
	if m.TTL() != DefaultTTL {
		t.Fatalf("TTL: got %v, want %v", m.TTL(), DefaultTTL)
	}
	if DefaultTTL != 7*24*time.Hour {
		t.Fatalf("DefaultTTL: got %v, want 7d", DefaultTTL)
	}
}

func TestCreateSession(t *testing.T) {
	m := NewOnMemorySessionManager()
	id, sess := m.CreateSession()

	if id == "" {
		t.Fatal("expected non-empty id")
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
	if sess.Id() != id {
		t.Fatalf("session id: got %q, want %q", sess.Id(), id)
	}
	// Fresh session must be live for the full TTL window.
	minExpiry := time.Now().Add(m.TTL() - time.Second)
	maxExpiry := time.Now().Add(m.TTL() + time.Second)
	if sess.Expiry().Before(minExpiry) || sess.Expiry().After(maxExpiry) {
		t.Fatalf("expiry %v not within ~TTL window [%v, %v]", sess.Expiry(), minExpiry, maxExpiry)
	}
	// It must be retrievable by id.
	if got, ok := m.GetSessionById(id); !ok || got != sess {
		t.Fatalf("GetSessionById: got (%p, %v), want (%p, true)", got, ok, sess)
	}
}

func TestGetSessionByIdMissing(t *testing.T) {
	m := NewOnMemorySessionManager()
	if sess, ok := m.GetSessionById("nope"); ok || sess != nil {
		t.Fatalf("expected miss for unknown id, got (%p, %v)", sess, ok)
	}
}

func TestGetSessionByIdEvictsExpired(t *testing.T) {
	m := NewOnMemorySessionManager()
	id, sess := m.CreateSession()
	// Force expiration and confirm lazy eviction on lookup.
	sess.expiresAt.Store(time.Now().Add(-time.Second).UnixNano())

	if got, ok := m.GetSessionById(id); ok || got != nil {
		t.Fatalf("expected expired session to be evicted, got (%p, %v)", got, ok)
	}
	// Eviction should remove it from the store.
	if _, ok := m.sessions.Load(id); ok {
		t.Fatal("expected expired session to be deleted from store")
	}
}

func TestRenewSession(t *testing.T) {
	m := NewOnMemorySessionManager()
	_, sess := m.CreateSession()
	old := sess.Expiry()

	// Sleep a touch so "now + TTL" moves measurably forward.
	time.Sleep(10 * time.Millisecond)
	m.RenewSession(sess)

	if !sess.Expiry().After(old) {
		t.Fatalf("RenewSession did not advance expiry: old=%v new=%v", old, sess.Expiry())
	}
	want := time.Now().Add(m.TTL())
	allow := time.Second
	if sess.Expiry().Before(want.Add(-allow)) || sess.Expiry().After(want.Add(allow)) {
		t.Fatalf("renewed expiry %v not near %v", sess.Expiry(), want)
	}
}

func TestContextRoundTrip(t *testing.T) {
	m := NewOnMemorySessionManager()
	_, sess := m.CreateSession()

	if got, ok := m.GetSessionFromContext(t.Context()); ok || got != nil {
		t.Fatal("expected no session in a bare context")
	}

	ctx := m.WithSession(t.Context(), sess)
	got, ok := m.GetSessionFromContext(ctx)
	if !ok || got != sess {
		t.Fatalf("GetSessionFromContext: got (%p, %v), want (%p, true)", got, ok, sess)
	}
}

func TestSetCookie(t *testing.T) {
	m := NewOnMemorySessionManager()
	rec := httptest.NewRecorder()
	m.SetCookie(rec, "the-id")

	c := rec.Result().Cookies()
	if len(c) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(c))
	}
	ck := c[0]
	if ck.Name != CookieName {
		t.Errorf("cookie Name: got %q, want %q", ck.Name, CookieName)
	}
	if ck.Value != "the-id" {
		t.Errorf("cookie Value: got %q, want %q", ck.Value, "the-id")
	}
	if ck.Path != "/" {
		t.Errorf("cookie Path: got %q, want %q", ck.Path, "/")
	}
	if want := int(m.TTL().Seconds()); ck.MaxAge != want {
		t.Errorf("cookie MaxAge: got %d, want %d", ck.MaxAge, want)
	}
	if !ck.HttpOnly {
		t.Error("expected HttpOnly cookie")
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite: got %v, want %v", ck.SameSite, http.SameSiteLaxMode)
	}
}

// runMiddleware runs WithSessionId once against the given request and returns
// the (possibly new) session attached to the request context.
func runMiddleware(t *testing.T, sm *OnMemorySessionManager, r *http.Request) (*Session, *httptest.ResponseRecorder) {
	t.Helper()
	var got *Session
	h := WithSessionId(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := sm.GetSessionFromContext(r.Context())
		if !ok {
			t.Fatal("middleware did not attach a session to context")
		}
		got = s
	}), sm)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return got, rec
}

func TestWithSessionIdCreatesWhenNoCookie(t *testing.T) {
	sm := NewOnMemorySessionManager()
	sess, rec := runMiddleware(t, sm, httptest.NewRequest(http.MethodGet, "/", nil))

	if sess == nil {
		t.Fatal("expected a created session")
	}
	// New session must be persisted and resolvable.
	if _, ok := sm.GetSessionById(sess.Id()); !ok {
		t.Fatal("created session not found in manager")
	}
	ck := rec.Result().Cookies()
	if len(ck) != 1 || ck[0].Value != sess.Id() {
		t.Fatalf("expected cookie to carry new id %q, got %+v", sess.Id(), ck)
	}
}

func TestWithSessionIdRenewsValidCookie(t *testing.T) {
	sm := NewOnMemorySessionManager()
	id, existing := sm.CreateSession()
	// Age the session so renewal is observable.
	existing.expiresAt.Store(time.Now().Add(time.Minute).UnixNano())
	before := existing.Expiry()

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: id})
	sess, rec := runMiddleware(t, sm, r)

	if sess != existing {
		t.Fatalf("expected to reuse existing session, got %p want %p", sess, existing)
	}
	if !sess.Expiry().After(before) {
		t.Fatalf("expected renewed expiry to advance, before=%v after=%v", before, sess.Expiry())
	}
	ck := rec.Result().Cookies()
	if len(ck) != 1 || ck[0].Value != id {
		t.Fatalf("expected cookie to echo existing id %q, got %+v", id, ck)
	}
}

func TestWithSessionIdRecreatesForUnknownCookie(t *testing.T) {
	sm := NewOnMemorySessionManager()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "unknown-id"})
	sess, rec := runMiddleware(t, sm, r)

	if sess == nil {
		t.Fatal("expected a created session")
	}
	if sess.Id() == "unknown-id" {
		t.Fatal("expected a NEW id, not reuse of the unknown one")
	}
	if _, ok := sm.GetSessionById(sess.Id()); !ok {
		t.Fatal("created session not found in manager")
	}
	ck := rec.Result().Cookies()
	if len(ck) != 1 || ck[0].Value != sess.Id() {
		t.Fatalf("expected cookie to carry new id %q, got %+v", sess.Id(), ck)
	}
}

func TestWithSessionIdRecreatesForExpiredCookie(t *testing.T) {
	sm := NewOnMemorySessionManager()
	id, existing := sm.CreateSession()
	existing.expiresAt.Store(time.Now().Add(-time.Second).UnixNano())

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: CookieName, Value: id})
	sess, _ := runMiddleware(t, sm, r)

	if sess == existing {
		t.Fatal("expected a NEW session, not reuse of the expired one")
	}
	if sess.Id() == id {
		t.Fatal("expected a NEW id for recreated session")
	}
}
