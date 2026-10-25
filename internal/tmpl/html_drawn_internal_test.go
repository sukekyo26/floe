package tmpl

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// attrNames returns the names of named an attribute gives, not text.
func attrNames(named []namedPart) []namedPart {
	var out []namedPart
	for _, n := range named {
		if !n.text {
			out = append(out, n)
		}
	}
	return out
}

// valuesOf returns the values of named, an attribute's, in order.
func valuesOf(named []namedPart) []string {
	values := make([]string, 0, len(named))
	for _, n := range attrNames(named) {
		values = append(values, n.value)
	}
	return values
}

// A drawn page's names are its elements' as a browser reads them: character references read, a second
// attribute of a name left out, a <div> in an <svg>'s <style> back out in HTML, a <template>'s elements
// too, and no tag of a <noscript>'s text, of an end tag, or of a tag a quote leaves open.
func TestDrawnNames(t *testing.T) {
	t.Parallel()

	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	for item, want := range map[string]string{
		`<div data-floe-part="&#110;ote"></div><p data-floe-look="note&#32;table"></p>`: "note|note table|note table",
		`<div data-floe-part="note" data-floe-part="nte"></div>`:                        "note",
		`<svg><style><div data-floe-part="note"></div></style></svg>`:                   "note",
		`<template><div data-floe-part="note"></div></template>`:                        "note",
		`<noscript><div data-floe-part="note"></div></noscript>`:                        "",
		`<p>x</p data-floe-part="note">`:                                                "",
		`<p>x</p><div data-floe-part='note>`:                                            "",
		`<script>'<b data-floe-part="note">'</script>`:                                  "",
	} {
		nodes, err := html.ParseFragment(strings.NewReader(item), body)
		if err != nil {
			t.Fatal(err)
		}
		named := drawnNames(nodes, item)
		if got := strings.Join(valuesOf(named), "|"); got != want {
			t.Errorf("drawnNames(%q) = %q, want %q", item, got, want)
		}
		if want == "" && strings.Contains(item, "data-floe-part=\"note\"") && len(named) == 0 {
			t.Errorf("drawnNames(%q) brings no style of the name written in it", item)
		}
	}
}

// An item's names are those of its start tags as the tokenizer reads them, wherever it goes: a <tr>'s too,
// which a fragment read in the <body> leaves out, but none of a <noscript>'s or a <style>'s text.
func TestItemNames(t *testing.T) {
	t.Parallel()

	for item, want := range map[string]string{
		`<tr data-floe-part="broken"><td data-floe-part="x">a</td></tr>`: "broken|x",
		`<div data-floe-part="note&#32;table"></div>`:                    "note table|note table",
		`<noscript><div data-floe-part="note"></div></noscript>`:         "",
		`<svg><style><div data-floe-part="note"></div></style></svg>`:    "",
		`<p>x</p><div data-floe-part="note table"`:                       "",
	} {
		if got := strings.Join(valuesOf(itemNames(item)), "|"); got != want {
			t.Errorf("itemNames(%q) = %q, want %q", item, got, want)
		}
	}
}

// A value's character references are read as a browser reads them.
func TestAttrValue(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]string{"&#110;ote": "note", "a&amp;b": "a&b", "note": "note"} {
		if got := attrValue(value); got != want {
			t.Errorf("attrValue(%q) = %q, want %q", value, got, want)
		}
	}
}
