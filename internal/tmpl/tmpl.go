// Package tmpl is where a data file is drawn as a page: it reads the data, finds its template and theme,
// checks the data against the template's schema, runs the checks and the computed values, and draws the page.
// Each stage stops the next one when it finds a mistake; within a stage, every mistake is told together.
package tmpl

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/draw"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Options are what a caller gives a drawing besides the data.
type Options struct {
	Theme    string // drawn with instead of the data's theme:, "" for none
	Untitled string // the title of a page whose data names none
}

// Draw draws the data file src, file in its mistakes, with the templates, parts and themes of k, as a page:
// the data read, with the names of its template and theme; then checked against the template's schema; then
// its checks, its parts' checks for each item they draw, and its computed values; then drawn (draw.Page).
func Draw(k *kit.Kit, file string, src []byte, o Options) ([]byte, error) {
	d, err := doc.Read(file, src, o.Theme)
	var found usererr.List
	if failed := found.Add(err); failed != nil {
		return nil, failed
	}
	if d == nil {
		return nil, err //nolint:wrapcheck // mistakes of the YAML
	}
	t, theme := pick(k, d, o.Theme, &found)
	if t == nil {
		return nil, found.Sorted().Err()
	}
	if t.Schema() == nil {
		return nil, fmt.Errorf("tmpl: the template %s has no schema", t.Name()) //nolint:err113 // floe's own mistake
	}
	if failed := found.Add(t.Schema().Validate(d)); failed != nil {
		return nil, failed //nolint:wrapcheck // floe's failure, which says what failed
	}
	if found != nil {
		return nil, found.Sorted().Err()
	}
	calc, err := run(k, t, d)
	if err != nil {
		return nil, err
	}
	//nolint:wrapcheck // mistakes of the page, or floe's failure
	return draw.Page(k, t, theme, d, calc, cmp.Or(d.Title, o.Untitled))
}

// pick returns the template of k that d names and the name of the theme it is drawn with, given, else d's,
// else the default; adding to found a template: left out or naming none, and a theme that is none: at d's
// theme:, or, given, at no place, as no line of d names it. The template is nil when it is none.
func pick(k *kit.Kit, d *doc.Data, given string, found *usererr.List) (*kit.Template, string) {
	_, _, themes, templates := k.Names()
	theme := cmp.Or(given, d.Theme, kit.DefaultTheme)
	if _, ok := k.Theme(theme); !ok {
		bad := usererr.New("tmpl_unknown_theme", theme, strings.Join(themes, ", "))
		if given == "" {
			bad = yamlpos.New(d.File, d.Where["theme"], "tmpl_unknown_theme", theme, strings.Join(themes, ", "))
		}
		*found = append(*found, bad)
	}
	_, written := d.Values["template"]
	switch {
	case d.Template == "" && !written:
		*found = append(*found, yamlpos.New(d.File, d.Where["template"], "tmpl_no_template", strings.Join(templates, ", ")))
	case d.Template == "":
		// Not text, which doc.Read told.
	default:
		if t, ok := k.Template(d.Template); ok {
			return t, theme
		}
		names := strings.Join(templates, ", ")
		*found = append(*found, yamlpos.New(d.File, d.Where["template"], "tmpl_unknown", d.Template, names))
	}
	return nil, theme
}

// run runs, on d, the checks of t, those of each part for each item of d it draws, and the computed values of
// t, all together, and returns the values computed (nil without computed.yaml): none uses what another gives.
func run(k *kit.Kit, t *kit.Template, d *doc.Data) (map[string]any, error) {
	var errs []error
	if c := t.Checks(); c != nil {
		errs = append(errs, c.Run(d, nil))
	}
	_, err := t.Schema().Walk(d.Values, func(it schema.Item, _ any) (any, error) {
		if p, ok := k.Part(it.Part); ok && p.Checks() != nil {
			errs = append(errs, p.Checks().Run(d, it.At))
		}
		return nil, nil //nolint:nilnil // what a check is given back is not read
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's failure: fn fails nothing
	}
	var calc map[string]any
	if c := t.Computed(); c != nil {
		calc, err = c.Run(d)
		errs = append(errs, err)
	}
	if err := usererr.Collect(errs...); err != nil {
		return nil, err //nolint:wrapcheck // mistakes at their places, or floe's failure
	}
	return calc, nil
}
