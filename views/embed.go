// Package views embeds the html/template page templates.
package views

import "embed"

//go:embed *.html */*.html
var FS embed.FS
