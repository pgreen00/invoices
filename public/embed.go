// Package public embeds the static assets: stylesheets, browser JavaScript,
// and icons.
package public

import "embed"

//go:embed *.css *.js *.svg *.ico
var FS embed.FS
