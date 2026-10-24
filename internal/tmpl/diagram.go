package tmpl

import (
	"cmp"
	"html/template"
	"slices"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// checkDiagrams reads the diagrams of d, its items of whole of the part diagram, from their nodes under root,
// checks them and draws them into d.parts with the icons of store, read once for an output: a server that
// kept them for longer would draw those a fetch replaced.
// As floe diagram render does, every mistake of a spec's shape is reported, and those of its placement once
// it has none. The ids are checked across the page: they name the markers and icons its diagrams define. Each
// diagram without a mistake is drawn, for the parts holding it to be checked.
func checkDiagrams(whole drawnItems, d *Data, root ast.Node, store *icons.Store) error {
	found := whole.ofPart("diagram")
	if found == nil {
		return nil
	}
	var errs []error
	specs := make([]*diagram.Spec, 0, len(found))
	paths := map[*diagram.Spec][]string{}
	for _, f := range found {
		_, n, _ := yamlpos.Find(root, f.at)
		spec, err := diagram.FromNode(d.File, n)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		specs = append(specs, spec)
		paths[spec] = f.at
	}
	// In the order of the file, so that of two diagrams of one id, the later one is reported.
	slices.SortFunc(specs, func(a, b *diagram.Spec) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
	})
	// Watched for a link: the fetch a mistake asks for, or one that replaces them, draws it again.
	d.icons = store.Indexes()
	svgs, err := diagram.Render(specs, store)
	// The part's element holds the zoom toolbar too, which the script puts before the diagram's frame. It
	// draws no box, so that the toolbar and the frame stand where they would without it.
	for i, svg := range svgs {
		if svg == "" {
			continue
		}
		d.diagrams = true
		html := `<div data-floe-part="diagram" style="display: contents">` + svg + `</div>`
		d.drew(paths[specs[i]], template.HTML(html)) //nolint:gosec // G203: drawn by diagram, its values escaped
	}
	return usererr.Collect(append(errs, err)...)
}
