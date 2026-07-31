// Package examdocs exposes HTTP handlers for exam documents.
package examdocs

import (
	"encoding/json"
	"net/http"

	"dcna-questions/pkg/models/question"
)

// ExamHandler is an http.Handler that lists exam documents. It streams the exams
// aggregated by an ExamRepository as NDJSON and is unconcerned with exam sessions
// or user sessions.
type ExamHandler struct {
	repo *question.ExamRepository
}

// NewExamHandler constructs an ExamHandler backed by the given repository.
func NewExamHandler(repo *question.ExamRepository) *ExamHandler {
	return &ExamHandler{repo: repo}
}

// ServeHTTP implements http.Handler. A GET request streams the repository's exam
// documents as NDJSON (Content-Type application/x-ndjson), one JSON object per
// line, emitting each exam as soon as it is loaded rather than buffering the
// whole set. Each line is either {"Data":{...}} for a loaded exam or
// {"Err":"..."} when loading a URL failed; the consumer checks the Err field to
// detect failures. Non-GET methods respond 405.
//
// Because streaming begins before every source has been loaded, the status is
// committed up front, so load failures are reported in-band as Err lines rather
// than as an HTTP error status.
func (h *ExamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

	// If the client disconnects mid-stream, keep draining the channel (discarding
	// events) so the producer goroutine, which sends on an unbuffered channel,
	// is not left blocked forever.
	clientGone := false
	for ev := range h.repo.ListExamDocuments() {
		if clientGone {
			continue
		}
		var data *question.ExamExcerpt
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

// ndjsonLine is the on-the-wire form of one streamed ExamDataEvent. Err holds the
// failure message when loading a URL failed; it is serialized as a string
// because a raw error interface value marshals to {}. Exactly one of Err or Data
// is set per line.
type ndjsonLine struct {
	Err  string                `json:"Err,omitempty"`
	Data *question.ExamExcerpt `json:"Data,omitempty"`
}
