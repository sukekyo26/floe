package browsertest_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// diagramKit is a part of one's own, pic, that draws its value as a diagram in its box, and a template that
// places a diagram of floe's part and a pic.
var diagramKit = map[string]string{
	"parts/pic/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
	"parts/pic/part.html":   `<div data-floe-part="pic">{{diagram}}</div>`,
	"parts/pic/part.css":    "",
	"templates/mine/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{.Data.plain}}\n{{.Data.pic}}\n</body></html>\n",
	"templates/mine/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"plain": {"$ref": "floe:///parts/diagram.json"}, "pic": {"$ref": "floe:///parts/pic.json"}}}`,
	"templates/mine/example.yaml": "lang: en\ntemplate: mine\n",
}

const diagramPage = `lang: en
template: mine
title: Diagrams
plain: { example: flow }
pic: { width: 300, height: 100, title: p, parts: [ { box: { x: 20, y: 20, w: 120, h: 50, rows: [pic] } } ] }
`

// measureDiagrams returns a line for each diagram whose toolbar is not 0.4rem above its frame, or whose frame and
// toolbar part.css does not reach: they are found from the SVG, not by the classes Go gives them, which part.css
// writes by hand.
const measureDiagrams = `(() => {
  const out = [];
  const want = 0.4 * parseFloat(getComputedStyle(document.documentElement).fontSize);
  for (const svg of document.querySelectorAll('svg.dg')) {
    const wrap = svg.parentElement;
    const box = wrap.closest('[data-floe-part="diagram"]');
    const name = box.parentElement.closest('[data-floe-part]')?.dataset.floePart ?? 'page';
    const tools = wrap.previousElementSibling;
    if (getComputedStyle(wrap).overflow !== 'hidden' || getComputedStyle(tools).display !== 'flex') {
      out.push(name + ': part.css does not reach the frame (' + wrap.className + ') or the toolbar (' + tools.className + ')');
    }
    const gap = wrap.getBoundingClientRect().top - tools.getBoundingClientRect().bottom;
    if (Math.abs(gap - want) > 0.5) {
      out.push(name + ': toolbar ' + gap.toFixed(1) + ' px above the frame, want ' + want.toFixed(1));
    }
  }
  return out.join('\n');
})()`

// TestUIDiagram draws a diagram of a template and one in a part of one's own, wide and as wide as a phone: each
// has its toolbar a little above its frame, not over it, and part.css reaches both.
func TestUIDiagram(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.own(t, diagramKit)
	s.floe(t, "publish", s.write(t, "diagrams.yaml", diagramPage), "--slug", "diagrams")
	for _, size := range []struct {
		width, height int64
		mobile        bool
	}{{1440, 900, false}, {390, 844, true}} {
		b.size(size.width, size.height, size.mobile)
		b.navigate("/a/diagrams/?raw")
		b.until("the toolbars", "[...document.querySelectorAll('svg.dg')].filter((s) => s.parentElement.previousElementSibling).length === 2")
		if got := b.eval(measureDiagrams); got != "" {
			t.Errorf("at %d px:\n  %s", size.width, strings.ReplaceAll(got, "\n", "\n  "))
		}
	}
}

const tabsPage = `template: report
lang: en
title: Tabs
sections:
  - heading: Tabs
    parts:
      - tabs:
          - title: Table
            parts:
              - table: { columns: [A], rows: [[a]] }
          - title: Diagram
            parts:
              - diagram: { width: 600, height: 200, title: d, parts: [ { box: { x: 20, y: 40, w: 160, h: 60, rows: [one] } } ] }
      - details:
          title: Folded
          parts:
            - diagram: { width: 600, height: 200, title: e, parts: [ { box: { x: 20, y: 40, w: 160, h: 60, rows: [two] } } ] }
`

// folded tells the diagram in the part named (tabs or details): its viewBox, its frame's size, and whether it is fitted
// (the button that fits it disabled) and zoomed.
func folded(part string) string {
	return `(() => {
  const w = document.querySelector('[data-floe-part=` + part + `] .dg-wrap');
  const fit = w.previousElementSibling.querySelector('[data-zoom=fit]');
  return w.querySelector('svg').getAttribute('viewBox') + ' ' + (w.clientWidth > 0 && w.clientHeight > 0 ? 'sized' : 'unsized') +
    (fit.disabled ? ' fitted' : '') + (w.classList.contains('zoomed') ? ' zoomed' : '');
})()`
}

