package draw

import (
	"maps"
	"slices"
	"strings"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// named is what the elements of a page drawn name: the boxes of parts (data-floe-part) and the looks they
// take without being a box (data-floe-look), each a part or a look of the Kit, sorted; and, by part, what it
// uses, which it places: the parts and looks its elements name, and the parts its schema may hold.
type named struct {
	boxes, looks []string
	uses         map[string][]string
}

// checkPage refuses in tree, the page drawn from d as a browser reads it, at d's template:, which draws it: a
// <style>, of HTML, SVG or MathML (floe puts the page's CSS together, in layers, and an SVG's styles the whole
// page as an HTML one does), a <base> (the page's CSP ignores it), a <meta http-equiv> (it moves the page, sets
// a cookie or changes its policy), a <noscript> (read as text with scripts on, its elements work with them
// off, unchecked), a name of an
// element's data-floe-part or data-floe-look that is no part or look of k, and a data-floe-part not of one
// part's name as written (the scope of its style, [data-floe-part="name"], would not reach it) or of a look,
// which has no box. It returns what the page names.
func checkPage(k *kit.Kit, ns names, d *doc.Data, tree *html.Node) (named, error) {
	var found usererr.List
	refuse := func(key string, args ...any) {
		found = append(found, yamlpos.New(d.File, d.Where["template"], key, args...))
	}
	boxes, taken := map[string]bool{}, map[string]bool{}
	for n := range tree.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "style":
			refuse("draw_style")
		case "base":
			refuse("draw_base")
		case "noscript":
			refuse("draw_noscript")
		case "meta":
			if equiv, ok := attrOf(n, "http-equiv"); ok {
				refuse("draw_meta_http_equiv", equiv)
			}
		}
		checkNames(k, ns, n, boxes, taken, refuse)
	}
	if found != nil {
		return named{}, found
	}
	return named{
		boxes: slices.Sorted(maps.Keys(boxes)), looks: slices.Sorted(maps.Keys(taken)), uses: uses(tree),
	}, nil
}

// names are the names of the parts of a Kit, and of its parts and looks, each sorted.
type names struct {
	parts, all []string
}

// namesIn returns the names of k.
func namesIn(k *kit.Kit) names {
	parts, looks, _, _ := k.Names()
	return names{parts: parts, all: slices.Sorted(slices.Values(slices.Concat(parts, looks)))}
}

// checkNames refuses, by refuse, the element n's data-floe-part that names no part (empty: the boundary of the
// parts' styles, [data-floe-part], would keep them from what it holds), or names that are not one part's as
// written, or a look's, or none, and a name of its data-floe-look that is no part or look of k (in ns), noting
// the others in boxes and taken.
func checkNames(
	k *kit.Kit, ns names, n *html.Node, boxes, taken map[string]bool, refuse func(key string, args ...any),
) {
	if value, ok := attrOf(n, "data-floe-part"); ok {
		if strings.TrimFunc(value, isSpace) == "" {
			refuse("draw_part_several", value)
		}
		for _, name := range fields(value) {
			_, isPart := k.Part(name)
			_, isLook := k.Look(name)
			switch {
			case value != name:
				refuse("draw_part_several", value)
			case isLook:
				refuse("draw_look_box", name)
			case !isPart:
				refuse("draw_part_unknown", name, strings.Join(ns.parts, ", "))
			default:
				boxes[name] = true
			}
		}
	}
	if value, ok := attrOf(n, "data-floe-look"); ok {
		for _, name := range fields(value) {
			if !slices.Contains(ns.all, name) {
				refuse("draw_part_unknown", name, strings.Join(ns.all, ", "))
				continue
			}
			taken[name] = true
		}
	}
}

// uses returns, by the name of each part's box in tree, what the elements it holds nearest name: the parts
// whose boxes they are, and the looks they take (data-floe-look).
func uses(tree *html.Node) map[string][]string {
	found := map[string][]string{}
	for n := range tree.Descendants() {
		if holder, ok := holderOf(n); ok {
			found[holder] = append(found[holder], namesOf(n)...)
		}
	}
	return found
}

// holderOf returns the part whose box holds n nearest.
func holderOf(n *html.Node) (string, bool) {
	for a := range n.Ancestors() {
		if holder, ok := boxOf(a); ok {
			return holder, true
		}
	}
	return "", false
}

// namesOf returns what n names, an element: the part it is the box of, and the looks it takes.
func namesOf(n *html.Node) []string {
	if n.Type != html.ElementNode {
		return nil
	}
	var names []string
	if name, ok := boxOf(n); ok {
		names = append(names, name)
	}
	if value, ok := attrOf(n, "data-floe-look"); ok {
		names = append(names, fields(value)...)
	}
	return names
}

// boxOf returns the part n, an element, is the box of: its data-floe-part, one name as written.
func boxOf(n *html.Node) (string, bool) {
	if n.Type != html.ElementNode {
		return "", false
	}
	value, ok := attrOf(n, "data-floe-part")
	return value, ok && value != "" && !strings.ContainsFunc(value, isSpace)
}

// fields returns the names value holds, as HTML splits an attribute of names: at its ASCII whitespace.
func fields(value string) []string {
	return strings.FieldsFunc(value, isSpace)
}

// isSpace reports whether r is ASCII whitespace, which HTML splits names at (and CSS's ~= too).
func isSpace(r rune) bool {
	return strings.ContainsRune(" \t\n\f\r", r)
}

// attrOf returns the value of n's attribute key, of no namespace.
func attrOf(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}
