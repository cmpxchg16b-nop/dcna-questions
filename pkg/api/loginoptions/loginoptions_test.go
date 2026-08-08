package loginoptions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginOptionsHandler_ServesOptions(t *testing.T) {
	h := NewLoginOptionsHandler([]LoginOption{
		{Kind: "github", Name: "github", DisplayName: "Github", LoginURL: "/api/login/oauth2/github/start"},
		{Kind: "visitor", Name: "visitor", Label: "Sign in as Visitor", LoginURL: "/api/login/visitor"},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/login/loginoptions", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q, want application/json", ct)
	}

	var got []LoginOption
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("options count: got %d, want 2", len(got))
	}
	if got[0].Name != "github" || got[1].Label != "Sign in as Visitor" {
		t.Fatalf("unexpected options payload: %+v", got)
	}
}

func TestLoginOptionsHandler_NilOptionsServedAsEmptyArray(t *testing.T) {
	h := NewLoginOptionsHandler(nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/login/loginoptions", nil))

	var got []LoginOption
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("nil options must be served as [], got %+v", got)
	}
}

func TestLoginOptionsHandler_RejectsNonGet(t *testing.T) {
	h := NewLoginOptionsHandler(nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/login/loginoptions", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET" {
		t.Fatalf("Allow: got %q, want GET", allow)
	}
}
