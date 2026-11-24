package ui

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"floe/internal/files"
	"floe/internal/i18n"
	"floe/internal/store"
	"floe/internal/yamlpos"
)

// screens are every screen the server draws.
var screens = []Screen{List, Tasks, Frame}

// screenPage draws screen as the server does for a page a, in the language of c.
func screenPage(t *testing.T, c *i18n.Catalog, screen Screen) string {
	t.Helper()

	framed := ""
	if screen == Frame {
		framed = "a"
	}
	b, err := Page(c, screen, framed, false, "")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPageRendersInTheReadersLanguage(t *testing.T) {
	t.Parallel()

	for lang, want := range map[i18n.Lang]string{"en": "Back to the page list", "ja": "ページ一覧へ戻る"} {
		for _, screen := range screens {
			page := screenPage(t, i18n.New(lang), screen)
			if !strings.Contains(page, `<html lang="`+string(lang)) || !strings.Contains(page, want) ||
				!strings.Contains(page, `"ui_delete_done":`) {
				t.Errorf("%s, %s: the page lacks its language, %q or the script's messages", lang, screen, want)
			}
			if lang == "en" && japaneseRe.MatchString(page) {
				t.Errorf("%s: the English page has untranslated text: %q", screen, japaneseRe.FindAllString(page, 3))
			}
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
	if body.Kind != yamlpos.Mapping {
		t.Fatalf("%s is not a mapping", name)
	}
	english := commentRe.FindAllString(src, -1)
	for _, v := range body.Pairs {
		if i18n.Tag(v.Name()) != i18n.English || v.Value.Kind != yamlpos.Mapping {
			continue
		}
		for _, w := range v.Value.Pairs {
			text, _ := yamlpos.Text(w.Value)
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

// notFetched are the URLs the sources name only as identifiers: namespaces, and the links React's and Tailwind's
// builds name in an error and a license comment.
var notFetched = map[string]bool{
	"http://www.w3.org/2000/svg":                   true,
	"https://json-schema.org/draft/2020-12/schema": true,
	"http://www.w3.org/1998/Math/MathML":           true,
	"http://www.w3.org/1999/xlink":                 true,
	"http://www.w3.org/XML/1998/namespace":         true,
	"https://react.dev/errors/":                    true,
	"https://tailwindcss.com":                      true,
}

// builtinSources reads the React build web/dist/index.html, floe's own templates, parts and themes (what template authors start from), and
// the zoom script and the diagram's SVG every page with a diagram or image holds, by their names. They are read from
// the source tree, not the embedded files: those of kit, zoom and diagram are not exported, and the tree holds all
// that is embedded.
func builtinSources(t *testing.T) map[string]string {
	t.Helper()

	sources := map[string]string{"web/dist/index.html": reactSource}
	for _, dir := range []string{"kit/floe", "zoom"} {
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
	// reactUsedRe finds the keys a React source names: 'ui_x', "ui_x" (a prop, words="ui_cancel") and the delete or
	// unlink variants t(`ui_${kind}_x`).
	reactUsedRe = regexp.MustCompile(`'(ui_\w+)'|"(ui_\w+)"|` + "`ui_\\$\\{\\w+(?:\\(\\w*\\))?\\}(\\w*)`")
	jsVerbsRe   = regexp.MustCompile(`%(?:\[\d+\])?[sd%]`)
	japaneseRe  = regexp.MustCompile(`.{0,20}[\p{Hiragana}\p{Katakana}\p{Han}]+.{0,20}`)
	// elsewhereRe finds URLs of other hosts, in full or scheme-relative ("//host", '//host', (//host
	// or =//host), wherever a page might load them from: an attribute, CSS or a script, its strings in any quotes.
	elsewhereRe = regexp.MustCompile("(?i)(?:https?:|[\"'(=`])//[^\\s\"'()<>`]+")
)

// TestPageMessages keeps the React screens and the ui_ messages in step: every key they name exists, every
// ui_ message is used, and each uses only the verbs the page's t() renders.
func TestPageMessages(t *testing.T) {
	t.Parallel()

	formats := i18n.EnglishCatalog().Formats("ui_")
	used := map[string]bool{}
	// name takes the keys source names, by re, each a key of the catalog: the delete or unlink variants both
	name := func(file, source string, re *regexp.Regexp) {
		for _, m := range re.FindAllStringSubmatch(source, -1) {
			keys := []string{m[1] + m[2]}
			if m[1]+m[2] == "" {
				keys = []string{"ui_delete" + m[3], "ui_unlink" + m[3]}
			}
			for _, key := range keys {
				used[key] = true
				if _, ok := formats[key]; !ok {
					t.Errorf("%s names %s, which no catalog has", file, key)
				}
			}
		}
	}
	// Their sources but the generated list of every key and the tests; not the gallery, whose keys are no screen's. The
	// sources, not the build: they name the delete or unlink variants as `ui_${kind}_x`, which the minified build does not
	// LIMIT: a key in a comment is taken as used. Read the build instead if a key left in a comment comes to hide one unused
	src := files.NewRoot("web/src")
	t.Cleanup(func() {
		if err := src.Close(); err != nil {
			t.Error(err)
		}
	})
	err := fs.WalkDir(src, ".", func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(file, ".test.") || "web/src/"+file == messagesGen ||
			!strings.HasSuffix(file, ".ts") && !strings.HasSuffix(file, ".tsx") {
			return err
		}
		b, err := src.ReadFile(file)
		name("web/src/"+file, string(b), reactUsedRe)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// A state's word is named by its value in the table, as the page reads it.
	for _, state := range store.States {
		key := "ui_state_" + state.Name
		used[key] = true
		if _, ok := formats[key]; !ok {
			t.Errorf("the state %s has no word %s", state.Name, key)
		}
	}
	for _, lang := range i18n.Langs() {
		for key, format := range i18n.New(lang).Formats("ui_") {
			if !used[key] && lang == i18n.English {
				t.Errorf("%s is used by none of the React screens", key)
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

// A failure says what failed in one shape: "…に失敗しました：reason" and "Failed to …: reason", the reason in brackets when words follow it.
// The words of an old shape are left to the states named, which tell how things are and no try that failed.
func TestFailureWords(t *testing.T) {
	t.Parallel()

	states := []string{
		"ui_server_down", "ui_server_down_host", "ui_load_slow", "ui_version_gone", "ui_diff_bad_url", "ui_tasks_unreadable",
		"ui_task_none_kids", "ui_category_drop_paged",
	}
	old := map[i18n.Lang]*regexp.Regexp{"ja": regexp.MustCompile(`ませんでした|めません`), "en": regexp.MustCompile(`(?i)could not|cannot`)}
	shape := map[i18n.Lang]*regexp.Regexp{
		"ja": regexp.MustCompile(`に失敗しました(：%[0-9\[\]]*[a-z]|（%[0-9\[\]]*[a-z]）。.+)$`),
		"en": regexp.MustCompile(`^Failed to .*(: %[0-9\[\]]*[a-z]| \(%[0-9\[\]]*[a-z]\)\. .+)$`),
	}
	for _, lang := range i18n.Langs() {
		formats := i18n.New(lang).Formats("ui_")
		for _, key := range states {
			if _, ok := formats[key]; !ok {
				t.Errorf("%s: the state %s is not in the catalog", lang, key)
			}
		}
		for key, format := range formats {
			if old[lang].MatchString(format) && !slices.Contains(states, key) {
				t.Errorf("%s %s: %q tells a failure in an old shape", lang, key, format)
			}
			if strings.HasSuffix(key, "_fail") {
				t.Errorf("%s %s: a failure's key ends in _failed", lang, key)
			}
			if strings.HasSuffix(key, "_failed") && !shape[lang].MatchString(format) {
				t.Errorf("%s %s: %q is not of the shape %s", lang, key, format, shape[lang])
			}
		}
	}
}

// A message that takes words (the help's states) is given as many as it has verbs, in every language: fmt writes %! where it is not.
func TestPageFormats(t *testing.T) {
	t.Parallel()

	for _, lang := range i18n.Langs() {
		for _, screen := range screens {
			page := screenPage(t, i18n.New(lang), screen)
			if i := strings.Index(page, "%!"); i >= 0 {
				t.Errorf("%s, %s: the page holds %q, which fmt writes for a verb it was not given words for", lang, screen, page[i:min(i+40, len(page))])
			}
		}
	}
}

// The tasks' rows are asked of the server in one place, web/src's lib/tasks.ts, as every screen's write of a task or a
// page's task goes, its rev with it.
func TestTaskRequests(t *testing.T) {
	t.Parallel()

	// LIMIT: the text /api/tasks alone is looked for in web/src: one made of parts ('/api/' + 'tasks') passes. Read the
	// TypeScript's tree if one comes to be written so.
	if got, want := namedIn(webSources(t), "/api/tasks"), []string{"lib/tasks.ts"}; !slices.Equal(got, want) {
		t.Errorf("/api/tasks is named in web/src's %v, want %v", got, want)
	}
}

// TestPageScreens keeps each screen served from the React build, its boot values naming that screen.
func TestPageScreens(t *testing.T) {
	t.Parallel()

	for _, screen := range screens {
		b, err := Page(i18n.New("en"), screen, "", false, "")
		if err != nil {
			t.Fatal(err)
		}
		_, rest, _ := strings.Cut(string(b), `id="boot">`)
		j, _, _ := strings.Cut(rest, "</script>")
		var got boot
		if err := json.Unmarshal([]byte(j), &got); err != nil || got.Screen != screen {
			t.Errorf("%s: served boot values of the screen %q (%v)", screen, got.Screen, err)
		}
	}
}

func TestNewBoot(t *testing.T) {
	t.Parallel()

	for _, lang := range []i18n.Lang{"en", "ja"} {
		c := i18n.New(lang)
		got := newBoot(c, Frame, "a", true, "3")
		want := boot{
			Screen: Frame, Lang: lang, Messages: c.Formats("ui_"), Framed: "a", Served: true, Version: "3",
			States: store.States, MaxURLs: store.MaxURLs,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: newBoot gave %+v, want %+v", lang, got, want)
		}
	}
}

// TestReactPage fills the marks of the React build: the reader's language, and the boot values as JSON that no
// text of theirs closes or breaks.
func TestReactPage(t *testing.T) {
	t.Parallel()

	b := boot{
		Screen: Tasks, Lang: "ja", Messages: map[string]string{"x": "</script><b>", "y": "a b"}, Framed: "f",
		Served: true, Version: "2", States: store.States, MaxURLs: 7,
	}
	out, err := reactPage(b)
	if err != nil {
		t.Fatal(err)
	}
	page := string(out)
	if !strings.Contains(page, `<html lang="ja">`) || strings.Contains(page, langMark) {
		t.Error("the page's lang is not the reader's")
	}
	if got, want := strings.Count(page, "</script>"), strings.Count(reactSource, "</script>"); got != want {
		t.Errorf("the page has %d </script>, the build %d: a value closed the script", got, want)
	}
	if strings.ContainsRune(page, ' ') {
		t.Error("the page holds a raw U+2028")
	}
	_, rest, _ := strings.Cut(page, `<script type="application/json" id="boot">`)
	j, _, _ := strings.Cut(rest, "</script>")
	var read boot
	if err := json.Unmarshal([]byte(j), &read); err != nil {
		t.Fatalf("the boot values are not JSON: %v", err)
	}
	// Open is the server's own (json:"-"), which the page is not told
	want := b
	want.States = slices.Clone(b.States)
	for i := range want.States {
		want.States[i].Open = false
	}
	if !reflect.DeepEqual(read, want) {
		t.Errorf("the boot values read %+v, want %+v", read, want)
	}
}

// The lang is written into an attribute escaped, whatever language a catalog comes to have.
func TestReactPageLangEscaped(t *testing.T) {
	t.Parallel()

	out, err := reactPage(boot{Lang: `x"><b>`})
	if err != nil {
		t.Fatal(err)
	}
	if page := string(out); strings.Contains(page, `x"><b>`) || !strings.Contains(page, `<html lang="x&#34;&gt;&lt;b&gt;">`) {
		t.Error("the lang is not escaped in the attribute")
	}
}

func TestCutPage(t *testing.T) {
	t.Parallel()

	if _, err := cutPage("a" + langMark + "b" + bootMark + "c"); err != nil {
		t.Errorf("a page with each mark once: %v", err)
	}
	for name, src := range map[string]string{
		"no lang":    "a" + bootMark,
		"no boot":    langMark + "a",
		"two langs":  langMark + langMark + bootMark,
		"two boots":  langMark + bootMark + bootMark,
		"boot first": bootMark + langMark,
	} {
		if _, err := cutPage(src); err == nil {
			t.Errorf("%s: cut with no error", name)
		}
	}
}
