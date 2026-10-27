package ui

import (
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/yamlpos"
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

// TestSourcesAreInEnglish keeps the files users and template authors read in English: their wording
// is in the catalogs, and their comments are English too. A part's words.yaml holds its words in every
// language, as a catalog does: its comment lines and its en, which a page of a language it has no words
// of is drawn in, are kept in English.
func TestSourcesAreInEnglish(t *testing.T) {
	t.Parallel()

	for name, src := range builtinSources(t) {
		if isWords(name) {
			src = englishOfWords(t, name, src)
		}
		if japaneseRe.MatchString(src) {
			t.Errorf("%s has Japanese: %q", name, japaneseRe.FindAllString(src, 3))
		}
	}
}

// isWords reports whether name is a words.yaml kit reads as one: right under a part or a template. Another file of
// that name (a template's asset) is held to English as a whole.
func isWords(name string) bool {
	for _, kind := range []string{"parts", "templates"} {
		if ok, _ := path.Match("kit/floe/"+kind+"/*/words.yaml", name); ok { //nolint:errcheck // the pattern is right
			return true
		}
	}
	return false
}

// englishOfWords returns what of src, the words.yaml file name, must be English: its comments, at the
// start of a line or after a space, and the words of en, its tag read as kit reads it (i18n.Tag).
func englishOfWords(t *testing.T, name, src string) string {
	t.Helper()

	body, err := yamlpos.Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	top, ok := body.(*ast.MappingNode)
	if !ok {
		t.Fatalf("%s is not a mapping", name)
	}
	english := commentRe.FindAllString(src, -1)
	for _, v := range top.Values {
		words, ok := v.Value.(*ast.MappingNode)
		if i18n.Tag(yamlpos.KeyName(v)) != i18n.English || !ok {
			continue
		}
		for _, w := range words.Values {
			text, _, err := yamlpos.Text(w.Value)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			english = append(english, text)
		}
	}
	return strings.Join(english, "\n")
}

// commentRe finds a comment of YAML: # at the start of a line or after a space, to the end of the line.
var commentRe = regexp.MustCompile(`(?m)(?:^|\s)#.*$`)

// TestBuiltinsLoadNothingFromElsewhere keeps the management page and floe's own templates, parts and themes off
// the network: a page that loads from elsewhere is its template author's choice, not floe's.
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

// builtinSources reads ui.html, floe's own templates, parts and themes (what template authors start from), and
// the diagram's script and SVG every page with a diagram holds, by their names. They are read from the source
// tree, not the embedded files: those of kit and diagram are not exported, and the tree holds all that is embedded.
func builtinSources(t *testing.T) map[string]string {
	t.Helper()

	sources := map[string]string{"ui.html": pageSource}
	for _, dir := range []string{"kit/floe", "diagram/assets"} {
		root := os.DirFS("../" + dir)
		n := 0
		err := fs.WalkDir(root, ".", func(name string, e fs.DirEntry, err error) error {
			switch {
			case err != nil:
				return err
			// What embed leaves out (an editor's .part.css.swp) is not read.
			case name != "." && (files.Hidden(e.Name()) || strings.HasPrefix(e.Name(), "_")):
				if e.IsDir() {
					return fs.SkipDir
				}
				return nil
			case e.IsDir():
				return nil
			}
			b, err := fs.ReadFile(root, name)
			sources[dir+"/"+name] = string(b)
			n++
			return err
		})
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		if n == 0 {
			t.Fatalf("read no file under %s", dir)
		}
	}
	b, err := os.ReadFile("../diagram/diagram.svg.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	sources["diagram/diagram.svg.tmpl"] = string(b)
	return sources
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
