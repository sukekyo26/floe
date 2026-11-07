package browsertest_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
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
// toolbar part.css does not reach, and one for each drawn no smaller than its actual size, which offers no zoom: they are found from the SVG, not by the classes Go gives them, which part.css
// writes by hand.
const measureDiagrams = `(() => {
  const out = [];
  const want = 0.4 * parseFloat(getComputedStyle(document.documentElement).fontSize);
  for (const svg of document.querySelectorAll('svg.dg')) {
    const wrap = svg.parentElement;
    const box = wrap.closest('[data-floe-part="diagram"]');
    const name = box.parentElement.closest('[data-floe-part]')?.dataset.floePart ?? 'page';
    const tools = wrap.previousElementSibling;
    if (tools.hidden) {
      const offers = getComputedStyle(tools).display !== 'none' || wrap.hasAttribute('role') || wrap.hasAttribute('aria-label') ||
        wrap.tabIndex !== -1 || getComputedStyle(wrap).touchAction !== 'auto';
      out.push(name + (offers ? ': a hidden toolbar shows, or its frame still offers to zoom' : ': no toolbar'));
      continue;
    }
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

// TestUIDiagram draws a diagram of a template and one in a part of one's own, wide and as wide as a phone: the
// template's, drawn smaller than its actual size, has its toolbar a little above its frame, not over it, and the look
// zoom reaches both; the part's, drawn no smaller, has none and does not offer to zoom.
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
		// pic, 300 px wide, is drawn at its actual size or larger at both
		if got := b.eval(measureDiagrams); got != "pic: no toolbar" {
			t.Errorf("at %d px:\n  %s", size.width, strings.ReplaceAll(got, "\n", "\n  "))
		}
	}
	// A diagram as wide as the body is not shrunk by its frame's line, which the diagram is drawn inside
	s.floe(t, "publish", s.write(t, "wide.yaml", "lang: en\ntemplate: mine\ntitle: Wide\n"+
		"plain: { width: 740, height: 60, title: w, parts: [ { box: { x: 20, y: 5, w: 120, h: 50, rows: [wide] } } ] }\n"+
		"pic: { width: 300, height: 100, title: p, parts: [ { box: { x: 20, y: 20, w: 120, h: 50, rows: [pic] } } ] }\n"), "--slug", "wide")
	b.size(756, 900, false)
	b.navigate("/a/wide/?raw")
	b.until("the frames set up", "document.querySelectorAll('.floe-zoom[data-zoomable=on]').length === 2")
	if got := b.eval(measureDiagrams); got != "page: no toolbar\npic: no toolbar" {
		t.Errorf("a diagram as wide as the body:\n  %s", strings.ReplaceAll(got, "\n", "\n  "))
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
  const w = document.querySelector('[data-floe-part=` + part + `] .floe-zoom');
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
	b.until("the toolbars", "document.querySelectorAll('.floe-zoom-tools').length === 2")
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

// tonesSpec is a diagram with a box and an edge of each tone, and raw SVG coloured as the docs tell (a palette's
// variable, a tone of a <g> in it, text in currentColor), as a spec of floe diagram render.
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
  - raw: { svg: '<rect x="300" y="120" width="30" height="40" fill="var(--accent-soft)" stroke="var(--accent)"/><g data-floe-tone="bad"><rect x="340" y="120" width="30" height="40" fill="var(--tone)"/></g><text x="380" y="140" font-size="12" fill="currentColor">r</text>' }
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
  read('.floe-zoom', ['border-top-color', 'background-color', 'color']);
  read('.dg', ['font-family']);
  read('.dg *', ['fill', 'stroke']);
  read('.floe-zoom-tools button', ['color', 'background-color', 'border-top-color']);
  read('.floe-zoom-hint', ['color', 'background-color', 'border-top-color']);
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
	// The diagrams: one of each tone, then every built-in example but the AWS ones, which draw every kind of part
	// between them.
	specs := []string{s.write(t, "tones.yaml", tonesSpec)}
	var data strings.Builder
	data.WriteString("template: report\nlang: en\ntitle: Diagrams\nsections:\n  - parts:\n      - diagram:\n")
	for line := range strings.Lines(tonesSpec) {
		data.WriteString("          " + line)
	}
	for line := range strings.Lines(s.floe(t, "diagram", "example")) {
		kind, _, _ := strings.Cut(line, "\t")
		if strings.HasPrefix(kind, "aws") { // their icons are fetched, which a test does not
			continue
		}
		specs = append(specs, s.write(t, kind+".yaml", "example: "+kind+"\n"))
		data.WriteString("      - diagram: { example: " + kind + " }\n")
	}
	fragment := s.floe(t, append([]string{"diagram", "render"}, specs...)...)
	s.floe(t, "publish", s.write(t, "diagrams.yaml", data.String()), "--slug", "diagrams")
	b.navigate("/a/diagrams/?raw")
	b.until("the toolbars", "document.querySelectorAll('.floe-zoom-tools').length === "+strconv.Itoa(len(specs)))
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
	b.until("the toolbars", "document.querySelectorAll('.floe-zoom-tools').length === "+strconv.Itoa(len(specs)))
	if got := b.eval(diagramColours("document")); got != want {
		t.Errorf("in a page without a palette:\n%s\nwant, as in floe's page:\n%s", got, want)
	}

	open("shadow.html", "", `<div id="h"></div><script>document.getElementById('h').attachShadow({mode: 'open'}).innerHTML = `+
		js(fragment)+`;</script>`)
	inShadow := b.eval(diagramColours("document.getElementById('h').shadowRoot"))
	var wantShadow []string
	for line := range strings.Lines(want) {
		if !strings.HasPrefix(line, ".floe-zoom-tools") && !strings.HasPrefix(line, ".floe-zoom-hint") { // the zoom script finds no diagram in a shadow root
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
	b.until("the toolbar", "document.querySelector('.floe-zoom-tools')")
	if got := b.eval(outside); got != alone {
		t.Errorf("outside the diagram: %s, want %s, as without it", got, alone)
	}

	const accent = "rgb(1, 2, 3)"
	rule := "<style>.floe-zoom-tools, .floe-zoom { --accent: " + accent + "; }</style>"
	stroke := "getComputedStyle(document.querySelector('.dg .box[data-floe-tone=accent] rect')).stroke"
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
// its shape, for each two boxes of a diagram that overlap, and for each label or frame's heading whose text overlaps
// a box, another's text or an edge (loops too; a label on its background has its own edge through it, and overlaps a
// second), or sticks out of its background: the width floe estimates for text against the width drawn.
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
    const labels = [...svg.querySelectorAll('text.label, g.frame > text')].map((t) => {
      const ground = t.previousElementSibling && t.previousElementSibling.matches('rect.label-bg') ? t.previousElementSibling.getBBox() : null;
      const r = t.getBBox();
      if (ground && (r.x < ground.x || r.x + r.width > ground.x + ground.width)) {
        out.push('label "' + t.textContent + '" sticks out of its ground');
      }
      return { text: t.textContent, r, ground };
    });
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
      const r = labels[i].r;
      let through = 0;
      for (const e of svg.querySelectorAll('path.edge')) {
        for (let at = 0, n = e.getTotalLength(); at <= n; at++) {
          const p = e.getPointAtLength(at);
          if (p.x > r.x && p.x < r.x + r.width && p.y > r.y && p.y < r.y + r.height) {
            if (!labels[i].ground || ++through > 1) {
              out.push('label "' + labels[i].text + '" overlaps the edge ' + e.getAttribute('d'));
            }
            break;
          }
        }
      }
    }
  }
  return out.join('\n');
})()`

