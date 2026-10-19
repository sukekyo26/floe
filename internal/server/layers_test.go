package server_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// TestUILayers checks, in the browser, how floe puts a page's CSS together. The template's page.css,
// though its selectors are the more specific, loses to the part's style and to the parts theme's, and
// still styles what neither of them does, such as where a part stands. It reaches the part's element but
// not what the part holds, whose colour it gives by inheritance alone. Its @keyframes and @font-face,
// in floe's @scope, work. A block page.css leaves open does not take in the theme.
func TestUILayers(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	for name, body := range map[string]string{
		"schema.json": `{"type": "object", "properties": {"note": {"$ref": "floe:///parts/note.json"}}, "additionalProperties": false}`,
		"page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
			`<body><header>h</header><p id="own">own</p>{{.Data.note}}</body></html>`,
		"page.css": "p { color: rgb(255, 0, 0); }\n" +
			"body div.floe-note.warn { background-color: rgb(255, 0, 0); margin-top: 7px; }\n" +
			"body div.floe-note.warn::before { top: 0px; }\n" +
			"@keyframes held { from { opacity: 0.5; } to { opacity: 0.5; } }\nheader { animation: held 1000s; }\n" +
			`@font-face { font-family: "own face"; src: url(data:,); }` + "\nfooter { color: red;",
		"themes/default.css": "body { color: rgb(0, 0, 2); }\n",
	} {
		e.write(t, "templates/layers/"+name, body)
	}
	slug := "layers"
	e.publish(t, e.write(t, "layers.yaml", "template: layers\nlang: en\nnote: { text: x, tone: warn }\n"), store.Options{Slug: &slug})
	b.navigate("/a/layers/?raw")
	b.until("the note", "!!document.querySelector('.floe-note')")
	// The part's background, the parts theme's top of the mark (0.85rem, over the part's 1.1rem), the
	// template's margin; the template's own paragraph and the note's; the animation and the face.
	got := b.eval(`(() => { const n = document.querySelector('.floe-note'), s = getComputedStyle(n);
		return [s.backgroundColor, getComputedStyle(n, '::before').top, s.marginTop,
			getComputedStyle(document.getElementById('own')).color, getComputedStyle(n.querySelector('p')).color,
			document.querySelector('header').getAnimations()[0]?.animationName,
			[...document.fonts].some((f) => f.family.replaceAll('"', '') === 'own face')].join(' | ') })()`)
	check(t, "layers", []string{got}, "rgb(246, 232, 210) | 13.6px | 7px | rgb(255, 0, 0) | rgb(0, 0, 2) | held | true")
}
