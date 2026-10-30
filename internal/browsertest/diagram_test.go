package browsertest_test

import (
	"os"
	"path/filepath"
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

// measureDiagrams returns a line for each diagram whose toolbar is not 0.4rem above its frame.
const measureDiagrams = `(() => {
  const out = [];
  const want = 0.4 * parseFloat(getComputedStyle(document.documentElement).fontSize);
  for (const wrap of document.querySelectorAll('.dg-wrap')) {
    const box = wrap.closest('[data-floe-part="diagram"]');
    const name = box.parentElement.closest('[data-floe-part]')?.dataset.floePart ?? 'page';
    const tools = wrap.previousElementSibling;
    const gap = wrap.getBoundingClientRect().top - tools.getBoundingClientRect().bottom;
    if (!tools.matches('.dg-tools') || Math.abs(gap - want) > 0.5) {
      out.push(name + ': toolbar ' + gap.toFixed(1) + ' px above the frame, want ' + want.toFixed(1));
    }
  }
  return out.join('\n');
})()`

// TestUIDiagram draws a diagram of a template and one in a part of one's own, wide and as wide as a phone: each
// has its toolbar a little above its frame, not over it.
func TestUIDiagram(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for name, text := range diagramKit {
		at := filepath.Join(s.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "publish", s.write(t, "diagrams.yaml", diagramPage), "--slug", "diagrams")
	for _, size := range []struct {
		width, height int64
		mobile        bool
	}{{1440, 900, false}, {390, 844, true}} {
		b.size(size.width, size.height, size.mobile)
		b.navigate("/a/diagrams/?raw")
		b.until("the toolbars", "document.querySelectorAll('.dg-tools').length === 2")
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