// TestUIGraph draws the graphs kept in the diagram tests, down and right, in English and Japanese: in each, the text
// the browser draws stays inside its box, and no two boxes overlap, nor a label's or a heading's text a box, another's
// or an edge.
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
	// The check itself: a second edge through a label on its edge, and a label sticking out of its background.
	got := b.eval(`(() => {
	  const g = document.querySelector('rect.label-bg + text.label');
	  const r = g.getBBox();
	  const p = g.closest('svg').querySelector('path.edge').cloneNode();
	  p.setAttribute('d', 'M' + (r.x + r.width / 2) + ',' + r.y + ' V' + (r.y + r.height));
	  g.closest('svg').appendChild(p);
	  g.previousElementSibling.setAttribute('width', 1);
	  return ` + measureBoxes + `;
	})()`)
	for _, want := range []string{"sticks out of its ground", "overlaps the edge"} {
		if !strings.Contains(got, want) {
			t.Errorf("measureBoxes tells %q, want %q", got, want)
		}
	}
}

// groundSpecs are two diagrams drawing what hides what lies under it: a hand-written edge's label, a hollow end and a
// hollow dot; and the labels of a Venn diagram's regions, with no plain, beside a bar's label.
var groundSpecs = []string{`title: edges
width: 380
height: 140
parts:
  - box: { name: a, x: 20, y: 20, w: 90, h: 36, rows: [A] }
  - box: { name: b, x: 270, y: 20, w: 90, h: 36, rows: [B] }
  - edge: { from: a, to: b, label: a label, end: triangle }
  - edge: { from: [20, 110], to: [200, 110] }
  - dot: { cx: 110, cy: 110, r: 7, hollow: true }
`, `title: venn
width: 520
height: 200
parts:
  - circle: { name: p, cx: 120, cy: 100, r: 80, tone: accent }
  - circle: { name: q, cx: 220, cy: 100, r: 80, tone: good }
  - label: { text: both, in: [p, q] }
  - label: { text: only p, in: [p] }
  - label: { text: only q, x: 250, y: 110, in: [q] }
  - bar: { x1: 330, x2: 420, y: 150, label: a bar }
`}

