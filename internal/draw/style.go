package draw

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/kit"
)

// pageStyle returns the CSS of each <style> of a page drawn in the theme named so, whose elements name what n
// holds, with own, its template's page.css. Each is a layer: floe's base.css, under the palettes (the default theme's,
// then the theme's), the theme's base.css, the looks, the template's own and the parts'. The theme's base.css
// reaches what the template's does, the page short of what the parts hold, so that the template, the looks and the
// parts each win over it, and a part draws what it holds from floe's base alone. The parts' are the style of
// each part the page names a box of, and of each look the page or those parts take, before the parts that take it,
// which adjust it; then the theme's style of each of those parts in the same order, which wins over theirs, their
// @media print too. An element that takes a look without being a part's box (a card of options) has it in
// floe.looks, under the template's and the parts': the part it is in, or the template it is the template's own of,
// adjusts it whatever the selectors, and the theme's base.css, a layer under, wins over none of its looks, unless
// with !important.
// Nested, the outer wins: floe.looks and floe.parts are each layered by how deep in the page the element a style
// reaches stands, counted in the boxes and the elements that take looks around it, the outer the later, so that a
// look's style for a look held in it, and a part's for a part's box held in it, win whatever the selectors, where
// the nearer scope would win in one layer. Between the looks and the parts the layers' order stands: a look loses
// to the boxes of the parts it holds. Within a layer the more specific rule wins, else the one of the nearer scope,
// else the later.
// A part's style and its theme's reach the part's elements, and the box of a part it holds but not what
// that part holds; the template's reaches the boxes of the parts, so that it places them, but not what
// they hold.
// Under the template's page.css, floe.page.defaults gives the body no margin and lays out the column (pageDefaults).
// Under the layers of depth, floe.looks.boxes and floe.parts.boxes lay out the lists that are parts' boxes or take
// looks (boxes), so that the look's or part's own rules at any depth set them over, and the page's and the theme's do
// not reach a part's box.
// LIMIT: a theme's file of a look wins over the adjustment of it by a part that takes it on the same element; move
// it after the look's if a part needs one.
// LIMIT: a part's box that takes a look, held in an element that takes another, has it in floe.parts, which wins over
// the outer look's rules for it. Make the outer a part's box if HTML comes to nest them so.
// LIMIT: each style is written once for each depth its elements stand at, so a deep page's CSS grows with its depth;
// no bound is set. Mark the depth on the elements instead if that comes to weigh.
// LIMIT: !important turns the layers' order round (an earlier layer's wins), and floe does not refuse it.
func pageStyle(k *kit.Kit, theme string, n named, own []byte) ([]string, error) {
	themed, ok := k.Theme(theme)
	base, okBase := k.Theme(kit.DefaultTheme)
	if !ok || !okBase {
		//nolint:err113 // floe's own mistake
		return nil, fmt.Errorf("draw: no theme %s or %s, which the data was checked with", theme, kit.DefaultTheme)
	}
	// The first sheet names the layers, which puts them in that order. Each file after it is a sheet of its
	// own, so that a mistake in it, a comment or a block left open, ends with it.
	var sheets []string
	// The layers of depth, the deepest first, so that the outer win.
	deepest := 0
	for _, d := range n.depths {
		deepest = max(deepest, slices.Max(d))
	}
	var order []string
	for _, kind := range []string{"floe.looks", "floe.parts"} {
		order = append(order, kind+".boxes")
		for d := deepest; d >= 0; d-- {
			order = append(order, fmt.Sprintf("%s.depth%d", kind, d))
		}
	}
	layers := "@layer floe.palette, floe.base, floe.theme, floe.looks, floe.marks, floe.page, floe.parts;\n@layer " +
		strings.Join(order, ", ") + ";\n" + boxes + "@layer floe.base {\n" + string(k.BaseCSS()) + "}\n"
	sheets = append(sheets, Sheet("", []byte(layers)))
	sheet := func(layer string, css []byte) {
		if len(css) != 0 {
			sheets = append(sheets, Sheet(layer, css))
		}
	}
	sheet("floe.palette", base.Palette())
	if theme != kit.DefaultTheme {
		sheet("floe.palette", themed.Palette())
	}
	tones, err := k.Tones()
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's own, which its tests check
	}
	sheet("floe.palette", Tones(tones))
	sheet("floe.theme", pageScoped(themed.Base()))
	// LIMIT: markdown.css reaches the whole page, a part's own elements too: a rule for p or ul there styles what a
	// part draws as such, where its part.css writes nothing. Keep Markdown's alone to it (what wraps every Markdown
	// output in an element of its own would move what parts space by > * + *; see S164).
	if md := themed.Markdown(); len(md) > 0 {
		sheet("floe.theme", InScope("(:root)", md))
	}
	names := inOrder(k, slices.Concat(n.boxes, n.looks))
	// Each style goes to the boxes that have its look, and to the other elements that take it, a layer under, each
	// in the layer of its depth.
	styled := func(name string, css []byte) {
		for _, kind := range []struct {
			layer string
			look  bool
			named []string
		}{{"floe.parts", false, n.boxes}, {"floe.looks", true, n.looks}} {
			byDepth := scoped(k, name, kind.named, kind.look, n.depths, css)
			for _, d := range slices.Sorted(maps.Keys(byDepth)) {
				sheet(fmt.Sprintf("%s.depth%d", kind.layer, d), byDepth[d])
			}
		}
	}
	for _, name := range names {
		styled(name, cssOf(k, name))
	}
	for _, name := range names {
		styled(name, themed.CSS(name))
	}
	sheet("floe.marks", marks(k, n))
	sheet("floe.page.defaults", pageScoped([]byte(pageDefaults)))
	sheet("floe.page", pageScoped(own))
	return sheets, nil
}

