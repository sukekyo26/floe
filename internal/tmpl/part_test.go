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

const memoItems = `"risks": {"$ref": "floe:///parts/table.json"}, "aside": {"$ref": "floe:///parts/note.json", "description": "An aside."}`

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
	var aside tmpl.Item
	for _, it := range d.Items {
		if it.Name == "aside" {
			aside = it
		}
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

	dir := memo(t, `"risks": {"$ref": "floe:///parts/chart.json"}`)
	want := `floe has no part "chart"; use cards, note, table`
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), want) {
		t.Errorf("Check of an unknown part = %v", mistakes)
	}
	if _, err := draw(tmpl.NewLibrary(dir, ""), "template: memo\nlang: en\n"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("Read of a template with an unknown part = %v", err)
	}
	// One floe does not follow to would never be drawn.
	dir = memo(t, `"risks": {"anyOf": [{"$ref": "floe:///parts/table.json"}]}`)
	want = "#/properties/risks/anyOf/0: the part table draws an item"
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), want) {
		t.Errorf("Check of a part under anyOf = %v", mistakes)
	}
}
