package server_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// TestUILayers checks, in the browser, how floe puts a page's CSS together. The template's page.css,
// though its selectors are the more specific, loses to the part's style and to the theme's, and
// still styles what neither of them does, such as where a part stands. It reaches the part's element but
// not what the part holds, whose colour it gives by inheritance alone. Its @keyframes and @font-face,
// in floe's @scope, work. A block page.css leaves open does not take in the theme, nor one a part of one's
// own leaves open the parts after it. A part's style stops at a part it holds. floe's theme does not
// style a part of one's own that took the place of floe's, but does those drawn in its look. floe's base
// draws the page's ground, colour and face in the theme's, under a template that draws none of them.
func TestUILayers(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	for name, body := range map[string]string{
		"schema.json": `{"type": "object", "properties": {"note": {"$ref": "floe:///parts/note.json"}}, "additionalProperties": false}`,
		"page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
			`<body><header>h</header><p id="own">own</p><div id="boxed" data-floe-look="panel">b</div>{{.Data.note}}</body></html>`,
		"page.css": "body { color: rgb(0, 0, 2); }\np { color: rgb(255, 0, 0); }\n" +
			"body div[data-floe-part=note].warn { background-color: rgb(255, 0, 0); margin-top: 7px; }\n" +
			"body div[data-floe-part=note].warn::before { top: 0px; }\n" +
			"#boxed { background-color: rgb(0, 0, 4); }\n" +
			"@keyframes held { from { opacity: 0.5; } to { opacity: 0.5; } }\nheader { animation: held 1000s; }\n" +
			`@font-face { font-family: "own face"; src: url(data:,); }` + "\nfooter { color: red;",
	} {
		e.write(t, "templates/layers/"+name, body)
	}
	slug := "layers"
	e.publish(t, e.write(t, "layers.yaml", "template: layers\nlang: en\nnote: { text: x, tone: warn }\n"), store.Options{Slug: &slug})
	b.navigate("/a/layers/?raw")
	b.until("the note", "!!document.querySelector('[data-floe-part=note]')")
	// The part's background, the theme's top of the mark (0.85rem, over the part's 1.1rem), the
	// template's margin; the template's own paragraph and the note's; the animation and the face.
	got := b.eval(`(() => { const n = document.querySelector('[data-floe-part=note]'), s = getComputedStyle(n);
		return [s.backgroundColor, getComputedStyle(n, '::before').top, s.marginTop,
			getComputedStyle(document.getElementById('own')).color, getComputedStyle(n.querySelector('p')).color,
			document.querySelector('header').getAnimations()[0]?.animationName,
			[...document.fonts].some((f) => f.family.replaceAll('"', '') === 'own face')].join(' | ') })()`)
	check(t, "layers", []string{got}, "rgb(246, 232, 210) | 13.6px | 7px | rgb(255, 0, 0) | rgb(0, 0, 2) | held | true")
	// The template's own element that takes the panel's look has it under the template's CSS, which
	// adjusts it: its background, the corners of default's panel.
	got = b.eval(`(() => { const s = getComputedStyle(document.getElementById('boxed')); return s.backgroundColor + ' ' + s.borderTopLeftRadius })()`)
	check(t, "a look of the template's own", []string{got}, "rgb(0, 0, 4) 20px")

	// A part's style stops at a part it holds: the note's code is on the surface, the code of the table in
	// it on floe's base.
	slug = "nested"
	e.publish(t, e.write(t, "nested.yaml", "template: report\nlang: en\ntheme: soft\nsections:\n  - parts:\n      - html: >-\n"+
		`          <div data-floe-part="note"><p><code id="own">a</code></p><div data-floe-part="table"><table><tbody><tr><td>`+
		`<code id="held">b</code></td></tr></tbody></table></div></div>`+"\n"), store.Options{Slug: &slug})
	b.navigate("/a/nested/?raw")
	b.until("the code", "!!document.getElementById('held')")
	got = b.eval(`['own', 'held'].map((id) => getComputedStyle(document.getElementById(id)).backgroundColor).join(' | ')`)
	check(t, "nested", []string{got}, "rgb(255, 255, 255) | rgb(220, 235, 236)")

	// A block a part of one's own leaves open ends with its <style>: the note after it keeps its style.
	e.write(t, "parts/leaky/schema.json", `{"type": "object", "properties": {"text": {"type": "string"}}}`)
	e.write(t, "parts/leaky/part.html", `<p data-floe-part="leaky">{{.text}}</p>`)
	e.write(t, "parts/leaky/part.css", ":scope { color: rgb(0, 0, 3); }\n.open { color: red;\n")
	e.write(t, "templates/leaky/schema.json", `{"type": "object", "properties": {"leaky": {"$ref": "floe:///parts/leaky.json"}, `+
		`"note": {"$ref": "floe:///parts/note.json"}}, "additionalProperties": false}`)
	e.write(t, "templates/leaky/page.html", `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body>{{.Data.leaky}}{{.Data.note}}</body></html>`)
	e.write(t, "templates/leaky/page.css", ":scope { --fg: #000; }\n")
	slug = "leaky"
	e.publish(t, e.write(t, "leaky.yaml", "template: leaky\nlang: en\nleaky: { text: a }\nnote: { text: b, tone: warn }\n"),
		store.Options{Slug: &slug})
	b.navigate("/a/leaky/?raw")
	b.until("the note", "!!document.querySelector('[data-floe-part=note]')")
	got = b.eval(`[getComputedStyle(document.querySelector('[data-floe-part=leaky]')).color,
		getComputedStyle(document.querySelector('[data-floe-part=note]')).backgroundColor].join(' | ')`)
	check(t, "leaky", []string{got}, "rgb(0, 0, 3) | rgb(246, 232, 210)")

	e.write(t, "templates/bare/schema.json", `{"type": "object", "additionalProperties": false}`)
	e.write(t, "templates/bare/page.html", `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><p>a</p></body></html>`)
	slug = "bare"
	e.publish(t, e.write(t, "bare.yaml", "template: bare\nlang: en\ntheme: washi\n"), store.Options{Slug: &slug})
	b.navigate("/a/bare/?raw")
	b.until("the text", "!!document.querySelector('p')")
	ground := `(() => { const r = getComputedStyle(document.documentElement), s = getComputedStyle(document.querySelector('p'));
		return [r.backgroundColor, getComputedStyle(document.body).backgroundColor, s.color, s.fontFamily.split(',')[0]].join(' | ') })()`
	check(t, "bare", []string{b.eval(ground)}, `rgb(237, 228, 207) | rgba(0, 0, 0, 0) | rgb(29, 27, 24) | "Hiragino Mincho ProN"`)
	// One that sets their variables on :scope draws its own.
	e.write(t, "templates/bare/page.css", ":scope { --page-bg: rgb(0, 0, 5); --fg: rgb(0, 0, 6); --font: own; }\n")
	e.publish(t, e.write(t, "bare.yaml", "template: bare\nlang: en\ntheme: washi\n"), store.Options{Slug: &slug})
	b.navigate("/a/bare/?raw")
	b.until("its ground", "getComputedStyle(document.documentElement).backgroundColor === 'rgb(0, 0, 5)'")
	check(t, "bare of its own ground", []string{b.eval(ground)}, `rgb(0, 0, 5) | rgba(0, 0, 0, 0) | rgb(0, 0, 6) | own`)
	// A ground of a pattern, blueprint's squares, is drawn whole: its layers, their size, the colour under them.
	e.write(t, "templates/squares/schema.json", `{"type": "object", "additionalProperties": false}`)
	e.write(t, "templates/squares/page.html", `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><p>a</p></body></html>`)
	slug = "squares"
	e.publish(t, e.write(t, "squares.yaml", "template: squares\nlang: en\ntheme: blueprint\n"), store.Options{Slug: &slug})
	b.navigate("/a/squares/?raw")
	b.until("the text", "!!document.querySelector('p')")
	got = b.eval(`(() => { const r = getComputedStyle(document.documentElement);
		return [r.backgroundImage.split('linear-gradient').length - 1, r.backgroundSize, r.backgroundColor].join(' | ') })()`)
	check(t, "squares", []string{got}, "2 | 16px 16px, 16px 16px, auto | rgb(238, 244, 252)")

	// A table of one's own takes the place of floe's, and the panel's look by its part.yaml: floe's washi
	// styles it and compare, drawn in the table's look, by the panel's file (its double rules), not by the
	// table's, which would take the rules off.
	e.write(t, "parts/table/schema.json", `{"type": "object", "properties": {"text": {"type": "string"}}}`)
	e.write(t, "parts/table/part.html", `<div data-floe-part="table">{{.text}}</div>`)
	e.write(t, "parts/table/part.css", "")
	e.write(t, "parts/table/part.yaml", "looks: [panel]\n")
	slug = "own-table"
	e.publish(t, e.write(t, "own-table.yaml", "template: report\nlang: en\ntheme: washi\nsections:\n  - parts:\n"+
		"      - table: { text: mine }\n      - compare: { before: a, after: b, rows: [{ what: x, before: p, after: q }] }\n"),
		store.Options{Slug: &slug})
	b.navigate("/a/own-table/?raw")
	b.until("the compare", "!!document.querySelector('[data-floe-part=compare]')")
	got = b.eval(`['table', 'compare'].map((name) => {
		const s = getComputedStyle(document.querySelector('[data-floe-part=' + name + ']'));
		return s.backgroundColor + ' ' + s.borderTopStyle; }).join(' | ')`)
	check(t, "own table", []string{got}, "rgb(247, 241, 227) double | rgb(247, 241, 227) double")

	// A card takes the panel's look without being a part's box, a layer under the parts': the cards and
	// default's file for them keep it a face of the surface on the panel of default, the nearer scope though.
	slug = "cards"
	e.publish(t, e.write(t, "cards.yaml", "template: report\nlang: en\nsections:\n  - parts:\n"+
		"      - cards: [{ title: a }]\n      - steps: [{ title: b }]\n"), store.Options{Slug: &slug})
	b.navigate("/a/cards/?raw")
	b.until("the card", "!!document.querySelector('.card')")
	got = b.eval(`['.card', '[data-floe-part=steps]'].map((at) => {
		const s = getComputedStyle(document.querySelector(at)); return s.backgroundColor + ' ' + s.borderTopLeftRadius; }).join(' | ')`)
	check(t, "cards", []string{got}, "rgb(255, 255, 255) 14px | color(srgb 0.898235 0.914902 0.907451) 20px")
}