// frames waits for two frames, after which a ResizeObserver has told what changed before it.
const frames = "new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(1))))"

// TestUIDiagramInTabs draws a diagram in a tab not chosen: hidden, it keeps the viewBox written; printed, it shows
// whole though its tab is not chosen; its tab chosen, it fits its frame. A diagram in details fits its frame when
// the details open.
func TestUIDiagramInTabs(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "tabs.yaml", tabsPage), "--slug", "tabs")
	b.navigate("/a/tabs/?raw")
	b.until("the toolbars", "document.querySelectorAll('.dg-tools').length === 2")
	if got := b.eval(folded("tabs")); got != "0 0 600 200 unsized" {
		t.Errorf("hidden: %s, want 0 0 600 200 unsized", got)
	}
	b.media("print")
	b.eval(`dispatchEvent(new Event('beforeprint')), 1`)
	b.still()
	if got := b.eval(folded("tabs")); !strings.HasPrefix(got, "0 0 600 200 sized") {
		t.Errorf("printed: %s, want 0 0 600 200 sized", got)
	}
	b.media("")
	// Hidden again before its tab is chosen, as after a print: the size it had printing is not the one it had last
	b.eval(`dispatchEvent(new Event('afterprint')), ` + frames)
	b.eval(`document.querySelectorAll('[data-floe-part=tabs] input')[1].click(), 1`)
	// The frame is measured when its ResizeObserver tells it, after the tab shows
	b.until("the diagram in the tab fitted", folded("tabs")+".endsWith(' sized fitted')")
	b.eval(`document.querySelector('[data-floe-part=details]').open = true, 1`)
	b.until("the diagram in details fitted", folded("details")+".endsWith(' sized fitted')")
	if got := b.eval(folded("details")); strings.HasPrefix(got, "0 0 600 200") {
		t.Errorf("opened: %s, want the viewBox of the frame's ratio", got)
	}
}

// tonesSpec is a diagram with a box and an edge of each tone, as a spec of floe diagram render.
const tonesSpec = `width: 420
height: 200
title: Tones
parts:
  - box: { x: 20, y: 20, w: 100, h: 40, tone: accent, rows: [a] }
  - box: { x: 160, y: 20, w: 100, h: 40, tone: good, rows: [g] }
  - box: { x: 300, y: 20, w: 100, h: 40, tone: warn, rows: [w] }
  - box: { x: 20, y: 120, w: 100, h: 40, tone: bad, rows: [b] }
  - box: { x: 160, y: 120, w: 100, h: 40, rows: [plain] }
  - edge: { from: [120, 40], to: [160, 40], tone: accent }
  - edge: { from: [260, 140], to: [300, 140] }
`

// diagramColours lists, in the tree root names (document, or a shadow root), what the diagram's CSS paints:
// the frame's border, the font, every shape's fill and stroke, and the toolbar's text and buttons.
func diagramColours(root string) string {
	return `((root) => {
  const out = [];
  const read = (sel, props) => root.querySelectorAll(sel).forEach((e, i) => {
    const cs = getComputedStyle(e);
    out.push(sel + ' ' + i + ': ' + props.map((p) => cs.getPropertyValue(p)).join(' '));
  });
  read('.dg-wrap', ['border-top-color']);
  read('.dg', ['font-family']);
  read('.dg *', ['fill', 'stroke']);
  read('.dg-tools span', ['color']);
  read('.dg-tools button', ['color', 'background-color', 'border-top-color']);
  return out.join('\n');
})(` + root + `)`
}

// unpainted lists the shapes of the diagrams that a rule of part.css leaves in the browser's defaults, as one that
// reads a variable no palette sets: a line without a stroke, another shape black, or with neither fill nor stroke.
// LIMIT: a filled shape's stroke left so is not told; it draws no outline, as some shapes are meant to.
const unpainted = `(() => {
  const out = new Set();
  for (const e of document.querySelectorAll('svg.dg :is(rect, path, circle, ellipse, line, polyline, polygon, text)')) {
    if (e.closest('defs')) continue;
    const c = getComputedStyle(e);
    if (e.tagName === 'line' ? c.stroke === 'none' : c.fill === 'rgb(0, 0, 0)' || (c.fill === 'none' && c.stroke === 'none')) {
      out.add(e.tagName + '.' + [...e.classList].join('.') + ' in .' + [...e.parentElement.classList].join('.'));
    }
  }
  return [...out].join('\n');
})()`

