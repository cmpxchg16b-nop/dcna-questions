// Package exam exposes HTTP handlers for exam documents.
package exam

import (
	"encoding/json"
	"net/http"

	"dcna-questions/pkg/models/question"
)

// ExamHandler is an http.Handler that lists exam documents. It serves the exams
// aggregated by an ExamRepository as JSON and is unconcerned with exam sessions
// or user sessions.
type ExamHandler struct {
	repo *question.ExamRepository
}

// NewExamHandler constructs an ExamHandler backed by the given repository.
func NewExamHandler(repo *question.ExamRepository) *ExamHandler {
	return &ExamHandler{repo: repo}
}

// ServeHTTP implements http.Handler. A GET request responds with the
// repository's exam documents encoded as a JSON array.
//
// The ExamRepository stream is fully drained before responding: even if a
// source fails to load, the handler keeps reading so the producer goroutine
// completes and closes its channel. If any error occurred the handler responds
// 500 with the first error (rather than serving a silently partial list);
// non-GET methods respond 405.
func (h *ExamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	exams := make([]*question.Exam, 0)
	var firstErr error
	for ev := range h.repo.ListExamDocuments() {
		if ev.Err != nil {
			if firstErr == nil {
				firstErr = ev.Err
			}
			continue
		}
		exams = append(exams, ev.Data)
	}
	if firstErr != nil {
		http.Error(w, firstErr.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(exams)
}
