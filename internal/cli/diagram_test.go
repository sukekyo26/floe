package cli_test

import (
	"strings"
	"testing"
)

func TestDiagramRender(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	const head = "width: 100\nheight: 50\ntitle: t\nparts:\n"
	e.write(t, "specs/a.yaml", "id: a\n"+head+"  - box: { x: 10, y: 10, w: 40, h: 30, rows: [A] }\n")
	e.write(t, "specs/b.yaml", "id: b\n"+head+"  - dot: { cx: 50, cy: 25 }\n")
	r := e.run(t, "specs", "diagram", "render", "a.yaml", "b.yaml")
	if r.code != 0 || r.stderr != "" || strings.Count(r.stdout, `<div class="dg-wrap"`) != 2 ||
		!strings.Contains(r.stdout, `aria-label="t"`) || !strings.HasSuffix(r.stdout, "</svg></div>\n") {
		t.Fatalf("render = %+v, want two diagrams", r)
	}
	if !strings.HasPrefix(r.stdout, "<script>(") || strings.Count(r.stdout, "<style>") != 1 {
		t.Errorf("render = %q, want the assets once before the diagrams", r.stdout)
	}
	r = e.run(t, "specs", "diagram", "render", "--no-assets", "a.yaml")
	if r.code != 0 || !strings.HasPrefix(r.stdout, `<div class="dg-wrap"`) || strings.Contains(r.stdout, "<script>") {
		t.Errorf("render --no-assets = %+v, want the diagram alone", r)
	}

	// Every spec is read: a mistake in one's shape, the layout of another and a reused id all show.
	e.write(t, "specs/shape.yaml", "id: c\n"+head+"  - dot: { cx: 1 }\n")
	e.write(t, "specs/layout.yaml", "id: a\n"+head+"  - dot: { cx: 0, cy: 25 }\n")
	want(t, "render with mistakes", e.run(t, "specs", "diagram", "render", "a.yaml", "shape.yaml", "layout.yaml"), 1, "",
		"shape.yaml:6:5: dot: give cy\n"+
			"layout.yaml:1:1: diagram id \"a\" is already used in a.yaml:1; give each diagram its own id\n"+
			"layout.yaml:6:5: dot at 0,25 sticks out of the 100x50 canvas\n")
	want(t, "mistakes in the order of the files", e.run(t, "specs", "diagram", "render", "layout.yaml", "shape.yaml"), 1, "",
		"layout.yaml:6:5: dot at 0,25 sticks out of the 100x50 canvas\n"+
			"shape.yaml:6:5: dot: give cy\n")
	// A mistake about the whole of the diagrams, icons not fetched, is told beside those of the specs.
	e.write(t, "specs/icon.yaml", "id: i\n"+head+"  - icon: { x: 10, y: 10, name: aws/aws-lambda }\n")
	want(t, "render with icons not fetched", e.run(t, "specs", "diagram", "render", "icon.yaml", "shape.yaml"), 1, "",
		"shape.yaml:6:5: dot: give cy\nfloe: AWS icons are not in ")
	want(t, "render a missing file", e.run(t, "specs", "diagram", "render", "a.yaml", "nope.yaml"), 1, "",
		"floe: cannot read nope.yaml (no such file or directory)")
	want(t, "render nothing", e.run(t, "", "diagram", "render"), 2, "", "floe: requires at least 1 arg(s)")
}

func TestDiagramExample(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	r := e.run(t, "", "diagram", "example")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "architecture\tArchitecture: ") || strings.Count(r.stdout, "\n") != 10 {
		t.Errorf("example = %+v, want the examples with their summaries", r)
	}
	r = e.run(t, "", "diagram", "example", "venn")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "# Venn diagram: ") || !strings.Contains(r.stdout, "\nid: venn\n") {
		t.Errorf("example venn = %+v, want its spec", r)
	}
	e.write(t, "venn.yaml", r.stdout)
	if r := e.run(t, "", "diagram", "render", "venn.yaml"); r.code != 0 {
		t.Errorf("render of the venn example = %+v", r)
	}
	// In English whatever language the environment names; --lang picks another.
	e.vars["LANG"] = "ja_JP.UTF-8"
	if r := e.run(t, "", "diagram", "example"); r.code != 0 || !strings.HasPrefix(r.stdout, "architecture\tArchitecture: ") {
		t.Errorf("example with LANG=ja = %+v, want the English summaries", r)
	}
	if r := e.run(t, "", "diagram", "example", "venn", "--lang", "en"); r.code != 0 || !strings.HasPrefix(r.stdout, "# Venn diagram: ") {
		t.Errorf("example venn --lang en = %+v, want the English spec", r)
	}
	delete(e.vars, "LANG")
	if r := e.run(t, "", "diagram", "example", "venn", "--lang", "ja"); r.code != 0 || !strings.HasPrefix(r.stdout, "# ベン図: ") {
		t.Errorf("example venn --lang ja = %+v, want the Japanese spec", r)
	}
	if r := e.run(t, "", "diagram", "example", "venn", "--lang", "pt-BR"); r.code != 0 || !strings.HasPrefix(r.stdout, "# Venn diagram: ") {
		t.Errorf("example venn --lang pt-BR = %+v, want the English spec", r)
	}
	want(t, "an invalid language", e.run(t, "", "diagram", "example", "--lang", "ja_JP"), 2, "",
		`floe: invalid language "ja_JP": give --lang a BCP 47 tag such as en, ja or pt-BR`)
	want(t, "an unknown example", e.run(t, "", "diagram", "example", "pie"), 2, "",
		`floe: unknown example "pie"; use architecture, aws, class, er, flow, git, sequence, state, timeline, venn`)
}
