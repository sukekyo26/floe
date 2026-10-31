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
	layers := "@layer floe.palette, floe.base, floe.theme, floe.looks, floe.page, floe.parts;\n@layer " +
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
	sheet("floe.theme", pageScoped(themed.Base()))
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
	sheet("floe.page", pageScoped(own))
	return sheets, nil
}

// boxes lays out a list (ul, ol, menu: the elements a browser marks and pads) that is a part's box or takes a look, as
// what draws it does: without a list's marks, padding or the room between items, in a grid. Each goes in the lowest
// layer of what draws it, under the layers of depth, so that the part's or look's own rules at any depth set it over.
// A part's box is so over the page's and the theme's rules for lists; an element that takes a look over the theme's
// alone, as the template that borrows a look adjusts it. floe's own and fixed, it goes unchecked by css.Check.
const boxes = "@layer floe.parts.boxes { :is(ul, ol, menu)[data-floe-part] { " + boxList + " } }\n" +
	"@layer floe.looks.boxes { :is(ul, ol, menu)[data-floe-look] { " + boxList + " } }\n"

const boxList = "display: grid; list-style: none; padding: 0; gap: 0;"

// pageScoped is css reaching the page short of what the parts hold, or nil when css is empty.
func pageScoped(css []byte) []byte {
	if len(css) == 0 {
		return nil
	}
	return InScope("(:root) to ([data-floe-part] > *)", css)
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
		if n != name && !slices.Contains(takenBy(k, n), name) {
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

// takenBy returns the looks name takes, and those they take in turn, each once. No look takes itself:
// kit's check refuses one that would.
func takenBy(k *kit.Kit, name string) []string {
	var found []string
	var follow func(at string)
	follow = func(at string) {
		for _, look := range looksOf(k, at) {
			if !slices.Contains(found, look) {
				found = append(found, look)
				follow(look)
			}
		}
	}
	follow(name)
	return found
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
