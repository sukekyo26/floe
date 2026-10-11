// Package ui embeds the HTML that floe serves around and into the pages.
package ui

import _ "embed"

// Template is the source of the template that wraps a body-only page. internal/page parses it with
// html/template and fills .Lang, .Title and .Content.
//
//go:embed template.html
var Template string

// Page is the management page, which is also the frame around each page.
//
//go:embed ui.html
var Page []byte
