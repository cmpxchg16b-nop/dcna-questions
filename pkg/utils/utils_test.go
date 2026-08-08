package utils

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestGetRemoteAddr(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name: "no headers falls back to RemoteAddr",
			want: "192.0.2.1:1234",
		},
		{
			name: "cf-connecting-ip takes precedence",
			headers: map[string]string{
				"Cf-Connecting-Ip": "198.51.100.7",
				"X-Forwarded-For":  "203.0.113.9",
				"X-Real-IP":        "203.0.113.10",
			},
			want: "198.51.100.7",
		},
		{
			name: "x-forwarded-for when no cf header",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.9",
				"X-Real-IP":       "203.0.113.10",
			},
			want: "203.0.113.9",
		},
		{
			name: "x-real-ip when no cf or forwarded-for header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.10",
			},
			want: "203.0.113.10",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "192.0.2.1:1234"
			for k, v := range tc.headers {
				r.Header.Set(k, v)
			}
			if got := GetRemoteAddr(r); got != tc.want {
				t.Errorf("GetRemoteAddr = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestErrorResponseJSONKey(t *testing.T) {
	out, err := json.Marshal(ErrorResponse{Error: "boom"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"err":"boom"}`; string(out) != want {
		t.Errorf("marshaled ErrorResponse = %s, want %s", out, want)
	}
}

// TestCtxKeysAreDistinct guards against two context keys accidentally sharing
// the same string value, which would make them collide in context storage.
func TestCtxKeysAreDistinct(t *testing.T) {
	keys := []CtxKey{
		CtxKeySessionId, CtxKeySessionTTLSecs, CtxKeySessionObject,
		CtxKeySubjectId, CtxLogTraceId, CtxKeyJustIssuedJWTToken,
		CtxKeyJWTSecret, CtxKeyUsername, CtxKeyEmail,
	}
	seen := make(map[CtxKey]struct{}, len(keys))
	for _, k := range keys {
		if _, dup := seen[k]; dup {
			t.Errorf("duplicate context key value %q", string(k))
		}
		seen[k] = struct{}{}
	}
}
