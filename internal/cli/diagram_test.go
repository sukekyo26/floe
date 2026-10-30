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
		"l.yaml": "# a list\n- a\n",
		"t.yaml": "hello\n",
		"n.yaml": "~\n",
		"w.yaml": "# a spec\nwidth: 100\n",
		"s.yaml": "source: true\nexample: flow\n",
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
	code, _, errs = runIn(dir, nil, "diagram", "render", "b.yaml", "c.yaml", "gone.yaml")
	want := "b.yaml:5:5: additional properties 'boxx' not allowed\n" +
		"floe: there is no directory for icons, as HOME is not an absolute path; set FLOE_ICONS_DIR to an absolute path\n" +
		"gone.yaml: no such file or directory\n"
	if code != 1 || errs != want {
		t.Errorf("render of mistakes = %d, %q, want %q", code, errs, want)
	}
	code, _, errs = runIn(dir, env, "diagram", "render", "b.yaml", "e.yaml")
	if want := "b.yaml:5:5: additional properties 'boxx' not allowed\n" +
		"e.yaml: missing properties 'width', 'height', 'title', 'parts'\n"; code != 1 || errs != want {
		t.Errorf("render of a spec of nothing = %d, %q, want %q", code, errs, want)
	}
	// What the whole spec lacks is told at the file alone, not at its first key.
	code, _, errs = runIn(dir, env, "diagram", "inspect", "w.yaml")
	if want := "w.yaml: missing properties 'height', 'title', 'parts'\n"; code != 1 || errs != want {
		t.Errorf("inspect of a spec lacking keys = %d, %q, want %q", code, errs, want)
	}
	// A top that is not keys and values is refused as goccy/go-yaml refuses it in a data file, at its place.
	code, _, errs = runIn(dir, env, "diagram", "inspect", "l.yaml", "t.yaml", "n.yaml")
	if want := "l.yaml:2:1: sequence was used where mapping is expected\n" +
		"t.yaml:1:1: string was used where mapping is expected\n" +
		"n.yaml:1:1: null was used where mapping is expected\n"; code != 1 || errs != want {
		t.Errorf("inspect of a top not keys and values = %d, %q, want %q", code, errs, want)
	}
	// A diagram shows no spec: source is no key of it, and nothing is drawn.
	code, page, errs := runIn(dir, env, "diagram", "render", "--no-assets", "s.yaml")
	if code != 1 || page != "" || !strings.Contains(errs, "additional properties 'source' not allowed") {
		t.Errorf("render of source = %d, %q, %q", code, page, errs)
	}
	if _, found, _ := runIn(dir, env, "diagram", "inspect", "a.yaml"); found != "box \"k\" 10,10 64x29\n" {
		t.Errorf("inspect = %q", found)
	}
	_, out, _ = runIn(dir, env, "diagram", "example")
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 14 || !strings.HasPrefix(lines[0], "architecture\t") {
		t.Errorf("example = %q, want 14 kinds", out)
	}
	if _, er, _ := runIn(dir, env, "diagram", "example", "er"); !strings.Contains(er, `ja: "ページ（meta.json）"`) {
		t.Errorf("example er = %q, want it with its Japanese", er)
	}
	code, _, errs = runIn(dir, env, "diagram", "example", "pie")
	if want := "floe: no example pie; use architecture, aws, class, er, flow, gantt, git, org, quadrant, sequence, state, swimlane, timeline, venn\n"; code != 1 || errs != want {
		t.Errorf("example pie = %d, %q", code, errs)
	}
}
