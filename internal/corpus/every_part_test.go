package corpus_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// everyPart is the sample whose page takes every part floe part list lists.
const everyPart = "every-part"

// takesEveryPart fails when a part of floe part list is not in the page of the every-part sample, so that a
// part added to floe is added to the sample too and its HTML and CSS are held in golden/. A part is in the
// page when an element of it (data-floe-part) is drawn, or a part drawn takes it as a look; script, which
// draws no element of its own, when every-part.yaml has an item of it.
func (s *sample) takesEveryPart(t *testing.T) {
	t.Helper()

	// The parts as floe part list reads them, of the sample's directories.
	parts, broken, err := tmpl.NewLibrary(s.vars["FLOE_TEMPLATES_DIR"], s.vars["FLOE_PARTS_DIR"],
		s.vars["FLOE_THEMES_DIR"], s.vars["FLOE_ICONS_DIR"]).Run().Parts()
	if err != nil || len(broken) > 0 {
		t.Fatalf("read the parts: %v %v", err, broken)
	}
	looks := map[string][]string{}
	for _, p := range parts {
		looks[p.Name] = p.Looks
	}

	doc, err := html.Parse(bytes.NewReader(s.files[everyPart+".html"]))
	if err != nil {
		t.Fatal(err)
	}
	taken := map[string]bool{}
	for n := range doc.Descendants() {
		for _, a := range n.Attr {
			if n.Type == html.ElementNode && a.Key == "data-floe-part" {
				taken[a.Val] = true
				for _, look := range looks[a.Val] {
					taken[look] = true
				}
			}
		}
	}
	taken["script"] = s.itemsOf(t, everyPart+".yaml")["script"]
	for _, p := range parts {
		if !taken[p.Name] {
			t.Errorf("the every-part sample does not take the part %s: write it in every-part.yaml and run `just golden`", p.Name)
		}
	}
}

// itemsOf returns the names of the items of the data file, those of a list each one key of a mapping. It
// fails the test when an html item writes data-floe-, which takesEveryPart could not tell from a part's.
func (s *sample) itemsOf(t *testing.T, file string) map[string]bool {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(s.dir, file))
	if err != nil {
		t.Fatal(err)
	}
	var data any
	if err := yaml.Unmarshal(b, &data); err != nil {
		t.Fatal(err)
	}
	items := map[string]bool{}
	var walk func(v any, inList bool)
	walk = func(v any, inList bool) {
		switch v := v.(type) {
		case []any:
			for _, e := range v {
				walk(e, true)
			}
		case map[string]any:
			for k, e := range v {
				if inList && len(v) == 1 {
					items[k] = true
					if text, ok := e.(string); ok && k == "html" && strings.Contains(text, "data-floe-") {
						t.Errorf("%s: an html item writes data-floe-, which the page cannot tell from a part's: %s", file, text)
					}
				}
				walk(e, false)
			}
		}
	}
	walk(data, false)
	return items
}
