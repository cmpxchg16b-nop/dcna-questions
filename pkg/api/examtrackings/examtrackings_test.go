package examtrackings_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dcna-questions/pkg/api/examtrackings"
	"dcna-questions/pkg/models/examreport"
	"dcna-questions/pkg/session"
	pkgutils "dcna-questions/pkg/utils"
)

// fakeTrackingServer is an ExamTrackingServer that records the userid it was
// queried with and returns canned results. Only GetExamReportsByUserId is
// exercised by the handler; Put is stubbed to satisfy the interface so the fake
// can also be reused in the end-to-end test.
type fakeTrackingServer struct {
	getUserid  string
	getReports []examreport.ExamReport
	getErr     error

	// Put records every report handed to it, in order.
	putUserid  string
	putReports []examreport.ExamReport
	putErr     error
}

func (s *fakeTrackingServer) Put(_ context.Context, userid string, report examreport.ExamReport) error {
	s.putUserid = userid
	s.putReports = append(s.putReports, report)
	return s.putErr
}

func (s *fakeTrackingServer) GetExamReportsByUserId(_ context.Context, userid string) ([]examreport.ExamReport, error) {
	s.getUserid = userid
	return s.getReports, s.getErr
}

// sampleReport builds a distinguishable ExamReport for assertion targets.
func sampleReport(id string) examreport.ExamReport {
	return examreport.ExamReport{
		Id:            id,
		ExamId:        "exam-doc-" + id,
		ExamShortName: "DCACI",
		ExamCode:      "300-620",
		Title:         "Implementing Cisco ACI",
		ExamSessionId: "session-" + id,
		FinishedAt:    1700000000000,
	}
}

// listResponse mirrors the handler's on-the-wire listResponse so tests can decode
// the body. The slice is a pointer so a "null" body (absent reports) decodes to
// nil rather than panicking.
type listResponse struct {
	ExamReports []examreport.ExamReport `json:"exam_reports"`
}

// decodeJSON unmarshals body into v, failing the test on error.
func decodeJSON(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
}

// testEnv wires a handler behind a ServeMux that mirrors the documented mount.
// The session subsystem is stateless: instead of allocating a session object,
// serve sets the context value the JWT middleware would set (the subject id) and
// runs the request through WithSessionId so a Session is built and attached.
type testEnv struct {
	sm        *session.OnMemorySessionManager
	ts        *fakeTrackingServer
	subjectID string
	mux       *http.ServeMux
}

func newTestEnv(t *testing.T, ts *fakeTrackingServer) *testEnv {
	t.Helper()
	sm := session.NewOnMemorySessionManager()
	h := examtrackings.NewExamTrackingsHandler(sm, ts)
	mux := http.NewServeMux()
	mux.Handle("/api/examtrackings", h)
	mux.Handle("/api/examtrackings/", h)
	return &testEnv{sm: sm, ts: ts, subjectID: "subject-test", mux: mux}
}

