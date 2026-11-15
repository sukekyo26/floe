package kit

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"floe/internal/doc"
	"floe/internal/usererr"
)

// The tables of names in the docs for agents name what floe holds, each in its first column and no other: the keys
// every data file shares, the files of a part and of a template, the values of marks:, which the table of part.yaml
// names in its row of marks: too; and the widths a spec's width and height take.
// LIMIT: a table's first column alone is held, and the second where said: what the other columns say of a file
// (required, In a look) is not. Hold them if a doc's column comes to go wrong.
func TestAgentDocLists(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		file, heading string
		holds         []string
	}{
		{"guide.md", "The data file", doc.Shared},
		{"parts.md", "Files", partFiles},
		{"templates.md", "Files", templateFiles},
		{"parts.md", "Rows with a mark", markKinds},
	} {
		got := slices.Sorted(slices.Values(docFirstColumn(t, tt.file, tt.heading)))
		if want := slices.Sorted(slices.Values(tt.holds)); !slices.Equal(got, want) {
			t.Errorf("the table of %s in docs/agents/%s names %v, floe holds %v: write a row for each, and no other", tt.heading, tt.file, got, want)
		}
	}

	marks := slices.IndexFunc(docTables(t, "parts.md")["part.yaml"], func(r docKeyRow) bool { return slices.Equal(r.keys, []string{"marks:"}) })
	if marks < 0 {
		t.Error("docs/agents/parts.md has no row of marks: in the table of part.yaml")
	} else if got := docTables(t, "parts.md")["part.yaml"][marks].code; !slices.Equal(got, markKinds) {
		t.Errorf("docs/agents/parts.md, part.yaml: marks: takes %v in the table, floe %v", got, markKinds)
	}

	takes := "a number, " + canvasRange(t)
	rows := docTables(t, "diagrams.md")["The spec"]
	for _, key := range []string{"width", "height"} {
		i := slices.IndexFunc(rows, func(r docKeyRow) bool { return slices.Equal(r.keys, []string{key}) })
		switch {
		case i < 0:
			t.Errorf("docs/agents/diagrams.md has no row of %s in the table of The spec", key)
		case rows[i].takes != takes:
			t.Errorf("docs/agents/diagrams.md, \"The spec\": %s takes %q, the schema %q", key, rows[i].takes, takes)
		}
	}
}

// canvasRange returns the range of a canvas's width and height in the diagram's schema, as the doc of diagrams writes
// it: from its minimum to its maximum, the digits grouped.
func canvasRange(t *testing.T) string {
	t.Helper()
	var schema struct {
		Defs struct {
			Canvas struct{ Minimum, Maximum float64 } `json:"canvas"`
		} `json:"$defs"`
	}
	schemaOf(t, "floe/parts/diagram/schema.json", &schema)
	return numOf(t, schema.Defs.Canvas.Minimum) + " to " + numOf(t, schema.Defs.Canvas.Maximum)
}

// numOf returns args as page.html's num writes them.
func numOf(t *testing.T, args ...any) string {
	t.Helper()
	s, err := formatNum(args...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// The names the tables of themes.md's files and of part.yaml's keys give are ones floe reads: a theme of the files the
// table names, each empty, reads without a mistake, and a part.yaml of each key the table names is not refused for its
// key (its value may be).
// LIMIT: one way alone, as floe holds no list of either (a switch reads them): a file or key floe comes to read that
// the doc does not name passes. Hold both ways if floe comes to keep them in a list.
func TestAgentDocReadNames(t *testing.T) {
	t.Parallel()

	base, err := Floe()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range [][2]string{{"themes.md", "Files"}, {"parts.md", "part.yaml"}} {
		for _, row := range docTables(t, table[0])[table[1]] {
			if len(row.keys) == 0 {
				t.Errorf("docs/agents/%s: a row of the table of %s names nothing in code in its first column, which this reads", table[0], table[1])
			}
		}
	}
	theme := fstest.MapFS{}
	for _, name := range docFirstColumn(t, "themes.md", "Files") {
		if !strings.Contains(name, "<") {
			theme["mine/"+name] = &fstest.MapFile{Data: nil}
		}
	}
	if len(theme) == 0 {
		t.Error("docs/agents/themes.md names no file of a theme in the table of Files")
	}
	if _, err := Load(base, Sources{Themes: Dir{FS: theme, Name: "themes"}}); err != nil {
		t.Errorf("a theme of the files docs/agents/themes.md names is refused: %s", told(err))
	}

	for _, key := range docFirstColumn(t, "parts.md", "part.yaml") {
		key = strings.TrimSuffix(key, ":")
		part := fstest.MapFS{
			"mine/schema.json": {Data: []byte(`{"type": "object"}`)},
			"mine/part.html":   {Data: []byte(`<div data-floe-part="mine"></div>`)},
			"mine/part.css":    {Data: nil},
			"mine/part.yaml":   {Data: []byte(key + ": x\n")},
		}
		_, err := Load(base, Sources{Parts: Dir{FS: part, Name: "parts"}})
		var all usererr.List
		if errors.As(err, &all) && slices.ContainsFunc(all, func(e *usererr.Error) bool { return e.Key == "kit_part_yaml_key" }) {
			t.Errorf("docs/agents/parts.md names %s: in the table of part.yaml, which floe refuses: %s", key, told(err))
		}
	}
}
