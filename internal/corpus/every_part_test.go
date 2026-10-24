package corpus_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// everyPart is the sample whose page takes every part floe part list lists.
const everyPart = "every-part"

// scriptStart begins the <script> of the script part, which draws no element of its own (drawScript).
const scriptStart = "{const floeScript = "

// takesEveryPart fails when a part of floe part list is not in the page of the every-part sample, so that a
// part added to floe is added to the sample too and its HTML and CSS are held in golden/. A part is in the
// page when an element of it (data-floe-part) is drawn, or a part drawn takes it as a look; script, when
// its <script> is.
func (s *sample) takesEveryPart(t *testing.T) {
	t.Helper()

	// What the page tells of its parts, the data's HTML could write as well.
	data, err := os.ReadFile(filepath.Join(s.dir, everyPart+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, mark := range []string{"data-floe-", "floeScript"} {
		if bytes.Contains(data, []byte(mark)) {
			t.Errorf("%s.yaml writes %s, which the page cannot tell from what a part draws; leave it out", everyPart, mark)
		}
	}

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
		if n.Type != html.ElementNode {
			continue
		}
		if n.DataAtom == atom.Script && n.FirstChild != nil && strings.HasPrefix(n.FirstChild.Data, scriptStart) {
			taken["script"] = true
		}
		for _, a := range n.Attr {
			if a.Key == "data-floe-part" {
				taken[a.Val] = true
				for _, look := range looks[a.Val] {
					taken[look] = true
				}
			}
		}
	}
	for _, p := range parts {
		if !taken[p.Name] {
			t.Errorf("the every-part sample does not take the part %s: write it in every-part.yaml and run `just golden`", p.Name)
		}
	}
}
