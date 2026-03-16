//go:build !release

package tree

import "io/fs"

// embeddedAppWebRoot is populated by render_html_embed.go in release builds.
var embeddedAppWebRoot fs.FS
