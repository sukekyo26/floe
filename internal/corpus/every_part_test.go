package corpus_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/cli"
)

// TestEveryPart fails when a part of floe part list is not in the page of the every-part sample: an element
// it draws (data-floe-part) or one that takes it as a look (data-floe-look). A part added to floe is added to
// the sample too, so that its HTML and CSS are held in golden/.
func TestEveryPart(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	vars := map[string]string{
		"HOME": filepath.Join(tmp, "home"), "FLOE_DIR": filepath.Join(tmp, "store"),
		"FLOE_PARTS_DIR": filepath.Join(tmp, "none", "parts"), "FLOE_THEMES_DIR": filepath.Join(tmp, "none", "themes"),
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run(t.Context(), []string{"part", "list"}, cli.Env{
		Getenv: func(k string) string { return vars[k] }, Dir: tmp, Stdout: &stdout, Stderr: &stderr, Environ: nil,
	}); code != 0 {
		t.Fatalf("floe part list: exit %d\n%s", code, stderr.String())
	}

	page, err := os.ReadFile(filepath.Join("testdata", "every-part", "golden", "every-part.html"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	// script draws an empty element and its <script>, neither marked: every-part.yaml takes it once.
	drawn := map[string]bool{"script": true}
	for n := range doc.Descendants() {
		for _, a := range n.Attr {
			if n.Type == html.ElementNode && (a.Key == "data-floe-part" || a.Key == "data-floe-look") {
				for _, name := range strings.Fields(a.Val) {
					drawn[name] = true
				}
			}
		}
	}
	// The columns of floe part list are for scripts to read: the name, where it is and its description.
	for line := range strings.Lines(stdout.String()) {
		if name, _, _ := strings.Cut(line, "\t"); !drawn[name] {
			t.Errorf("the every-part sample does not take the part %s: write it in every-part.yaml and run `just golden`", name)
		}
	}
}
