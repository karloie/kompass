//go:build webembed

package main

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embeddedDist embed.FS

func init() {
	root, err := fs.Sub(embeddedDist, "dist")
	if err == nil {
		embeddedWebRoot = root
	}
}
