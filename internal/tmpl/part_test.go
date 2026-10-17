package tmpl_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	if strings.Join(names, " ") != "text: title: tone:accent|good|warn|bad" {
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
			told += w.Error() + "\n"
		}
	}
	if !strings.Contains(told, "--accent") || !strings.Contains(told, "--good-soft") || strings.Contains(told, "--fg,") ||
		!strings.Contains(told, "--muted, which the template's parts and diagrams take from it, so they keep values of their own: light colours, in dark mode too\n") ||
		!strings.Contains(told, "does not define --text-width, --mono, which the template's parts and diagrams take from it, so they keep values of their own\n") {
		t.Errorf("the warning of the theme = %q", told)
	}
	// A theme that leaves out no colour is not told of dark mode.
	colours := memo(t, memoItems)
	theme := ":root { --fg: #000; --surface: #fff; --line: #ccc; --muted: #666; --accent: #00f; --accent-soft: #eef; " +
		"--good: #0a0; --good-soft: #efe; --warn: #a60; --warn-soft: #fed; --bad: #a00; --bad-soft: #fee; }\n"
	if err := os.WriteFile(filepath.Join(colours, "memo", "themes", "default.css"), []byte(theme), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings, _ = tmpl.Check(filepath.Join(colours, "memo"), "")
	for _, w := range warnings {
		if strings.Contains(w.Error(), "themes/default.css") && (strings.Contains(w.Error(), "dark mode") || !strings.Contains(w.Error(), "--mono")) {
			t.Errorf("the warning of a theme of every colour = %q", w.Error())
		}
	}

	for _, ref := range []string{"floe:///parts/chart.json", "floe:///parts/table"} {
		dir := memo(t, `"risks": {"$ref": "`+ref+`"}`)
		want := ref + " is not a part of floe's; refer to one as floe:///parts/<name>.json, the name one of bars, cards, cell, checklist, code, compare, decision, details, diagram, diff, facts, faq, fields, findings, glossary, keys, links, matrix, note, options, progress, risks, script, stack, stats, steps, table, tabs, terminal, timeline, tree"
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
	dir := memo(t, `"risks": {"$ref": "floe:///parts/table.json", "properties": {"x": {"type": "string", "format": "image"}}}`)
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 ||
		!strings.Contains(mistakes[0].Error(), "#/properties/risks/properties/x: the format image marks an item to draw") {
		t.Errorf("Check of a format beside a part = %v", mistakes)
	}
	// One into a part would be drawn as plain data: a publish refuses it too.
	ref := "floe:///parts/table.json#/properties/columns"
	dir = memo(t, `"risks": {"$ref": "`+ref+`"}`)
	want := ref + " points into a part of floe's, which would be checked but not drawn as the part; refer to the whole part, floe:///parts/table.json"
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), "#/properties/risks: "+want) {
		t.Errorf("Check of a ref into a part = %v", mistakes)
	}
	if _, err := draw(tmpl.NewLibrary(dir, ""), "template: memo\nlang: en\n"); err == nil || !strings.Contains(err.Error(), "schema.json: "+want) {
		t.Errorf("Read of a template with a ref into a part = %v", err)
	}
	// Through $defs too, beside the schema's other mistakes.
	dir = memo(t, `"risks": {"$ref": "#/$defs/rows"}, "aside": {"type": "string", "format": "colour"}}, "$defs": {"rows": {"$ref": "`+ref+`"}`)
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 2 ||
		!strings.Contains(mistakes[0].Error()+mistakes[1].Error(), "#/$defs/rows: "+want) {
		t.Errorf("Check of a ref into a part through $defs = %v", mistakes)
	}
	// An empty fragment is the whole part.
	l = tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/table.json#"}, "aside": {"type": "string"}`), "")
	if page, err := draw(l, "template: memo\nlang: en\nrisks: { columns: [{title: a}], rows: [[x]] }\n"); err != nil ||
		!strings.Contains(page, `<div class="floe-table">`) {
		t.Errorf("a page of a part with an empty fragment = %v", err)
	}
	// One floe does not follow to would never be drawn.
	dir = memo(t, `"risks": {"anyOf": [{"$ref": "floe:///parts/table.json"}]}`)
	want = "#/properties/risks/anyOf/0: the part table draws an item"
	if _, mistakes = tmpl.Check(filepath.Join(dir, "memo"), ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), want) {
		t.Errorf("Check of a part under anyOf = %v", mistakes)
	}
}

