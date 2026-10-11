// Package ui embeds the HTML that floe serves around and into the pages.
package ui

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Template is the source of the template that wraps a body-only page. internal/page parses it with
// html/template and fills .Lang, .Title and .Content.
//
//go:embed template.html
var Template string

//go:embed ui.html
var pageSource string

var pageTemplate = template.Must(template.New("ui.html").Parse(pageSource))

// pageData fills ui.html: its markup takes .Lang and {{.Msg "ui_…"}}, its script the ui_ formats
// (.Messages), which it renders itself.
type pageData struct {
	*i18n.Catalog

	Messages map[string]string
}

// Page renders the management page, which is also the frame around each page, in the language of c.
func Page(c *i18n.Catalog) ([]byte, error) {
	var b bytes.Buffer
	if err := pageTemplate.Execute(&b, pageData{Catalog: c, Messages: c.Formats("ui_")}); err != nil {
		return nil, fmt.Errorf("render ui.html: %w", err)
	}
	return b.Bytes(), nil
}
