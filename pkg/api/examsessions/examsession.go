// Package examsessions implements the /api/examsessions HTTP endpoint, exposing
// CRUD over exam sessions scoped to the caller's user session.
//
//	POST   /api/examsessions           create a session for the exam whose id is
//	        given in the request body as {"exam_id": "..."}; returns the new
//	        session id as {"exam_session_id": "..."}.
//	GET    /api/examsessions           list the caller's sessions as
//	        {"exam_session_ids": ["...", ...]}.
//	DELETE /api/examsessions/{exam_id} terminate the named session.
//	GET    /api/examsessions/{exam_id}/questions?cursor_id=<cursor>
//	        fetch the next question via GetNextQuestion; responds
//	        {"cursor_id": <next or null>, "question": {...} or null}.
//	PUT    /api/examsessions/{exam_id}/cursors?cursor_id=<cursor>&index=<n>
//	        reposition the cursor via SeekCursorTo; responds {"cursor_id": <new>}.
//
// Mount it as a subtree so the handler receives every path beneath
// /api/examsessions and resolves the routes internally, e.g.:
//
//	mux.Handle("/api/examsessions", h)
//	mux.Handle("/api/examsessions/", h)
//
// The caller's user session is resolved from the request context via the
// SessionManager (see package session); it must already be attached by the
// session middleware.
package examsessions

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

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

// nextQuestionResponse is the JSON body of a successful GET .../questions. Both
// fields are null when the session has no more questions.
type nextQuestionResponse struct {
	CursorID *string            `json:"cursor_id"`
	Question *question.Question `json:"question"`
}

// seekCursorResponse is the JSON body of a successful PUT .../cursors.
type seekCursorResponse struct {
	CursorID string `json:"cursor_id"`
}

// apiPrefix is the path the handler is mounted under. Every route beneath it is
// resolved inside ServeHTTP, so the handler owns its own route tree rather than
// relying on the ServeMux's wildcard captures.
const apiPrefix = "/api/examsessions"

// ServeHTTP routes the request by parsing the path beneath apiPrefix. The
// handler is mounted as a subtree and resolves the collection root, a single
// item, and the questions/cursors sub-resources itself:
//
//	""                     -> collection (POST create, GET list)
//	"/{exam_id}"           -> item (DELETE)
//	"/{exam_id}/questions" -> next question (GET)
//	"/{exam_id}/cursors"   -> seek cursor (PUT)
//
// Anything else beneath the prefix responds 404.
func (h *ExamSessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.sm.GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "session not found", http.StatusInternalServerError)
		return
	}

	// segments is the path beneath /api/examsessions split on '/', e.g.
	// [] (collection), [{exam_id}], or [{exam_id}, "questions"].
	rel := strings.TrimPrefix(r.URL.Path, apiPrefix)
	trimmed := strings.Trim(rel, "/")
	var segments []string
	if trimmed != "" {
		segments = strings.Split(trimmed, "/")
	}

	switch {
	case len(segments) == 0:
		// Collection: POST create, GET list.
		switch r.Method {
		case http.MethodPost:
			h.handleCreate(w, r, sess.Id())
		case http.MethodGet:
			h.handleList(w, r, sess.Id())
		default:
			h.methodNotAllowed(w, "GET, POST")
		}
	case len(segments) == 1:
		// Item: DELETE.
		if r.Method != http.MethodDelete {
			h.methodNotAllowed(w, "DELETE")
			return
		}
		h.handleDelete(w, r, examserver.ExamId(segments[0]), sess.Id())
	case len(segments) == 2 && segments[1] == "questions":
		if r.Method != http.MethodGet {
			h.methodNotAllowed(w, "GET")
			return
		}
		h.handleGetNextQuestion(w, r, examserver.ExamId(segments[0]), sess.Id())
	case len(segments) == 2 && segments[1] == "cursors":
		if r.Method != http.MethodPut {
			h.methodNotAllowed(w, "PUT")
			return
		}
		h.handleSeekCursor(w, r, examserver.ExamId(segments[0]), sess.Id())
	default:
		http.NotFound(w, r)
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

// handleDelete terminates a single exam session by id. The caller's user id is
// forwarded to EndExamSession so ownership can be enforced by the server.
func (h *ExamSessionHandler) handleDelete(w http.ResponseWriter, r *http.Request, examID examserver.ExamId, userSessionID string) {
	if err := h.server.EndExamSession(r.Context(), examID, userSessionID); err != nil {
		// The only failure is a missing session (or the server shutting down);
		// treat both as not found so a repeated delete converges to 404.
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetNextQuestion returns the next question in the session plus the cursor
// to continue from. An absent cursor_id starts from the beginning; when no more
// questions remain, both cursor_id and question are null.
func (h *ExamSessionHandler) handleGetNextQuestion(w http.ResponseWriter, r *http.Request, examID examserver.ExamId, userSessionID string) {
	q, next, err := h.server.GetNextQuestion(r.Context(), examID, userSessionID, parseCursorID(r))
	if err != nil {
		// examserver's sentinels are unexported, so not-found cannot be told
		// apart from an invalid cursor here; surface a generic server error.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var cursorID *string
	if next != nil {
		s := string(*next)
		cursorID = &s
	}
	writeJSON(w, nextQuestionResponse{CursorID: cursorID, Question: q})
}

// handleSeekCursor repositions the session cursor to a new virtual index and
// returns the fresh cursor to read from. The index is the required "index"
// query parameter.
func (h *ExamSessionHandler) handleSeekCursor(w http.ResponseWriter, r *http.Request, examID examserver.ExamId, userSessionID string) {
	index, ok := parseIndex(r)
	if !ok {
		http.Error(w, `invalid or missing "index" query parameter`, http.StatusBadRequest)
		return
	}
	repositioned, err := h.server.SeekCursorTo(r.Context(), examID, userSessionID, parseCursorID(r), index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cursorID := ""
	if repositioned != nil {
		cursorID = string(*repositioned)
	}
	writeJSON(w, seekCursorResponse{CursorID: cursorID})
}

// methodNotAllowed reports 405 with the given methods in the Allow header.
func (h *ExamSessionHandler) methodNotAllowed(w http.ResponseWriter, allow string) {
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

// parseCursorID reads the optional cursor_id query parameter. An empty or
// absent cursor_id yields nil, which ExamServer treats as "start from the
// beginning".
func parseCursorID(r *http.Request) *examserver.QuestionCursor {
	s := r.URL.Query().Get("cursor_id")
	if s == "" {
		return nil
	}
	c := examserver.QuestionCursor(s)
	return &c
}

// parseIndex reads the required "index" query parameter as a non-negative int.
func parseIndex(r *http.Request) (int, bool) {
	s := r.URL.Query().Get("index")
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// writeJSON encodes v as the response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
