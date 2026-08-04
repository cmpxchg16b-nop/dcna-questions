package profile_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dcna-questions/pkg/api/profile"
	pkgutils "dcna-questions/pkg/utils"
)

// profileResponse mirrors the handler's on-the-wire ProfileResponse so tests can
// decode the body for assertion targets.
type profileResponse struct {
	SessionID string `json:"session_id"`
	SubjectID string `json:"subject_id"`
}

// decodeJSON unmarshals body into v, failing the test on error.
func decodeJSON(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
}

// newRequest seeds the request context with session/subject ids the way the JWT
// middleware does in production. An empty value leaves that context key unset.
func newRequest(method, target, sessionID, subjectID string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	ctx := r.Context()
	if sessionID != "" {
		ctx = context.WithValue(ctx, pkgutils.CtxKeySessionId, sessionID)
	}
	if subjectID != "" {
		ctx = context.WithValue(ctx, pkgutils.CtxKeySubjectId, subjectID)
	}
	return r.WithContext(ctx)
}

func TestProfileHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		sessionID  string
		subjectID  string
		wantStatus int
		wantAllow  string
		wantCT     string
		want       profileResponse
		check      func(t *testing.T, rr *httptest.ResponseRecorder)
	}{
		{
			name:       "GET returns both session and subject ids",
			method:     http.MethodGet,
			sessionID:  "sess-123",
			subjectID:  "subj-456",
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
			want:       profileResponse{SessionID: "sess-123", SubjectID: "subj-456"},
		},
		{
			name:       "GET with only a session id omits the subject id",
			method:     http.MethodGet,
			sessionID:  "sess-only",
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
			want:       profileResponse{SessionID: "sess-only"},
		},
		{
			name:       "GET with only a subject id omits the session id",
			method:     http.MethodGet,
			subjectID:  "subj-only",
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
			want:       profileResponse{SubjectID: "subj-only"},
		},
		{
			name:       "GET with no context values returns empty fields, not null",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
			want:       profileResponse{},
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				// Empty strings must be emitted as "" rather than null.
				if !strings.Contains(rr.Body.String(), "\"session_id\":\"\"") {
					t.Errorf("body = %q, want empty session_id string", rr.Body.String())
				}
				if !strings.Contains(rr.Body.String(), "\"subject_id\":\"\"") {
					t.Errorf("body = %q, want empty subject_id string", rr.Body.String())
				}
			},
		},
		{
			name:       "POST responds 405 with Allow: GET",
			method:     http.MethodPost,
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET",
			check: func(t *testing.T, rr *httptest.ResponseRecorder) {
				if !strings.Contains(rr.Body.String(), "method not allowed") {
					t.Errorf("body = %q, want it to mention method not allowed", rr.Body.String())
				}
			},
		},
		{
			name:       "DELETE responds 405 with Allow: GET",
			method:     http.MethodDelete,
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET",
		},
		{
			name:       "PUT responds 405 with Allow: GET",
			method:     http.MethodPut,
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET",
		},
	}

	h := profile.NewProfileHandler()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, newRequest(tc.method, "/api/profile", tc.sessionID, tc.subjectID))

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantAllow != "" {
				if got := rr.Header().Get("Allow"); got != tc.wantAllow {
					t.Errorf("Allow = %q, want %q", got, tc.wantAllow)
				}
			}
			if tc.wantCT != "" {
				if got := rr.Header().Get("Content-Type"); !strings.Contains(got, tc.wantCT) {
					t.Errorf("Content-Type = %q, want it to contain %q", got, tc.wantCT)
				}
			}
			// Only the success path produces a JSON body worth decoding.
			if tc.wantStatus == http.StatusOK {
				var got profileResponse
				decodeJSON(t, rr.Body.String(), &got)
				if got != tc.want {
					t.Errorf("response = %+v, want %+v", got, tc.want)
				}
			}
			if tc.check != nil {
				tc.check(t, rr)
			}
		})
	}
}

// TestProfileHandler_RouteMounted runs the handler behind a ServeMux at the
// documented mount point to confirm the wiring in main.go (mux.Handle) reaches
// it correctly.
func TestProfileHandler_RouteMounted(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/api/profile", profile.NewProfileHandler())

	r := newRequest(http.MethodGet, "/api/profile", "sess-route", "subj-route")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
	}
	var got profileResponse
	decodeJSON(t, rr.Body.String(), &got)
	if got.SessionID != "sess-route" || got.SubjectID != "subj-route" {
		t.Errorf("response = %+v, want sess-route/subj-route", got)
	}
}
