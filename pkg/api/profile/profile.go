package profile

import (
	"encoding/json"
	"net/http"

	"dcna-questions/pkg/utils"
)

// ProfileHandler is an http.Handler that serves the caller's profile (session
// and subject ids) at GET /api/profile.
type ProfileHandler struct{}

// NewProfileHandler constructs a ProfileHandler.
func NewProfileHandler() *ProfileHandler {
	return &ProfileHandler{}
}

type ProfileResponse struct {
	SessionID string `json:"session_id"`
	SubjectID string `json:"subject_id"`
}

func (h *ProfileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Context().Value(utils.CtxKeySessionId)
	subjectID := r.Context().Value(utils.CtxKeySubjectId)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := &ProfileResponse{}
	if sessionID != nil {
		resp.SessionID = sessionID.(string)
	}
	if subjectID != nil {
		resp.SubjectID = subjectID.(string)
	}
	json.NewEncoder(w).Encode(resp)
}
