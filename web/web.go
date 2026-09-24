// Package web embeds the amberIMPROVE status page.
package web

import "embed"

// FS holds the embedded static assets (index.html).
//
//go:embed index.html
var FS embed.FS