// outside tells what a page draws outside the diagrams with its own palette, and its root's color-scheme.
const outside = `(() => {
  const c = getComputedStyle(document.querySelector('.probe'));
  return [c.color, c.backgroundColor, c.width, c.borderTopStyle, c.boxShadow,
    getComputedStyle(document.documentElement).colorScheme].join(' | ');
})()`

// TestUIDiagramFragment pastes the fragment of floe diagram render, a diagram of each tone and every built-in example,
// into pages of one's own, their style in the head before it. In a page without a palette, the diagram is painted as in a page of floe's default theme, its toolbar
// too, and in a shadow root as well. The page outside the diagrams keeps what it draws: its palette in a layer of its
// own, its color-scheme of a meta, the variables it reads with a fallback. A page's colours written on the
// diagrams' roots win, before the fragment or after it.
func TestUIDiagramFragment(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	// The diagrams: one of each tone, then every built-in example but aws, which draw every kind of part between them.
	specs := []string{s.write(t, "tones.yaml", tonesSpec)}
	var data strings.Builder
	data.WriteString("template: report\nlang: en\ntitle: Diagrams\nsections:\n  - parts:\n      - diagram:\n")
	for line := range strings.Lines(tonesSpec) {
		data.WriteString("          " + line)
	}
	for line := range strings.Lines(s.floe(t, "diagram", "example")) {
		kind, _, _ := strings.Cut(line, "\t")
		if kind == "aws" { // its icons are fetched, which a test does not
			continue
		}
		specs = append(specs, s.write(t, kind+".yaml", "example: "+kind+"\n"))
		data.WriteString("      - diagram: { example: " + kind + " }\n")
	}
	fragment := s.floe(t, append([]string{"diagram", "render"}, specs...)...)
	s.floe(t, "publish", s.write(t, "diagrams.yaml", data.String()), "--slug", "diagrams")
	b.navigate("/a/diagrams/?raw")
	b.until("the toolbars", "document.querySelectorAll('.dg-tools').length === "+strconv.Itoa(len(specs)))
	if got := b.eval(unpainted); got != "" {
		t.Errorf("shapes part.css leaves unpainted in floe's page:\n%s", got)
	}
	want := b.eval(diagramColours("document"))

	dir := t.TempDir()
	open := func(name, head, body string) {
		t.Helper()
		file := filepath.Join(dir, name)
		page := "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><title>" + name + "</title>" + head +
			"</head>\n<body>\n" + body + "\n</body></html>\n"
		if err := os.WriteFile(file, []byte(page), 0o600); err != nil {
			t.Fatal(err)
		}
		b.navigate("file://" + file)
	}

	open("bare.html", "", fragment)
	b.until("the toolbars", "document.querySelectorAll('.dg-tools').length === "+strconv.Itoa(len(specs)))
	if got := b.eval(diagramColours("document")); got != want {
		t.Errorf("in a page without a palette:\n%s\nwant, as in floe's page:\n%s", got, want)
	}

	open("shadow.html", "", `<div id="h"></div><script>document.getElementById('h').attachShadow({mode: 'open'}).innerHTML = `+
		js(fragment)+`;</script>`)
	inShadow := b.eval(diagramColours("document.getElementById('h').shadowRoot"))
	var wantShadow []string
	for line := range strings.Lines(want) {
		if !strings.HasPrefix(line, ".dg-tools") { // the zoom script finds no diagram in a shadow root
			wantShadow = append(wantShadow, strings.TrimSuffix(line, "\n"))
		}
	}
	if w := strings.Join(wantShadow, "\n"); inShadow != w {
		t.Errorf("in a shadow root:\n%s\nwant:\n%s", inShadow, w)
	}

	own := `<meta name="color-scheme" content="light dark"><style>
@layer host { :root { --bg: rgb(10, 20, 30); --accent: rgb(200, 0, 100); } }
.probe { color: var(--accent, rgb(1, 1, 1)); background: var(--bg); width: var(--text-width, 33px);
  border-top: var(--rule, 3px dotted rgb(2, 2, 2)); box-shadow: var(--shadow, 0 0 0 1px rgb(3, 3, 3)); }
</style>`
	open("alone.html", own, `<p class="probe">p</p>`)
	alone := b.eval(outside)
	open("around.html", own, `<p class="probe">p</p>`+fragment)
	b.until("the toolbar", "document.querySelector('.dg-tools')")
	if got := b.eval(outside); got != alone {
		t.Errorf("outside the diagram: %s, want %s, as without it", got, alone)
	}

	const accent = "rgb(1, 2, 3)"
	rule := "<style>.dg-tools, .dg-wrap { --accent: " + accent + "; }</style>"
	stroke := "getComputedStyle(document.querySelector('.dg .box.accent rect')).stroke"
	for _, page := range []struct{ name, head, body string }{
		{"before.html", rule, fragment},
		{"after.html", "", fragment + rule},
	} {
		open(page.name, page.head, page.body)
		if got := b.eval(stroke); got != accent {
			t.Errorf("%s: the accent box's stroke %s, want the page's %s", page.name, got, accent)
		}
	}
}