// serve issues a request through the env's mux. When withSession is true the
// request is first run through WithSessionId (mirroring the production
// middleware chain) after seeding the subject id in the context, so the handler
// receives a resolved Session. When false, no session is attached and the
// handler's GetSessionFromContext misses (producing the 500 guarded below).
func (e *testEnv) serve(t *testing.T, method, target string, withSession bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	h := http.Handler(e.mux)
	if withSession {
		ctx := context.WithValue(r.Context(), pkgutils.CtxKeySubjectId, e.subjectID)
		r = r.WithContext(ctx)
		h = session.WithSessionId(e.mux, e.sm)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	return rr
}

func TestExamTrackingsHandler(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		noSession  bool
		ts         *fakeTrackingServer
		wantStatus int
		wantAllow  string
		wantCT     string
		check      func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv)
	}{
		{
			name:       "GET returns the caller's reports as JSON",
			method:     http.MethodGet,
			target:     "/api/examtrackings",
			ts:         &fakeTrackingServer{getReports: []examreport.ExamReport{sampleReport("r1"), sampleReport("r2")}},
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
			check: func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv) {
				var got listResponse
				decodeJSON(t, rr.Body.String(), &got)
				if len(got.ExamReports) != 2 {
					t.Fatalf("len(ExamReports) = %d, want 2 (body %q)", len(got.ExamReports), rr.Body.String())
				}
				if got.ExamReports[0].Id != "r1" || got.ExamReports[1].Id != "r2" {
					t.Errorf("report ids = %q, %q, want r1, r2", got.ExamReports[0].Id, got.ExamReports[1].Id)
				}
				if env.ts.getUserid != env.subjectID {
					t.Errorf("tracking server queried with userid %q, want subject id %q", env.ts.getUserid, env.subjectID)
				}
			},
		},
		{
			name:       "GET with no reports returns an empty array, not null",
			method:     http.MethodGet,
			target:     "/api/examtrackings",
			ts:         &fakeTrackingServer{getReports: nil},
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
			check: func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv) {
				// The handler normalizes nil -> [] so the body is "[]", not "null".
				if !strings.Contains(rr.Body.String(), "\"exam_reports\":[]") {
					t.Fatalf("body = %q, want it to contain \"exam_reports\":[]", rr.Body.String())
				}
				var got listResponse
				decodeJSON(t, rr.Body.String(), &got)
				if len(got.ExamReports) != 0 {
					t.Errorf("len(ExamReports) = %d, want 0", len(got.ExamReports))
				}
			},
		},
		{
			name:       "GET on trailing slash still serves the collection root",
			method:     http.MethodGet,
			target:     "/api/examtrackings/",
			ts:         &fakeTrackingServer{getReports: []examreport.ExamReport{sampleReport("r1")}},
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
			check: func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv) {
				var got listResponse
				decodeJSON(t, rr.Body.String(), &got)
				if len(got.ExamReports) != 1 || got.ExamReports[0].Id != "r1" {
					t.Errorf("ExamReports = %+v, want one report r1", got.ExamReports)
				}
			},
		},
		{
			name:       "subject id flows as the user id: a caller sees only its own reports",
			method:     http.MethodGet,
			target:     "/api/examtrackings",
			ts:         &fakeTrackingServer{getReports: []examreport.ExamReport{sampleReport("only-mine")}},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv) {
				if env.ts.getUserid == "" {
					t.Fatal("tracking server was never queried")
				}
				// The userid passed must be the subject id, not a fixed constant
				// or empty string.
				if env.ts.getUserid != env.subjectID {
					t.Errorf("userid = %q, want subject id %q", env.ts.getUserid, env.subjectID)
				}
			},
		},
		{
			name:       "deeper path beneath the prefix is a 404",
			method:     http.MethodGet,
			target:     "/api/examtrackings/r1",
			ts:         &fakeTrackingServer{},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "POST on the collection responds 405 with Allow: GET",
			method:     http.MethodPost,
			target:     "/api/examtrackings",
			ts:         &fakeTrackingServer{},
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET",
			check: func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv) {
				if !strings.Contains(rr.Body.String(), "method not allowed") {
					t.Errorf("body = %q, want it to mention method not allowed", rr.Body.String())
				}
			},
		},
		{
			name:       "DELETE on the collection responds 405 with Allow: GET",
			method:     http.MethodDelete,
			target:     "/api/examtrackings",
			ts:         &fakeTrackingServer{},
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET",
		},
		{
			name:       "missing session in context responds 500",
			method:     http.MethodGet,
			target:     "/api/examtrackings",
			noSession:  true,
			ts:         &fakeTrackingServer{},
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv) {
				if !strings.Contains(rr.Body.String(), "session not found") {
					t.Errorf("body = %q, want it to mention session not found", rr.Body.String())
				}
				if env.ts.getUserid != "" {
					t.Errorf("tracking server was queried as %q, want no call when session is absent", env.ts.getUserid)
				}
			},
		},
		{
			name:       "tracking server error surfaces as 500",
			method:     http.MethodGet,
			target:     "/api/examtrackings",
			ts:         &fakeTrackingServer{getErr: errors.New("storage unavailable")},
			wantStatus: http.StatusInternalServerError,
			check: func(t *testing.T, rr *httptest.ResponseRecorder, env *testEnv) {
				if !strings.Contains(rr.Body.String(), "storage unavailable") {
					t.Errorf("body = %q, want it to surface the tracking server error", rr.Body.String())
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t, tc.ts)
			rr := env.serve(t, tc.method, tc.target, !tc.noSession)

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
			if tc.check != nil {
				tc.check(t, rr, env)
			}
		})
	}
}

// TestExamTrackingsHandler_EndToEnd walks the documented write/read-back flow: a
// report Put under a subject id (as the exam server does on session end) is
// returned to a subsequent GET scoped to that same subject id. It uses the real
// OnMemoryExamTrackingServer rather than a fake, so it exercises the actual
// store the handler is wired to in main.go.
func TestExamTrackingsHandler_EndToEnd(t *testing.T) {
	ts := examreport.NewOnMemoryExamTrackingServer()
	sm := session.NewOnMemorySessionManager()
	subjectID := "subject-endtoend"

	// Simulate the exam server persisting two finished-session reports under the
	// caller's subject id.
	ctx := context.Background()
	if err := ts.Put(ctx, subjectID, sampleReport("e1")); err != nil {
		t.Fatalf("Put e1: %v", err)
	}
	if err := ts.Put(ctx, subjectID, sampleReport("e2")); err != nil {
		t.Fatalf("Put e2: %v", err)
	}
	// A second, unrelated subject has its own reports that must not leak.
	if err := ts.Put(ctx, "subject-other", sampleReport("not-mine")); err != nil {
		t.Fatalf("Put other: %v", err)
	}

	h := examtrackings.NewExamTrackingsHandler(sm, ts)
	mux := http.NewServeMux()
	mux.Handle("/api/examtrackings", h)
	wrapped := session.WithSessionId(mux, sm)

	// Seed the subject id in the context (as the JWT middleware would) and run
	// through WithSessionId so the handler receives a resolved Session.
	r := httptest.NewRequest(http.MethodGet, "/api/examtrackings", nil)
	r = r.WithContext(context.WithValue(r.Context(), pkgutils.CtxKeySubjectId, subjectID))
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rr.Code, rr.Body.String())
	}
	var got listResponse
	decodeJSON(t, rr.Body.String(), &got)
	if len(got.ExamReports) != 2 {
		t.Fatalf("len(ExamReports) = %d, want 2 (the caller's own reports)", len(got.ExamReports))
	}
	for _, r := range got.ExamReports {
		if strings.Contains(r.Id, "not-mine") {
			t.Errorf("leaked report %q from another session; isolation broken", r.Id)
		}
	}
	if got.ExamReports[0].Id != "e1" || got.ExamReports[1].Id != "e2" {
		t.Errorf("report ids = %q, %q, want e1, e2 in insertion order", got.ExamReports[0].Id, got.ExamReports[1].Id)
	}
}
