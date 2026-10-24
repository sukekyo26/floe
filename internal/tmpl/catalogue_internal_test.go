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
	d, err := NewLibrary("", "", "", "").Read(catalogue, src, "", cwd)
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
	written := map[string]map[string]bool{} // path=value of each part's data, by the part's name
	readOccurrences(data, set, written)
	for name, p := range set.parts {
		if p.shape == nil {
			continue
		}
		enums := map[string][]string{}
		enumsOf(p.shape, "", enums, true)
		for path, values := range enums {
			// One may be written short, for the object it is a key of (matrix's cells: [yes], for { mark: yes }).
			parent := path[:max(0, strings.LastIndex(path, "."))]
			for _, v := range values {
				if !written[name][path+"="+v] && !written[name][parent+"="+v] {
					t.Errorf("%s draws %s with no %s: %s", catalogue, name, strings.TrimPrefix(path, "."), v)
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
		case takesTone(p.shape, true):
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

// read reads the box n and the elements it holds but the parts it holds, whose tones are theirs.
func (d *drawnTones) read(n *html.Node) {
	elements := []*html.Node{n}
	var hold func(*html.Node)
	hold = func(p *html.Node) {
		for c := p.FirstChild; c != nil; c = c.NextSibling {
			if nodeAttr(c, "data-floe-part") == "" {
				elements = append(elements, c)
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

// readOccurrences adds to written each value written in the data of each part in v, a data file read, as
// valuesIn tells: a part is an object of one key, its name, in a list of parts.
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
								maps.Copy(written[name], valuesIn(data, ""))
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

// valuesIn returns each string written in v, the data of a part at path, as path=value, path the keys it
// is written at from the part's (rows.cells.mark), but in the lists of the parts it holds.
func valuesIn(v any, path string) map[string]bool {
	found := map[string]bool{}
	switch v := v.(type) {
	case string:
		found[path+"="+v] = true
	case map[string]any:
		for k, w := range v {
			if k != "parts" {
				maps.Copy(found, valuesIn(w, path+"."+k))
			}
		}
	case []any:
		for _, w := range v {
			maps.Copy(found, valuesIn(w, path))
		}
	}
	return found
}

// enumsOf adds to enums the values of each enum under s, a part's shape at path, by its path as valuesIn
// writes it; a part's own, root, but not those of the parts it holds, nor the tones, which a part draws in
// a class of each.
func enumsOf(s *shape, path string, enums map[string][]string, root bool) {
	if s == nil || !root && s.part != nil || strings.HasSuffix(s.ref, "tone.json") {
		return
	}
	for _, v := range s.enum {
		if value, ok := v.(string); ok {
			enums[path] = append(enums[path], value)
		}
	}
	in := s.inside()
	for k, sub := range in.keys {
		enumsOf(sub, path+"."+k, enums, false)
	}
	enumsOf(in.each, path, enums, false)
}

// takesTone reports whether s, a part's shape, takes a tone, root, but not through a part it holds.
func takesTone(s *shape, root bool) bool {
	switch {
	case s == nil || !root && s.part != nil:
		return false
	case strings.HasSuffix(s.ref, "tone.json"):
		return true
	}
	in := s.inside()
	for _, sub := range in.keys {
		if takesTone(sub, false) {
			return true
		}
	}
	return takesTone(in.each, false)
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
