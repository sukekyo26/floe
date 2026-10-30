package draw

import (
	"fmt"
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
// each part the page names a box of, and of each look the page or those parts take, before the parts that take it
// and the parts that place it (those whose boxes hold its box), which adjust it; then the theme's style of each of
// those parts in the same order, which wins over theirs, their @media print too.
// Within the layer the more specific rule wins, else the later.
// An element that takes a look without being a part's box (a card of options) has it a layer under the
// template's and the parts', floe.looks: the part it is in, or the template it is the template's own of,
// adjusts it whatever the selectors, as a part does the looks it takes, where the nearer scope would win
// otherwise.
// LIMIT: such elements share the layer, so the look of one held in another wins over the outer's rules for
// it by the nearer scope. Make the outer a part's box (data-floe-part) if HTML comes to nest them so.
// A part's style and its theme's reach the part's elements, and the box of a part it holds but not what
// that part holds; the template's reaches the boxes of the parts, so that it places them, but not what
// they hold.
// LIMIT: a theme's file of a look wins over a part's adjustment of it; move it after the look's if a part needs one.
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
	// No </style in a CSS, in a comment or a string too, closes its <style> and leaves what follows to be read
	// as the page's HTML.
	write := func(body string) {
		sheets = append(sheets, "\n"+styleEnd.ReplaceAllString(body, `<\/$1`))
	}
	write("@layer floe.palette, floe.base, floe.theme, floe.looks, floe.page, floe.parts;\n@layer floe.base {\n" +
		string(k.BaseCSS()) + "}\n")
	sheet := func(layer string, css []byte) {
		if len(css) != 0 {
			write("@layer " + layer + " {\n" + string(css) + "}\n")
		}
	}
	sheet("floe.palette", base.Palette())
	if theme != kit.DefaultTheme {
		sheet("floe.palette", themed.Palette())
	}
	sheet("floe.theme", pageScoped(themed.Base()))
	names := inOrder(k, slices.Concat(n.boxes, n.looks), n.uses)
	// Each style goes to the boxes that have its look, and to the other elements that take it, a layer under.
	styled := func(name string, css []byte) {
		sheet("floe.parts", scoped(k, name, n.boxes, false, css))
		sheet("floe.looks", scoped(k, name, n.looks, true, css))
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

// pageScoped is css reaching the page short of what the parts hold, or nil when css is empty.
func pageScoped(css []byte) []byte {
	if len(css) == 0 {
		return nil
	}
	return []byte("@scope (:root) to ([data-floe-part] > *) {\n" + string(css) + "}\n")
}

// styleEnd finds </style, of either case, which HTML reads as the end of a <style>. Written <\/style it is
// the same CSS in a string, a comment or a url(): the / is escaped, and no comment starts at a / that a
// letter follows.
// LIMIT: elsewhere, as a variable's value (--x: </style>), the value keeps the \; written so, it closed the
// <style> anyway.
var styleEnd = regexp.MustCompile(`(?i)</(style)`)

// inOrder returns names, parts and looks of k, with the looks they take, each after its looks and the parts
// it places that names holds (uses), those it places first, else in order.
func inOrder(k *kit.Kit, names []string, uses map[string][]string) []string {
	names = slices.Compact(slices.Sorted(slices.Values(names)))
	done := map[string]bool{}
	ordered := make([]string, 0, len(names))
	var visit func(name string)
	visit = func(name string) {
		done[name] = true
		for _, used := range uses[name] {
			if slices.Contains(names, used) && !done[used] {
				visit(used)
			}
		}
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
// its look, from their own on, short of what the parts in them hold: of named, the parts a page names, each
// that is name or takes its look, the boxes (data-floe-part="table"), or the elements that take it without
// being a box when look (data-floe-look="table"). It is nil when none of them has the look, or css is empty.
func scoped(k *kit.Kit, name string, named []string, look bool, css []byte) []byte {
	var roots []string
	for _, n := range named {
		switch {
		case n != name && !slices.Contains(takenBy(k, n), name):
		case look:
			roots = append(roots, `[data-floe-look~="`+n+`"]`)
		default:
			roots = append(roots, `[data-floe-part="`+n+`"]`)
		}
	}
	if roots == nil || len(css) == 0 {
		return nil
	}
	return []byte("@scope (" + strings.Join(roots, ", ") + ") to (:scope [data-floe-part] > *) {\n" + string(css) + "}\n")
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
