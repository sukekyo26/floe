package diagram

import (
	_ "embed" // the zoom script and the diagrams' style
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

var (
	//go:embed assets/zoom.js
	zoomJS string
	//go:embed assets/diagram.css
	diagramCSS string
)

// Assets returns what a page holds once, before or after its diagrams: the style that colours them
// from the page's CSS variables, and the script that zooms them, its toolbar worded in every
// language with a catalog for the page's <html lang> to pick from. Holding it twice does no harm.
func Assets() (string, error) {
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
