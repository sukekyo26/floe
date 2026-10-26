package draw

import (
	"bytes"
	"maps"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

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

// checkPage reads page, drawn from d, as a browser does, and refuses, at d's template:, which draws it: a
// <style> (floe puts the page's CSS together, in layers), a <base> (the page's CSP ignores it), a name of an
// element's data-floe-part or data-floe-look that is no part or look of k, a data-floe-part not of one part's
// name as written (the scope of its style, [data-floe-part="name"], would not reach it) or of a look, which has
// no box, and no </head> before which a <style> styles the page. It returns where in page that </head> is, and
// what the page names.
func checkPage(k *kit.Kit, d *doc.Data, page []byte) (int, named, error) {
	tree, _ := html.Parse(bytes.NewReader(page)) //nolint:errcheck // a bytes.Reader fails no read, Parse's one error
	var found usererr.List
	refuse := func(key string, args ...any) {
		found = append(found, yamlpos.New(d.File, d.Where["template"], key, args...))
	}
	boxes, taken := map[string]bool{}, map[string]bool{}
	parts, looks, _, _ := k.Names()
	ns := names{parts: parts, all: slices.Sorted(slices.Values(slices.Concat(parts, looks)))}
	for n := range tree.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "style":
			refuse("draw_style")
		case "base":
			refuse("draw_base")
		}
		checkNames(k, ns, n, boxes, taken, refuse)
	}
	head := headAt(page)
	if head < 0 {
		refuse("draw_head")
	}
	if found != nil {
		return 0, named{}, found
	}
	return head, named{
		boxes: slices.Sorted(maps.Keys(boxes)), looks: slices.Sorted(maps.Keys(taken)), uses: uses(tree),
	}, nil
}

// names are the names of the parts of a Kit, and of its parts and looks, each sorted.
type names struct {
	parts, all []string
}

// checkNames refuses, by refuse, the element n's data-floe-part that names no part, or names that are not one
// part's as written, or a look's, or none, and its data-floe-look that names none, or a name that is no part
// or look of k (in ns), noting the others in boxes and taken. An empty one is refused too: the boundary of the
// parts' styles, [data-floe-part], would keep them from what it holds.
func checkNames(
	k *kit.Kit, ns names, n *html.Node, boxes, taken map[string]bool, refuse func(key string, args ...any),
) {
	if value, ok := attrOf(n, "data-floe-part"); ok {
		if strings.TrimFunc(value, isSpace) == "" {
			refuse("draw_part_several", value)
		}
		for _, name := range strings.Fields(value) {
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
		if strings.TrimFunc(value, isSpace) == "" {
			refuse("draw_look_empty", value)
		}
		for _, name := range strings.Fields(value) {
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
		names = append(names, strings.Fields(value)...)
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

// headEnd finds where a </head> may be written: what follows </head ends its name. headAt asks the parser
// which of those it is.
var headEnd = regexp.MustCompile(`(?i)</head[\t\n\f\r />]`)

// probeAttr marks the <style> headAt puts into a page to see where a browser puts the page's styles.
const probeAttr = "data-floe-probe"

// headAt returns where in page the first </head> is that a <style> put before it styles the page: one in
// no <template>, whose content is not drawn, and in no <svg> or <math>, where it styles nothing; in the head,
// or in the body a <div> of the head moved it to, where it styles the page all the same. Each place a
// </head> may be written is tried with the parser, which says what it is: in a comment, a value or a
// script's text, the <style> put there is no element. -1 for none.
// LIMIT: the page is read once for each </head> tried; read fewer, from the tree, if pages come to write many.
func headAt(page []byte) int {
	for _, at := range headEnd.FindAllIndex(page, -1) {
		probe := []byte("<style " + probeAttr + "></style>")
		tried := slices.Concat(page[:at[0]], probe, page[at[0]:])
		tree, _ := html.Parse(bytes.NewReader(tried)) //nolint:errcheck // a bytes.Reader fails no read, Parse's one error
		if styled(tree) {
			return at[0]
		}
	}
	return -1
}

// styled reports whether tree holds the <style> headAt puts, as it puts it, where it styles the page.
func styled(tree *html.Node) bool {
	for n := range tree.Descendants() {
		// Its attribute alone and no content: one of the page's, which the probe went into as a value, is not it.
		if n.Type != html.ElementNode || n.DataAtom != atom.Style || n.Namespace != "" || n.FirstChild != nil ||
			len(n.Attr) != 1 || n.Attr[0].Key != probeAttr {
			continue
		}
		for a := range n.Ancestors() {
			if a.Type == html.ElementNode && (a.Namespace != "" || a.DataAtom == atom.Template) {
				return false
			}
		}
		return true
	}
	return false
}
