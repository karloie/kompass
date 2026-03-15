//go:build !release

package main

import "io/fs"

// embeddedWebRoot is populated by main_embed.go in release builds.
var embeddedWebRoot fs.FS
