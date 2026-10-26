package yamlpos

import (
	"errors"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

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
