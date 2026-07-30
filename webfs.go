// Package dcnaquestions embeds the static web assets shipped with the binary.
package dcnaquestions

import (
	"embed"
	"io/fs"
)

// web holds the contents of the web/ directory at compile time.
//
//go:embed web
var web embed.FS

// WebFS returns the embedded web assets as a read-only filesystem rooted at
// the web/ directory (i.e. paths do not carry a "web/" prefix).
func WebFS() fs.FS {
	fsys, err := fs.Sub(web, "web")
	if err != nil {
		// fs.Sub only errors if the name is invalid or not contained in fsys;
		// "web" is embedded above, so this is unreachable.
		panic(err)
	}
	return fsys
}
