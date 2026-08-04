// Package examtrackings exposes the /api/examtrackings HTTP endpoint, returning
// the exam reports recorded for the caller's user session.
//
// Until an accounting system is in place, the caller's user session id (see
// package session) is used as the user id against the ExamTrackingServer. The
// ExamTrackingServer handed to the constructor must be the same instance that the
// exam server writes finished reports to, so the reports surfaced here are the
// ones persisted on session end.
package examtrackings

import (
	"encoding/json"
	"net/http"
	"strings"

	"dcna-questions/pkg/models/examreport"
	"dcna-questions/pkg/session"
)

// apiPrefix is the path the handler is mounted under.
const apiPrefix = "/api/examtrackings"

// ExamTrackingsHandler is an http.Handler that serves the caller's exam reports.
type ExamTrackingsHandler struct {
	sm             session.SessionManager
	trackingServer examreport.ExamTrackingServer
}

// NewExamTrackingsHandler constructs an ExamTrackingsHandler. sm resolves the
// caller's user session from the request context; its session id is used as the
// user id until an accounting system exists. trackingServer must be the same
// ExamTrackingServer instance handed to the exam server (OnMemoryExamServer), so
// the reports persisted on session end are the ones returned here.
func NewExamTrackingsHandler(sm session.SessionManager, trackingServer examreport.ExamTrackingServer) *ExamTrackingsHandler {
	return &ExamTrackingsHandler{sm: sm, trackingServer: trackingServer}
}

// listResponse is the JSON body of a successful GET /api/examtrackings.
type listResponse struct {
	ExamReports []examreport.ExamReport `json:"exam_reports"`
}

// ServeHTTP implements http.Handler. A GET /api/examtrackings returns the caller's
// exam reports as {"exam_reports": [...]}. Any path beneath the prefix, or a
// non-GET method, responds 404 / 405 respectively.
//
// The caller's user session must already be attached to the request context by
// the session middleware (see package session).
func (h *ExamTrackingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.sm.GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "session not found", http.StatusInternalServerError)
		return
	}

	// Resolve the path beneath the prefix so the handler also works when mounted
	// as a subtree (e.g. "/api/examtrackings/"). Only the collection root is
	// served; any deeper path is a 404.
	rel := strings.TrimPrefix(r.URL.Path, apiPrefix)
	if strings.Trim(rel, "/") != "" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// The user session id stands in for a user id until accounting is built.
	reports, err := h.trackingServer.GetExamReportsByUserId(r.Context(), sess.SubjectId())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Normalize nil to an empty slice so the body is "[]", not "null" — matching
	// the /api/examsessions list handler's wire shape.
	if reports == nil {
		reports = []examreport.ExamReport{}
	}
	writeJSON(w, listResponse{ExamReports: reports})
}

// writeJSON encodes v as the response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
