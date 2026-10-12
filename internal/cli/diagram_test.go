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

	// Every spec is read: a mistake in one's shape, the layout of another and a reused id all show.
	e.write(t, "specs/shape.yaml", "id: c\n"+head+"  - dot: { cx: 1 }\n")
	e.write(t, "specs/layout.yaml", "id: a\n"+head+"  - dot: { cx: 0, cy: 25 }\n")
	want(t, "render with mistakes", e.run(t, "specs", "diagram", "render", "a.yaml", "shape.yaml", "layout.yaml"), 1, "",
		"floe: shape.yaml:6:5: dot: give cy\n"+
			"layout.yaml:1:1: diagram id \"a\" is already used in a.yaml:1; give each diagram its own id\n"+
			"layout.yaml:6:5: dot at 0,25 sticks out of the 100x50 canvas\n")
	want(t, "render a missing file", e.run(t, "specs", "diagram", "render", "a.yaml", "nope.yaml"), 1, "",
		"floe: cannot read nope.yaml (no such file or directory)")
	want(t, "render nothing", e.run(t, "", "diagram", "render"), 2, "", "floe: requires at least 1 arg(s)")
}

func TestDiagramExample(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	r := e.run(t, "", "diagram", "example")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "architecture\tArchitecture: ") || strings.Count(r.stdout, "\n") != 9 {
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
	want(t, "an unknown example", e.run(t, "", "diagram", "example", "pie"), 2, "",
		`floe: unknown example "pie"; use architecture, class, er, flow, git, sequence, state, timeline, venn`)
}
