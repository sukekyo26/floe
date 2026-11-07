package browsertest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/i18n"
)

// zoomHintPage is an image and a diagram that zoom alone, in five columns, an image that does not zoom, and the same
// two across the page.
func zoomHintPage(lang i18n.Lang) string {
	var col strings.Builder
	for range 5 {
		col.WriteString("          - parts:\n              - image: { file: shots/a.png, alt: In a column }\n" +
			"              - diagram: { width: 900, height: 400, title: d, parts: [{ box: { x: 20, y: 20, w: 100, h: 40, rows: [a] } }] }\n")
	}
	return "lang: " + string(lang) + "\ntemplate: report\ntitle: Z\nsections:\n  - heading: A\n    parts:\n" +
		"      - image: { file: shots/a.png, alt: Across }\n      - image: { file: shots/small.png, alt: Small }\n" +
		"      - diagram: { width: 2000, height: 400, title: d, parts: [{ box: { x: 20, y: 20, w: 100, h: 40, rows: [a] } }] }\n" +
		"      - columns:\n" + col.String()
}

// measureZoomHint tells of each frame that offers to zoom, at this width: what its toolbar holds but buttons, the
// buttons not in one line, and a title not hint; and of each that does not, that it has a title.
const measureZoomHint = `((hint) => {
  const out = [];
  for (const f of document.querySelectorAll('.floe-zoom')) {
    const tools = f.previousElementSibling;
    if (tools.hidden) { if (f.hasAttribute('title')) out.push('a frame that does not zoom has a title'); continue; }
    if (f.title !== hint) out.push('title ' + f.title);
    if ([...tools.children].some((e) => e.tagName !== 'BUTTON') || tools.children.length !== 4) out.push('holds ' + tools.innerHTML);
    const tops = new Set([...tools.children].map((b) => Math.round(b.getBoundingClientRect().top)));
    if (tops.size !== 1) out.push('in ' + tops.size + ' lines at ' + f.getBoundingClientRect().width + ' px');
  }
  return [...new Set(out)].join('\n');
})`

// TestUIZoomHint shows the toolbar of a frame that zooms as its buttons alone, in one line in columns too, and says how
// it is used as the frame's title, which a screen reader reads as its description.
func TestUIZoomHint(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, func(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
		t.Helper()

		s := newSite(t)
		b := newBrowser(t, s.Base, lang)
		s.write(t, "z/shots/a.png", pngOf(t, 3000, 300))
		s.write(t, "z/shots/small.png", pngOf(t, 40, 20))
		s.floe(t, "publish", s.write(t, "z/z.yaml", zoomHintPage(lang)), "--slug", "z")
		hint, err := json.Marshal(msg("zoom_hint"))
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []int64{1440, 400} {
			b.size(width, 900, false)
			b.navigate("/a/z/?raw")
			b.until("the frames set up", `document.querySelectorAll('.floe-zoom[data-zoomable=on]').length >= 13`)
			if got := b.eval(measureZoomHint + "(" + string(hint) + ")"); got != "" {
				t.Errorf("at %d px:\n%s", width, got)
			}
		}
		b.unsize()

		tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
		if err != nil {
			t.Fatal(err)
		}
		frames := 0
		for _, n := range tree.Nodes {
			if n.Ignored || n.Name == nil {
				continue
			}
			var name, description string
			if json.Unmarshal(n.Name.Value, &name) != nil || name != msg("zoom_frame") {
				continue
			}
			frames++
			if n.Description != nil && json.Unmarshal(n.Description.Value, &description) != nil {
				t.Fatal("a description that is not text")
			}
			if description != msg("zoom_hint") {
				t.Errorf("a frame's description is %q", description)
			}
		}
		if frames == 0 {
			t.Error("no frame named as one to zoom")
		}
	})
}
