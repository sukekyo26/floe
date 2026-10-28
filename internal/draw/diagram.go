package draw

import (
	"fmt"
	"html/template"
	"sync"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Diagrams is the drawer of the diagrams of pages and of floe diagram: floe's part diagram's schema, compiled
// once. Parts of one's own take no name of floe's, so no kit holds another schema at its URL.
var Diagrams = sync.OnceValues(func() (*diagram.Drawer, error) {
	k, err := kit.Floe()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own, which its tests check
	}
	c, err := schema.CompilePart(diagram.URL, k.Resources())
	if err != nil {
		return nil, err //nolint:wrapcheck // as above
	}
	return diagram.New(c), nil
})

// diagramOf returns the function diagram of a part.html that draws the value of it, an item of d, in d's
// language: in the box of floe's part diagram, so that its CSS is the page's whoever draws it, after the
// script that zooms it. Its mistakes are at JSON pointers from the root of the value, or of no place (icons); what
// fails otherwise is floe's failure.
func (s *items) diagramOf(d *doc.Data, it schema.Item) func() (template.HTML, error) {
	return func() (template.HTML, error) {
		dr, err := Diagrams()
		if err != nil {
			return "", fmt.Errorf("%w: diagram: %w", kit.ErrFunc, err)
		}
		s.diagrams++
		o := diagram.Options{Lang: i18n.Lang(d.Lang), Icons: s.icons, Seq: s.diagrams}
		svg, err := dr.Draw(it.Value, o)
		var found usererr.List
		if failed := found.Add(err); failed != nil {
			return "", fmt.Errorf("%w: diagram: %w", kit.ErrFunc, failed)
		}
		if err != nil {
			return "", err //nolint:wrapcheck // mistakes at their pointers, which draw places
		}
		script, err := diagram.Script()
		if err != nil {
			return "", fmt.Errorf("%w: diagram: %w", kit.ErrFunc, err)
		}
		//nolint:gosec // G203: drawn by diagram, its values escaped, and floe's script
		return template.HTML(`<div data-floe-part="diagram">` + script + string(svg) + `</div>`), nil
	}
}