// groundParts are the diagrams of groundSpecs as parts, indented by n.
func groundParts(n int) string {
	var b strings.Builder
	for _, spec := range groundSpecs {
		b.WriteString(strings.Repeat(" ", n) + "- diagram:\n")
		for line := range strings.Lines(spec) {
			b.WriteString(strings.Repeat(" ", n+4) + line)
		}
	}
	return b.String()
}

// measureGround returns a line for each diagram whose ground, the first thing up from its frame that paints one, is
// not its frame, for each thing hiding what lies under it not painted the frame's ground, and for each Venn diagram
// with a box under more labels than its bar's.
const measureGround = `(() => {
  const out = [];
  document.querySelectorAll('.floe-zoom').forEach((wrap, i) => {
    let e = wrap;
    while (e && getComputedStyle(e).backgroundColor === 'rgba(0, 0, 0, 0)' && getComputedStyle(e).backgroundImage === 'none') {
      e = e.parentElement;
    }
    if (e !== wrap) {
      out.push('diagram ' + i + ': its ground is ' + (e ? e.tagName.toLowerCase() + ' ' + e.className : 'none'));
      return;
    }
    const ground = getComputedStyle(wrap).backgroundColor;
    wrap.querySelectorAll('.label-bg, .mk-hollow, .dot.hollow').forEach((h) => {
      const fill = getComputedStyle(h).fill;
      if (fill !== ground) {
        out.push('diagram ' + i + ': ' + h.getAttribute('class') + ' painted ' + fill + ' on ' + ground);
      }
    });
    if (wrap.querySelector('.venn') && wrap.querySelectorAll('.label-bg').length !== 1) {
      out.push('diagram ' + i + ': ' + wrap.querySelectorAll('.label-bg').length + ' label boxes, want the bar\'s alone');
    }
  });
  return out.join('\n');
})()`

