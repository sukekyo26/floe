package tmpl

import (
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// catalogue is the page of every part, testdata/parts.yaml, which the browser's test of the colours draws in
// every theme: what it leaves out is not measured.
const catalogue = "testdata/parts.yaml"

// tones are the classes a part draws a tone in, and "" for one left out.
var tones = []string{"accent", "good", "warn", "bad", ""}

// The page of every part draws every part of floe's, a look alone but, and each part that takes a tone in
// every tone and in none: an element of the same kind as one drawn in a tone, without one.
func TestCatalogueDrawsEveryPart(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile(catalogue)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewLibrary("", "", "", "").Read(catalogue, src, "", "")
	if err != nil {
		t.Fatal(err)
	}
	page, err := d.Render("t")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(string(page)))
	if err != nil {
		t.Fatal(err)
	}
	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	drawn := map[string]*drawnTones{} // by the part's name
	for n := range doc.Descendants() {
		if name := nodeAttr(n, "data-floe-part"); name != "" {
			if drawn[name] == nil {
				drawn[name] = &drawnTones{found: map[string]bool{}, toned: map[string]bool{}, plain: map[string]bool{}}
			}
			drawn[name].read(n)
		}
	}
	for name, p := range set.parts {
		switch {
		case p.isLook(), p.css == "":
			// A look draws nothing of its own, and a part of no style (script) no colour.
		case drawn[name] == nil:
			t.Errorf("%s draws no %s", catalogue, name)
		case takesTone(set.docs[partsURL+name+".json"]):
			for _, tone := range tones {
				if !drawn[name].has(tone) {
					t.Errorf("%s draws %s in no tone %q", catalogue, name, tone)
				}
			}
		}
	}
}

// drawnTones are the tones a part's boxes are drawn in: the classes of tones, and the kinds of element
// (a tag, a part and the other classes) drawn in a tone and in none.
type drawnTones struct {
	found, toned, plain map[string]bool
}

// read reads the box n and the elements it holds.
func (d *drawnTones) read(n *html.Node) {
	elements := []*html.Node{n}
	for e := range n.Descendants() {
		elements = append(elements, e)
	}
	for _, e := range elements {
		if e.Type != html.ElementNode {
			continue
		}
		kind := []string{e.Data, nodeAttr(e, "data-floe-part")}
		tone := ""
		for _, c := range strings.Fields(nodeAttr(e, "class")) {
			if slices.Contains(tones, c) {
				tone = c
			} else {
				kind = append(kind, c)
			}
		}
		k := strings.Join(kind, " ")
		if tone != "" {
			d.found[tone], d.toned[k] = true, true
		} else {
			d.plain[k] = true
		}
	}
}

// has reports whether the part is drawn in tone, "" for none: an element of a kind drawn in a tone
// without one.
func (d *drawnTones) has(tone string) bool {
	if tone != "" {
		return d.found[tone]
	}
	for k := range d.toned {
		if d.plain[k] {
			return true
		}
	}
	return false
}

// takesTone reports whether doc, a part's schema, refers to the schema of a tone, in it or under it.
func takesTone(doc any) bool {
	switch v := doc.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok && strings.HasSuffix(ref, "tone.json") {
			return true
		}
		for _, w := range v {
			if takesTone(w) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(v, takesTone)
	}
	return false
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
