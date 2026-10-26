package diagram

import (
	_ "embed" // the template, the zoom script and the diagrams' style
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/schema"
)

var (
	//go:embed diagram.svg.tmpl
	svgTemplate string
	//go:embed assets/zoom.js
	zoomJS string
	//go:embed assets/diagram.css
	diagramCSS string
)

// page is the template of a diagram, its text written in a language as each draw binds t.
var page = template.Must(template.New("svg").Funcs(template.FuncMap{
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

// New returns a drawer of the specs c, the compiled Schema, takes.
func New(c *schema.Compiled) *Drawer {
	return &Drawer{schema: c}
}

// Own returns a drawer of Schema compiled alone, for the specs of floe diagram.
func Own() (*Drawer, error) {
	r, err := schema.NewResource(URL, "diagram.json", Schema, schema.Part)
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own schema, which its tests check
	}
	c, err := schema.CompilePart(URL, map[string]schema.Resource{URL: r})
	if err != nil {
		return nil, err //nolint:wrapcheck // as above
	}
	return New(c), nil
}

// Draw checks spec, a spec's value, against the schema, lays it out and checks the layout, and draws it in
// opts.Lang as an HTML fragment of inline SVG. Every mistake is at a JSON pointer in spec, with no file.
func (d *Drawer) Draw(spec any, opts Options) (template.HTML, error) {
	l, err := d.layout(spec, opts)
	if err != nil {
		return "", err
	}
	return l.draw(opts.Lang)
}

func (d *Drawer) layout(v any, opts Options) (*layout, error) {
	if err := d.schema.ValidateAt(v); err != nil {
		return nil, err //nolint:wrapcheck // mistakes in the library's words, at their pointers
	}
	s, err := read(v)
	if err != nil {
		return nil, err
	}
	l := newLayout(s, opts)
	if l.failure != nil {
		return nil, l.failure
	}
	l.check()
	if err := l.problems.Sorted().Err(); err != nil {
		return nil, err
	}
	return l, nil
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
	t, err := page.Clone()
	if err != nil {
		return "", fmt.Errorf("diagram: copy the template: %w", err)
	}
	t.Funcs(template.FuncMap{"t": func(x Text) string { return x.in(lang) }})
	var b strings.Builder
	err = t.ExecuteTemplate(&b, "diagram", map[string]any{
		"W": l.s.Width, "H": l.s.Height, "MaxH": math.Floor(l.s.Height*maxScale + 0.5), "Title": l.s.Title,
		"Markers": defs, "Icons": l.iconDefs, "Elems": elems,
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

// Assets are what a page holds once, before or after its diagrams: the style that colours them from the page's
// CSS variables, and the script that zooms them, its toolbar worded in every language for the page's <html lang>
// to pick from. Holding them twice does no harm.
func Assets() (string, error) {
	s, err := script()
	if err != nil {
		return "", err
	}
	return s + "\n<style>\n" + diagramCSS + "</style>", nil
}

var script = sync.OnceValues(func() (string, error) {
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
