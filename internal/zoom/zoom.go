// Package zoom holds the script that zooms a diagram or an image inside its frame, and the classes it works on.
package zoom

import (
	_ "embed" // the script
	"html/template"
	"sync"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/page"
)

//go:embed zoom.js
var js string

// The classes of a frame that zooms what it holds, which the part writes, and of its toolbar, which the script puts
// before it. The look zoom styles them.
const (
	Frame = "floe-zoom"
	Tools = "floe-zoom-tools"
)

// Roots selects a frame, its toolbar and the line under it, the roots of what the look zoom styles.
const Roots = "." + Tools + ", ." + Frame + ", ." + Frame + "-hint"

// keys are the words of the toolbar and the frame.
var keys = []string{
	"zoom_hint", "zoom_in", "zoom_out", "zoom_fit", "zoom_fit_label", "zoom_one", "zoom_one_label", "zoom_frame",
}

// Script is the script that zooms the frames of a page, its words in every language for the page's <html lang> to
// pick from. A page may hold it more than once: a second run does nothing to a frame the first set up.
var Script = sync.OnceValues(func() (template.HTML, error) {
	words := map[i18n.Lang]map[string]string{}
	for _, lang := range i18n.Langs() {
		c := i18n.New(lang)
		words[lang] = map[string]string{}
		for _, k := range keys {
			words[lang][k] = c.Msg(k)
		}
	}
	s, err := page.Script(js, map[string]any{"words": words, "frame": Frame, "tools": Tools})
	//nolint:gosec // G203: floe's script, its words escaped as JSON
	return template.HTML(s), err
})
