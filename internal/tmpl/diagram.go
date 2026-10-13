package tmpl

import (
	"cmp"
	"errors"
	"html/template"
	"slices"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// checkDiagrams reads the diagrams of d, its items its schema s declares of format diagram, from their
// nodes under root, checks them and draws them into d.diagrams with the icons kept in iconsDir. The
// icons are read afresh for each data file: a server that kept them would draw those a fetch replaced.
// As floe diagram render does, a spec stops at the first mistake in its shape, and every mistake of
// its placement is reported. The ids are checked across the page: they name the markers and icons its diagrams define.
func checkDiagrams(s *schema, d *Data, root ast.Node, iconsDir string) error {
	var found []itemAt
	s.find("diagram", d.Values, nil, &found)
	if found == nil {
		return nil
	}
	var problems yamlpos.Problems
	specs := make([]*diagram.Spec, 0, len(found))
	paths := map[*diagram.Spec][]string{}
	for _, f := range found {
		_, n, _ := yamlpos.Find(root, f.at)
		spec, err := diagram.FromNode(d.File, n)
		var e *yamlpos.Error
		switch {
		case errors.As(err, &e):
			problems = append(problems, e)
			continue
		case err != nil:
			return err //nolint:wrapcheck // already says what failed
		}
		specs = append(specs, spec)
		paths[spec] = f.at
	}
	// In the order of the file, so that of two diagrams of one id, the later one is reported.
	slices.SortFunc(specs, func(a, b *diagram.Spec) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
	})
	svgs, err := diagram.Render(specs, icons.New(iconsDir))
	var more yamlpos.Problems
	switch {
	case errors.As(err, &more):
		problems = append(problems, more...)
	case err != nil:
		return err //nolint:wrapcheck // already says what failed: icons not fetched
	}
	if problems != nil {
		return problems.Sorted()
	}
	d.diagrams = make(map[string]template.HTML, len(specs))
	for i, spec := range specs {
		d.diagrams[pathKey(paths[spec])] = template.HTML(svgs[i]) //nolint:gosec // G203: drawn by diagram, its values escaped
	}
	return nil
}

// checkItems returns the mistakes of the checks of a stage, errs, together: those at a line and column
// all at once, sorted, unless one failed otherwise.
func checkItems(errs ...error) error {
	var problems yamlpos.Problems
	for _, err := range errs {
		var more yamlpos.Problems
		switch {
		case errors.As(err, &more):
			problems = append(problems, more...)
		case err != nil:
			return err
		}
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}
