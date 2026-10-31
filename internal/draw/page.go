package draw

import (
	"maps"
	"slices"
	"strings"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/page"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// named is what the elements of a page drawn name: the boxes of parts (data-floe-part) and the looks they
// take without being a box (data-floe-look), each a part or a look of the Kit, sorted; and, by "part " or "look "
// and the name, the depths those elements stand at, as depths counts them.
type named struct {
	boxes, looks []string
	depths       map[string][]int
}

// checkPage refuses in tree, the page drawn from d as a browser reads it, at d's template:, which draws it: an
// element page.Refused refuses, of any namespace, in the words of refusal, in an HTML <select> an element of
// textInSelect or svg or math (heldAsText), a name of an element's data-floe-part or data-floe-look that is no
// part or look of k, and a data-floe-part not of one part's name as written (the scope of its style,
// [data-floe-part="name"], would not reach it) or of a look, which has no box, and an attribute where
// html/template wrote ZgotmplZ (replaced), in an element that the switch's checks (page.Refused, an element in a
// <select>) have not refused. It returns what the page names.
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
		switch bad := page.Refused(n); bad {
		case "":
			if heldAsText(n) && inSelect(n) {
				refuse("draw_select_text", n.Data)
			} else if tag := replaced(n); tag != "" {
				refuse("draw_zgotmplz", tag)
			}
		default:
			var args []any
			if bad == "meta" {
				var b strings.Builder
				_ = html.Render(&b, n) //nolint:errcheck // a strings.Builder fails no write
				args = append(args, yamlpos.Head(b.String()))
			}
			refuse(refusal[bad], args...)
		}
		checkNames(k, ns, n, boxes, taken, refuse)
	}
	if found != nil {
		return named{}, found
	}
	return named{
		boxes: slices.Sorted(maps.Keys(boxes)), looks: slices.Sorted(maps.Keys(taken)), depths: depths(tree),
	}, nil
}

// refusal is the key that tells each element page.Refused refuses, with why a page does not take it; a <meta>'s
// words take its tag.
var refusal = map[string]string{
	"base": "draw_base", "meta": "draw_meta", "noscript": "draw_noscript", "style": "draw_style",
}

// textInSelect are the HTML elements whose content is read as text, as the HTML fragment parsing algorithm sets
// the tokenizer's state by its context element (RCDATA, RAWTEXT, script data, PLAINTEXT), written as it is:
// style and noscript are refused anywhere above, before they are looked up here.
var textInSelect = map[string]bool{
	"iframe": true, "noembed": true, "noframes": true, "noscript": true, "plaintext": true, "script": true,
	"style": true, "textarea": true, "title": true, "xmp": true,
}

// heldAsText reports whether n is an HTML element of textInSelect, or where svg or math starts under an HTML
// one: in a <select>, what a parser of the old "in select" mode reads apart from x/net/html, which takes the
// select's content as the body's, so that what it holds as text the other may read as elements once a tag in
// it closes the select.
func heldAsText(n *html.Node) bool {
	if n.Namespace == "" {
		return textInSelect[n.Data]
	}
	return (n.Namespace == "svg" || n.Namespace == "math") && n.Parent != nil && n.Parent.Namespace == ""
}

// inSelect reports whether an HTML <select> holds n.
// LIMIT: a browser of the new select copies the chosen option's content into a <selectedcontent>, which
// x/net/html does not; which browsers still read a select the old way is unknown. Check in browsers if a page
// comes to differ there.
func inSelect(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Namespace == "" && p.Data == "select" {
			return true
		}
	}
	return false
}

// replaced returns the start tag of n with only its first attribute where html/template wrote ZgotmplZ in place
// of a value it does not take there (in a name, a style's value, a URL, an empty value without quotes), "" for
// none: a name is lower case here, as the parser reads it, and a value is compared as html/template writes it,
// so that a link to a heading's id (#zgotmplz-…) is not refused.
// LIMIT: a ZgotmplZ written as it is (title="ZgotmplZ", a diagram's title, a name data-zgotmplz-x) is refused
// too; if one asks to write it, wrap html/template's escapers so that a value replaced is an error.
// LIMIT: in an xmp, iframe, noembed, noframes or plaintext, whose content x/net/html reads as text, html/template
// writes elements, and a ZgotmplZ in their attributes is missed; no browser draws them as elements there, so they
// do nothing. Read that content again if pages come to write elements in it.
func replaced(n *html.Node) string {
	i := slices.IndexFunc(n.Attr, func(a html.Attribute) bool {
		return strings.Contains(a.Key, "zgotmplz") || strings.Contains(a.Val, "ZgotmplZ")
	})
	if i < 0 {
		return ""
	}
	// The start tag alone, of the attribute alone: the others may be long, and the element holds the rest.
	bare := &html.Node{ //nolint:exhaustruct_v5 // a tag of no tree
		Type: html.ElementNode, DataAtom: n.DataAtom, Data: n.Data, Namespace: n.Namespace, Attr: n.Attr[i : i+1],
	}
	var b strings.Builder
	_ = html.Render(&b, bare) //nolint:errcheck // a strings.Builder fails no write
	tag := b.String()
	// On one line, whole: Render leaves a line break in a value as it is, and a value cut short may lose ZgotmplZ.
	return strings.ReplaceAll(tag[:strings.IndexByte(tag, '>')+1], "\n", " ")
}

// names are the names of the parts of a Kit, and of its parts and looks, each sorted.
// LIMIT: a name that is none is offered text and script too, which draw no element; leave them out if that misleads.
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

// depths returns, by "part " and the name of each part's box in tree and "look " and each look an element takes,
// the depths those elements stand at, sorted: how many elements that nest hold each.
func depths(tree *html.Node) map[string][]int {
	found := map[string][]int{}
	for n := range tree.Descendants() {
		if !nests(n) {
			continue
		}
		depth := 0
		for a := range n.Ancestors() {
			if nests(a) {
				depth++
			}
		}
		if name, ok := boxOf(n); ok {
			found["part "+name] = append(found["part "+name], depth)
		}
		if value, ok := attrOf(n, "data-floe-look"); ok {
			for _, name := range fields(value) {
				found["look "+name] = append(found["look "+name], depth)
			}
		}
	}
	for key, d := range found {
		found[key] = slices.Compact(slices.Sorted(slices.Values(d)))
	}
	return found
}

// nests reports whether n is an element with a data-floe-part or a data-floe-look, of any value, empty too: as
// the browser counts it by the selector nester.
func nests(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	_, box := attrOf(n, "data-floe-part")
	_, look := attrOf(n, "data-floe-look")
	return box || look
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
