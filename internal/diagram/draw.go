package diagram

import (
	_ "embed" // the template
	"errors"
	"fmt"
	"html/template"
	"maps"
	"math"
	"slices"
	"strings"

	"floe/internal/i18n"
	"floe/internal/icons"
	"floe/internal/schema"
	"floe/internal/usererr"
	"floe/internal/zoom"
)

//go:embed diagram.svg.tmpl
var svgTemplate string

// svgPage is the template of a diagram, its text written in a language as each draw binds t.
var svgPage = template.Must(template.New("svg").Funcs(template.FuncMap{
	"n": num,
	"t": func(Text) string { return "" },
}).Parse(svgTemplate))

// maxScale stops a diagram growing on a wide screen once its bold text is as large as a page's body text (16px).
const maxScale = 16.0 / sizeMain

// Options are what a diagram is drawn with: the language its text is written in, the icons its icon parts show,
// and Seq, its number among the diagrams drawn into one page, which its ids hold so that they differ.
type Options struct {
	Lang  i18n.Lang
	Icons Icons
	Seq   int
}

// Icons finds an icon's SVG by name, as icons.Store.Lookup does.
type Icons interface {
	Lookup(name string) ([]byte, error)
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
// those of the icons kept: at their file, or of the whole output. Icons of nil are the built-in sets alone.
func (d *Drawer) Draw(spec any, opts Options) (template.HTML, error) {
	l, err := d.layout(spec, opts)
	if err != nil {
		return "", err
	}
	return l.draw(opts.Lang)
}

// layout lays the spec v out and checks it, its mistakes in the error: with the layout when they are those of the
// checks, which Inspect shows and Draw does not; a nil layout when it did not get as far.
func (d *Drawer) layout(v any, opts Options) (*layout, error) {
	if opts.Icons == nil {
		opts.Icons = icons.Without(nil)
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
		if s, err = d.example(s); err != nil {
			return nil, err
		}
	}
	l := newLayout(s, opts)
	if err = l.checked(); err != nil {
		return nil, err
	}
	found := l.problems
	if name != "" {
		if found, err = exampleMistakes(name, found); err != nil {
			return nil, err
		}
	}
	return l, found.Sorted().Err()
}

// example returns the spec of the built-in example s names, or the mistake, at #/example, that there is none. An
// example that its schema refuses is floe's failure.
func (d *Drawer) example(s *spec) (*spec, error) {
	e, err := ExampleOf(s.Example)
	var none *usererr.Error
	if errors.As(err, &none) {
		return nil, none.PlacedIn("#/example")
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
	return ex, nil
}

// exampleMistakes returns the mistakes of drawing the example name that are the user's: those of icons, which the
// icons kept decide (a set fetched anew may not hold an icon the example names), an icon's name at the root of the
// diagram rather than in the example, those of the icons' files or of the whole output where they are. Any other is
// floe's failure.
func exampleMistakes(name string, found usererr.List) (usererr.List, error) {
	mine := make(usererr.List, 0, len(found))
	for _, e := range found {
		if e.At() != "" && !strings.HasPrefix(e.Key, "diagram_icon_") {
			return nil, fmt.Errorf("diagram: the example %s: %s", name, e.Error()) //nolint:err113 // floe's own
		}
		mine = append(mine, e.PlacedIn("#"))
	}
	return mine, nil
}

// elem is an element drawn: Kind names which of the template's, and the fields it reads.
type elem struct {
	Kind         string
	X, Y, X2, Y2 float64
	W, H, R, RX  float64
	D, Class     string
	Tone         string // its data-floe-tone, "" for none
	Anchor       string
	Start, End   string // the ids of an edge's markers
	Ref          string // the id a use places
	Text         Text
	Raw          template.HTML
	Kids         []elem
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
	// LIMIT: the schema refuses an empty title, not text that shows nothing (spaces, tabs, a zero-width space), which
	// tells a reader who cannot see the diagram nothing; refuse it too if such titles come up.
	err = t.ExecuteTemplate(&b, "diagram", map[string]any{
		"W": l.w, "H": l.h, "MaxH": math.Floor(l.h*maxScale + 0.5), "Title": l.s.Title,
		"Markers": defs, "Icons": l.iconDefs, "Elems": elems, "Wrap": zoom.Frame,
	})
	if err != nil {
		return "", fmt.Errorf("diagram: draw: %w", err)
	}
	// Drawn by html/template, its values escaped in their places.
	return template.HTML(b.String()), nil //nolint:gosec // see above
}

// Inspect lays spec out as Draw does, and tells where each part went, a line each, as the mistakes name them: the
// diagram by its title with its canvas, then a part's area, a frame's heading, an edge's path and the ends it joins,
// and the region a label is in. With mistakes of the checks it tells them too: what it tells then is a guide, as a
// spec with a mistake is laid out without its straight edges (newLayout), and a graph that failed is partly there.
func (d *Drawer) Inspect(spec any, opts Options) (string, error) {
	l, err := d.layout(spec, opts)
	if l == nil {
		return "", err
	}
	return "diagram " + words(l.s.Title, "") + " " + num(l.w) + "x" + num(l.h) + "\n" + l.inspect(), err
}

func (l *layout) inspect() string {
	lines := make([]string, 0, len(l.frames)+len(l.heads)+len(l.boxes)+len(l.marks)+len(l.areas)+len(l.paths)+
		len(l.labels))
	area := func(r rect) string {
		if r.byPlace { // named by where it stands already
			return r.what
		}
		return r.what + " " + placeOf(r)
	}
	for _, r := range l.frames {
		lines = append(lines, area(r))
	}
	for _, r := range l.heads {
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
	if len(lines) == 0 {
		return "" // nothing listed: a line part, a raw part, no part
	}
	return strings.Join(lines, "\n") + "\n"
}

// regionNames are the circles the centre of r is in, by name; "" for none.
func (l *layout) regionNames(r rect) string {
	var in []string
	for _, name := range slices.Sorted(maps.Keys(l.circles)) {
		c := l.circles[name]
		if math.Hypot(r.x+r.w/2-c.CX, r.y+r.h/2-c.CY) < c.R-eps {
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
