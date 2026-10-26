package tmpl_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/draw"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// floeKit returns floe's own parts, themes and templates.
func floeKit(t *testing.T) *kit.Kit {
	t.Helper()
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// told words err as the command line does.
func told(err error) string {
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

// Each data file of testdata/develop is drawn byte for byte as the .html beside it, with the CSP floe render
// puts in: the parts, the themes, the languages and Markdown of report, without diagrams, scripts and items of
// format html. The .html were written from what floe draws, after it was checked once that develop 24c377c's
// floe render drew the same pages, both read with x/net/html and written anew (but for the spaces between the
// elements of the head, which the CSP's place changes).
func TestDrawAsDevelop(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	files, err := filepath.Glob(filepath.Join("testdata", "develop", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no data files: %v", err)
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		page, err := tmpl.Draw(k, filepath.Base(file), src, tmpl.Options{Theme: "", Untitled: ""})
		if err != nil {
			t.Errorf("%s: %s", file, told(err))
			continue
		}
		want, err := os.ReadFile(strings.TrimSuffix(file, ".yaml") + ".html")
		if err != nil {
			t.Fatal(err)
		}
		if got := draw.CutOffPage(page); !bytes.Equal(got, want) {
			t.Errorf("%s: drawn otherwise than develop: %s", file, firstDiff(got, want))
		}
	}
}

// firstDiff tells the first line got and want differ at.
func firstDiff(got, want []byte) string {
	g, w := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range min(len(g), len(w)) {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d:\n got  %.300s\n want %.300s", i+1, g[i], w[i])
		}
	}
	return fmt.Sprintf("%d lines, want %d", len(g), len(w))
}
