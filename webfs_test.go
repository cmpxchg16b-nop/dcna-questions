package dcnaquestions

import (
	"io/fs"
	"strings"
	"testing"
)

func TestWebFSContainsIndex(t *testing.T) {
	fsys := WebFS()

	want := "index.html"
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}

	b, err := fs.ReadFile(fsys, want)
	if err != nil {
		t.Fatalf("read %s from embedded FS (entries=%v): %v", want, names, err)
	}
	if !strings.Contains(string(b), "Hello, World!") {
		t.Fatalf("unexpected %s content: %q", want, b)
	}
}
