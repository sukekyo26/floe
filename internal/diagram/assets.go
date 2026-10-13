package diagram

import (
	_ "embed" // the zoom script and the diagrams' style
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/sukekyo26/floe/internal/i18n"
)

var (
	//go:embed assets/zoom.js
	zoomJS string
	//go:embed assets/diagram.css
	diagramCSS string
)

// Variables are the CSS variables of the page that diagrams take their colours and fonts from, by
// name (--fg): those diagram.css falls back from. A page without one draws its diagrams in light
// colours that dark mode does not change.
var Variables = sync.OnceValue(func() []string {
	found := regexp.MustCompile(`var\(\s*(--[\w-]+)\s*,`).FindAllStringSubmatch(diagramCSS, -1)
	names := make([]string, 0, len(found))
	for _, m := range found {
		names = append(names, m[1])
	}
	return names
})

// Assets returns what a page holds once, before or after its diagrams: the style that colours them
// from the page's CSS variables, and the script that zooms them, its toolbar worded in every
// language with a catalog for the page's <html lang> to pick from. Holding it twice does no harm.
var Assets = sync.OnceValues(assets)

func assets() (string, error) {
	messages := map[i18n.Lang]map[string]string{}
	for _, lang := range i18n.Supported() {
		messages[lang] = i18n.New(lang).Formats("diagram_zoom_")
	}
	// json.Marshal writes <, > and & as \u escapes, so the messages cannot end the script.
	words, err := json.Marshal(messages)
	if err != nil {
		return "", fmt.Errorf("write the zoom messages: %w", err)
	}
	script := "<script>(" + strings.TrimSpace(zoomJS) + ")(" + string(words) + ");</script>"
	return script + "\n<style>\n" + diagramCSS + "</style>", nil
}
