package tmpl

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"golang.org/x/net/html"
)

// catalogue is the page of every part, testdata/parts.yaml, which the browser's test of the colours draws in
// every theme: what it leaves out is not measured.
const catalogue = "testdata/parts.yaml"

// tones are the classes a part draws a tone in, and "" for one left out.
var tones = []string{"accent", "good", "warn", "bad", ""}

// The page of every part draws every part of floe's, a look alone but, each part that takes a tone in every
// tone and in none (an element of the same kind as one drawn in a tone, without one), and every value of
// each enum of a part's schema (findings' level, steps' state) in its data.
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
	var data any
	if err = yaml.Unmarshal(src, &data); err != nil {
		t.Fatal(err)
	}
	// The values written in each part's data, by the part's name: at any key, as a value may be written short
	// (matrix's cells: [yes], for { mark: yes }).
	written := map[string]map[string]bool{}
	readOccurrences(data, set, written)
	for name, p := range set.parts {
		for key, values := range enumsOf(set.docs[partsURL+name+".json"], "") {
			for _, v := range values {
				if p.pages != nil && !written[name][v] {
					t.Errorf("%s draws %s with no %s: %s", catalogue, name, key, v)
				}
			}
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

// read reads the box n and the elements it holds, the boxes of the parts it holds but not what they hold:
// their tones are theirs, but for that of their box (a pill's, which draws a decision's status).
func (d *drawnTones) read(n *html.Node) {
	elements := []*html.Node{n}
	var hold func(*html.Node)
	hold = func(p *html.Node) {
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			elements = append(elements, c)
			if nodeAttr(c, "data-floe-part") == "" {
				hold(c)
			}
		}
	}
	hold(n)
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

// readOccurrences adds to written each value written in the data of each part in v, a data file read, by
// the key it is written at: a part is an object of one key, its name, in a list of parts.
func readOccurrences(v any, set *partSet, written map[string]map[string]bool) {
	switch v := v.(type) {
	case map[string]any:
		for k, w := range v {
			if list, ok := w.([]any); ok && k == "parts" {
				for _, item := range list {
					if one, ok := item.(map[string]any); ok && len(one) == 1 {
						for name, data := range one {
							if set.parts[name] != nil {
								if written[name] == nil {
									written[name] = map[string]bool{}
								}
								maps.Copy(written[name], valuesIn(data))
							}
						}
					}
				}
			}
			readOccurrences(w, set, written)
		}
	case []any:
		for _, w := range v {
			readOccurrences(w, set, written)
		}
	}
}

// valuesIn returns each string written in v, the data of a part, but in the lists of the parts it holds.
func valuesIn(v any) map[string]bool {
	found := map[string]bool{}
	switch v := v.(type) {
	case string:
		found[v] = true
	case map[string]any:
		for k, w := range v {
			if k != "parts" {
				maps.Copy(found, valuesIn(w))
			}
		}
	case []any:
		for _, w := range v {
			maps.Copy(found, valuesIn(w))
		}
	}
	return found
}

// enumsOf returns the values of each enum in doc, a part's schema, by the key it is at, key the one doc
// is at; a $ref, to the tones or to another part, is that schema's.
func enumsOf(doc any, key string) map[string][]string {
	found := map[string][]string{}
	add := func(more map[string][]string) {
		for k, values := range more {
			found[k] = append(found[k], values...)
		}
	}
	switch v := doc.(type) {
	case map[string]any:
		for k, w := range v {
			switch k {
			case "$ref":
			case "enum":
				if values, ok := w.([]any); ok {
					for _, value := range values {
						if s, ok := value.(string); ok {
							found[key] = append(found[key], s)
						}
					}
				}
			case "properties":
				if props, ok := w.(map[string]any); ok {
					for name, prop := range props {
						add(enumsOf(prop, name))
					}
				}
			default:
				add(enumsOf(w, key))
			}
		}
	case []any:
		for _, w := range v {
			add(enumsOf(w, key))
		}
	}
	return found
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
