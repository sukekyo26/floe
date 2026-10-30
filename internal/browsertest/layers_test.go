package browsertest_test

import (
	"testing"
)

// layersKit is templates and a theme of one's own that tell which of floe's CSS layers wins where.
var layersKit = map[string]string{
	"templates/layers/schema.json": `{"type": "object", "properties": {"note": {"$ref": "floe:///parts/note.json"}}, "additionalProperties": false}`,
	"templates/layers/page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
		`<body><header>h</header><p id="own">own</p><div id="boxed" data-floe-look="panel">b</div>{{.Data.note}}</body></html>`,
	"templates/layers/page.css": "body { color: rgb(0, 0, 2); }\np { color: rgb(255, 0, 0); }\n" +
		"body div[data-floe-part=note].warn { background-color: rgb(255, 0, 0); margin-top: 7px; }\n" +
		"body div[data-floe-part=note].warn::before { top: 0px; }\n" +
		"#boxed { background-color: rgb(0, 0, 4); }\n" +
		"@keyframes held { from { opacity: 0.5; } to { opacity: 0.5; } }\nheader { animation: held 1000s; }\n" +
		`@font-face { font-family: "own face"; src: url(data:,); }` + "\n",
	"templates/themed/schema.json": `{"type": "object", "properties": {"text": {"type": "string", "format": "markdown"},
"note": {"$ref": "floe:///parts/note.json"}}, "additionalProperties": false}`,
	"templates/themed/page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
		`<body><p id="own">own</p><header data-floe-look="band"><h2 id="banded">b</h2></header>{{.Data.text}}{{.Data.note}}</body></html>`,
	"templates/themed/page.css":  "p { color: rgb(255, 0, 0); }\n",
	"themes/mine/base.css":       "p { color: rgb(0, 0, 7); }\nblockquote { background-color: rgb(0, 0, 8); }\nh2 { color: rgb(0, 0, 9); }\n",
	"themes/mine/band.css":       ":scope h2 { color: rgb(0, 0, 10); }\n",
	"templates/bare/schema.json": `{"type": "object", "additionalProperties": false}`,
	"templates/bare/page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
		`<body><p>a</p></body></html>`,
	"templates/brand/schema.json": `{"type": "object", "properties": {"note": {"$ref": "floe:///parts/note.json"}}, "additionalProperties": false}`,
	"templates/brand/page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
		`<body>{{.Data.note}}</body></html>`,
	"templates/brand/page.css":      ":scope { --accent: rgb(1, 2, 3); --surface: rgb(4, 5, 6); }\n",
	"parts/sheet/schema.json":       `{"type": "object"}`,
	"parts/sheet/part.html":         `<div data-floe-part="sheet"></div>`,
	"parts/sheet/part.css":          "",
	"parts/sheet/part.yaml":         "looks: [panel]\n",
	"templates/layers/example.yaml": "lang: en\ntemplate: layers\n",
	"templates/themed/example.yaml": "lang: en\ntemplate: themed\n",
	"templates/bare/example.yaml":   "lang: en\ntemplate: bare\n",
	"templates/brand/example.yaml":  "lang: en\ntemplate: brand\n",
}