// TestUIDiagramGround draws diagrams in a template's section, in details and tabs, and in ADR's panel, in every theme:
// each diagram paints its own ground, and what hides what lies under it (a label's box, a hollow end and dot) is painted
// that ground, as no CSS can read the ground outside it; the labels of a Venn diagram's regions have no box.
func TestUIDiagramGround(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	report := "template: report\nlang: en\ntitle: Ground\nsections:\n  - heading: Section\n    parts:\n" + groundParts(6) +
		"  - heading: Held\n    parts:\n      - details:\n          title: Open\n          open: true\n          parts:\n" +
		groundParts(12) + "      - tabs:\n          - title: One\n            parts:\n" + groundParts(14) +
		"          - title: Two\n            parts:\n              - text: two\n"
	adr := "template: adr\nlang: en\ntitle: Ground\nnumber: ADR 1\ndate: 2030-03-02\nstatus: accepted\n" +
		"context: Some context.\ndecision: Some decision.\nmore:\n  - heading: Diagrams\n    parts:\n" + groundParts(6)
	for _, theme := range themes {
		for name, page := range map[string]string{"report": report, "adr": adr} {
			slug := "ground-" + name + "-" + theme
			s.floe(t, "publish", s.write(t, slug+".yaml", page+"theme: "+theme+"\n"), "--slug", slug)
			s.b.navigate("/a/" + slug + "/?raw")
			want := 2
			if name == "report" {
				want = 6
			}
			s.b.until("the diagrams", "document.querySelectorAll('svg.dg').length === "+strconv.Itoa(want))
			if got := s.b.eval(measureGround); got != "" {
				t.Errorf("%s in %s:\n  %s", name, theme, strings.ReplaceAll(got, "\n", "\n  "))
			}
		}
	}
}

// faceOf in internal/diagram tells each class as part.css draws it: its size, weight and whether in code.
const measureFaces = `(() => {
  const svg = document.querySelector('svg.dg');
  const text = (parent, cls) => {
    const t = document.createElementNS('http://www.w3.org/2000/svg', 'text');
    if (cls) t.setAttribute('class', cls);
    t.textContent = 'x';
    parent.append(t);
    return t;
  };
  const plain = getComputedStyle(svg).fontFamily;
  const face = (t) => {
    const s = getComputedStyle(t);
    return s.fontSize + ' ' + s.fontWeight + (s.fontFamily === plain ? '' : ' code');
  };
  const out = {};
  for (const cls of ['main', 'sub', 'main code', 'sub code', 'cell code', 'type', 'label', 'label strong']) {
    out[cls] = face(text(svg, cls));
  }
  for (const cls of ['frame', 'frame grouped']) {
    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.setAttribute('class', cls);
    svg.append(g);
    out[cls] = face(text(g));
  }
  return JSON.stringify(out);
})()`

func TestUIDiagramFaces(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	const want = `{"main":"13px 700","sub":"12px 400","main code":"12px 700 code","sub code":"12px 400 code",` +
		`"cell code":"12px 400 code","type":"12px 400","label":"12px 400","label strong":"13px 700",` +
		`"frame":"12px 700","frame grouped":"13px 700"}`
	for _, theme := range themes {
		slug := "faces-" + theme
		page := "template: report\nlang: en\ntheme: " + theme + "\ntitle: Faces\nsections:\n  - heading: Section\n    parts:\n" +
			"      - diagram: { title: D, width: 200, height: 100, parts: [{ box: { x: 10, y: 10, rows: [A] } }] }\n"
		s.floe(t, "publish", s.write(t, slug+".yaml", page), "--slug", slug)
		s.b.navigate("/a/" + slug + "/?raw")
		s.b.until("the diagram", "document.querySelectorAll('svg.dg').length === 1")
		if got := s.b.eval(measureFaces); got != want {
			t.Errorf("in %s, part.css draws the classes of a diagram as\n  %s\nand internal/diagram's faceOf measures them as\n  %s\nchange them together", theme, got, want)
		}
	}
}
