package server_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// TestUILayers checks, in the browser, the layers floe puts a page's CSS in: the template's page.css,
// though its selectors are the more specific, loses to the part's style and to the parts theme's, and
// still styles what neither of them does.
func TestUILayers(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	for name, body := range map[string]string{
		"schema.json": `{"type": "object", "properties": {"note": {"$ref": "floe:///parts/note.json"}}, "additionalProperties": false}`,
		"page.html":   `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body>{{.Data.note}}</body></html>`,
		"page.css": "body div.floe-note.warn { background-color: rgb(255, 0, 0); margin-top: 7px; }\n" +
			"body div.floe-note.warn::before { top: 0px; }\n",
		"themes/default.css": ":root { --fg: #000; }\n",
	} {
		e.write(t, "templates/layers/"+name, body)
	}
	slug := "layers"
	e.publish(t, e.write(t, "layers.yaml", "template: layers\nlang: en\nnote: { text: x, tone: warn }\n"), store.Options{Slug: &slug})
	b.navigate("/a/layers/?raw")
	b.until("the note", "!!document.querySelector('.floe-note')")
	// The part's background, the parts theme's top of the mark (0.85rem, over the part's 1.1rem), the template's margin.
	got := b.eval(`(() => { const n = document.querySelector('.floe-note'), s = getComputedStyle(n);
		return [s.backgroundColor, getComputedStyle(n, '::before').top, s.marginTop].join(' ') })()`)
	check(t, "layers", []string{got}, "rgb(246, 232, 210) 13.6px 7px")
}