// TestUILayers checks, in the browser, how floe puts a page's CSS together. The template's page.css, though its
// selectors are the more specific, loses to the part's style and to the theme's style of the part, and still styles
// what neither of them does, such as where a part stands. It reaches the part's element but not what the part holds,
// whose colour it gives by inheritance alone. Its @keyframes and @font-face, in floe's @scope, work. The theme's
// base.css is under the template's page.css and the looks, and reaches the page but not what a part holds. A part's
// style stops at a part it holds. floe's theme does not style a part of one's own that took the place of
// floe's, but does those drawn in its look. floe's base draws the page's ground, colour and face in the theme's,
// under a template that draws none of them, and the theme the parts in the colours a template sets.
func TestUILayers(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.own(t, layersKit)
	open := func(slug, data, ready string) {
		t.Helper()

		s.floe(t, "publish", s.write(t, slug+".yaml", data), "--slug", slug)
		b.navigate("/a/" + slug + "/?raw")
		b.until(ready, "!!document.querySelector("+js(ready)+")")
	}

	open("layers", "template: layers\nlang: en\nnote: { text: x, tone: warn }\n", "[data-floe-part=note]")
	// The part's background, the theme's top of the mark (default's 0.85rem, over the part's), the template's margin;
	// the template's own paragraph and the note's; the animation and the face.
	got := b.eval(`(() => { const n = document.querySelector('[data-floe-part=note]'), s = getComputedStyle(n);
		return [s.backgroundColor, getComputedStyle(n, '::before').top, s.marginTop,
			getComputedStyle(document.getElementById('own')).color, getComputedStyle(n.querySelector('p')).color,
			document.querySelector('header').getAnimations()[0]?.animationName,
			[...document.fonts].some((f) => f.family.replaceAll('"', '') === 'own face')].join(' | ') })()`)
	check(t, "layers", []string{got}, "rgb(246, 232, 210) | 13.6px | 7px | rgb(255, 0, 0) | rgb(0, 0, 2) | held | true")
	// The template's own element that takes the panel's look has it under the template's CSS, which adjusts it: its
	// background, the corners of default's panel.
	got = b.eval(`(() => { const s = getComputedStyle(document.getElementById('boxed')); return s.backgroundColor + ' ' + s.borderTopLeftRadius })()`)
	check(t, "a look of the template's own", []string{got}, "rgb(0, 0, 4) 20px")

	// The theme's base.css: the template's paragraph wins over it; it draws the quote on the page but not the one in
	// the note, whose paragraph floe's base draws in its quote's colour (default's --muted); the theme's band.css wins
	// over it on the band.
	open("themed", "template: themed\nlang: en\ntheme: mine\ntext: '> page'\nnote: { text: '> held' }\n", "[data-floe-part=note] blockquote")
	got = b.eval(`(() => { const c = (q, p) => getComputedStyle(document.querySelector(q))[p];
		return [c('#own', 'color'), c('body > blockquote', 'backgroundColor'), c('[data-floe-part=note] blockquote', 'backgroundColor'),
			c('[data-floe-part=note] p', 'color'), c('#banded', 'color')].join(' | ') })()`)
	check(t, "the theme's base.css", []string{got}, "rgb(255, 0, 0) | rgb(0, 0, 8) | rgba(0, 0, 0, 0) | rgb(97, 95, 90) | rgb(0, 0, 10)")

	// A part's style stops at a part it holds: the note's code is on the surface, the code of the table in it on
	// floe's base.
	open("nested", "template: report\nlang: en\ntheme: soft\nsections:\n  - parts:\n      - html: >-\n"+
		`          <div data-floe-part="note"><p><code id="own">a</code></p><div data-floe-part="table"><table><tbody><tr><td>`+
		`<code id="held">b</code></td></tr></tbody></table></div></div>`+"\n", "#held")
	got = b.eval(`['own', 'held'].map((id) => getComputedStyle(document.getElementById(id)).backgroundColor).join(' | ')`)
	check(t, "nested", []string{got}, "rgb(255, 255, 255) | rgb(220, 235, 236)")

	// floe's base draws the page's ground, colour and face in the theme's, under a template that draws none of them.
	open("bare", "template: bare\nlang: en\ntheme: washi\n", "p")
	ground := `(() => { const r = getComputedStyle(document.documentElement), s = getComputedStyle(document.querySelector('p'));
		return [r.backgroundColor, getComputedStyle(document.body).backgroundColor, s.color, s.fontFamily.split(',')[0]].join(' | ') })()`
	check(t, "bare", []string{b.eval(ground)}, `rgb(237, 228, 207) | rgba(0, 0, 0, 0) | rgb(29, 27, 24) | "Hiragino Mincho ProN"`)

	// A ground of a pattern, blueprint's squares, is drawn whole: its layers, their size, the colour under them.
	open("squares", "template: bare\nlang: en\ntheme: blueprint\n", "p")
	got = b.eval(`(() => { const r = getComputedStyle(document.documentElement);
		return [r.backgroundImage.split('linear-gradient').length - 1, r.backgroundSize, r.backgroundColor].join(' | ') })()`)
	check(t, "squares", []string{got}, "2 | 16px 16px, 16px 16px, auto | rgb(238, 244, 252)")

	// The colours a template sets on :scope reach what the theme draws in the parts, the text on a filled face among
	// them, which takes the surface the template set.
	open("brand", "template: brand\nlang: en\ntheme: ink\nnote: { title: t, text: x }\n", "[data-floe-part=note] .title")
	got = b.eval(`(() => { const n = document.querySelector('[data-floe-part=note]'), s = getComputedStyle(n.querySelector('.title'));
		return [getComputedStyle(n).borderTopColor, s.backgroundColor, s.color].join(' | ') })()`)
	check(t, "brand", []string{got}, "rgb(1, 2, 3) | rgb(1, 2, 3) | rgb(4, 5, 6)")

	// A part of one's own takes the panel's look by its part.yaml: floe's washi styles it by the panel's file (its
	// surface and double rules).
	open("own-sheet", "template: report\nlang: en\ntheme: washi\nsections:\n  - parts:\n"+
		"      - html: <div data-floe-part=\"sheet\">mine</div>\n", "[data-floe-part=sheet]")
	got = b.eval(`(() => { const s = getComputedStyle(document.querySelector('[data-floe-part=sheet]'));
		return s.backgroundColor + ' ' + s.borderTopStyle })()`)
	check(t, "own sheet", []string{got}, "rgb(247, 241, 227) double")

	// A card takes the panel's look without being a part's box, a layer under the parts': options and default's
	// cards.css keep it a face of the surface on the panel of default, the nearer scope though.
	open("cards", "template: report\nlang: en\nsections:\n  - parts:\n"+
		"      - options: [{ title: a }]\n      - steps: [{ title: b }]\n", ".card")
	got = b.eval(`['.card', '[data-floe-part=steps]'].map((at) => {
		const s = getComputedStyle(document.querySelector(at)); return s.backgroundColor + ' ' + s.borderTopLeftRadius; }).join(' | ')`)
	check(t, "cards", []string{got}, "rgb(255, 255, 255) 14px | color(srgb 0.91098 0.903529 0.884706) 20px")
}