// A page gets a part's style wherever it uses the part's classes: a table of Markdown takes the table
// part's look, and so does HTML that names its classes; a page that uses none gets no style.
func TestPartStylesByClass(t *testing.T) {
	t.Parallel()

	page := render(t, "template: report\ntheme: soft\nlang: en\nsections:\n  - parts:\n      - text: |\n          | a | b |\n          | - | - |\n          | 1 | 2 |\n")
	if !strings.Contains(page, `<div class="floe-table"><table>`) || strings.Count(page, ".floe-table {") != 1 {
		t.Errorf("a page of a Markdown table:\n%s", page)
	}
	page = render(t, "template: report\ntheme: soft\nlang: en\nsections:\n  - parts:\n      - html: <span class=\"floe-pill good\">ok</span>\n")
	if strings.Count(page, ".floe-pill {") != 1 || strings.Contains(page, ".floe-note {") {
		t.Errorf("a page of a pill in HTML:\n%s", page)
	}
	page = render(t, "template: report\ntheme: soft\nlang: en\nsections:\n  - parts:\n      - html: <p class=\"floe-note-own\">x</p>\n")
	if strings.Contains(page, ".floe-note {") {
		t.Errorf("a class a part's starts gets the part's style:\n%s", page)
	}
	page = render(t, "template: report\ntheme: soft\nlang: en\nsections:\n  - parts:\n      - text: the `floe-note` class\n")
	if strings.Contains(page, ".floe-note {") {
		t.Errorf("a class named in prose gets the part's style:\n%s", page)
	}
	if page = render(t, "template: report\ntheme: soft\nlang: en\nsections:\n  - parts:\n      - text: plain\n"); strings.Contains(page, ".floe-") {
		t.Errorf("a page of no part holds a part's style:\n%s", page)
	}
}

// Diagrams, scripts and Markdown are floe's to draw in a template of one's own too: a diagram and a
// script as parts, Markdown as a format; floe template show tells the parts.
func TestGoPartsAndMarkdown(t *testing.T) {
	t.Parallel()

	dir := memo(t, `"risks": {"$ref": "floe:///parts/diagram.json"}, "aside": {"type": "string", "format": "markdown"}`)
	l := tmpl.NewLibrary(dir, "")
	page, err := draw(l, "template: memo\nlang: en\naside: '**bold**'\n"+
		"risks: { id: d, width: 100, height: 60, title: T, parts: [{ box: { x: 10, y: 10, w: 80, h: 40, rows: [A] } }] }\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`aria-label="T"`, "<p><strong>bold</strong></p>", "diagram_zoom_fit"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page lacks %s:\n%s", want, page)
		}
	}
	d, err := l.Describe("memo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = draw(tmpl.NewLibrary(memo(t, `"plans": {"type": "array", "items": {"type": "object", "format": "script"}}`), ""),
		"template: memo\nlang: en\nplans: [{js: 'function () {}'}]\n")
	if err == nil || !strings.Contains(err.Error(), `#/properties/plans/items: a script is a part of floe's, not a format; write "$ref": "floe:///parts/script.json"`) {
		t.Errorf("a script written as a format = %v", err)
	}
	i := slices.IndexFunc(d.Items, func(it tmpl.Item) bool { return it.Name == "risks" })
	if i < 0 || d.Items[i].Part != "diagram" || !strings.HasPrefix(d.Items[i].Description, "A diagram: a spec") {
		t.Errorf("the items = %+v", d.Items)
	}
}

