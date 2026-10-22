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
// checks them and draws them into d.parts with the icons kept in iconsDir. The icons are read afresh for
// each data file: a server that kept them would draw those a fetch replaced.
// As floe diagram render does, a spec stops at the first mistake in its shape, and every mistake of
// its placement is reported. The ids are checked across the page: they name the markers and icons its diagrams define.
func checkDiagrams(whole drawnItems, d *Data, root ast.Node, iconsDir string) error {
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
	store := icons.New(iconsDir)
	// Watched for a link: the fetch a mistake asks for, or one that replaces them, draws it again.
	d.icons = store.Indexes()
	svgs, err := diagram.Render(specs, store)
	if err = checkItems(append(errs, err)...); err != nil {
		return err
	}
	d.diagrams = true
	// The part's element holds the zoom toolbar too, which the script puts before the diagram's frame. It
	// draws no box, so that the toolbar and the frame stand where they would without it.
	for i, spec := range specs {
		html := `<div data-floe-part="diagram" style="display: contents">` + svgs[i] + `</div>`
		d.drew(paths[spec], template.HTML(html)) //nolint:gosec // G203: drawn by diagram, its values escaped
	}
	return nil
}

// checkItems returns the mistakes of the checks of a stage, errs, together: those at a line and column
// all at once, sorted, unless one is a mistake without a place, about the whole stage, or a failure,
// returned alone.
func checkItems(errs ...error) error {
	var problems usererr.List
	for _, err := range errs {
		if failed := problems.AddPlaced(err); failed != nil {
			return failed //nolint:wrapcheck // a mistake about the whole stage, or what failed
		}
	}
	return problems.Sorted().Err()
}
