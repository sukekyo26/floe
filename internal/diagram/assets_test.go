package diagram_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/page"
)

func TestAssets(t *testing.T) {
	t.Parallel()

	got, err := diagram.Assets()
	if err != nil {
		t.Fatal(err)
	}
	// Pasted into body-only HTML, the assets must not make it read as a whole document, or the page
	// would lose the template (and its CSS variables) around it.
	if page.IsDocument(got) {
		t.Error("Assets() reads as a whole HTML document")
	}
	for _, w := range []string{`<script>(// `, ");</script>\n<style>\n", ".dg .box rect", "</style>"} {
		if !strings.Contains(got, w) {
			t.Errorf("Assets() does not contain %s", w)
		}
	}
	// The script is called with the toolbar's wording: every language's diagram_zoom_ messages.
	end := strings.Index(got, ");</script>")
	var messages map[i18n.Lang]map[string]string
	if err := json.Unmarshal([]byte(got[strings.LastIndex(got[:end], "})(")+3:end]), &messages); err != nil {
		t.Fatalf("the script's messages: %v", err)
	}
	for _, lang := range i18n.Supported() {
		if want := i18n.New(lang).Formats("diagram_zoom_"); !reflect.DeepEqual(messages[lang], want) {
			t.Errorf("messages in %s = %v, want %v", lang, messages[lang], want)
		}
	}
	// Pages carry these files as they are, so they are written in English.
	for _, f := range []string{"assets/zoom.js", "assets/diagram.css"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if i := strings.IndexFunc(string(b), func(r rune) bool { return r > unicode.MaxASCII }); i >= 0 {
			t.Errorf("%s is not in plain English at byte %d", f, i)
		}
	}
}
