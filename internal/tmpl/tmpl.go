// Package tmpl is where a data file is drawn as a page: it reads the data, finds its template and theme,
// checks the data against the template's schema, runs the checks and the computed values, and draws the page.
// Each stage stops the next one when it finds a mistake; within a stage, every mistake is told together.
package tmpl

import (
	"cmp"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"floe/internal/diagram"
	"floe/internal/doc"
	"floe/internal/draw"
	"floe/internal/kit"
	"floe/internal/schema"
	"floe/internal/usererr"
	"floe/internal/yamlpos"
)

// Options are what a caller gives a drawing besides the data.
type Options struct {
	Theme     string // drawn with instead of the data's theme:, "" for none
	Default   string // drawn with when neither Theme nor the data's theme: names one, "" for floe's default
	Untitled  string // the title of a page whose data names none
	Images    fs.FS  // the data's images, nil for none; its maker closes it
	ImagesDir string // the directory mistakes name the images under, "" for the data file's
	// ImageAt is told the line and column of each item that names an image, every one, as it is drawn; nil for none.
	ImageAt func(name string, line, column int)
	Icons   diagram.Icons // the icons the diagrams show, nil for the built-in sets alone
	CutOff  bool          // the page opens without the headers (a file floe render writes): its CSP goes in it
}

// Draw draws the data file src, file in its mistakes, with the templates, parts and themes of k, as a page:
// the data read, with the names of its template and theme; then checked against the template's schema; then
// its checks, its parts' checks for each item they draw, and its computed values; then drawn (draw.Page), with
// the warnings of the page drawn.
func Draw(k *kit.Kit, file string, src []byte, o Options) ([]byte, []*usererr.Error, error) {
	d, t, theme, err := checked(k, file, src, o.Theme, o.Default)
	if err != nil {
		return nil, nil, err
	}
	calc, err := run(k, t, d)
	if err != nil {
		return nil, nil, err
	}
	//nolint:wrapcheck // mistakes of the page, or floe's failure
	return draw.Page(k, t, theme, d, calc, cmp.Or(d.Title, o.Untitled), o.Images, o.ImagesDir, o.ImageAt, o.Icons,
		o.CutOff)
}

// Diagrams returns the data file src, file in its mistakes, read and checked against its template's schema as Draw
// does, and the values of its parts diagram in the order they are written (Walk goes by the keys' names). Nothing is
// drawn, and neither the checks nor the computed values run, so a page they refuse has its diagrams too.
// LIMIT: only the part diagram: a part of one's own whose part.html calls {{diagram}} is not among them. Collect them
// while drawing the parts' values if one comes to need it.
func Diagrams(k *kit.Kit, file string, src []byte) (*doc.Data, []schema.Item, error) {
	d, t, _, err := checked(k, file, src, "", "")
	if err != nil {
		return nil, nil, err
	}
	var items []schema.Item
	_, err = t.Schema().Walk(d.Values, func(it schema.Item, ready any) (any, error) {
		if it.Part == "diagram" {
			items = append(items, it)
		}
		return ready, nil
	})
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // floe's failure
	}
	slices.SortStableFunc(items, func(a, b schema.Item) int {
		at, bt := d.PosOf(a.At), d.PosOf(b.At)
		return cmp.Or(cmp.Compare(at.Line, bt.Line), cmp.Compare(at.Column, bt.Column))
	})
	return d, items, nil
}

// checked is the first stages of a drawing: the data file read, with its template and theme, in the theme given,
// else its own, else def, and checked against the template's schema; or their mistakes.
func checked(k *kit.Kit, file string, src []byte, given, def string) (*doc.Data, *kit.Template, string, error) {
	d, err := doc.Read(file, src, given)
	var found usererr.List
	if failed := found.Add(err); failed != nil {
		return nil, nil, "", failed
	}
	if d == nil {
		return nil, nil, "", err //nolint:wrapcheck // mistakes of the YAML
	}
	t, theme, err := pick(k, d, def, &found)
	if err != nil {
		return nil, nil, "", err
	}
	if t == nil {
		return nil, nil, "", found.Sorted().Err()
	}
	if t.Schema() == nil {
		return nil, nil, "", fmt.Errorf("tmpl: the template %s has no schema", t.Name()) //nolint:err113 // floe's own mistake
	}
	if failed := found.Add(t.Schema().Validate(d)); failed != nil {
		return nil, nil, "", failed //nolint:wrapcheck // floe's failure, which says what failed
	}
	if found != nil {
		return nil, nil, "", found.Sorted().Err()
	}
	return d, t, theme, nil
}

// pick returns the template of k that d names and the name of the theme it is drawn with, as doc.Read read
// them: the theme given or written, else def, else floe's default. It adds to found a template left out, empty or
// naming none, but one doc.Read refused, which it told; and a theme that is none: at d's theme: when written there,
// else, given or def, at no place, as no line of d names it. The template is nil when it is none; the error is floe's
// failure to compile it.
func pick(k *kit.Kit, d *doc.Data, def string, found *usererr.List) (*kit.Template, string, error) {
	_, _, themes, templates := k.Names()
	theme := cmp.Or(d.Theme, def, kit.DefaultTheme)
	if _, ok := k.Theme(theme); !ok {
		bad := usererr.New("tmpl_unknown_theme", theme, strings.Join(themes, ", "))
		if d.Theme != "" && !d.ThemeGiven {
			bad = yamlpos.New(d.File, d.Where["theme"], "tmpl_unknown_theme", theme, strings.Join(themes, ", "))
		}
		*found = append(*found, bad)
	}
	_, written := d.Values["template"]
	switch {
	case written && d.Template == "":
		// One not text, which doc.Read told.
	case d.Template == "":
		*found = append(*found, yamlpos.New(d.File, d.Where["template"], "tmpl_no_template", strings.Join(templates, ", ")))
	default:
		t, ok, err := k.Template(d.Template)
		if err != nil {
			return nil, theme, err //nolint:wrapcheck // floe's failure, which says what failed
		}
		if ok {
			return t, theme, nil
		}
		bad := yamlpos.New(d.File, d.Where["template"], "tmpl_unknown", d.Template, strings.Join(templates, ", "))
		*found = append(*found, bad)
	}
	return nil, theme, nil
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
