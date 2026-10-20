package tmpl_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// Parts tells of floe's parts and those of one's own by name, one in place of floe's of its name once,
// and the mistakes of those that cannot be used after; DescribePart tells what a part takes, as its
// schema's root does, the looks it takes in the order they go into a page, the parts it holds and the
// parts themes with a file of it.
func TestDescribeParts(t *testing.T) {
	t.Parallel()

	files := quote("quote")
	for name, body := range quote("table") {
		files[name] = body
	}
	files["themes/default/table.css"] = ":scope { margin: 0; }\n"
	files["frame/part.css"] = ":scope { border: 1px solid; }\n"
	files["frame/part.yaml"] = "description: |\n  A frame\nlooks: [panel]\n"
	files["listed/part.css"] = ""
	files["listed/part.yaml"] = "description: [a, frame]\n"
	files["bad/part.css"] = ""
	files["bad/part.yaml"] = "description: Not a look's\n"
	files["bad/schema.json"] = `{"type": "object"}`
	files["bad/part.html"] = `<p data-floe-part="bad"></p>`
	dir := ownParts(t, files)
	l := tmpl.NewLibrary("", dir, "")

	parts, broken, err := l.Parts()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(parts))
	for _, d := range parts {
		names = append(names, d.Name)
	}
	// Sorted, a name once, so table once.
	if !slices.IsSorted(names) || len(slices.Compact(slices.Clone(names))) != len(names) || names[0] != "bars" ||
		slices.Contains(names, "tone") || slices.Contains(names, "bad") || !slices.Contains(names, "panel") ||
		!slices.Contains(names, "diagram") || !slices.Contains(names, "quote") {
		t.Errorf("parts = %v", names)
	}
	if len(broken) != 2 || !strings.Contains(broken[0].Error(), "the part bad cannot be used") ||
		!strings.Contains(broken[0].Error(), "part.yaml:1:1: a part's description is its schema.json's") ||
		!strings.Contains(broken[1].Error(), "part.yaml:1:14: write description: as text") {
		t.Errorf("broken = %v", broken)
	}

	for _, c := range []struct {
		l         *tmpl.Library
		name      string
		path      string
		replaces  bool
		look      bool
		about     string
		types, of []string
		items     []string
		looks     []string
		holds     []string
		themes    []string
	}{
		{l: l, name: "quote", path: filepath.Join(dir, "quote"), types: []string{"object"}, items: []string{"source", "text"}},
		{
			l: l, name: "table", path: filepath.Join(dir, "table"), replaces: true, types: []string{"object"},
			items: []string{"source", "text"}, themes: []string{"default"},
		},
		{l: l, name: "frame", path: filepath.Join(dir, "frame"), look: true, about: "A frame\n", looks: []string{"panel"}},
		{
			// floe's table takes the panel's look; the table of one's own in l takes none.
			l: tmpl.NewLibrary("", "", ""), name: "risks", types: []string{"array"}, of: []string{"object"}, items: []string{"fix", "impact", "likely", "what"},
			looks: []string{"panel", "table"},
		},
		{
			l: tmpl.NewLibrary("", "", ""), name: "table", types: []string{"object"}, items: []string{"columns", "rows"},
			looks: []string{"panel"}, holds: []string{"cell"},
			themes: []string{"blueprint", "brutal", "default", "editorial", "ink", "notebook", "swiss", "washi"},
		},
		{
			l: l, name: "risks", types: []string{"array"}, of: []string{"object"}, items: []string{"fix", "impact", "likely", "what"},
			looks: []string{"table"},
		},
		{l: l, name: "cell", types: []string{"string", "object"}, items: []string{"pill", "tone"}, themes: []string{"blueprint", "brutal", "default", "editorial", "ink", "notebook", "swiss", "washi"}},
		{l: l, name: "details", types: []string{"object"}, items: []string{"open", "parts", "title"}, looks: []string{"panel"}, holds: []string{"stack"}, themes: []string{"washi"}},
		{l: l, name: "rows", look: true, about: "Rows in a panel", looks: []string{"panel"}, themes: []string{"default"}},
		{l: l, name: "diagram", about: "A diagram", types: []string{"object"}},
	} {
		d, err := c.l.DescribePart(c.name)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		items := make([]string, 0, len(d.Takes.Items))
		for _, it := range d.Takes.Items {
			items = append(items, it.Name)
		}
		if d.Name != c.name || d.Path != c.path || d.Replaces != c.replaces || d.Look != c.look ||
			!strings.HasPrefix(d.About, c.about) || !slices.Equal(d.Takes.Types, c.types) || !slices.Equal(d.Takes.Of, c.of) ||
			!slices.Equal(items, c.items) || !slices.Equal(d.Looks, c.looks) || !slices.Equal(d.Holds, c.holds) ||
			!slices.Equal(d.Themes, c.themes) {
			t.Errorf("%s = %+v, items %v", c.name, d, items)
		}
	}
	for name, want := range map[string]string{"bad": "the part bad cannot be used", "nope": `unknown part "nope"; use bars, `} {
		if _, err := l.DescribePart(name); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("describing %s = %v", name, err)
		}
	}
}
