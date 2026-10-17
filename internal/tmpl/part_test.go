package tmpl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// memo writes a template of one's own, memo, whose items are drawn by floe's parts, with schema as its
// items and a theme that defines --fg alone.
func memo(t *testing.T, items string) string {
	t.Helper()

	dir := t.TempDir()
	files := map[string]string{
		"schema.json": `{"type": "object", "properties": {` + items + `}, "additionalProperties": false}`,
		"page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title><style>{{.Theme}}</style></head>` +
			`<body>{{.Data.risks}}{{with .Data.aside}}{{.}}{{end}}</body></html>`,
		"themes/default.css": ":root { --fg: #000; }\n",
		"example.yaml":       "template: memo\nlang: en\nrisks: { columns: [{title: Risk}], rows: [[Links]] }\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, "memo", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const memoItems = `"risks": {"$ref": "floe:///parts/table.json"}, "aside": {"$ref": "floe:///parts/note.json", "description": "An aside"}, ` +
	`"asides": {"type": "array", "items": {"$ref": "floe:///parts/note.json"}}`

// A template of one's own draws an item by a part of floe's with a $ref: the part checks it, at the
// data's lines and columns, draws it, and puts its style into the page's <head> once.
func TestPartsInOwnTemplate(t *testing.T) {
	t.Parallel()

	l := tmpl.NewLibrary(memo(t, memoItems), "")
	page, err := draw(l, "template: memo\nlang: en\n"+
		"risks: { columns: [{title: Risk}, {title: Pages, num: true}], rows: [[Links, '120'], [Search, { pill: open, tone: bad }]] }\n"+
		"aside: { text: Keep the old host, tone: warn }\n")
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(page, "</head>")
	// Before the template's own style, which wins over a part's where it says otherwise.
	if parts, own := strings.Index(head, ".floe-table {"), strings.Index(head, ":root { --fg: #000; }"); parts < 0 || own < parts {
		t.Errorf("the parts' style is at %d, the theme's at %d", parts, own)
	}
	for _, want := range []string{
		`<div class="floe-table"><table>`, `<td class="num">120</td>`, `<span class="floe-pill bad">open</span>`,
		`<div class="floe-note warn"><p>Keep the old host</p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page lacks %s:\n%s", want, page)
		}
	}
	for _, style := range []string{".floe-table {", ".floe-note {"} {
		if strings.Count(head, style) != 1 || strings.Count(page, style) != 1 {
			t.Errorf("the page holds %s %d times, %d in its <head>", style, strings.Count(page, style), strings.Count(head, style))
		}
	}
	if strings.Contains(page, ".floe-cards {") {
		t.Error("the page holds the style of a part it does not use")
	}

	for src, want := range map[string]string{
		"risks: { columns: [{title: a}, {title: b}], rows: [[x, y], [x]] }": "data.yaml:3:51: risks.rows: write as many cells in each row as there are columns",
		"risks: { columns: [{title: a}], rows: [[x]], colour: red }":        "data.yaml:3:46: risks.colour is not an item of the template memo (a typo?); use columns, rows",
		"aside: { text: x, tone: grey }":                                    `data.yaml:3:25: aside.tone: "grey" is not allowed; use accent, good, warn, bad`,
	} {
		if _, err := draw(l, "template: memo\nlang: en\n"+src+"\n"); err == nil || err.Error() != want {
			t.Errorf("Read of %s = %v, want %s", src, err, want)
		}
	}
}

// floe template show tells the items of a part, after the description written beside its $ref.
func TestDescribePart(t *testing.T) {
	t.Parallel()

	d, err := tmpl.NewLibrary(memo(t, memoItems), "").Describe("memo")
	if err != nil {
		t.Fatal(err)
	}
	var aside, asides tmpl.Item
	for _, it := range d.Items {
		switch it.Name {
		case "aside":
			aside = it
		case "asides":
			asides = it
		}
	}
	if asides.Part != "note" || asides.Of[0] != "object" {
		t.Errorf("asides = %+v", asides)
	}
	if aside.Part != "note" || !strings.HasPrefix(aside.Description, "An aside. A note set apart") {
		t.Errorf("aside = %+v", aside)
	}
	names := make([]string, 0, len(aside.Items))
	for _, it := range aside.Items {
		names = append(names, it.Name+":"+strings.Join(it.Enum, "|"))
	}
	if strings.Join(names, " ") != "text: tone:accent|good|warn|bad" {
		t.Errorf("the items of aside are %v", names)
	}
}

// floe template check warns of a theme that leaves out a variable the parts its template uses take
// their colours from, and refuses a part floe does not have, as a publish does.
func TestCheckParts(t *testing.T) {
	t.Parallel()

	warnings, mistakes := tmpl.Check(filepath.Join(memo(t, memoItems), "memo"), "")
	if mistakes != nil {
		t.Fatalf("Check = %v", mistakes)
	}
	var told string
	for _, w := range warnings {
		if strings.Contains(w.Error(), "themes/default.css") {
			told = w.Error()
		}
	}
	if !strings.Contains(told, "--accent") || !strings.Contains(told, "--good-soft") || strings.Contains(told, "--fg,") {
		t.Errorf("the warning of the theme = %q", told)
	}

	for _, ref := range []string{"floe:///parts/chart.json", "floe:///parts/table"} {
		dir := memo(t, `"risks": {"$ref": "`+ref+`"}`)
		want := ref + " is not a part of floe's; refer to one as floe:///parts/<name>.json, the name one of cards, note, table"
		if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), want) {
			t.Errorf("Check of %s = %v", ref, mistakes)
		}
		if _, err := draw(tmpl.NewLibrary(dir, ""), "template: memo\nlang: en\n"); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Read of a template with %s = %v", ref, err)
		}
	}
	// One written relative to the schema is the part all the same.
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "../../parts/table.json"}, "aside": {"type": "string"}`), "")
	if page, err := draw(l, "template: memo\nlang: en\nrisks: { columns: [{title: a}], rows: [[x]] }\n"); err != nil ||
		!strings.Contains(page, `<div class="floe-table">`) {
		t.Errorf("a page of a part named relatively = %v", err)
	}
	// What a part's item holds is not looked into: a format beside the $ref would never be drawn.
	dir := memo(t, `"risks": {"$ref": "floe:///parts/table.json", "properties": {"x": {"type": "object", "format": "diagram"}}}`)
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 ||
		!strings.Contains(mistakes[0].Error(), "#/properties/risks/properties/x: the format diagram marks an item to draw") {
		t.Errorf("Check of a format beside a part = %v", mistakes)
	}
	// One into a part would be drawn as plain data.
	ref := "floe:///parts/table.json#/properties/columns"
	dir = memo(t, `"risks": {"$ref": "`+ref+`"}`)
	want := "#/properties/risks: " + ref + " is not a part of floe's"
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), want) {
		t.Errorf("Check of a ref into a part = %v", mistakes)
	}
	// One floe does not follow to would never be drawn.
	dir = memo(t, `"risks": {"anyOf": [{"$ref": "floe:///parts/table.json"}]}`)
	want = "#/properties/risks/anyOf/0: the part table draws an item"
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), want) {
		t.Errorf("Check of a part under anyOf = %v", mistakes)
	}
}
