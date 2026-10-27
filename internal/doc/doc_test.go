package doc_test

import (
	"maps"
	"reflect"
	"testing"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

const noLang = "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)"

// Every key of Shared written as text is read into its field: a key added to Shared without a field fails here.
func TestReadShared(t *testing.T) {
	t.Parallel()

	src := ""
	for _, key := range doc.Shared {
		src += key + ": ja\n"
	}
	d, err := doc.Read("a.yaml", []byte(src), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Lang != "ja" || d.Template != "ja" || d.Theme != "ja" || d.Title != "ja" {
		t.Errorf("Read = %+v", d)
	}
	for i, key := range doc.Shared {
		if want := (yamlpos.Pos{Line: i + 1, Column: len(key) + 3}); d.Where[key] != want {
			t.Errorf("Where[%s] = %v, want %v", key, d.Where[key], want)
		}
	}
}

// Read moves the shared keys out of the values, each where it is written: its value, its key when the value is
// left out, the top of the file when the key is.
func TestRead(t *testing.T) {
	t.Parallel()

	// A null value is none written: left out of the values, told at its key when left empty.
	d, err := doc.Read("a.yaml", []byte("# head\ntemplate: report\nlang: ja\ntheme:\ntitle: ~\nx: 1\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Values["theme"]; ok || d.Theme != "" || d.Title != "" || d.Where["theme"] != (yamlpos.Pos{Line: 4, Column: 1}) ||
		d.Where["title"] != (yamlpos.Pos{Line: 5, Column: 8}) {
		t.Errorf("Read of a theme left empty and a title ~ = %+v", d)
	}
	// A theme given is read as one written, trimmed and on no line; one of spaces is none given.
	d, err = doc.Read("a.yaml", []byte("lang: ja\ntheme: ink\n"), " soft ")
	if err != nil || d.Theme != "soft" || d.Values["theme"] != "soft" || d.Where["theme"] != (yamlpos.Pos{}) {
		t.Errorf("Read with a theme given = %+v, %v", d, err)
	}
	d, err = doc.Read("a.yaml", []byte("lang: ja\ntheme: ' ink '\n"), " ")
	if err != nil || d.Theme != "ink" || d.Values["theme"] != "ink" || d.Where["theme"] != (yamlpos.Pos{Line: 2, Column: 8}) {
		t.Errorf("Read with spaces given = %+v, %v", d, err)
	}
	// So is one empty or of spaces.
	d, err = doc.Read("a.yaml", []byte("template: ' '\nlang: ja\ntheme: ''\ntitle: '  '\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(d.Values, map[string]any{"lang": "ja"}) || d.Template != "" || d.Theme != "" || d.Title != "" {
		t.Errorf("Read of shared keys empty or of spaces = %+v", d)
	}
	d, err = doc.Read("a.yaml", []byte("# head\ntemplate: report\nlang: ja\ntitle: '  Plan  '\nx: 1\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Template != "report" || d.Lang != "ja" || d.Theme != "" || d.Title != "Plan" || d.Root == nil {
		t.Errorf("Read = %+v", d)
	}
	if !maps.Equal(d.Values, map[string]any{"template": "report", "lang": "ja", "title": "Plan", "x": 1}) {
		t.Errorf("Values = %#v", d.Values)
	}
	want := map[string]yamlpos.Pos{"template": {Line: 2, Column: 11}, "lang": {Line: 3, Column: 7}, "theme": {Line: 2, Column: 1}, "title": {Line: 4, Column: 8}}
	if !maps.Equal(d.Where, want) {
		t.Errorf("Where = %v, want %v", d.Where, want)
	}
}

// Only the first byte order mark is left out: a second stays in the first key (LIMIT: in yamlpos).
func TestReadSecondBOM(t *testing.T) {
	t.Parallel()

	const bom = "\xef\xbb\xbf"
	d, err := doc.Read("a.yaml", []byte(bom+bom+"lang: ja\n"), "")
	if err == nil {
		t.Fatalf("Read = %+v, want its mistake", d)
	}
	if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); got != "a.yaml:1:1: "+noLang {
		t.Errorf("told %q", got)
	}
	if want := map[string]any{bom + "lang": "ja"}; !maps.Equal(d.Values, want) {
		t.Errorf("Values = %q, want %q", d.Values, want)
	}
}

// The mistakes of the shared keys are told together; a mistake of the walk or of reading the values stops
// before them; an empty file reads as {}, its mistakes at 1:1.
func TestReadMistakes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"template: report\ntitle: 3\n", "a.yaml:1:1: " + noLang + "\na.yaml:2:8: title must be text"},
		{"lang: \"\"\n", "a.yaml:1:7: " + noLang},
		{"lang: \"  \"\n", "a.yaml:1:7: " + noLang},
		{"lang:\n", "a.yaml:1:1: " + noLang},
		{"lang: ~\n", "a.yaml:1:7: " + noLang},
		{"lang: null\ntemplate: ~\ntitle: ~\n", "a.yaml:1:7: " + noLang},
		{"title: 3\ntheme: 4\nlang: 5\n", "a.yaml:1:8: title must be text\na.yaml:2:8: theme must be text\na.yaml:3:7: lang must be text"},
		{"", "a.yaml:1:1: " + noLang},
		{"# just a note\n", "a.yaml:1:1: " + noLang},
		{"&top\ntitle: 3\n", "a.yaml:1:1: anchors, aliases, merge keys (<<) and tags are not supported; write the value out"},
		{"- a\n- b\n", "a.yaml:1:1: sequence was used where mapping is expected"},
		{"~\n", "a.yaml:1:1: null was used where mapping is expected"},
		// The byte order mark at the start is no part of the first key.
		{"\xef\xbb\xbflang: en\ntitle: 1\n", "a.yaml:2:8: title must be text"},
		{"\xef\xbb\xbftitle: 1\nlang: en\n", "a.yaml:1:8: title must be text"},
	} {
		_, err := doc.Read("a.yaml", []byte(tt.src), "")
		got := ""
		if err != nil {
			got = usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
		}
		if got != tt.want {
			t.Errorf("Read(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
}

// A whole number that fits an int is read as one, in lists and keys and values too, above 0 (which goccy reads as
// uint64, which expr wraps around when it negates it) and below 0 (int64); one past it stays uint64.
func TestReadInts(t *testing.T) {
	t.Parallel()

	d, err := doc.Read("a.yaml", []byte("lang: en\na: 3\nb: [1, {c: 2}]\nbig: 18446744073709551615\nneg: -3\nf: 1.5\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"lang": "en", "a": 3, "b": []any{1, map[string]any{"c": 2}}, "big": uint64(18446744073709551615), "neg": -3,
		"f": 1.5,
	}
	if !reflect.DeepEqual(d.Values, want) {
		t.Errorf("Values = %#v, want %#v", d.Values, want)
	}
}

// A shared key Read refuses is left in the values as written, and Read returns the data with its mistakes, for
// the schema to be validated in the same run; the checks of the stage after it do not run on them.
func TestReadKeepsRefused(t *testing.T) {
	t.Parallel()

	d, err := doc.Read("a.yaml", []byte("lang: en\ntitle: 3\na: x\n"), "")
	if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); got != "a.yaml:2:8: title must be text" {
		t.Errorf("told %q", got)
	}
	if d == nil || d.Values["title"] != 3 || d.Values["a"] != "x" {
		t.Errorf("Read = %+v", d)
	}
}

// A theme given is drawn with: theme: is neither read nor refused, and the values hold the theme given.
func TestReadThemeGiven(t *testing.T) {
	t.Parallel()

	d, err := doc.Read("a.yaml", []byte("lang: ja\ntemplate: report\ntheme: 5\n"), "ink")
	if err != nil {
		t.Fatal(err)
	}
	if d.Theme != "ink" || d.Values["theme"] != "ink" {
		t.Errorf("Theme = %q, Values[theme] = %#v, want ink", d.Theme, d.Values["theme"])
	}
}
