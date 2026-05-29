//go:build !webdist

package webassets

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// DistFS reads the generated Svelte bundle from the local working tree.
func DistFS() fs.FS {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return os.DirFS("web/dist")
	}
	return os.DirFS(filepath.Join(filepath.Dir(file), "dist"))
}
