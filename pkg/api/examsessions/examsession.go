// Package examsessions implements the /api/examsessions HTTP endpoint, exposing
// CRUD over exam sessions scoped to the caller's user session.
//
//	POST   /api/examsessions           create a session for the exam whose id is
//	        given in the request body as {"exam_id": "..."}; returns the new
//	        session id as {"exam_session_id": "..."}.
//	GET    /api/examsessions           list the caller's sessions as
//	        {"exam_session_ids": ["...", ...]}.
//	DELETE /api/examsessions/{exam_id} terminate the named session.
//
// Mount it with the Go 1.22+ ServeMux so the {exam_id} wildcard reaches
// r.PathValue, e.g.:
//
//	mux.Handle("/api/examsessions", h)
//	mux.Handle("/api/examsessions/{exam_id}", h)
//
// The caller's user session is resolved from the request context via the
// SessionManager (see package session); it must already be attached by the
// session middleware.
package examsessions

import (
	"encoding/json"
	"io"
	"net/http"

	"dcna-questions/pkg/models/examserver"
	"dcna-questions/pkg/models/question"
	"dcna-questions/pkg/session"
)

// maxBodyBytes bounds the size of a POST body. A session-creation request is a
// small JSON object, so this comfortably rejects oversized payloads.
const maxBodyBytes = 1 << 20 // 1 MiB

// defaultExamOptions is the ExamOptions applied to newly created sessions. Zero
// means questions and options are presented in document order and the session is
// not seekable; override by extending the request payload if richer behavior is
// needed.
const defaultExamOptions examserver.ExamOptions = 0

// ExamSessionHandler serves the /api/examsessions API. It resolves the exam
// document to run from an ExamRepository and drives session lifecycle through an
// ExamServer, both scoped to the caller's user session.
type ExamSessionHandler struct {
	sm     session.SessionManager
	server examserver.ExamServer
	repo   *question.ExamRepository
}

// NewExamSessionHandler constructs an ExamSessionHandler. sm resolves the
// caller's user session from the request context; server manages exam session
// lifecycle; repo looks up the exam document to run when a session is created.
func NewExamSessionHandler(sm session.SessionManager, server examserver.ExamServer, repo *question.ExamRepository) *ExamSessionHandler {
	return &ExamSessionHandler{sm: sm, server: server, repo: repo}
}

// createRequest is the JSON body of a POST /api/examsessions request.
type createRequest struct {
	ExamID string `json:"exam_id"`
}

// createResponse is the JSON body of a successful POST.
type createResponse struct {
	ExamSessionID string `json:"exam_session_id"`
}

// listResponse is the JSON body of a successful GET.
type listResponse struct {
	ExamSessionIDs []string `json:"exam_session_ids"`
}

// ServeHTTP routes the request by HTTP method and the presence of the {exam_id}
// path value (populated when mounted at /api/examsessions/{exam_id}).
func (h *ExamSessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.sm.GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "session not found", http.StatusInternalServerError)
		return
	}

	examID := r.PathValue("exam_id")
	switch {
	case examID == "" && r.Method == http.MethodPost:
		h.handleCreate(w, r, sess.Id())
	case examID == "" && r.Method == http.MethodGet:
		h.handleList(w, r, sess.Id())
	case examID != "" && r.Method == http.MethodDelete:
		h.handleDelete(w, r, examserver.ExamId(examID))
	default:
		h.methodNotAllowed(w, examID != "")
	}
}

// handleCreate starts a new exam session for the exam document named in the
// request body.
func (h *ExamSessionHandler) handleCreate(w http.ResponseWriter, r *http.Request, userSessionID string) {
	req, ok := decodeCreate(r)
	if !ok {
		http.Error(w, `invalid request body: expected {"exam_id": "..."}`, http.StatusBadRequest)
		return
	}
	if req.ExamID == "" {
		http.Error(w, "exam_id is required", http.StatusBadRequest)
		return
	}

	exam, err := h.repo.GetExamDocumentById(req.ExamID)
	if err != nil {
		// GetExamDocumentById only fails when the exam is unavailable, so map the
		// lot to 404 rather than surfacing the repository internals.
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Mirror examserver's emptiness guard so an exam with no questions is
	// reported as a client error (400) rather than a server error.
	if len(exam.QuestionSet.QuestionCollections) == 0 {
		http.Error(w, "exam has no questions", http.StatusBadRequest)
		return
	}

	sessionID, err := h.server.StartNewExamSession(r.Context(), exam, userSessionID, defaultExamOptions)
	if err != nil {
		// With the empty-exam case handled above, the realistic remaining
		// failures are the server shutting down or the request being canceled,
		// both of which are transient: 503 asks the client to retry. examserver
		// keeps its sentinels unexported, so finer-grained mapping is not
		// possible here.
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, createResponse{ExamSessionID: string(sessionID)})
}

// handleList returns the caller's active exam sessions.
func (h *ExamSessionHandler) handleList(w http.ResponseWriter, r *http.Request, userSessionID string) {
	ids := h.server.ListExamSessions(r.Context(), userSessionID)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, string(id))
	}
	writeJSON(w, listResponse{ExamSessionIDs: out})
}

// handleDelete terminates a single exam session by id. EndExamSession does not
// take a user session, so this is not ownership-scoped; the id is whatever the
// caller presents in the path.
func (h *ExamSessionHandler) handleDelete(w http.ResponseWriter, r *http.Request, examID examserver.ExamId) {
	if err := h.server.EndExamSession(r.Context(), examID); err != nil {
		// The only failure is a missing session (or the server shutting down);
		// treat both as not found so a repeated delete converges to 404.
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// methodNotAllowed reports 405 with an Allow header tailored to whether the
// request targeted the collection or a single resource.
func (h *ExamSessionHandler) methodNotAllowed(w http.ResponseWriter, itemLevel bool) {
	allow := "GET, POST"
	if itemLevel {
		allow = "DELETE"
	}
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// decodeCreate parses and bounds-checks a POST body. An empty body or invalid
// JSON reports ok == false.
func decodeCreate(r *http.Request) (createRequest, bool) {
	var req createRequest
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return req, false
	}
	if len(raw) == 0 {
		return req, false
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return req, false
	}
	return req, true
}

// writeJSON encodes v as the response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
