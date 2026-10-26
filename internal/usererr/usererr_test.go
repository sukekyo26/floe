package usererr_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

func TestError(t *testing.T) {
	t.Parallel()

	if got := usererr.Library(errors.New("bad")).Error(); got != "bad" {
		t.Errorf("Error() = %q", got)
	}
}

// A library's words are told as UTF-8: a run of bytes that are no UTF-8 is written as U+FFFD, the rest as it is.
func TestLibraryUTF8(t *testing.T) {
	t.Parallel()

	const want = "a\uFFFDb 日"
	for _, e := range []*usererr.Error{
		usererr.Library(errors.New("a\xff\xfeb 日")),
		usererr.LibraryAt("f", 1, 2, "a\xff\xfeb 日"),
		usererr.LibraryIn("f", "#/x", "a\xff\xfeb 日"),
	} {
		if got := e.Args[0]; got != want {
			t.Errorf("%+v holds %q, want %q", e, got, want)
		}
	}
}

// A List takes mistakes and Lists, leaves out a nil *Error, and returns a failure; of none it is no error.
func TestListAdd(t *testing.T) {
	t.Parallel()

	var none *usererr.Error
	var l usererr.List
	failure := errors.New("disk on fire")
	for _, err := range []error{nil, none, usererr.LibraryAt("", 0, 0, "no lang"), usererr.List{none, usererr.LibraryAt("a.yaml", 2, 3, "no lang")}} {
		if got := l.Add(err); got != nil {
			t.Errorf("Add(%v) = %v", err, got)
		}
	}
	if got := l.Add(failure); !errors.Is(got, failure) {
		t.Errorf("Add of a failure = %v", got)
	}
	if len(l) != 2 {
		t.Fatalf("the list = %v", l)
	}
	if err := (usererr.List{}).Err(); err != nil {
		t.Errorf("Err of none = %v", err)
	}
	if got := l.Error(); got != "no lang\na.yaml:2:3: no lang" {
		t.Errorf("the list told = %q", got)
	}
	// Collect takes them as Add does, sorted, and returns a failure among them alone.
	collected := usererr.Collect(usererr.LibraryAt("a.yaml", 5, 1, "x"), nil, usererr.List{usererr.LibraryAt("a.yaml", 2, 1, "x")})
	var both usererr.List
	if !errors.As(collected, &both) || len(both) != 2 || both[0].Line != 2 {
		t.Errorf("Collect of mistakes = %v", collected)
	}
	if got := usererr.Collect(usererr.Library(errors.New("x")), failure); !errors.Is(got, failure) {
		t.Errorf("Collect with a failure = %v", got)
	}
	if got := usererr.Collect(nil, nil); got != nil {
		t.Errorf("Collect of none = %v", got)
	}
}

// Sorted keeps the files in the order they first come, and sorts the mistakes of each by line and column.
func TestListSorted(t *testing.T) {
	t.Parallel()

	l := usererr.List{
		usererr.LibraryAt("b.yaml", 9, 1, "x"), usererr.LibraryAt("a.yaml", 3, 1, "x"),
		usererr.LibraryAt("b.yaml", 2, 5, "x"), usererr.LibraryAt("b.yaml", 2, 1, "x"),
	}
	got := make([]string, 0, len(l))
	for _, e := range l.Sorted() {
		got = append(got, fmt.Sprintf("%s:%d:%d", e.File, e.Line, e.Column))
	}
	if want := "b.yaml:2:1 b.yaml:2:5 b.yaml:9:1 a.yaml:3:1"; strings.Join(got, " ") != want {
		t.Errorf("Sorted = %v, want %s", got, want)
	}
	// At one place: by the JSON pointer, then the key, then the arguments, whatever order they come in.
	same := usererr.List{
		usererr.LibraryAt("d.yaml", 1, 1, "Zed"), usererr.NewIn("s.json", "#/b", "library", "x"),
		usererr.LibraryAt("d.yaml", 1, 1, "Bad"), usererr.NewIn("s.json", "#/a", "library", "x"),
	}
	// The steps of pointers that are numbers, as numbers; each argument apart.
	steps := usererr.List{
		usererr.NewIn("s.json", "#/allOf/10", "library", "x"), usererr.NewIn("s.json", "#/allOf/2", "library", "x"),
		usererr.New("k", "ab", "c"), usererr.New("k", "a", "bc"),
	}
	if got := steps.Sorted(); got[0].At != "#/allOf/2" || got[2].Args[0] != "a" {
		t.Errorf("Sorted by steps and arguments = %v", got.Error())
	}
	// One order of all steps, whatever order they come in: indexes first, by number, then text.
	for _, order := range [][]string{{"10", "9", "1a"}, {"10", "1a", "9"}, {"9", "10", "1a"}, {"9", "1a", "10"}, {"1a", "10", "9"}, {"1a", "9", "10"}} {
		var l usererr.List
		for _, step := range order {
			l = append(l, usererr.NewIn("s.json", "#/properties/"+step, "library", "x"))
		}
		if got := l.Sorted().Error(); got != "s.json: #/properties/9: x\ns.json: #/properties/10: x\ns.json: #/properties/1a: x" {
			t.Errorf("Sorted of %v = %q", order, got)
		}
	}
	if got := same.Sorted().Error(); got != "d.yaml:1:1: Bad\nd.yaml:1:1: Zed\ns.json: #/a: x\ns.json: #/b: x" {
		t.Errorf("Sorted at one place = %q", got)
	}
}