// measureBoxes returns a line for each box of the page's diagrams whose text, as the browser draws it, is not inside
// its shape, for each two boxes of a diagram that overlap, and for each label whose text overlaps a box or another
// label's text: the width floe estimates for text against the width drawn.
const measureBoxes = `(() => {
  const out = [];
  for (const svg of document.querySelectorAll('svg.dg')) {
    const boxes = [...svg.querySelectorAll('g.box')].map((g) => ({ shape: g.firstElementChild.getBBox(), texts: [...g.querySelectorAll('text')] }));
    for (const b of boxes) {
      for (const t of b.texts) {
        const r = t.getBBox();
        if (r.x < b.shape.x || r.y < b.shape.y || r.x + r.width > b.shape.x + b.shape.width || r.y + r.height > b.shape.y + b.shape.height) {
          out.push('"' + t.textContent + '" sticks out of its box');
        }
      }
    }
    for (let i = 0; i < boxes.length; i++) {
      for (let j = i + 1; j < boxes.length; j++) {
        const a = boxes[i].shape, c = boxes[j].shape;
        if (a.x < c.x + c.width && c.x < a.x + a.width && a.y < c.y + c.height && c.y < a.y + a.height) {
          out.push('two boxes overlap: "' + boxes[i].texts[0].textContent + '" and "' + boxes[j].texts[0].textContent + '"');
        }
      }
    }
    const meet = (a, c) => a.x < c.x + c.width && c.x < a.x + a.width && a.y < c.y + c.height && c.y < a.y + a.height;
    const labels = [...svg.querySelectorAll('text.label')].map((t) => ({ text: t.textContent, r: t.getBBox() }));
    for (let i = 0; i < labels.length; i++) {
      for (const b of boxes) {
        if (meet(labels[i].r, b.shape)) {
          out.push('label "' + labels[i].text + '" overlaps box "' + b.texts[0].textContent + '"');
        }
      }
      for (let j = i + 1; j < labels.length; j++) {
        if (meet(labels[i].r, labels[j].r)) {
          out.push('two labels overlap: "' + labels[i].text + '" and "' + labels[j].text + '"');
        }
      }
    }
  }
  return out.join('\n');
})()`

// TestUIGraph draws the graphs kept in the diagram tests, down and right, in English and Japanese: in each, the text
// the browser draws stays inside its box, and no two boxes overlap, nor a label's text a box or another label's.
func TestUIGraph(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	files, err := filepath.Glob("../diagram/testdata/graph/*.yaml")
	if err != nil || len(files) < 2 {
		t.Fatalf("the graphs kept: %v %v", files, err)
	}
	for _, lang := range []string{"en", "ja"} {
		var page strings.Builder
		page.WriteString("template: report\nlang: " + lang + "\ntitle: Graphs\nsections:\n  - heading: Graphs\n    parts:\n")
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			page.WriteString("      - diagram:\n")
			for line := range strings.Lines(string(src)) {
				page.WriteString("          " + line)
			}
		}
		slug := "graphs-" + lang
		s.floe(t, "publish", s.write(t, slug+".yaml", page.String()), "--slug", slug)
		b.navigate("/a/" + slug + "/?raw")
		b.until("the diagrams", "document.querySelectorAll('svg.dg').length === "+strconv.Itoa(len(files)))
		if got := b.eval(measureBoxes); got != "" {
			t.Errorf("%s:\n  %s", lang, strings.ReplaceAll(got, "\n", "\n  "))
		}
	}
}
