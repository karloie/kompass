//go:build release

package tree

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embeddedAppDist embed.FS

var embeddedAppWebRoot fs.FS

func init() {
	root, err := fs.Sub(embeddedAppDist, "dist")
	if err == nil {
		embeddedAppWebRoot = root
	}
}