// Code goes in as it is, under its file, else its language; a terminal draws its prompt by the shell's
// class, with a command's later lines and its output apart.
func TestCodeAndTerminal(t *testing.T) {
	t.Parallel()

	page := render(t, "template: report\nlang: en\nsections:\n  - parts:\n"+
		"      - code: { file: a.go, lang: go, text: \"if a < b {\\n\\treturn\\n}\\n\" }\n"+
		"      - code: { lang: sh, text: x }\n"+
		"      - code: { text: y }\n"+
		"      - terminal: { steps: [{ run: \"make \\\\\\n  all\", out: done }, { run: ls }] }\n"+
		"      - terminal: { shell: powershell, steps: [{ run: dir }] }\n"+
		"      - terminal: { shell: zsh, steps: [{ run: ls }] }\n")
	for _, want := range []string{
		"<figure class=\"floe-code\"><figcaption><span class=\"file\">a.go</span><span class=\"lang\">go</span></figcaption><pre><code>if a &lt; b {\n\treturn\n}\n</code></pre></figure>",
		`<figure class="floe-code"><figcaption><span class="lang">sh</span></figcaption><pre><code>x</code></pre></figure>`,
		`<figure class="floe-code"><pre><code>y</code></pre></figure>`,
		"<figure class=\"floe-terminal sh\"><pre><span class=\"run\">make \\\n  all</span><span class=\"out\">done</span><span class=\"run\">ls</span></pre></figure>",
		`<figure class="floe-terminal powershell"><figcaption>PowerShell</figcaption><pre><span class="run">dir</span></pre></figure>`,
		`<figure class="floe-terminal zsh"><figcaption>zsh</figcaption><pre><span class="run">ls</span></pre></figure>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page lacks %s:\n%s", want, page)
		}
	}
	if strings.Count(page, ".floe-code {") != 1 || strings.Count(page, ".floe-terminal {") != 1 {
		t.Errorf("the parts' styles, once each:\n%s", page)
	}

	_, err := builtins.Read("bad.yaml", []byte("template: report\nlang: en\nsections:\n  - parts:\n"+
		"      - code: { lang: go }\n"+
		"      - terminal: { shell: fish, steps: [] }\n"), "", "")
	for _, want := range []string{
		"bad.yaml:5:17: sections[0].parts[0].code.text is required but missing",
		`bad.yaml:6:28: sections[0].parts[1].terminal.shell: "fish" is not allowed; use bash, zsh, sh, powershell, cmd`,
		"bad.yaml:6:41: sections[0].parts[1].terminal.steps: the number of items must be at least 1",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("the mistakes = %v, want %s", err, want)
		}
	}
	// A command written with its prompt would show two, and copy one: a first word ending in a prompt's mark is
	// one, after an environment's name too, while a comment, a redirect and $ further on are not.
	runs := []string{
		"$ ls", "% make", "❯ make", `PS C:\Program Files> dir`, `C:\work> dir`, "user@host:~/作業$ make",
		"root@host:~# apt update", "[me@host app]$ make", "(venv) $ pip install x", "me@mac% make",
		"> app.log", ">> app.log", "2> err.log", "&> all.log", "(cd web && make) >> build.log", "# a comment", "echo $ x",
		"%EDITOR% notes.txt", `C:\Python312\python.exe a.py > out.txt`, "cat > a.html <<EOF\n<p> hi</p>\n$ x\nEOF",
	}
	steps := make([]string, 0, len(runs))
	for _, r := range runs {
		steps = append(steps, fmt.Sprintf("{ run: %q }", r))
	}
	_, err = builtins.Read("bad.yaml", []byte("template: report\nlang: en\nsections:\n  - parts:\n"+
		"      - terminal: { steps: ["+strings.Join(steps, ", ")+"] }\n"), "", "")
	for i, r := range runs {
		want := fmt.Sprintf("terminal.steps[%d].run: write the command without its prompt", i)
		if refused := err != nil && strings.Contains(err.Error(), want); refused != (i < 10) {
			t.Errorf("run %q refused = %v: %v", r, refused, err)
		}
	}
}

// report sends code and commands to their parts by refusing a fenced block of code in text, in a list or
// a quote too, while prose, a quote and short code in a sentence pass, prompts or not.
func TestReportRefusesCodeInText(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"```go\\nx\\n```", "~~~\\nx\\n~~~", "```sh\\n$ ls\\n```", "> ```sh\\n> x\\n> ```", "- ```sh\\n  x",
	} {
		_, err := builtins.Read("t.yaml", []byte("template: report\nlang: en\nsections:\n  - parts:\n      - text: \"a\\n"+text+"\"\n"), "", "")
		if err == nil || strings.Count(err.Error(), "t.yaml:5:15: sections[0].parts[0].text: write ") != 1 {
			t.Errorf("text %q = %v", text, err)
		}
	}
	render(t, "template: report\nlang: en\nsections:\n  - parts:\n      - text: \"Run `floe list`.\\n\\n> a quote\\n\\n"+
		"$ 5 a month.\\n\\n100% done.\\n\\n`$ ls` lists.\"\n")
}

