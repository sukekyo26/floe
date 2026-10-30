package yamlpos

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// starts finds each token at the byte its text begins at, and gives up on the file when a token's text is not
// there, as it cannot tell where that token begins: goccy/go-yaml makes no such token, so one is made here.
func TestStarts(t *testing.T) {
	t.Parallel()

	src := []byte("a: 1\nb: 2\n")
	tks := lexer.Tokenize(string(src))
	if got, want := starts(src, tks), []int{0, 1, 3, 5, 6, 8}; !slices.Equal(got, want) {
		t.Errorf("starts = %v, want %v", got, want)
	}
	tks[3].Origin = "\nc"
	if got := starts(src, tks); got != nil {
		t.Errorf("starts of a token not there = %v, want nil", got)
	}
}

// Every token of floe's own YAML is found: a goccy/go-yaml whose Origin comes to hold another text fails here.
func TestStartsFloe(t *testing.T) {
	t.Parallel()

	err := filepath.WalkDir("../kit/floe", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if starts(src, lexer.Tokenize(string(src))) == nil {
			t.Errorf("starts does not find every token of %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// library places goccy/go-yaml's words at its token, without a place when it has none, and returns any other
// error as it is.
func TestLibrary(t *testing.T) {
	t.Parallel()

	told := func(err error) string { return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ") }
	var m map[string]any
	err := yaml.Unmarshal([]byte("- a\n"), &m)
	if got := told(library("a.yaml", err)); got != "a.yaml:1:1: sequence was used where mapping is expected" {
		t.Errorf("library of a mistake at a token = %q", got)
	}
	if got := told(library("a.yaml", &yaml.SyntaxError{Message: "bad", Token: nil})); got != "a.yaml: bad" {
		t.Errorf("library of a mistake without a token = %q", got)
	}
	failure := errors.New("disk on fire")
	if got := library("a.yaml", failure); !errors.Is(got, failure) {
		t.Errorf("library of a failure = %v", got)
	}
}