// boxes lays out a list (ul, ol, menu: the elements a browser marks and pads) that is a part's box or takes a look, as
// what draws it does: without a list's marks, padding or the room between items, in a grid. Each goes in the lowest
// layer of what draws it, under the layers of depth, so that the part's or look's own rules at any depth set it over.
// A part's box keeps these four over the page's and the theme's rules for lists; an element that takes a look over the
// theme's alone, as the template that borrows a look adjusts it (a rule for the items, li or ::marker, reaches both).
// floe's own and fixed, it goes unchecked by css.Check.
const boxes = "@layer floe.parts.boxes { :is(ul, ol, menu)[data-floe-part] { " + boxList + " } }\n" +
	"@layer floe.looks.boxes { :is(ul, ol, menu)[data-floe-look] { " + boxList + " } }\n"

// pageDefaults is what every template's page starts from: the body's margin and the column a template centres its page
// in (class floe-column), --column-width wide, the palette's --page-width unless the template sets it. Scoped as the
// template's page.css is, in a sublayer of floe.page under it, so that page.css wins over it whatever its selectors, as
// when each template wrote it, and the theme's base.css and the looks do not. floe's own and fixed, it goes unchecked
// by css.Check.
// LIMIT: --column-width is the template's to set; a theme that sets it in its palette widens the column of a template
// that sets none (report, design, handoff, postmortem) alone, as adr's and runbook's page.css win over it. Nothing
// checks it; set it in a template's page.css alone.
// LIMIT: with no font-size on the body, a browser sizes text in monospace alone from 13px, as no ancestor sets a size:
// a theme whose --mono is monospace alone draws a line's code about 11px. Name a face before monospace.
const pageDefaults = "body { margin: 0; }\n" +
	".floe-column { max-width: calc(var(--column-width, var(--page-width)) + 2 * var(--page-pad)); " +
	"margin-inline: auto; padding-inline: var(--page-pad); }\n"

const boxList = "display: grid; list-style: none; padding: 0; gap: 0;"

// Tones gives each element marked with a tone of tones (data-floe-tone) the tone's colours from the palette: --tone
// and --tone-soft from --<tone> and --<tone>-soft, and --tone-fill from --<tone>-fill, a theme's own fill for the tone,
// or else --<tone>-soft; and the root (:scope) and each part's box accent's, so that a part takes no tone from around
// it and a rule outside a tone reads accent. A part's box that takes a tone holds them as specific: the tone's rule,
// written after, wins. floe's own and fixed, it goes unchecked by css.Check.
// LIMIT: a data-floe-tone of no tone matches none, as if not written: a part's box draws in accent's colours, another
// element in the tone around it.
func Tones(tones []string) []byte {
	var b strings.Builder
	b.WriteString(toneRule(":where(:scope, [data-floe-part])", "accent"))
	for _, t := range tones {
		b.WriteString(toneRule(fmt.Sprintf(":where([data-floe-tone=%q])", t), t))
	}
	return []byte(b.String())
}

