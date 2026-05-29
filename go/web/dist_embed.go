//go:build webdist

package webassets

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// DistFS reads the generated Svelte bundle embedded into a release binary.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
