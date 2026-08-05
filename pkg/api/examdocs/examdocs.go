// Package examdocs exposes HTTP handlers for exam documents.
package examdocs

import (
	"encoding/json"
	"net/http"

	"dcna-questions/pkg/models/question"
	"dcna-questions/pkg/session"
)

// ExamHandler is an http.Handler that lists exam documents. It streams the
// exams visible to the caller — the caller's own per-user exams (e.g. from
// associated uploads) first, then the system-wide ones — as NDJSON.
type ExamHandler struct {
	sm   session.SessionManager
	repo *question.ExamRepository
}

// NewExamHandler constructs an ExamHandler backed by the given repository. sm
// resolves the caller's session from the request context; its subject id (user
// id) scopes the per-user exam listing, exactly as /api/examtrackings and
// /api/useruploads use it. The repo's per-user exams come from sources whose
// GetByUserId yields entries (e.g. FsBasedAssociationManager).
func NewExamHandler(sm session.SessionManager, repo *question.ExamRepository) *ExamHandler {
	return &ExamHandler{sm: sm, repo: repo}
}

// ServeHTTP implements http.Handler. A GET request streams the exams visible
// to the caller as NDJSON (Content-Type application/x-ndjson), one JSON object
// per line, emitting each exam as soon as it is loaded rather than buffering
// the whole set. Per-user exams are streamed first, then system-wide ones.
// Each line is either {"Data":{...}} for a loaded exam or {"Err":"..."} when
// loading a URL failed; the consumer checks the Err field to detect failures.
// Non-GET methods respond 405.
//
// The caller's session must already be attached to the request context by the
// session middleware (see package session); its subject id (user id) scopes
// the per-user listing.
//
// Because streaming begins before every source has been loaded, the status is
// committed up front, so load failures are reported in-band as Err lines rather
// than as an HTTP error status.
func (h *ExamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sess, ok := h.sm.GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "session not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	// X-Accel-Buffering is a magic header honored by nginx (and compatible
	// proxies): "no" disables response buffering for this request so each
	// streamed line is forwarded to the client immediately instead of being
	// held until a buffer fills.
	w.Header().Set("X-Accel-Buffering", "no")
	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)

	// stream drains one event channel, encoding each event as an NDJSON line.
	// If the client disconnects mid-stream, draining continues (discarding
	// events) so the producer goroutine, which sends on an unbuffered channel,
	// is not left blocked forever.
	clientGone := false
	stream := func(events <-chan question.ExamDataEvent) {
		for ev := range events {
			if clientGone {
				continue
			}
			var data *question.ExamDocumentExcerpt
			if ev.Data != nil {
				excerpt := question.ExamExcerptFrom(ev.Data)
				data = &excerpt
			}
			line := ndjsonLine{Data: data}
			if ev.Err != nil {
				line.Err = ev.Err.Error()
			}
			if err := enc.Encode(line); err != nil {
				clientGone = true
				continue
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}

	// The caller's own exams first: they are the ones the caller just made
	// visible (e.g. by associating an upload) and the reason this endpoint is
	// typically re-polled; the system-wide exams follow.
	stream(h.repo.ListExamDocumentsByUserId(r.Context(), sess.SubjectId()))
	stream(h.repo.ListExamDocuments())
}

// ndjsonLine is the on-the-wire form of one streamed ExamDataEvent. Err holds the
// failure message when loading a URL failed; it is serialized as a string
// because a raw error interface value marshals to {}. Exactly one of Err or Data
// is set per line.
type ndjsonLine struct {
	Err  string                     `json:"Err,omitempty"`
	Data *question.ExamDocumentExcerpt `json:"Data,omitempty"`
}
