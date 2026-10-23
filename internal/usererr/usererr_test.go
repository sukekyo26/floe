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

	e := usererr.New("link_exposes_dir", "/home/me/top.html", "/home/me")
	if got := e.Error(); got != "/home/me/top.html would expose all of /home/me; move it into a directory of its own and link it there" {
		t.Errorf("Error() = %q", got)
	}
	if got := e.Localize(i18n.New("ja")); !strings.HasPrefix(got, "/home/me/top.html を登録すると /home/me 全体が公開されます") {
		t.Errorf("Localize(ja) = %q", got)
	}
	if e.Conflict {
		t.Error("New marks a conflict")
	}
}

// A List takes mistakes and Lists, leaves out a nil *Error, and returns a failure; of none it is no error.
func TestListAdd(t *testing.T) {
	t.Parallel()

	var none *usererr.Error
	var l usererr.List
	failure := errors.New("disk on fire")
	for _, err := range []error{nil, none, usererr.New("tmpl_no_lang"), usererr.List{usererr.NewAt("a.yaml", 2, 3, "tmpl_no_lang")}} {
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
	if got := l.Error(); got != "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)\na.yaml:2:3: name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)" {
		t.Errorf("the list told = %q", got)
	}
	// Collect takes them as Add does, sorted, and returns a failure among them alone.
	collected := usererr.Collect(usererr.NewAt("a.yaml", 5, 1, "tmpl_no_lang"), nil,
		usererr.List{usererr.NewAt("a.yaml", 2, 1, "tmpl_no_lang")})
	var both usererr.List
	if !errors.As(collected, &both) || len(both) != 2 || both[0].Line != 2 {
		t.Errorf("Collect of mistakes = %v", collected)
	}
	if got := usererr.Collect(usererr.New("tmpl_no_lang"), failure); !errors.Is(got, failure) {
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
		usererr.NewAt("b.yaml", 9, 1, "tmpl_no_lang"), usererr.NewAt("a.yaml", 3, 1, "tmpl_no_lang"),
		usererr.NewAt("b.yaml", 2, 5, "tmpl_no_lang"), usererr.NewAt("b.yaml", 2, 1, "tmpl_no_lang"),
	}
	got := make([]string, 0, len(l))
	for _, e := range l.Sorted() {
		got = append(got, fmt.Sprintf("%s:%d:%d", e.File, e.Line, e.Column))
	}
	if want := "b.yaml:2:1 b.yaml:2:5 b.yaml:9:1 a.yaml:3:1"; strings.Join(got, " ") != want {
		t.Errorf("Sorted = %v, want %s", got, want)
	}
}

// Show tells a placed mistake, a list of them, one of a placed one among others, one that stands for a placed
// one, and a name in use from a refusal and from floe's failure; the command line names floe before each
// mistake that is not at a place.
func TestShow(t *testing.T) {
	t.Parallel()

	const lang = "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)"
	conflict := usererr.New("tmpl_no_lang")
	conflict.Conflict = true
	over := usererr.Over("p", usererr.List{usererr.NewAt("a.yaml", 1, 2, "tmpl_no_lang")}, "tmpl_no_lang")
	for _, tt := range []struct {
		err  error
		want usererr.Shown
	}{
		{usererr.New("tmpl_no_lang"), usererr.Shown{Text: lang, Named: "floe: " + lang, Fixable: true, Placed: false, Conflict: false}},
		{usererr.NewAt("a.yaml", 1, 2, "tmpl_no_lang"), usererr.Shown{Text: "a.yaml:1:2: " + lang, Named: "a.yaml:1:2: " + lang, Fixable: true, Placed: true, Conflict: false}},
		{usererr.List{usererr.New("tmpl_no_lang")}, usererr.Shown{Text: lang, Named: "floe: " + lang, Fixable: true, Placed: false, Conflict: false}},
		{usererr.List{usererr.New("tmpl_no_lang"), usererr.NewAt("a.yaml", 1, 2, "tmpl_no_lang")}, usererr.Shown{
			Text: lang + "\na.yaml:1:2: " + lang, Named: "floe: " + lang + "\na.yaml:1:2: " + lang, Fixable: true, Placed: true, Conflict: false,
		}},
		{usererr.List{over}, usererr.Shown{Text: lang + "\n  a.yaml:1:2: " + lang, Named: lang + "\n  a.yaml:1:2: " + lang, Fixable: true, Placed: true, Conflict: false}},
		{usererr.List{conflict}, usererr.Shown{Text: lang, Named: "floe: " + lang, Fixable: true, Placed: false, Conflict: true}},
		{usererr.List{}, usererr.Shown{Text: "", Named: "", Fixable: true, Placed: false, Conflict: false}},
		{conflict, usererr.Shown{Text: lang, Named: "floe: " + lang, Fixable: true, Placed: false, Conflict: true}},
		{errors.New("disk on fire"), usererr.Shown{Text: "disk on fire", Named: "floe: disk on fire", Fixable: false, Placed: false, Conflict: false}},
	} {
		if got := usererr.Show(i18n.EnglishCatalog(), tt.err); got != tt.want {
			t.Errorf("Show(%v) = %+v, want %+v", tt.err, got, tt.want)
		}
	}
}

// A mistake is worded after as much of its place as it has, and Placed places a copy.
func TestPlace(t *testing.T) {
	t.Parallel()

	const what = "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)"
	e := usererr.New("tmpl_no_lang")
	for _, tt := range []struct {
		got  *usererr.Error
		want string
	}{
		{e, what},
		{e.Placed("a.yaml", 0, 0, ""), "a.yaml: " + what},
		{e.Placed("a.css", 3, 0, ""), "a.css:3: " + what},
		{e.Placed("a.yaml", 3, 4, ""), "a.yaml:3:4: " + what},
		{e.Placed("s.json", 0, 0, "#/properties/x"), "s.json: #/properties/x: " + what},
		{e.Placed("s.json", 2, 5, "#/properties/x"), "s.json:2:5: #/properties/x: " + what},
		{e.Placed("", 0, 0, "#/properties/x"), "#/properties/x: " + what},
	} {
		if got := tt.got.Error(); got != tt.want {
			t.Errorf("Error = %q, want %q", got, tt.want)
		}
	}
	if e.File != "" || e.At != "" {
		t.Errorf("Placed changed the mistake it copies: %+v", e)
	}
}

// A mistake that stands for others tells them under it, indented, once in a list: another of the same
// says they are told above.
func TestOver(t *testing.T) {
	t.Parallel()

	under := usererr.List{usererr.NewAt("parts/q/part.css", 1, 0, "tmpl_no_lang")}
	l := usererr.List{
		usererr.Over("part:q", under, "tmpl_part_broken", "q").Placed("a.json", 0, 0, "#/x"),
		usererr.Over("part:q", under, "tmpl_part_broken", "q").Placed("a.json", 0, 0, "#/y"),
	}
	want := "a.json: #/x: the part q cannot be used\n" +
		"  parts/q/part.css:1: name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)\n" +
		"a.json: #/y: the part q cannot be used, as told above"
	if got := l.Error(); got != want {
		t.Errorf("the list = %q, want %q", got, want)
	}
	if got := l[1].Error(); !strings.Contains(got, "\n  parts/q/part.css:1: ") {
		t.Errorf("one alone = %q, want its mistakes under it", got)
	}
}
