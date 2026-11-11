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
		"o.yaml": "title: O\nparts:\n  - box: { name: a, x: 10, y: 10, rows: [A] }\n  - box: { name: b, x: 12, y: 12, rows: [B] }\n",
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
	// What Inspect tells of a diagram the checks refuse does not come out here: nothing is drawn until all are fixed.
	code, page0, errs := runIn(dir, env, "diagram", "render", "--no-assets", "a.yaml", "o.yaml")
	if want := "o.yaml:4:3: box \"a\" overlaps box \"b\"\n"; code != 1 || page0 != "" || errs != want {
		t.Errorf("render of a right spec and a wrong one = %d, %q, %q, want %q", code, page0, errs, want)
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
		"e.yaml: missing properties 'title', 'parts'\n"; code != 1 || errs != want {
		t.Errorf("render of a spec of nothing = %d, %q, want %q", code, errs, want)
	}
	// What the whole spec lacks is told at the file alone, not at its first key.
	code, _, errs = runIn(dir, env, "diagram", "inspect", "w.yaml")
	if want := "w.yaml: missing properties 'title', 'parts'\n"; code != 1 || errs != want {
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
	if _, found, _ := runIn(dir, env, "diagram", "inspect", "a.yaml"); found != "a.yaml: diagram \"あ\" 200x100\nbox \"k\" 10,10 64x29\n" {
		t.Errorf("inspect = %q", found)
	}
	_, out, _ = runIn(dir, env, "diagram", "example")
	if lines := strings.Split(strings.TrimSpace(out), "\n"); len(lines) != 15 || !strings.HasPrefix(lines[0], "architecture\t") {
		t.Errorf("example = %q, want 15 kinds", out)
	}
	if _, er, _ := runIn(dir, env, "diagram", "example", "er"); !strings.Contains(er, `ja: 'ページ（meta.json）'`) {
		t.Errorf("example er = %q, want it with its Japanese", er)
	}
	code, _, errs = runIn(dir, env, "diagram", "example", "pie")
	if want := "floe: no example pie; use architecture, aws-graph, aws, class, er, flow, gantt, git, org, quadrant, sequence, state, swimlane, timeline, venn\n"; code != 1 || errs != want {
		t.Errorf("example pie = %d, %q", code, errs)
	}
}

// floe diagram inspect of data files: by the key template, their diagrams in the order written, in details and tabs
// too, each under where it is written; what the checks refuse printed with its mistakes, at the lines floe check
// tells; and what gets no layout printing no line.
func TestInspectData(t *testing.T) {
	t.Parallel()

	const box = "{ title: %s, parts: [ { box: { name: a, x: 10, y: 10, rows: [A] } } ] }"
	diagramOf := func(title string) string { return strings.Replace(box, "%s", title, 1) }
	const report = "template: report\nlang: en\ntitle: P\nsections:\n  - heading: H\n    parts:\n"
	dir := write(t, map[string]string{
		"page.yaml": report + "      - diagram: " + diagramOf("First") + "\n      - details:\n          title: d\n          parts:\n            - diagram: " + diagramOf("Folded") + "\n      - tabs:\n          - title: x\n            parts:\n" +
			"              - diagram: { example: flow }\n          - title: y\n            parts:\n              - text: t\n",
		"keys.yaml": "template: mine\nlang: en\ntimeline: " + diagramOf("Written first") + "\nmore: " + diagramOf("Written last") + "\n",
		"keysbad.yaml": "template: mine\nlang: en\ntimeline:\n  title: T\n  parts:\n    - box: { name: a, x: 10, y: 10, rows: [A] }\n    - box: { name: b, x: 12, y: 12, rows: [B] }\n" +
			"more:\n  title: M\n  parts:\n    - box: { name: a, x: 10, y: 10, rows: [A] }\n    - box: { name: b, x: 12, y: 12, rows: [B] }\n",
		"over.yaml": "title: O\nparts:\n  - box: { name: a, x: 10, y: 10, rows: [A] }\n  - box: { name: b, x: 12, y: 12, rows: [B] }\n",
		"overpage.yaml": report + "      - diagram:\n          title: O\n          parts:\n            - box: { name: a, x: 10, y: 10, rows: [A] }\n" +
			"            - box: { name: b, x: 12, y: 12, rows: [B] }\n",
		"whole.yaml":    report + "      - diagram: { title: W, parts: [ { icon: { icon: aws/aws-lambda, x: 10, y: 10 } } ] }\n",
		"icons.yaml":    report + "      - diagram: { example: aws }\n",
		"line.yaml":     "title: L\nwidth: 100\nheight: 100\nparts:\n  - line: { x1: 10, y1: 10, x2: 90, y2: 10 }\n",
		"empty.yaml":    "title: E\nwidth: 100\nheight: 100\nparts: []\n",
		"shape.yaml":    "title: S\nparts:\n  - boxx: {}\n",
		"example.yaml":  report + "      - diagram: " + diagramOf("Told") + "\n      - diagram: { example: nothing }\n",
		"good.yaml":     "title: G\nparts:\n  - box: { name: k, x: 10, y: 10, rows: [K] }\n",
		"goodpage.yaml": report + "      - diagram: " + diagramOf("Right") + "\n",
		"none.yaml":     report + "      - text: t\n",
		"other.yaml":    report + "      - textt: t\n      - diagram: " + diagramOf("Unseen") + "\n",
		"typo.yaml":     "templat: report\nlang: en\n",
		"lang.yaml":     "lang: en\ntitle: P\nsections: []\n",
		"tspec.yaml":    "template: report\ntitle: T\nparts: []\n",
		"unknown.yaml":  "template: nothing\nlang: en\n",
		"refused.yaml":  "template: refusing\nlang: en\ntimeline: " + diagramOf("Passes") + "\n",

		"templates/mine/page.html":        "<!doctype html><html><head><title>{{.Title}}</title></head><body></body></html>",
		"templates/mine/schema.json":      diagramsSchema,
		"templates/mine/example.yaml":     "template: mine\nlang: en\n",
		"templates/refusing/page.html":    "<!doctype html><html><head><title>{{.Title}}</title></head><body></body></html>",
		"templates/refusing/schema.json":  diagramsSchema,
		"templates/refusing/example.yaml": "template: refusing\nlang: en\n",
		"templates/refusing/checks.yaml":  "- expr: false\n  message: refused by the template's checks\n",
		"broken/parts/bad/schema.json":    "{",
	})
	env := map[string]string{"HOME": "/h", "FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
	const noExample = "no example nothing; use architecture, aws-graph, aws, class, er, flow, gantt, git, org, quadrant, sequence, state, swimlane, timeline, venn"
	for _, c := range []struct {
		name string
		args []string
		code int
		out  string
		errs string
	}{
		{
			"keys written apart, in the order written",
			[]string{"keys.yaml"},
			0,
			"keys.yaml:3:1: diagram \"Written first\" 78x59\nbox \"a\" 10,10 48x29\nkeys.yaml:4:1: diagram \"Written last\" 78x59\nbox \"a\" 10,10 48x29\n", "",
		},
		{
			"their mistakes Sorted",
			[]string{"keysbad.yaml"},
			1,
			"keysbad.yaml:3:1: diagram \"T\" 80x61\nbox \"a\" 10,10 48x29\nbox \"b\" 12,12 48x29\nkeysbad.yaml:8:1: diagram \"M\" 80x61\nbox \"a\" 10,10 48x29\nbox \"b\" 12,12 48x29\n",
			"keysbad.yaml:7:5: box \"a\" overlaps box \"b\"\nkeysbad.yaml:12:5: box \"a\" overlaps box \"b\"\n",
		},
		{
			"a spec the checks refuse",
			[]string{"over.yaml"},
			1,
			"over.yaml: diagram \"O\" 80x61\nbox \"a\" 10,10 48x29\nbox \"b\" 12,12 48x29\n", "over.yaml:4:3: box \"a\" overlaps box \"b\"\n",
		},
		{
			"a page's diagram the checks refuse",
			[]string{"overpage.yaml"},
			1,
			"overpage.yaml:7:9: diagram \"O\" 80x61\nbox \"a\" 10,10 48x29\nbox \"b\" 12,12 48x29\n", "overpage.yaml:11:13: box \"a\" overlaps box \"b\"\n",
		},
		{"a shape gets no layout", []string{"shape.yaml"}, 1, "", "shape.yaml:3:5: additional properties 'boxx' not allowed\n"},
		{
			"one of two gets no layout",
			[]string{"example.yaml"},
			1,
			"example.yaml:7:9: diagram \"Told\" 78x59\nbox \"a\" 10,10 48x29\n", "example.yaml:8:29: " + noExample + "\n",
		},
		{
			"three files, each in its turn",
			[]string{"over.yaml", "goodpage.yaml", "good.yaml"},
			1,
			"over.yaml: diagram \"O\" 80x61\nbox \"a\" 10,10 48x29\nbox \"b\" 12,12 48x29\ngoodpage.yaml:7:9: diagram \"Right\" 78x59\nbox \"a\" 10,10 48x29\ngood.yaml: diagram \"G\" 78x59\nbox \"k\" 10,10 48x29\n",
			"over.yaml:4:3: box \"a\" overlaps box \"b\"\n",
		},
		{"a page of no diagram", []string{"none.yaml"}, 0, "", ""},
		{"a mistake off the diagrams stops it", []string{"other.yaml"}, 1, "", "other.yaml:7:16: additional properties 'textt' not allowed\n"},
		{
			"a key misspelt reads as a spec",
			[]string{"typo.yaml"},
			1, "",
			"typo.yaml: additional properties 'lang', 'templat' not allowed\ntypo.yaml: missing properties 'title', 'parts'\n",
		},
		{
			"a page lacking the key reads as a spec",
			[]string{"lang.yaml"},
			1, "",
			"lang.yaml: additional properties 'lang', 'sections' not allowed\nlang.yaml: missing property 'parts'\n",
		},
		{
			"a spec with the key reads as a page",
			[]string{"tspec.yaml"},
			1, "",
			"tspec.yaml: name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)\ntspec.yaml:3:1: additional properties 'parts' not allowed\n",
		},
		{
			"a template there is none of",
			[]string{"unknown.yaml"},
			1, "",
			"unknown.yaml:1:11: unknown template \"nothing\"; use adr, design, handoff, mine, postmortem, refusing, report, runbook, slides\n",
		},
	} {
		code, out, errs := runIn(dir, env, append([]string{"diagram", "inspect"}, c.args...)...)
		if code != c.code || out != c.out || errs != c.errs {
			t.Errorf("%s = %d\n%s%s\nwant %d\n%s%s", c.name, code, out, errs, c.code, c.out, c.errs)
		}
	}
	// In details and tabs, and an example: the lines of the diagrams, in the order written.
	code, out, errs := runIn(dir, env, "diagram", "inspect", "page.yaml")
	var heads []string
	for line := range strings.Lines(out) {
		if strings.Contains(line, ": diagram ") {
			heads = append(heads, line)
		}
	}
	want := "page.yaml:7:9: diagram \"First\" 78x59\n" + "page.yaml:11:15: diagram \"Folded\" 78x59\n" +
		"page.yaml:15:17: diagram \"What floe publish checks\" 1072x901\n"
	if got := strings.Join(heads, ""); code != 0 || errs != "" || got != want {
		t.Errorf("inspect of a page = %d, %q, its diagrams\n%swant\n%s", code, errs, got, want)
	}
	// A diagram of nothing listed is its line alone, no empty line after it.
	code, out, errs = runIn(dir, env, "diagram", "inspect", "line.yaml", "empty.yaml", "good.yaml")
	want = "line.yaml: diagram \"L\" 100x100\nempty.yaml: diagram \"E\" 100x100\ngood.yaml: diagram \"G\" 78x59\nbox \"k\" 10,10 48x29\n"
	if code != 0 || errs != "" || out != want {
		t.Errorf("inspect of diagrams of nothing listed = %d, %q, %q, want %q", code, out, errs, want)
	}
	// Where a refusal is told is where floe check tells it.
	for _, file := range []string{"overpage.yaml", "whole.yaml", "example.yaml", "icons.yaml"} {
		_, _, inspected := runIn(dir, env, "diagram", "inspect", file)
		_, _, checked := runIn(dir, env, "check", file)
		if inspected == "" || inspected != checked {
			t.Errorf("%s: inspect tells %q, check %q", file, inspected, checked)
		}
	}
	// The checks and the computed values do not run: the page floe check refuses has its diagrams.
	code, out, errs = runIn(dir, env, "diagram", "inspect", "refused.yaml")
	if want := "refused.yaml:3:1: diagram \"Passes\" 78x59\nbox \"a\" 10,10 48x29\n"; code != 0 || errs != "" || out != want {
		t.Errorf("inspect of a page its checks refuse = %d, %q, %q", code, out, errs)
	}
	if code, _, errs := runIn(dir, env, "check", "refused.yaml"); code != 1 || !strings.Contains(errs, "refused by the template's checks") {
		t.Errorf("check of a page its checks refuse = %d, %q", code, errs)
	}
	// Parts of one's own are read for a data file alone: a broken one stops no spec.
	broken := map[string]string{"HOME": "/h", "FLOE_PARTS_DIR": filepath.Join(dir, "broken", "parts")}
	if code, out, errs := runIn(dir, broken, "diagram", "inspect", "good.yaml"); code != 0 || errs != "" || out == "" {
		t.Errorf("inspect of a spec with a part of one's own broken = %d, %q, %q", code, out, errs)
	}
	if code, out, errs := runIn(dir, broken, "diagram", "inspect", "goodpage.yaml"); code != 1 || out != "" || errs == "" {
		t.Errorf("inspect of a page with a part of one's own broken = %d, %q, %q", code, out, errs)
	}
}

// diagramsSchema is a template's schema with a diagram at two keys, of which the one written first comes later by name.
const diagramsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "timeline": { "$ref": "floe:///parts/diagram.json" },
    "more": { "$ref": "floe:///parts/diagram.json" }
  },
  "additionalProperties": false
}`
