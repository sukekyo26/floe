package ui

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
)

func TestPageRendersInTheReadersLanguage(t *testing.T) {
	t.Parallel()

	for lang, want := range map[i18n.Lang]string{"en": "Back to the list", "ja": "一覧へ戻る"} {
		b, err := Page(i18n.New(lang))
		if err != nil {
			t.Fatal(err)
		}
		page := string(b)
		if !strings.Contains(page, `<html lang="`+string(lang)+`">`) || !strings.Contains(page, want) ||
			!strings.Contains(page, `"ui_delete_done":`) || strings.Contains(page, "{{") {
			t.Errorf("%s: the page lacks its language, %q or the script's messages", lang, want)
		}
		// html/template drops the comments, so Japanese left in the English page is untranslated.
		if lang == "en" && japaneseRe.MatchString(page) {
			t.Errorf("the English page has untranslated text: %q", japaneseRe.FindAllString(page, 3))
		}
	}
}

// ui.html is read by users: its words are in the catalogs, and its comments are English.
func TestSourceIsInEnglish(t *testing.T) {
	t.Parallel()

	if japaneseRe.MatchString(pageSource) {
		t.Errorf("ui.html has Japanese: %q", japaneseRe.FindAllString(pageSource, 3))
	}
}

// The management page loads nothing from elsewhere.
func TestLoadsNothingFromElsewhere(t *testing.T) {
	t.Parallel()

	for _, url := range elsewhereRe.FindAllString(pageSource, -1) {
		if url != "http://www.w3.org/2000/svg" {
			t.Errorf("ui.html names %s", url)
		}
	}
}

var (
	// usedRe finds the keys ui.html names: {{.Msg "ui_x"}}, t('ui_x') and the delete or unlink
	// variants t(`ui_${kind(a)}_x`).
	usedRe     = regexp.MustCompile(`\{\{\.Msg "(ui_\w+)"\}\}|'(ui_\w+)'|` + "`ui_\\$\\{\\w+\\(\\w*\\)\\}(\\w*)`")
	jsVerbsRe  = regexp.MustCompile(`%(?:\[\d+\])?[sd%]`)
	japaneseRe = regexp.MustCompile(`.{0,20}[\p{Hiragana}\p{Katakana}\p{Han}]+.{0,20}`)
	// elsewhereRe finds URLs of other hosts, in full or scheme-relative ("//host", '//host', (//host
	// or =//host), wherever a page might load them from: an attribute, CSS or a script.
	elsewhereRe = regexp.MustCompile(`(?i)(?:https?:|["'(=])//[^\s"'()<>]+`)
)

// TestPageMessages keeps ui.html and the ui_ messages in step: every key the page names exists, every
// ui_ message is used, and each uses only the verbs the page's t() renders.
func TestPageMessages(t *testing.T) {
	t.Parallel()

	formats := i18n.EnglishCatalog().Formats("ui_")
	used := map[string]bool{}
	for _, m := range usedRe.FindAllStringSubmatch(pageSource, -1) {
		keys := []string{m[1] + m[2]}
		if m[1]+m[2] == "" {
			keys = []string{"ui_delete" + m[3], "ui_unlink" + m[3]}
		}
		for _, key := range keys {
			used[key] = true
			if _, ok := formats[key]; !ok {
				t.Errorf("ui.html names %s, which no catalog has", key)
			}
		}
	}
	for _, lang := range i18n.Langs() {
		for key, format := range i18n.New(lang).Formats("ui_") {
			if !used[key] && lang == i18n.English {
				t.Errorf("%s is not used by ui.html", key)
			}
			if rest := jsVerbsRe.ReplaceAllString(format, ""); strings.Contains(rest, "%") {
				t.Errorf("%s %s: %q has a verb t() does not render", lang, key, format)
			}
		}
	}
	if !slices.Contains(i18n.Langs(), "ja") {
		t.Error("the Japanese catalog is not registered")
	}
}
