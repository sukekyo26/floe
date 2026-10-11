// Package ui embeds the HTML that floe serves around and into the pages.
package ui

import _ "embed"

// Template wraps a body-only page. It is an html/template with .Title and .Content.
//
//go:embed template.html
var Template string
