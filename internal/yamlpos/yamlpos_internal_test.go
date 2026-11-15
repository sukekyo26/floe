package yamlpos

import (
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"floe/internal/i18n"
	"floe/internal/usererr"
)

// library places go.yaml.in/yaml/v4's words at its line and column, or at its byte when it gives no line, and
// returns any other error as floe's failure.
func TestLibrary(t *testing.T) {
	t.Parallel()

	told := func(err error) string { return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ") }
	src := []byte("a: 1\nb: x\n")
	at := &yaml.LoadError{Message: "bad", Mark: yaml.Mark{Index: 0, Line: 2, Column: 4}}
	if got := told(library("a.yaml", src, at)); got != "a.yaml:2:4: bad" {
		t.Errorf("library of a mistake at a mark = %q", got)
	}
	byIndex := &yaml.LoadError{Message: "bad", Mark: yaml.Mark{Index: 8, Line: 0, Column: 0}}
	if got := told(library("a.yaml", src, byIndex)); !strings.HasPrefix(got, "a.yaml:2:4: ") {
		t.Errorf("library of a mistake at a byte = %q", got)
	}
	failure := errors.New("disk on fire")
	if got := library("a.yaml", src, failure); !errors.Is(got, failure) {
		t.Errorf("library of a failure = %v", got)
	}
}