// The parts drawn in another's look name its classes and bring its style; a list part checks each of its
// items, and a tree's paths nest.
func TestPartsOfNumbersAndChanges(t *testing.T) {
	t.Parallel()

	page := render(t, "template: report\ntheme: soft\nlang: en\nsections:\n  - parts:\n"+
		"      - stats: [{ label: Pages, value: '18', unit: MB, note: '+5', tone: good }]\n"+
		"      - progress: [{ label: Copy, done: 5, of: 8, tone: warn }]\n"+
		"      - steps: [{ title: A, state: done }, { title: B, text: '**now**', state: doing }, { title: C }]\n"+
		"      - checklist: [{ text: '`ci`', done: true }, { text: b }]\n"+
		"      - compare: { before: Now, after: Next, rows: [{ what: a, before: x, after: y }, { what: b, before: z, after: z }] }\n"+
		"      - options: [{ title: A, pick: Take, pros: [p], cons: [c] }]\n"+
		"      - facts: [{ name: State, value: { pill: open, tone: bad } }, { name: Due, value: '`soon`' }]\n"+
		"      - diff: { file: a.go, text: \"@@ -1 +1 @@\\n-a\\n+b\\n\" }\n"+
		"      - tree: [{ path: a/b.go, note: B, change: added }]\n"+
		"      - links: [{ title: PR, url: 'https://example.com/1', text: merged }]\n")
	for _, want := range []string{
		`<div class="floe-card floe-stat good"><dt>Pages</dt><dd class="value">18<span class="unit">MB</span></dd><dd class="note">&#43;5</dd></div>`,
		`<label class="floe-bar warn"><span class="label">Copy</span><span class="count">5 / 8</span><progress value="5" max="8">5 / 8</progress></label>`,
		`<li class="done"><strong>A</strong></li>`,
		`<li class="doing" aria-current="step"><strong>B</strong><p><strong>now</strong></p>`,
		`<li class="todo"><strong>C</strong></li>`,
		`<li class="done"><code>ci</code></li>`,
		`<tr><td class="what">a</td><td>x</td><td class="after">y</td></tr>`,
		`<tr class="same"><td class="what">b</td>`,
		`<div class="floe-card floe-option pick"><h3>A <span class="floe-pill">Take</span></h3><ul><li class="pro">p</li><li class="con">c</li></ul></div>`,
		`<dt>State</dt><dd><span class="floe-pill bad">open</span></dd>`,
		`<dt>Due</dt><dd><code>soon</code></dd>`,
		`<span class="file">a.go</span><span class="added">+1</span><span class="removed">−1</span></figcaption><pre><code><span class="hunk">@@ -1 &#43;1 @@` + "\n" + `</span><span class="del">-a` + "\n",
		`<li><span class="name dir">a/</span><ul>` + "\n" + `<li class="added"><span class="name">b.go</span><span class="note">B</span></li>`,
		`<li class="floe-card"><a href="https://example.com/1">PR</a><p>merged</p></li>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page lacks %s:\n%s", want, page)
		}
	}
	// Each style once: those of the parts drawn, and of the table, the cards and code they are drawn in the look of.
	for _, style := range []string{".floe-stats {", ".floe-progress {", ".floe-steps {", ".floe-table {", ".floe-cards {", ".floe-code {", ".floe-diff pre {", ".floe-tree {", ".floe-pill {"} {
		if strings.Count(page, style) != 1 {
			t.Errorf("the page has %s %d times", style, strings.Count(page, style))
		}
	}

	_, err := builtins.Read("bad.yaml", []byte("template: report\nlang: en\nsections:\n  - parts:\n"+
		"      - progress: [{ label: a, done: 1, of: 2 }, { label: b, done: 3, of: 2 }]\n"+
		"      - tree: [{ path: a }, { path: // }]\n"), "", "")
	for _, want := range []string{
		"bad.yaml:5:68: sections[0].parts[0].progress[1].done: write done no greater than of, the whole",
		"bad.yaml:6:37: sections[0].parts[1].tree[1].path: write a path naming a file or a folder",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("the mistakes = %v, want %s", err, want)
		}
	}
}

// The parts of reviews and references draw their words in the page's language, and tabs and details
// draw the parts they hold.
func TestPartsOfReviewsAndReferences(t *testing.T) {
	t.Parallel()

	src := "template: report\ntheme: soft\nlang: %s\nsections:\n  - parts:\n" +
		"      - findings: [{ level: must, at: 'a.go:1', text: '`x` leaks', why: later, status: { pill: fixed, tone: good } }, { level: nit, text: y }]\n" +
		"      - details: { title: Log, parts: [{ terminal: { steps: [{ run: make }] } }, { text: '**done**' }] }\n" +
		"      - matrix: { columns: [linux, mac], rows: [{ what: serve, cells: [yes, { mark: part, note: some }] }] }\n" +
		"      - decision: { title: Take B, status: { pill: decided }, date: '2026-10-12', text: Why, rejected: [A] }\n" +
		"      - risks: [{ what: r, likely: high, impact: mid, fix: f }, { what: s, likely: low, impact: mid }]\n" +
		"      - fields: [{ name: --open, type: boolean, required: true, default: false, text: Opens }, { name: --x }]\n" +
		"      - timeline: [{ date: '10-02', title: T, text: x, tone: good }]\n" +
		"      - bars: [{ label: a, value: 1 }, { label: b, value: 4, tone: warn }]\n" +
		"      - faq: [{ q: Why?, a: '[See](https://example.com)', open: true }]\n" +
		"      - glossary: [{ term: part, text: A piece }]\n" +
		"      - keys: [{ keys: [Ctrl, K], text: Find }]\n" +
		"      - tabs: [{ title: bash, parts: [{ code: { text: ls } }] }, { title: cmd, parts: [{ note: { text: n } }] }]\n"
	page := render(t, fmt.Sprintf(src, "en"))
	for _, want := range []string{
		`<tr><td class="level"><span class="floe-pill bad">must</span></td><td><code class="at">a.go:1</code><div><code>x</code> leaks</div><div class="why">later</div></td><td class="status"><span class="floe-pill good">fixed</span></td></tr>`,
		`<td class="level"><span class="floe-pill nit">nit</span></td><td><div>y</div></td><td class="status"></td>`,
		`<details class="floe-details"><summary>Log</summary><div class="floe-stack"><figure class="floe-terminal sh"><pre><span class="run">make</span></pre></figure><p><strong>done</strong></p>` + "\n</div></details>",
		`<td class="yes"><span class="mark"><span>Yes</span></span></td><td class="part"><span class="mark"><span>Partly</span></span><small>some</small></td>`,
		`<p class="meta"><span class="floe-pill">decided</span><span>2026-10-12</span></p><h3>Take B</h3><p>Why</p>` + "\n" + `<p class="set-aside">Set aside</p><ul><li>A</li></ul>`,
		`<th scope="col">Risk</th><th scope="col">Likelihood</th><th scope="col">Impact</th><th scope="col">What we do</th>`,
		`<tr class="bad"><td class="what">r</td><td class="high"><span class="level">High</span></td><td class="mid"><span class="level">Medium</span></td><td>f</td></tr>`,
		`<tr class="good"><td class="what">s</td>`,
		`<td class="name"><code>--open</code><span class="required">required</span><span class="type">boolean</span></td><td>Opens<span class="default">Default: <code>false</code></span></td>`,
		`<td class="name"><code>--x</code></td><td></td>`,
		`<li class="good"><span class="date">10-02</span><div><strong>T</strong><p>x</p>`,
		`<div><span class="label">a</span><span class="track"><span style="width: 25%"></span></span><span class="value">1</span></div>`,
		`<div class="warn"><span class="label">b</span><span class="track"><span style="width: 100%"></span></span><span class="value">4</span></div>`,
		`<details open><summary>Why?</summary><div><p><a href="https://example.com">See</a></p>`,
		`<div><dt>part</dt><dd><p>A piece</p>`,
		`<div><dt><kbd>Ctrl</kbd><span>+</span><kbd>K</kbd></dt><dd>Find</dd></div>`,
		`<label><input type="radio" name="tab" checked>bash</label><div class="floe-stack"><figure class="floe-code"><pre><code>ls</code></pre></figure></div>`,
		`<label><input type="radio" name="tab">cmd</label><div class="floe-stack"><div class="floe-note"><p>n</p>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page lacks %s:\n%s", want, page)
		}
	}
	// The parts held bring their style too.
	for _, style := range []string{".floe-terminal {", ".floe-code {", ".floe-note {", ".floe-tabs {"} {
		if strings.Count(page, style) != 1 {
			t.Errorf("the page has %s %d times", style, strings.Count(page, style))
		}
	}
	page = render(t, fmt.Sprintf(src, "ja-JP"))
	for _, want := range []string{`<span>一部</span>`, `<th scope="col">起きやすさ</th>`, `<span class="level">高</span>`, `<span class="required">必須</span>`, `<p class="set-aside">退けた案</p>`} {
		if !strings.Contains(page, want) {
			t.Errorf("the Japanese page lacks %s", want)
		}
	}

	_, err := builtins.Read("bad.yaml", []byte("template: report\nlang: en\nsections:\n  - parts:\n"+
		"      - matrix: { columns: [a, b], rows: [{ what: x, cells: [yes] }, { what: y, cells: [yes, maybe] }] }\n"+
		"      - tabs: [{ title: a, parts: [{ terminal: { steps: [{ run: '$ ls' }] } }] }, { title: b, parts: [{ text: x }] }]\n"), "", "")
	for _, want := range []string{
		"sections[0].parts[1].tabs[0].parts[0].terminal.steps[0].run: write the command without its prompt",
		"sections[0].parts[0].matrix.rows[0].cells: write as many cells in each row as there are columns",
		"sections[0].parts[0].matrix.rows[1].cells: write each cell yes, part or no, or { mark, note }",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("the mistakes = %v, want %s", err, want)
		}
	}
}
