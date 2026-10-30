// Package ui embeds the HTML that floe serves around and into the pages.
package ui

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"

	"github.com/sukekyo26/floe/internal/i18n"
)

//go:embed ui.html
var pageSource string

var pageTemplate = template.Must(template.New("ui.html").Parse(pageSource))

// pageData fills ui.html: its markup takes .Lang, .Framed and {{.Msg "ui_…"}}, its script the ui_ formats
// (.Messages), which it renders itself.
type pageData struct {
	*i18n.Catalog

	Messages map[string]string
	Framed   string
	// Served tells that the page of a folder framed is there, as the server found it
	Served bool
}

// Page renders the management page, which is also the frame around each page, in the language of c: the frame
// around the page framed, a slug, or the list for "". served tells that the folder's page it frames is there.
func Page(c *i18n.Catalog, framed string, served bool) ([]byte, error) {
	var b bytes.Buffer
	data := pageData{Catalog: c, Messages: c.Formats("ui_"), Framed: framed, Served: served}
	if err := pageTemplate.Execute(&b, data); err != nil {
		return nil, fmt.Errorf("render ui.html: %w", err)
	}
	return b.Bytes(), nil
}
