package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// floe diagram render, inspect and example: specs checked and drawn in the language asked for, every mistake of
// every spec at its line and column.
func TestDiagram(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for name, src := range map[string]string{
		"a.yaml": "width: 200\nheight: 100\ntitle: { en: A, ja: あ }\nparts:\n  - box: { name: k, x: 10, y: 10, rows: [{ en: Yes, ja: はい }] }\n",
		"b.yaml": "width: 200\nheight: 100\ntitle: B\nparts:\n  - boxx: {}\n",
		"c.yaml": "width: 200\nheight: 100\ntitle: C\nparts:\n  - icon: { icon: aws/aws-lambda, x: 10, y: 10 }\n",
		"e.yaml": "# nothing yet\n",
		"s.yaml": "\ufeff# shown\r\nsource: true\r\nexample: flow\r\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	env := map[string]string{"HOME": "/h"}
	code, out, errs := runIn(dir, env, "diagram", "render", "--no-assets", "--lang", "ja", "a.yaml")
	if code != 0 || errs != "" || !strings.Contains(out, ">はい</text>") || strings.Contains(out, "<script>") {
		t.Errorf("render --lang ja = %d, %q, %q", code, out, errs)
	}
	if _, page, _ := runIn(dir, env, "diagram", "render", "a.yaml"); !strings.HasPrefix(page, "<script>") || !strings.Contains(page, ">Yes</text>") {
		t.Errorf("render = %q, want the assets first, and the English", page)
	}
	// As a page draws it: each diagram in the part's box, its style scoped to the box.
	if !strings.HasPrefix(out, `<div data-floe-part="diagram"><div class="dg-wrap"`) {
		t.Errorf("render --no-assets = %q, want the diagram in the box of the part diagram", out)
	}
	if _, page, _ := runIn(dir, env, "diagram", "render", "a.yaml"); !strings.Contains(page, "<style>\n@scope ([data-floe-part=\"diagram\"])") {
		t.Errorf("render = %q, want the style scoped to the box of the part diagram", page)
	}
	code, _, errs = runIn(dir, nil, "diagram", "render", "b.yaml", "c.yaml", "gone.yaml")
	want := "b.yaml:5:5: additional properties 'boxx' not allowed\n" +
		"floe: there is no directory for icons, as HOME is not an absolute path; set FLOE_ICONS_DIR to an absolute path\n" +
		"gone.yaml: no such file or directory\n"
	if code != 1 || errs != want {
		t.Errorf("render of mistakes = %d, %q, want %q", code, errs, want)
	}
	code, _, errs = runIn(dir, env, "diagram", "render", "b.yaml", "e.yaml")
	if want := "b.yaml:5:5: additional properties 'boxx' not allowed\n" +
		"e.yaml:1:1: missing properties 'width', 'height', 'title', 'parts'\n"; code != 1 || errs != want {
		t.Errorf("render of a spec of nothing = %d, %q, want %q", code, errs, want)
	}
	// An example shows its file; the spec's own text is without its BOM and CRs.
	if _, page, _ := runIn(dir, env, "diagram", "render", "--no-assets", "s.yaml"); !strings.Contains(page, "<pre><code># Decision flow:") {
		t.Errorf("render of an example with its source = %q", page)
	}
	if err := os.WriteFile(filepath.Join(dir, "s.yaml"), []byte("\ufeff# shown\r\nsource: true\r\nwidth: 9\r\nheight: 9\r\ntitle: t\r\nparts: []\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, page, _ := runIn(dir, env, "diagram", "render", "--no-assets", "s.yaml"); !strings.Contains(page, "<pre><code># shown\nsource: true\nwidth: 9") {
		t.Errorf("render with its source = %q", page)
	}
	if _, found, _ := runIn(dir, env, "diagram", "inspect", "a.yaml"); found != "box \"k\" 10,10 64x29\n" {
		t.Errorf("inspect = %q", found)
	}
	_, out, _ = runIn(dir, env, "diagram", "example")
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 10 || !strings.HasPrefix(lines[0], "architecture\t") {
		t.Errorf("example = %q, want 10 kinds", out)
	}
	if _, er, _ := runIn(dir, env, "diagram", "example", "er"); !strings.Contains(er, `ja: "ページ（meta.json）"`) {
		t.Errorf("example er = %q, want it with its Japanese", er)
	}
	code, _, errs = runIn(dir, env, "diagram", "example", "pie")
	if want := "floe: no example pie; use architecture, aws, class, er, flow, git, sequence, state, timeline, venn\n"; code != 1 || errs != want {
		t.Errorf("example pie = %d, %q", code, errs)
	}
}
