// Package web embeds the amberIMPROVE status page.
package web

import "embed"

// FS holds the embedded static assets (index.html and the amberSUITE favicon —
// the same icon as amberPLAY and amberDSP, so the tab is recognisable).
//
//go:embed index.html favicon.svg favicon-32.png
var FS embed.FS
