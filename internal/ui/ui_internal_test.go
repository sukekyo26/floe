package ui

import (
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
)

// TestSourcesAreInEnglish keeps the files users and template authors read in English: their wording
// is in the catalogs, and their comments are English too.
func TestSourcesAreInEnglish(t *testing.T) {
	t.Parallel()

	for name, src := range builtinSources(t) {
		if japaneseRe.MatchString(src) {
			t.Errorf("%s has Japanese: %q", name, japaneseRe.FindAllString(src, 3))
		}
	}
}

// TestBuiltinsLoadNothingFromElsewhere keeps the management page and the built-in templates off the
// network: a page that loads from elsewhere is its template author's choice, not floe's.
func TestBuiltinsLoadNothingFromElsewhere(t *testing.T) {
	t.Parallel()

	for name, src := range builtinSources(t) {
		for _, url := range elsewhereRe.FindAllString(src, -1) {
			if !notFetched[url] {
				t.Errorf("%s names %s", name, url)
			}
		}
	}
}

// notFetched are the URLs the sources name only as identifiers.
var notFetched = map[string]bool{
	"http://www.w3.org/2000/svg":                   true,
	"https://json-schema.org/draft/2020-12/schema": true,
}

// builtinSources reads ui.html, the files of the built-in templates, the ones template authors start
// from, and the diagram assets every page with a diagram holds, by their names.
func builtinSources(t *testing.T) map[string]string {
	t.Helper()

	sources := map[string]string{"ui.html": pageSource}
	for _, dir := range []string{"tmpl/builtin", "diagram/assets"} {
		root := os.DirFS("../" + dir)
		err := fs.WalkDir(root, ".", func(name string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return err
			}
			b, err := fs.ReadFile(root, name)
			sources[dir+"/"+name] = string(b)
			return err
		})
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
	}
	if len(sources) < 3 {
		t.Fatalf("read %d files", len(sources))
	}
	return sources
}

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
	for _, lang := range i18n.Supported() {
		for key, format := range i18n.New(lang).Formats("ui_") {
			if !used[key] && lang == i18n.English {
				t.Errorf("%s is not used by ui.html", key)
			}
			if rest := jsVerbsRe.ReplaceAllString(format, ""); strings.Contains(rest, "%") {
				t.Errorf("%s %s: %q has a verb t() does not render", lang, key, format)
			}
		}
	}
	if !slices.Contains(i18n.Supported(), "ja") {
		t.Error("the Japanese catalog is not registered")
	}
}
