// Package web embeds the static frontend assets into the binary.
package web

import "embed"

// FS contains all static files in the web/ directory.
//
//go:embed index.html
var FS embed.FS