func toneRule(selector, tone string) string {
	return fmt.Sprintf("%[1]s { --tone: var(--%[2]s); --tone-soft: var(--%[2]s-soft); "+
		"--tone-fill: var(--%[2]s-fill, var(--%[2]s-soft)); }\n", selector, tone)
}

// pageScoped is css reaching the page short of what the parts and texts hold (a text draws Markdown in a box of no
// part, data-floe-text, which has no ground or tone of its own), or nil when css is empty.
func pageScoped(css []byte) []byte {
	if len(css) == 0 {
		return nil
	}
	return InScope("(:root) to ([data-floe-part] > *, [data-floe-text] > *)", css)
}

// InScope is css in @scope with the prelude scope, its root :scope.
func InScope(scope string, css []byte) []byte {
	return []byte("@scope " + scope + " {\n" + string(css) + "}\n")
}

// Sheet is css in the layer named layer (in none for ""), as the text of a <style> of its own, so that a mistake in
// it, a comment or a block left open, ends with it. No </style in it, in a comment or a string too, closes the
// <style> and leaves what follows to be read as the page's HTML.
func Sheet(layer string, css []byte) string {
	body := string(css)
	if layer != "" {
		body = "@layer " + layer + " {\n" + body + "}\n"
	}
	return "\n" + styleEnd.ReplaceAllString(body, `<\/$1`)
}

// styleEnd finds </style, of either case, which HTML reads as the end of a <style>. Written <\/style it is
// the same CSS in a string, a comment or a url(): the / is escaped, and no comment starts at a / that a
// letter follows.
// LIMIT: elsewhere, as a variable's value (--x: </style>), the value keeps the \; written so, it closed the
// <style> anyway.
var styleEnd = regexp.MustCompile(`(?i)</(style)`)

// inOrder returns names, parts and looks of k, with the looks they take, each after its looks, else in order.
func inOrder(k *kit.Kit, names []string) []string {
	names = slices.Compact(slices.Sorted(slices.Values(names)))
	done := map[string]bool{}
	ordered := make([]string, 0, len(names))
	var visit func(name string)
	visit = func(name string) {
		done[name] = true
		for _, look := range looksOf(k, name) {
			if !done[look] {
				visit(look)
			}
		}
		ordered = append(ordered, name)
	}
	for _, name := range names {
		if !done[name] {
			visit(name)
		}
	}
	return ordered
}

// scoped is css, the style of the part or look name or its theme's for it, reaching the elements that have
// its look, from their own on, short of what the parts in them hold, by the depth they stand at (depths): of
// named, the parts a page names, each that is name or takes its look, the boxes (data-floe-part="table"), or the
// elements that take it without being a box when look (data-floe-look="table"). It is nil when none of them has
// the look, or css is empty.
func scoped(k *kit.Kit, name string, named []string, look bool, depths map[string][]int, css []byte) map[int][]byte {
	roots := map[int][]string{}
	for _, n := range named {
		if n != name && !slices.Contains(k.Taken(n), name) {
			continue
		}
		key, root := "part "+n, `[data-floe-part="`+n+`"]`
		if look {
			key, root = "look "+n, `[data-floe-look~="`+n+`"]`
		}
		for _, d := range depths[key] {
			roots[d] = append(roots[d], at(d, root))
		}
	}
	if len(roots) == 0 || len(css) == 0 {
		return nil
	}
	out := map[int][]byte{}
	for d, r := range roots {
		out[d] = []byte("@scope (" + strings.Join(r, ", ") + ") to (:scope [data-floe-part] > *) {\n" + string(css) + "}\n")
	}
	return out
}

// nester is an element that nests, as nests counts it: a part's box or one that takes a look, of any value.
const nester = ":is([data-floe-part], [data-floe-look])"

