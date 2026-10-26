package yamlpos_test

import (
	"errors"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Place puts each mistake of no file at a JSON pointer at its line and column under prefix; one with a file of
// its own, or with no pointer, stays as it is, and a failure is returned as it is.
func TestPlace(t *testing.T) {
	t.Parallel()

	root, err := yamlpos.Parse("d.yaml", []byte("top: 1\ndiagram:\n  parts:\n    - box: { x: 1 }\n  a/b: 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = usererr.Collect(
		usererr.NewIn("", "#/parts/0/box/x", "library", "bad x"),
		usererr.NewIn("", "#/a~1b", "library", "bad a/b"),
		usererr.NewIn("", "#", "library", "the whole diagram"),
		usererr.NewAt("icons/aws/index.json", 1, 2, "library", "its own place"),
		usererr.New("library", "no place"),
	)
	// The whole of the diagram is at its first key; Collect put the mistake of no place first.
	want := "floe: no place\nd.yaml:3:3: the whole diagram\nd.yaml:4:17: bad x\nd.yaml:5:8: bad a/b\nicons/aws/index.json:1:2: its own place"
	placed := yamlpos.Place(err, "d.yaml", root, []string{"diagram"})
	if got := usererr.Tell(i18n.EnglishCatalog(), usererr.Collect(placed)).Render("floe: "); got != want {
		t.Errorf("Place = %q, want %q", got, want)
	}
	// Placed, the mistakes are in the order of their lines, whatever the order of their pointers.
	list := usererr.List{
		usererr.NewIn("", "#/a~1b", "library", "second"),
		usererr.NewIn("", "#/parts/0/box/x", "library", "first"),
	}
	got := usererr.Tell(i18n.EnglishCatalog(), yamlpos.Place(list, "d.yaml", root, []string{"diagram"})).Render("")
	if want := "d.yaml:4:17: first\nd.yaml:5:8: second"; got != want {
		t.Errorf("Place of pointers out of the lines' order = %q, want %q", got, want)
	}
	if got := usererr.Tell(i18n.EnglishCatalog(), yamlpos.Place(list[:1], "e.yaml", nil, nil)).Render(""); got != "e.yaml:1:1: second" {
		t.Errorf("Place under no root = %q", got)
	}
	failure := errors.New("floe's own")
	if got := yamlpos.Place(failure, "d.yaml", root, nil); !errors.Is(got, failure) {
		t.Errorf("Place of a failure = %v", got)
	}
	if yamlpos.Place(nil, "d.yaml", root, nil) != nil {
		t.Error("Place of nil is not nil")
	}
}
