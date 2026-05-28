package webassets

import "embed"

// FS contains the browser app shell templates and static web assets.
//
//go:embed static templates
var FS embed.FS
