package tmpl_test

import (
	"cmp"
	"os"
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

// NewPart copies a part, floe's or one's own, and the files of the parts themes for it: the copy names
// itself as its box, takes the looks the original takes, and draws as it does; one of the name of floe's
// takes its place, and every page draws as before.
func TestNewPart(t *testing.T) {
	t.Parallel()

	themes := []string{"blueprint", "brutal", "default", "editorial", "ink", "notebook", "swiss", "washi"}
	parts := filepath.Join(t.TempDir(), "parts")
	l := tmpl.NewLibrary("", parts, "")
	made, err := l.NewPart("grid", "table")
	if err != nil {
		t.Fatal(err)
	}
	if made.At != filepath.Join(parts, "grid") || !made.Builtin || made.Replaces || !slices.Equal(made.Themes, themes) ||
		made.ThemesAt != filepath.Join(parts, "themes") {
		t.Errorf("made = %+v", made)
	}
	html, err := os.ReadFile(filepath.Join(parts, "grid", "part.html"))
	if err != nil || !strings.Contains(string(html), `data-floe-part="grid"`) || strings.Contains(string(html), `data-floe-part="table"`) {
		t.Errorf("the copy's part.html = %s, %v", html, err)
	}
	if b, rerr := os.ReadFile(filepath.Join(parts, "grid", "part.yaml")); rerr != nil || string(b) != "looks: [panel]\n" {
		t.Errorf("the copy's part.yaml = %q, %v", b, rerr)
	}
	for _, theme := range themes {
		got, gerr := os.ReadFile(filepath.Join(parts, "themes", theme, "grid.css"))
		want, werr := os.ReadFile(filepath.Join("parts", "themes", theme, "table.css"))
		if gerr != nil || werr != nil || string(got) != string(want) {
			t.Errorf("%s/grid.css = %v, %v: not a copy of table.css", theme, gerr, werr)
		}
	}
	// A copy of one's own, and a copy that takes the place of floe's.
	if made, err = l.NewPart("grid2", "grid"); err != nil || made.Builtin || made.Replaces || !slices.Equal(made.Themes, themes) {
		t.Errorf("a copy of one's own = %+v, %v", made, err)
	}
	memo := memo(t, `"risks": {"$ref": "floe:///parts/terminal.json"}, "aside": {"$ref": "floe:///parts/code.json"}`)
	pages := map[string]string{}
	for _, theme := range themes {
		src := "template: memo\nlang: en\nparts-theme: " + theme + "\nrisks: { steps: [{ run: ls }] }\naside: { text: x }\n"
		if pages[theme], err = draw(tmpl.NewLibrary(memo, "", ""), src); err != nil {
			t.Fatal(err)
		}
	}
	if made, err = l.NewPart("code", "code"); err != nil || !made.Replaces {
		t.Fatalf("a copy in place of floe's = %+v, %v", made, err)
	}
	for _, theme := range themes {
		src := "template: memo\nlang: en\nparts-theme: " + theme + "\nrisks: { steps: [{ run: ls }] }\naside: { text: x }\n"
		if page, err := draw(tmpl.NewLibrary(memo, parts, ""), src); err != nil || page != pages[theme] {
			t.Errorf("%s: the page drawn with the copy of code = %v:\n%s\nnot as with floe's:\n%s", theme, err, page, pages[theme])
		}
	}
	if d, err := l.DescribePart("code"); err != nil || d.Path != filepath.Join(parts, "code") || !slices.Equal(d.Themes, themes) {
		t.Errorf("the copy of code = %+v, %v", d, err)
	}
	if _, broken, err := l.Parts(); err != nil || broken != nil {
		t.Errorf("the parts made = %v, %v", broken, err)
	}
}

// Every part of floe's that has files, a look alone too, copies into a part of one's own of another name
// that can be used: what its files refer to by name, the parts it holds and the looks it takes, reads
// from the directory of parts of one's own as from floe's.
func TestNewPartOfEach(t *testing.T) {
	t.Parallel()

	parts := filepath.Join(t.TempDir(), "parts")
	l := tmpl.NewLibrary("", parts, "")
	floe, _, err := tmpl.NewLibrary("", "", "").Parts()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range floe {
		if d.Name == "diagram" || d.Name == "script" {
			continue
		}
		if _, err := l.NewPart(d.Name+"-copy", d.Name); err != nil {
			t.Errorf("copying %s: %v", d.Name, err)
		}
	}
	if warnings, mistakes := tmpl.CheckOwnParts(parts); warnings != nil || mistakes != nil {
		t.Errorf("CheckOwnParts of the copies = %v, %v", warnings, mistakes)
	}
}

// NewPart refuses a name a part cannot take, one there is already, a part with nothing to copy, and a
// file of a parts theme there already, leaving nothing.
func TestNewPartRefused(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"quote/part.html":           "",
		"themes/washi/details2.css": "",
	})
	l := tmpl.NewLibrary("", parts, "")
	for _, c := range []struct{ name, from, want string }{
		{"Grid", "table", "Grid cannot name a part"},
		{"a/b", "table", "a/b cannot name a part"},
		{"diagram", "table", "cannot be named diagram"},
		{"script", "table", "cannot be named script"},
		{"tone", "table", "cannot be named tone"},
		{"base", "table", "cannot be named base"},
		{"themes", "table", "cannot be named themes"},
		{"grid", "diagram", "floe draws the part diagram in Go"},
		{"grid", "quote", "the part quote cannot be used"},
		{"grid", "nope", `unknown part "nope"`},
		{"quote", "table", "quote is there already"},
		{"details2", "details", "the parts themes have a file of the part details2 already: " + filepath.Join(parts, "themes", "washi", "details2.css")},
	} {
		if _, err := l.NewPart(c.name, c.from); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("NewPart(%s, %s) = %v", c.name, c.from, err)
		}
	}
	// A theme that cannot be written leaves nothing, the files written before it included.
	if err := os.WriteFile(filepath.Join(parts, "themes", "ink"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.NewPart("grid", "table"); err == nil || !strings.Contains(err.Error(), filepath.Join(parts, "themes", "ink", "grid.css")+" cannot be written") {
		t.Errorf("NewPart with a theme that cannot be written = %v", err)
	}
	var left []string
	err := filepath.WalkDir(parts, func(at string, _ os.DirEntry, err error) error {
		rel, rerr := filepath.Rel(parts, at)
		left = append(left, rel)
		return cmp.Or(err, rerr)
	})
	if want := []string{".", "quote", "quote/part.html", "themes", "themes/ink", "themes/washi", "themes/washi/details2.css"}; err != nil ||
		!slices.Equal(left, want) {
		t.Errorf("left = %v, %v", left, err)
	}
	if _, err := tmpl.NewLibrary("", "", "").NewPart("grid", "table"); err == nil || !strings.Contains(err.Error(), "FLOE_PARTS_DIR") {
		t.Errorf("NewPart without a directory of parts = %v", err)
	}
}

// CheckOwnParts refuses every part and parts theme of one's own that cannot be used, and warns of what goes
// into no page; with none of them, or none wrong, it tells nothing.
func TestCheckOwnParts(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{"", filepath.Join(t.TempDir(), "none")} {
		if warnings, mistakes := tmpl.CheckOwnParts(dir); warnings != nil || mistakes != nil {
			t.Errorf("CheckOwnParts(%q) = %v, %v", dir, warnings, mistakes)
		}
	}
	parts := ownParts(t, quote("quote"))
	if _, err := tmpl.NewLibrary("", parts, "").NewPart("grid", "table"); err != nil {
		t.Fatal(err)
	}
	if warnings, mistakes := tmpl.CheckOwnParts(parts); warnings != nil || mistakes != nil {
		t.Errorf("CheckOwnParts of parts that are right = %v, %v", warnings, mistakes)
	}

	files := quote("quote")
	files["unnamed/schema.json"] = `{"type": "object"}`
	files["unnamed/part.html"] = "<p></p>"
	files["unnamed/part.css"] = ""
	files["diagram/part.css"] = ""
	files["themes/mine/note.css"] = "}\n"
	files["themes/default/nope.css"] = ""
	parts = ownParts(t, files)
	warnings, mistakes := tmpl.CheckOwnParts(parts)
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), filepath.Join(parts, "themes", "default", "nope.css")+" goes into no page") {
		t.Errorf("warnings = %v", warnings)
	}
	told := make([]string, 0, len(mistakes))
	for _, m := range mistakes {
		told = append(told, m.Error())
	}
	if len(told) != 3 || !strings.HasPrefix(told[0], "the part diagram cannot be used: "+filepath.Join(parts, "diagram")+": a part of your own cannot be named diagram") ||
		!strings.HasPrefix(told[1], "the part unnamed cannot be used: "+filepath.Join(parts, "unnamed", "part.html")+": no element of the part unnamed is its box") ||
		!strings.HasPrefix(told[2], "your own parts theme mine cannot be used: "+filepath.Join(parts, "themes", "mine", "note.css")+":1:") {
		t.Errorf("mistakes = %q", told)
	}
}