// at is the selector of the elements of root that stand depth deep: held by depth elements that nest, no more. The
// depth is in :where, so that the root is as specific as root alone at every depth, whatever specificity a browser
// gives & in a scope (Chrome gives it none).
func at(depth int, root string) string {
	within := ""
	if depth > 0 {
		within = ":where(" + strings.Repeat(nester+" ", depth) + "*)"
	}
	return root + within + ":where(:not(" + strings.Repeat(nester+" ", depth+1) + "*))"
}

// looksOf returns the looks the part or look name takes, in order.
func looksOf(k *kit.Kit, name string) []string {
	if p, ok := k.Part(name); ok {
		return p.Looks()
	}
	if l, ok := k.Look(name); ok {
		return l.Looks()
	}
	return nil
}

// cssOf returns the part.css of the part or look name.
func cssOf(k *kit.Kit, name string) []byte {
	if p, ok := k.Part(name); ok {
		return p.CSS()
	}
	if l, ok := k.Look(name); ok {
		return l.CSS()
	}
	return nil
}

// marks is the layout of the rows with a mark at their left, in the layer floe.marks: of the parts and looks the
// page names, and the looks they take, those whose part.yaml's marks: says so (box, items or joined), each row under
// the root of the name the page names. It places and sizes the mark (::before) and, in a row of box or items, a mark
// over it (::after); in one of joined the line to the next (::after). Colours, corners and content are theirs.
// LIMIT: a template's page.css, a part's part.css, a look a part takes and a theme's file of a part, all in layers
// over floe.marks, that write a row's display, gap or grid-template win over it unsaid; TestUIMarks measures floe's.
// LIMIT: the ::after of a row of box or items is the slot of a mark over the mark: a decoration of its own drawn in it
// moves over the mark unsaid (links alone draws one, the sign over its box).
// LIMIT: kit refuses two marks: down one part.yaml's looks alone; an element that HTML gives two of them (an ol of
// data-floe-look="numbered marked", a part's box that takes marked) has both rules on its rows, not refused.
func marks(k *kit.Kit, n named) []byte {
	var rows, stacked, joined []string
	add := func(name, root string) {
		for _, m := range append([]string{name}, k.Taken(name)...) {
			switch k.Marks(m) {
			case "box":
				rows = append(rows, root)
				stacked = append(stacked, root)
			case "items":
				rows = append(rows, root+" > li")
				stacked = append(stacked, root+" > li")
			case "joined":
				rows = append(rows, root+" > li")
				joined = append(joined, root+" > li")
			}
		}
	}
	for _, b := range slices.Compact(slices.Sorted(slices.Values(n.boxes))) {
		add(b, `[data-floe-part="`+b+`"]`)
	}
	for _, l := range slices.Compact(slices.Sorted(slices.Values(n.looks))) {
		add(l, `[data-floe-look~="`+l+`"]`)
	}
	if len(rows) == 0 {
		return nil
	}
	const mark = `grid-area: mark; align-self: first baseline; display: grid; place-items: center;
  box-sizing: border-box; width: var(--floe-mark); height: var(--floe-mark); margin-inline-end: var(--floe-mark-gap);`
	r := ":where(" + strings.Join(rows, ", ") + ")"
	css := r + ` {
  position: relative; anchor-scope: --floe-mark; display: grid; gap: 0;
  grid-template: [mark-start] auto [mark-end] / [mark-start] auto [mark-end text-start] minmax(0, 1fr) [text-end];
}
` + r + ` > * { grid-column: text; align-self: first baseline; }
` + r + `::before {
  anchor-name: --floe-mark; ` + mark + `
}
`
	if len(stacked) > 0 {
		css += ":where(" + strings.Join(stacked, ", ") + ")::after {\n  " + mark + "\n}\n"
	}
	if len(joined) > 0 {
		css += ":where(" + strings.Join(joined, ", ") + `):not(:last-child)::after {
  content: ""; position: absolute; top: calc(anchor(--floe-mark bottom) + 0.35rem); bottom: 0.35rem;
  left: calc(anchor(--floe-mark center) - 1px); width: 2px;
}
`
	}
	return []byte(css)
}
