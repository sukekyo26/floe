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

// Read moves the shared keys out of the values, each where it is written: its value, its key when the value is
// left out, the top of the file when the key is.
func TestRead(t *testing.T) {
	t.Parallel()

	d, err := doc.Read("a.yaml", []byte("# head\ntemplate: report\nlang: ja\ntheme:\ntitle: '  Plan  '\nx: 1\n"))
	if err == nil {
		t.Fatalf("Read of a theme left empty = %+v, want its mistake", d)
	}
	if got := usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "); got != "a.yaml:4:1: theme must be text" {
		t.Errorf("told %q", got)
	}
	d, err = doc.Read("a.yaml", []byte("# head\ntemplate: report\nlang: ja\ntitle: '  Plan  '\nx: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Template != "report" || d.Lang != "ja" || d.Theme != "" || d.Title != "Plan" || d.Root == nil {
		t.Errorf("Read = %+v", d)
	}
	if !maps.Equal(d.Values, map[string]any{"x": 1}) {
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
	d, err := doc.Read("a.yaml", []byte(bom+bom+"lang: ja\n"))
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
		{"lang:\n", "a.yaml:1:1: lang must be text"},
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
		_, err := doc.Read("a.yaml", []byte(tt.src))
		got := ""
		if err != nil {
			got = usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
		}
		if got != tt.want {
			t.Errorf("Read(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
}

// A whole number above 0 that fits an int is read as one, in lists and keys and values too, so that expr
// does not wrap it around when it negates it; one past it stays uint64, and one below 0 int64, as goccy reads it.
func TestReadInts(t *testing.T) {
	t.Parallel()

	d, err := doc.Read("a.yaml", []byte("lang: en\na: 3\nb: [1, {c: 2}]\nbig: 18446744073709551615\nneg: -3\nf: 1.5\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"a": 3, "b": []any{1, map[string]any{"c": 2}}, "big": uint64(18446744073709551615), "neg": int64(-3), "f": 1.5,
	}
	if !reflect.DeepEqual(d.Values, want) {
		t.Errorf("Values = %#v, want %#v", d.Values, want)
	}
}