// Tell words a mistake, one at as much of a place as it has, a list of them in the order given, one wrapped in
// another error, and floe's failure; Render lays them out to the letter as the command line and the browser
// show them, floe's name before each that is not at a place.
func TestTell(t *testing.T) {
	t.Parallel()

	lang := usererr.Library(errors.New("no lang"))
	for _, tt := range []struct {
		name        string
		err         error
		text, named string
	}{
		{"one", lang, "no lang", "floe: no lang"},
		{"one in a file", usererr.LibraryAt("a.yaml", 0, 0, "no lang"), "a.yaml: no lang", "a.yaml: no lang"},
		{"one at a line", usererr.LibraryAt("a.css", 3, 0, "no lang"), "a.css:3: no lang", "a.css:3: no lang"},
		{"one at a column", usererr.LibraryAt("a.yaml", 3, 4, "no lang"), "a.yaml:3:4: no lang", "a.yaml:3:4: no lang"},
		{"one at a pointer", usererr.LibraryIn("s.json", "#/properties/x", "no lang"), "s.json: #/properties/x: no lang", "s.json: #/properties/x: no lang"},
		{
			"a list, in the order given",
			usererr.List{usererr.LibraryAt("b.yaml", 2, 1, "b"), lang, usererr.LibraryAt("a.yaml", 1, 1, "a")},
			"b.yaml:2:1: b\nno lang\na.yaml:1:1: a", "b.yaml:2:1: b\nfloe: no lang\na.yaml:1:1: a",
		},
		{"one wrapped, without the words of what wraps it", fmt.Errorf("read: %w", lang), "no lang", "floe: no lang"},
		{"no mistake", (*usererr.Error)(nil), "*usererr.Error with no mistake", "floe: *usererr.Error with no mistake"},
		{"an empty list", usererr.List{}, "usererr.List with no mistake", "floe: usererr.List with no mistake"},
		{"none", nil, "<nil> with no mistake", "floe: <nil> with no mistake"},
		{"floe's failure", errors.New("disk on fire"), "disk on fire", "floe: disk on fire"},
		{"floe's failure without words", errors.New(""), "*errors.errorString", "floe: *errors.errorString"},
	} {
		r := usererr.Tell(i18n.EnglishCatalog(), tt.err)
		if got := r.Render(""); got != tt.text {
			t.Errorf("%s: Render(\"\") = %q, want %q", tt.name, got, tt.text)
		}
		if got := r.Render("floe: "); got != tt.named {
			t.Errorf("%s: Render(\"floe: \") = %q, want %q", tt.name, got, tt.named)
		}
	}
	// A library's words stay as it said them in every language.
	if got := usererr.Tell(i18n.New("ja"), lang).Render(""); got != "no lang" {
		t.Errorf("Tell(ja) = %q", got)
	}
}
