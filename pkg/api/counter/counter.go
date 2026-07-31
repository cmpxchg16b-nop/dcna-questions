// Package counter implements the /api/counter HTTP endpoint, exposing a simple
// per-session integer counter.
//
//   - GET    /api/counter           returns the current value as {"data": <n>}
//   - POST   /api/counter           increments the value by 1
//   - PUT    /api/counter           sets the value; the request body is the new
//     value as a decimal string (e.g. "42")
//   - DELETE /api/counter           resets the value to 0
//
// The value is scoped to the request's session, which a SessionManager attaches
// to the request context (see package session).
package counter

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"dcna-questions/pkg/session"
)

// maxBodyBytes bounds the bytes read from a PUT request; an int64 in decimal is
// at most 20 characters, so this comfortably rejects absurd payloads.
const maxBodyBytes = 64

// Handler serves the per-session counter API. It depends only on the
// SessionManager interface, not on any concrete implementation.
type Handler struct {
	sm session.SessionManager
}

// NewHandler constructs a counter Handler backed by the given SessionManager.
func NewHandler(sm session.SessionManager) *Handler {
	return &Handler{sm: sm}
}

// counterResponse is the JSON body returned by every successful method.
type counterResponse struct {
	Data int64 `json:"data"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.sm.GetSessionFromContext(r.Context())
	if !ok {
		http.Error(w, "session not found", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, counterResponse{Data: sess.Counter()})
	case http.MethodPost:
		writeJSON(w, counterResponse{Data: sess.IncrCounter()})
	case http.MethodPut:
		n, ok := parseCounter(r)
		if !ok {
			http.Error(w, "invalid counter value: expected a decimal string", http.StatusBadRequest)
			return
		}
		sess.SetCounter(n)
		writeJSON(w, counterResponse{Data: sess.Counter()})
	case http.MethodDelete:
		sess.SetCounter(0)
		writeJSON(w, counterResponse{Data: sess.Counter()})
	default:
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// parseCounter reads and validates the decimal-string body of a PUT request.
// A JSON-encoded string ("42") is accepted in addition to a bare decimal.
func parseCounter(r *http.Request) (int64, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(raw))
	// Tolerate a JSON-encoded string body, e.g. "42".
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// writeJSON encodes v as the response body.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
