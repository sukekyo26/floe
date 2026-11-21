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

func TestPageRendersInTheReadersLanguage(t *testing.T) {
	t.Parallel()

	for lang, want := range map[i18n.Lang]string{"en": "Back to the page list", "ja": "ページ一覧へ戻る"} {
		b, err := Page(i18n.New(lang), List, "", false, "")
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

// builtinSources reads ui.html, the React build web/dist/index.html, floe's own templates, parts and themes (what template authors start from), and
// the zoom script and the diagram's SVG every page with a diagram or image holds, by their names. They are read from
// the source tree, not the embedded files: those of kit, zoom and diagram are not exported, and the tree holds all
// that is embedded.
func builtinSources(t *testing.T) map[string]string {
	t.Helper()

	sources := map[string]string{"ui.html": pageSource, "web/dist/index.html": reactSource}
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
	// usedRe finds the keys ui.html names: .Msg "ui_x", alone ({{.Msg "ui_x"}}), with arguments or as one
	// ({{.Msg "ui_x" (.Msg "ui_y")}}), t('ui_x') and the delete or unlink variants t(`ui_${kind(a)}_x`) and t(`ui_${now}_x`).
	usedRe     = regexp.MustCompile(`\.Msg "(ui_\w+)"|'(ui_\w+)'|` + "`ui_\\$\\{\\w+(?:\\(\\w*\\))?\\}(\\w*)`")
	jsVerbsRe  = regexp.MustCompile(`%(?:\[\d+\])?[sd%]`)
	japaneseRe = regexp.MustCompile(`.{0,20}[\p{Hiragana}\p{Katakana}\p{Han}]+.{0,20}`)
	// elsewhereRe finds URLs of other hosts, in full or scheme-relative ("//host", '//host', (//host
	// or =//host), wherever a page might load them from: an attribute, CSS or a script, its strings in any quotes.
	elsewhereRe = regexp.MustCompile("(?i)(?:https?:|[\"'(=`])//[^\\s\"'()<>`]+")
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
		b, err := Page(i18n.New(lang), List, "", false, "")
		if err != nil {
			t.Fatal(err)
		}
		if i := strings.Index(string(b), "%!"); i >= 0 {
			t.Errorf("%s: the page holds %q, which fmt writes for a verb it was not given words for", lang, string(b)[i:min(i+40, len(b))])
		}
	}
}

// The tasks' rows are asked of the server in one place each: read by readTasks, removed by forgetTask, and pages and tasks written by
// changeTasks (as every screen's write of a task or a page's task goes, its rev with it); the methods are the browser tests'. The set dialog
// has no field of a task's own, which its page writes.
func TestTaskRequests(t *testing.T) {
	t.Parallel()

	funcRe := regexp.MustCompile(`(?:async\s+)?function\s+(\w+)\(`)
	starts := funcRe.FindAllStringSubmatchIndex(pageSource, -1)
	inPage := func(path string) []string {
		named := regexp.MustCompile("['`]"+regexp.QuoteMeta(path)).FindAllStringIndex(pageSource, -1)
		got := make([]string, 0, len(named))
		for _, at := range named {
			in := ""
			for _, f := range starts {
				if f[0] < at[0] {
					in = pageSource[f[2]:f[3]]
				}
			}
			got = append(got, in)
		}
		slices.Sort(got)
		return got
	}
	if got, want := inPage("/api/tasks"), []string{"changeTasks", "forgetTask", "readTasks"}; !slices.Equal(got, want) {
		t.Errorf("/api/tasks is named in %v, want %v once each", got, want)
	}
	// LIMIT: the text /api/tasks alone is looked for in web/src: one made of parts ('/api/' + 'tasks') passes. Read the
	// TypeScript's tree if one comes to be written so.
	if got, want := namedIn(webSources(t), "/api/tasks"), []string{"lib/tasks.ts"}; !slices.Equal(got, want) {
		t.Errorf("/api/tasks is named in web/src's %v, want %v", got, want)
	}
	if got := inPage("/api/work"); len(got) > 0 {
		t.Errorf("/api/work, which is no more, is named in %v", got)
	}
	for _, id := range []string{`id="set-parent"`, `id="set-url"`} {
		if strings.Contains(pageSource, id) {
			t.Errorf("the set dialog has %s, a task's own field, which its page writes", id)
		}
	}
}

// TestPageScreens keeps the choice of each screen's HTML to reactScreens: the React build for a screen in it,
// ui.html for the others.
func TestPageScreens(t *testing.T) {
	t.Parallel()

	for _, screen := range []Screen{List, Tasks, Frame} {
		b, err := Page(i18n.New("en"), screen, "", false, "")
		if err != nil {
			t.Fatal(err)
		}
		react := strings.Contains(string(b), `id="boot">`)
		old := strings.Contains(string(b), `id="screens"`)
		if react != reactScreens[screen] || old == reactScreens[screen] {
			t.Errorf("%s: served the React build %t and ui.html %t; reactScreens says %t", screen, react, old,
				reactScreens[screen])
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
