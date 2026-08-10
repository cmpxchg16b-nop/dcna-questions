package common

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

const DefaultJWTCookieKey = "jwt"
const DefaultNonceCookieKey = "nonce"

// RequestOrigin determines the externally-visible origin ("scheme://host") of
// an incoming request. The Origin header is preferred (browsers send it on
// cross-site and non-GET requests); top-level navigations typically carry no
// Origin header, so the origin is then reconstructed from the request's TLS
// state (or the X-Forwarded-Proto header set by a trusted reverse proxy) and
// the Host header.
func RequestOrigin(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		return origin
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// A reverse proxy may terminate TLS; honor its forwarded scheme. Only the
	// first hop's value is used, and only well-known schemes are accepted.
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			fwd = strings.TrimSpace(fwd[:i])
		}
		if fwd == "http" || fwd == "https" {
			scheme = fwd
		}
	}
	return scheme + "://" + r.Host
}

// ResolveRedirectURL turns a configured OAuth/OIDC redirect URL into the
// absolute URL sent to the identity provider. Absolute URLs are returned
// unchanged. A relative URL (starting with "/") is resolved against the
// origin of the incoming request, so a single configuration serves every
// origin the deployment is reachable under (e.g. "http://localhost:8080"
// during development and "https://app.example.com" in production).
//
// Because the request's origin is attacker-controllable (Host header), it is
// only trusted when it appears in allowedOrigins; otherwise an error is
// returned and the login attempt must be aborted. An empty allowedOrigins
// list therefore disables relative redirect URLs entirely.
func ResolveRedirectURL(redirectURL string, allowedOrigins []string, r *http.Request) (string, error) {
	if !strings.HasPrefix(redirectURL, "/") {
		return redirectURL, nil
	}
	origin := RequestOrigin(r)
	if !slices.Contains(allowedOrigins, origin) {
		return "", fmt.Errorf("origin %q is not in the list of allowed origins", origin)
	}
	return origin + redirectURL, nil
}
