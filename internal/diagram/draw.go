package diagram

import (
	"cmp"
	_ "embed" // the template and the zoom script
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

var (
	//go:embed diagram.svg.tmpl
	svgTemplate string
	//go:embed assets/zoom.js
	zoomJS string
)

// svgPage is the template of a diagram, its text written in a language as each draw binds t.
var svgPage = template.Must(template.New("svg").Funcs(template.FuncMap{
	"n": num,
	"t": func(Text) string { return "" },
}).Parse(svgTemplate))

// maxScale stops a diagram growing on a wide screen once its bold text is as large as a page's body text (16px).
const maxScale = 16.0 / sizeMain

// Options are what a diagram is drawn with: the language its text is written in, the icons its icon parts show,
// Seq, its number among the diagrams drawn into one page, which its ids hold so that they differ, and Source, the
// spec as written, shown under it when it says source: true.
type Options struct {
	Lang   i18n.Lang
	Icons  Icons
	Seq    int
	Source string
}

// Icons finds an icon's SVG by name, as icons.Store.Lookup does.
type Icons interface {
	Lookup(name string) ([]byte, error)
}

// NoIcons are no icons to look up: there is no directory for them, Err says why, which a diagram with an icon
// is told.
type NoIcons struct{ Err error }

// Lookup returns Err, or icons_no_dir for none.
func (n NoIcons) Lookup(string) ([]byte, error) {
	return nil, cmp.Or(n.Err, error(usererr.New("icons_no_dir"))) //nolint:wrapcheck // Err as given, or the mistake
}

// Drawer draws specs that its schema checks: the one entry of a spec, wherever it comes from.
type Drawer struct {
	schema *schema.Compiled
}

// New returns a drawer of the specs c, the schema at URL compiled, takes.
func New(c *schema.Compiled) *Drawer {
	return &Drawer{schema: c}
}

// Draw checks spec, a spec's value, against the schema, lays it out and checks the layout, and draws it in
// opts.Lang as an HTML fragment of inline SVG. Every mistake is at a JSON pointer in spec, with no file, but
// that of icons not kept (of no place). Icons of nil are none kept.
func (d *Drawer) Draw(spec any, opts Options) (template.HTML, error) {
	l, err := d.layout(spec, opts)
	if err != nil {
		return "", err
	}
	return l.draw(opts.Lang)
}

func (d *Drawer) layout(v any, opts Options) (*layout, error) {
	if opts.Icons == nil {
		opts.Icons = NoIcons{Err: nil}
	}
	if err := d.schema.ValidateAt(v); err != nil {
		return nil, err //nolint:wrapcheck // mistakes in the library's words, at their pointers
	}
	s, err := read(v)
	if err != nil {
		return nil, err
	}
	name := s.Example
	if name != "" {
		if s, err = d.example(s, &opts); err != nil {
			return nil, err
		}
	}
	l := newLayout(s, opts)
	if l.failure != nil {
		return nil, l.failure
	}
	l.check()
	found := l.problems
	if name != "" {
		if found, err = exampleMistakes(name, found); err != nil {
			return nil, err
		}
	}
	if err := found.Sorted().Err(); err != nil {
		return nil, err
	}
	return l, nil
}

// example returns the spec of the built-in example s names, shown as s says, and makes opts.Source the text of its
// file; or the mistake, at #/example, that there is none. An example that its schema refuses is floe's failure.
func (d *Drawer) example(s *spec, opts *Options) (*spec, error) {
	e, err := ExampleOf(s.Example)
	var none *usererr.Error
	if errors.As(err, &none) {
		at := *none
		at.At = "#/example"
		return nil, &at
	}
	if err != nil {
		return nil, err
	}
	// In words, not wrapped: a mistake of floe's own example is no mistake of the user's.
	if err = d.schema.ValidateAt(e.value); err != nil {
		return nil, fmt.Errorf("diagram: the example %s: %s", e.Name, err.Error()) //nolint:err113,errorlint // see above
	}
	ex, err := read(e.value)
	if err != nil {
		return nil, fmt.Errorf("diagram: the example %s: %s", e.Name, err.Error()) //nolint:err113,errorlint // see above
	}
	ex.Source = s.Source
	opts.Source = yamlpos.File(e.Spec)
	return ex, nil
}

// exampleMistakes returns the mistakes of drawing the example name that are the user's: those of icons, which the
// icons kept decide (a set fetched anew may not hold an icon the example names), at the root of the diagram rather
// than in the example. Any other is floe's failure.
func exampleMistakes(name string, found usererr.List) (usererr.List, error) {
	mine := make(usererr.List, 0, len(found))
	for _, e := range found {
		if e.At != "" && !strings.HasPrefix(e.Key, "diagram_icon_") {
			return nil, fmt.Errorf("diagram: the example %s: %s", name, e.Error()) //nolint:err113 // floe's own
		}
		at := *e
		at.At = "#"
		mine = append(mine, &at)
	}
	return mine, nil
}

// elem is an element drawn: Kind names which of the template's, and the fields it reads.
type elem struct {
	Kind         string
	X, Y, X2, Y2 float64
	W, H, R, RX  float64
	D, Class     string
	Anchor       string
	Start, End   string // the ids of an edge's markers
	Ref          string // the id a use places
	Text         Text
	Raw          template.HTML
	Kids         []elem
}

// shown is the spec shown under a diagram, folded, and the words of its summary.
type shown struct {
	Label, Text string
}

type markerDef struct {
	marker
	ID string
}

func (l *layout) draw(lang i18n.Lang) (template.HTML, error) {
	var elems []elem
	elems = append(elems, l.areaEls...)
	for i := len(l.gridEls) - 1; i >= 0; i-- {
		elems = append(elems, l.gridEls[i])
	}
	elems = append(append(append(elems, l.edgeEls...), l.boxEls...), l.labelEls...)
	defs := make([]markerDef, 0, len(l.markers))
	for _, k := range l.markers {
		for _, m := range markers {
			if m.Kind == k {
				defs = append(defs, markerDef{marker: m, ID: l.id + "_" + k})
			}
		}
	}
	t, err := svgPage.Clone()
	if err != nil {
		return "", fmt.Errorf("diagram: copy the template: %w", err)
	}
	t.Funcs(template.FuncMap{"t": func(x Text) string { return x.in(lang) }})
	var b strings.Builder
	var source *shown
	if l.s.Source {
		source = &shown{Label: i18n.New(lang).Msg("diagram_source"), Text: l.opts.Source}
	}
	err = t.ExecuteTemplate(&b, "diagram", map[string]any{
		"W": l.s.Width, "H": l.s.Height, "MaxH": math.Floor(l.s.Height*maxScale + 0.5), "Title": l.s.Title,
		"Markers": defs, "Icons": l.iconDefs, "Elems": elems, "Source": source,
	})
	if err != nil {
		return "", fmt.Errorf("diagram: draw: %w", err)
	}
	// Drawn by html/template, its values escaped in their places.
	return template.HTML(b.String()), nil //nolint:gosec // see above
}

// Inspect lays spec out as Draw does, and tells where each part went, a line each, as the mistakes name them: a
// part's area,
// an edge's path and the ends it joins, and the region a label is in.
func (d *Drawer) Inspect(spec any, opts Options) (string, error) {
	l, err := d.layout(spec, opts)
	if err != nil {
		return "", err
	}
	return l.inspect(), nil
}

func (l *layout) inspect() string {
	lines := make([]string, 0, len(l.frames)+len(l.boxes)+len(l.marks)+len(l.areas)+len(l.paths)+len(l.labels))
	area := func(r rect) string {
		return fmt.Sprintf("%s %s,%s %sx%s", r.what, num(r.x), num(r.y), num(r.w), num(r.h))
	}
	for _, r := range l.frames {
		lines = append(lines, area(r))
	}
	for _, r := range l.boxes {
		lines = append(lines, area(r))
	}
	for _, r := range l.marks {
		lines = append(lines, area(r))
	}
	for _, r := range l.areas {
		lines = append(lines, area(r))
	}
	for _, p := range l.paths {
		lines = append(lines, fmt.Sprintf("%s %s", p.what, segsData(p.segs)))
	}
	for _, r := range l.labels {
		line := area(r)
		if in := l.regionNames(r); in != "" {
			line += " in " + in
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}

// regionNames are the circles the centre of r is in, by name; "" for none.
func (l *layout) regionNames(r rect) string {
	var in []string
	for _, name := range slices.Sorted(maps.Keys(l.circles)) {
		c := l.circles[name]
		if math.Hypot(r.x+r.w/2-c.CX, r.y+r.h/2-c.CY) < c.R {
			in = append(in, name)
		}
	}
	return strings.Join(in, ", ")
}

func segsData(segs []seg) string {
	if len(segs) == 0 {
		return ""
	}
	pts := [][2]float64{{segs[0][0], segs[0][1]}}
	for _, s := range segs {
		pts = append(pts, [2]float64{s[2], s[3]})
	}
	return pathData(pts)
}

// Script is the script that zooms the diagrams of a page, its toolbar worded in every language for the page's
// <html lang> to pick from. A page may hold it more than once: a second run does nothing.
var Script = sync.OnceValues(func() (string, error) {
	words := map[i18n.Lang]map[string]string{}
	for _, lang := range i18n.Langs() {
		c := i18n.New(lang)
		words[lang] = map[string]string{}
		for _, k := range zoomKeys {
			words[lang][k] = c.Msg(k)
		}
	}
	// json.Marshal writes <, > and & as \u escapes, so the words cannot end the script.
	b, err := json.Marshal(words)
	if err != nil {
		return "", fmt.Errorf("diagram: write the zoom words: %w", err)
	}
	return "<script>(" + strings.TrimSpace(zoomJS) + ")(" + string(b) + ");</script>", nil
})

var zoomKeys = []string{
	"diagram_zoom_hint", "diagram_zoom_in", "diagram_zoom_out", "diagram_zoom_fit", "diagram_zoom_fit_label",
	"diagram_zoom_one", "diagram_zoom_one_label",
}
