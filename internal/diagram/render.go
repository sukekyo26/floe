package diagram

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/usererr"
)

// maxScale stops a diagram growing on a wide screen once its bold text is as large as the template's
// body text (16px).
const maxScale = 16.0 / sizeMain

// marker is an end of an edge. Each is drawn with its right end on the end of the edge (the side of a
// box), and turns round at the start.
type marker struct {
	kind, viewBox, body string
	w, h, rx, ry        float64
}

var markers = []marker{
	{"arrow", "0 0 10 10", `<path d="M0,0 L10,5 L0,10 z" class="mk-fill"/>`, 10, 10, 9, 5},
	{"triangle", "0 0 14 14", `<path d="M1,1 L13,7 L1,13 z" class="mk-hollow"/>`, 14, 14, 13, 7},
	{"diamond", "0 0 18 10", `<path d="M1,5 L9,1 L17,5 L9,9 z" class="mk-fill"/>`, 18, 10, 17, 5},
	{"odiamond", "0 0 18 10", `<path d="M1,5 L9,1 L17,5 L9,9 z" class="mk-hollow"/>`, 18, 10, 17, 5},
	{"one", "0 0 14 14", `<path d="M6,1 V13 M10,1 V13" class="mk-line"/>`, 14, 14, 14, 7},
	{"many", "0 0 14 14", `<path d="M0,7 L14,1 M0,7 L14,7 M0,7 L14,13" class="mk-line"/>`, 14, 14, 14, 7},
	{
		"zero-one", "0 0 22 14",
		`<circle cx="6" cy="7" r="4" class="mk-hollow"/><path d="M15,1 V13" class="mk-line"/>`, 22, 14, 22, 7,
	},
	{
		"zero-many", "0 0 24 14",
		`<circle cx="5" cy="7" r="4" class="mk-hollow"/><path d="M10,7 L24,1 M10,7 L24,7 M10,7 L24,13" class="mk-line"/>`,
		24, 14, 24, 7,
	},
}

// markerKinds are what an edge's start and end take: none, or a marker.
var markerKinds = func() []string {
	kinds := make([]string, 0, 1+len(markers))
	kinds = append(kinds, "none")
	for _, m := range markers {
		kinds = append(kinds, m.kind)
	}
	return kinds
}()

// Render checks the specs and draws them, in order, as one HTML fragment of inline SVG, with the icons
// of store. The ids of the diagrams name their markers, so they must differ between the specs drawn
// together. On a mistake it draws nothing and returns the Problems of every spec.
func Render(specs []*Spec, store *icons.Store) (string, error) {
	var problems Problems
	layouts := make([]*layout, len(specs))
	first := map[string]*Spec{}
	for i, s := range specs {
		if f, ok := first[s.ID]; ok {
			err := usererr.New("diagram_id_used", s.ID, f.File, f.Line)
			problems = append(problems, &Error{File: s.File, Pos: s.Pos, Err: err})
		} else {
			first[s.ID] = s
		}
		l, err := newLayout(s, store)
		if err != nil {
			return "", err
		}
		layouts[i] = l
		problems = append(problems, layouts[i].check()...)
	}
	if len(problems) > 0 {
		return "", problems
	}
	out := make([]string, len(layouts))
	for i, l := range layouts {
		out[i] = string(l.svg())
	}
	return strings.Join(out, "\n"), nil
}

func (l *layout) svg() markup {
	s := l.spec
	var b strings.Builder
	for _, k := range l.markers {
		m := markers[slices.IndexFunc(markers, func(m marker) bool { return m.kind == k })]
		b.WriteString(string(svgf(`<marker id="%v_%v" viewBox="%v" refX="%v" refY="%v" markerWidth="%v" markerHeight="%v" `+
			`markerUnits="userSpaceOnUse" orient="auto-start-reverse">%v</marker>`,
			s.ID, k, m.viewBox, m.rx, m.ry, m.w, m.h, markup(m.body))))
	}
	defs := b.String()
	b.Reset()
	for i := len(l.grids) - 1; i >= 0; i-- {
		b.WriteString(string(l.grids[i]))
	}
	for _, layer := range [][]markup{l.edgeSVG, l.boxSVG, l.labelSVG} {
		for _, m := range layer {
			b.WriteString(string(m))
		}
	}
	var areas strings.Builder
	for _, m := range l.areaSVG {
		areas.WriteString(string(m))
	}
	return svgf(`<div class="dg-wrap" style="aspect-ratio: %v / %v; max-height: min(85vh, %vpx)">`+
		`<svg viewBox="0 0 %v %v" class="dg" role="img" aria-label="%v"><defs>%v</defs>%v%v</svg></div>`,
		s.Width, s.Height, math.Floor(s.Height*maxScale+0.5), s.Width, s.Height, s.Title,
		markup(defs), markup(areas.String()), markup(b.String()))
}

// markup is SVG already written, which svgf puts in as it is.
type markup string

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

// svgf formats SVG (with %v verbs) with every value escaped: a float64 is written as JavaScript writes numbers, a
// string is escaped for an attribute or an element, and markup goes in as it is.
func svgf(format string, args ...any) markup {
	for i, a := range args {
		switch a := a.(type) {
		case float64:
			args[i] = formatNum(a)
		case string:
			args[i] = escaper.Replace(a)
		case markup:
			args[i] = string(a)
		}
	}
	return markup(fmt.Sprintf(format, args...))
}

// formatNum writes v as JavaScript's Number#toString does, so that numbers in messages and SVG read
// as diagram.mjs writes them.
func formatNum(v float64) string {
	if v == 0 {
		return "0" // and -0
	}
	if a := math.Abs(v); a >= 1e21 || a < 1e-6 {
		mant, exp, _ := strings.Cut(strconv.FormatFloat(v, 'e', -1, 64), "e")
		return mant + "e" + exp[:1] + strings.TrimLeft(exp[1:], "0")
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
