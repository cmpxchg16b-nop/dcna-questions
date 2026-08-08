// Package loginoptions serves the login page's configurable IdP list to the
// frontend as JSON at GET /api/login/loginoptions. The list comes from the
// <loginOptions/> section of the server configuration document (see
// pkg/models/serverconfig and serverConfig.xsd in the project root).
package loginoptions

import (
	"encoding/json"
	"net/http"
)

// LoginOption is one entry of the login options list: kind identifies the
// IdP type (the frontend uses it to pick the login icon), name is the
// option's unique key, displayName and label build the button caption, and
// loginURL is where the button navigates. It carries the same fields as
// serverconfig.LoginOptionXML under JSON wire names.
type LoginOption struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Label       string `json:"label,omitempty"`
	LoginURL    string `json:"loginURL"`
}

// LoginOptionsHandler is an http.Handler that serves the configured login
// options as a JSON array. The list is fixed at construction time, so the
// handler is stateless and safe for concurrent use.
type LoginOptionsHandler struct {
	options []LoginOption
}

// NewLoginOptionsHandler constructs a LoginOptionsHandler serving the given
// options. A nil slice is served as an empty JSON array, never null.
func NewLoginOptionsHandler(options []LoginOption) *LoginOptionsHandler {
	if options == nil {
		options = []LoginOption{}
	}
	return &LoginOptionsHandler{options: options}
}

func (h *LoginOptionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h.options)
}
