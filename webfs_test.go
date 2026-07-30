package dcnaquestions

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFS(t *testing.T) {
	fsys := WebFS()

	for _, name := range []string{"index.html", "404.html", "favicon.ico"} {
		if !fileExists(fsys, name) {
			t.Errorf("expected %q in embedded FS", name)
		}
	}

	// The "all:" embed prefix must include underscore-prefixed directories
	// like _next/, which hold the Next.js JS/CSS bundles.
	if info, err := fs.Stat(fsys, "_next"); err != nil || !info.IsDir() {
		t.Errorf("expected _next/ directory in embedded FS (err=%v)", err)
	}
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	check := func(path string, wantStatus int) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != wantStatus {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, wantStatus)
		}
	}

	check("/", http.StatusOK)
	check("/index.html", http.StatusOK)
	check("/favicon.ico", http.StatusOK)
	check("/this-page-does-not-exist", http.StatusNotFound)

	// The home page should be served as HTML.
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("GET /: Content-Type = %q, want text/html", ct)
	}
}
