package usererr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
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

// Tell words a mistake, a list of them, one standing for placed ones, its second time, one in the words of
// another, one of the whole output, after the others, a name in use and floe's failure; Render lays them out to the letter as the command line and the
// browser show them, floe's name before each that is not at a place; Status answers 400, 409 or 500. A report read back from JSON, as the
// command line reads the server's, lays out the same to the letter.
func TestTell(t *testing.T) {
	t.Parallel()

	const lang = "name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)"
	const part = "the part p cannot be used\n  a.yaml:1:2: " + lang
	const noDir = `there is no directory for icons; set FLOE_ICONS_DIR, then fetch a set with "floe icons fetch <set>"`
	placed := usererr.NewAt("a.yaml", 1, 2, "tmpl_no_lang")
	conflict := usererr.New("tmpl_no_lang")
	conflict.Conflict = true
	over := func() *usererr.Error { return usererr.Over("p", usererr.List{placed}, "tmpl_part_broken", "p") }
	// One mistake given twice, as one read once is given to each input that uses what it is of.
	once := usererr.New("tmpl_no_lang")
	for _, tt := range []struct {
		name        string
		err         error
		text, named string
		status      int
	}{
		{"one not at a place", usererr.New("tmpl_no_lang"), lang, "floe: " + lang, 400},
		{"one at a place", placed, "a.yaml:1:2: " + lang, "a.yaml:1:2: " + lang, 400},
		{"one at a pointer", usererr.NewIn("s.json", "#/a", "tmpl_no_lang"), "s.json: #/a: " + lang, "s.json: #/a: " + lang, 400},
		{"a list of both", usererr.List{usererr.New("tmpl_no_lang"), placed}, lang + "\na.yaml:1:2: " + lang, "floe: " + lang + "\na.yaml:1:2: " + lang, 400},
		{"one standing for a placed one", usererr.List{over()}, part, part, 400},
		{
			"its second time",
			usererr.List{over().Placed("b.yaml", 3, 0, ""), over().Placed("c.yaml", 4, 0, "")},
			"b.yaml:3: " + part + "\nc.yaml:4: the part p cannot be used, as told above",
			"b.yaml:3: " + part + "\nc.yaml:4: the part p cannot be used, as told above", 400,
		},
		{"one made twice", usererr.List{over(), over()}, part, part, 400},
		{"one in the words of another", usererr.New("tmpl_item", "x", over()), "x: " + part, "floe: x: " + part, 400},
		{
			"one in the words of another, and alone, last as of the whole output",
			usererr.List{over(), usererr.New("tmpl_item", "x", over())},
			"x: " + part + "\n" + part, "floe: x: " + part + "\n" + part, 400,
		},
		{
			"one of the whole output, last, the others as given",
			usererr.List{usererr.NewWhole("icons_no_dir"), usererr.NewAt("b.yaml", 2, 1, "tmpl_no_lang"), placed},
			"b.yaml:2:1: " + lang + "\na.yaml:1:2: " + lang + "\n" + noDir,
			"b.yaml:2:1: " + lang + "\na.yaml:1:2: " + lang + "\nfloe: " + noDir, 400,
		},
		{
			"one standing for a placed one, placed, where it is",
			usererr.List{over().Placed("b.yaml", 3, 0, ""), placed},
			"b.yaml:3: " + part + "\na.yaml:1:2: " + lang, "b.yaml:3: " + part + "\na.yaml:1:2: " + lang, 400,
		},
		{"one mistake given twice", usererr.List{once, placed, once}, lang + "\na.yaml:1:2: " + lang, "floe: " + lang + "\na.yaml:1:2: " + lang, 400},
		{"two alike", usererr.List{usererr.New("tmpl_no_lang"), usererr.New("tmpl_no_lang")}, lang, "floe: " + lang, 400},
		{"a name in use", conflict, lang, "floe: " + lang, 409},
		{"an empty list", usererr.List{}, "usererr.List with no mistake", "floe: usererr.List with no mistake", 500},
		{"no mistake", (*usererr.Error)(nil), "*usererr.Error with no mistake", "floe: *usererr.Error with no mistake", 500},
		{"a report told already", reported{usererr.Report{Mistakes: []usererr.Told{{Text: "told"}}}}, "told", "floe: told", 400},
		{"floe's failure", errors.New("disk on fire"), "disk on fire", "floe: disk on fire", 500},
	} {
		r := usererr.Tell(i18n.EnglishCatalog(), tt.err)
		if got := r.Render(""); got != tt.text {
			t.Errorf("%s: Render(\"\") = %q, want %q", tt.name, got, tt.text)
		}
		if got := r.Render("floe: "); got != tt.named {
			t.Errorf("%s: Render(\"floe: \") = %q, want %q", tt.name, got, tt.named)
		}
		if r.Status() != tt.status {
			t.Errorf("%s: Status = %d, want %d", tt.name, r.Status(), tt.status)
		}
		var l i18n.Localizer
		if errors.As(tt.err, &l) && tt.err.Error() != tt.text {
			t.Errorf("%s: Error = %q, want %q", tt.name, tt.err.Error(), tt.text)
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var back usererr.Report
		if err = json.Unmarshal(b, &back); err != nil || back.Render("floe: ") != tt.named || back.Status() != tt.status {
			t.Errorf("%s: read back from %s = %q %d, %v", tt.name, b, back.Render("floe: "), back.Status(), err)
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

// reported is an error that carries a report already told, as the server's does to the command line.
type reported struct{ report usererr.Report }

func (r reported) Error() string { return r.report.Render("") }

func (r reported) Report() usererr.Report { return r.report }

// A file that cannot be written is told by the step that could not be made when the error names it or a
// directory above it, so that "its directory" is the one to check; as not to be written in when the error
// names what floe made in it; else by the file.
func TestCannotWrite(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ path, want string }{
		{"/a/b", "cannot write /a/b (permission denied)"},                 // a directory above /a/b/c could not be made
		{"/a/b/c", "cannot write /a/b/c (permission denied)"},             // the file itself
		{"/a/b.tmp-123", "cannot write /a/b/c (permission denied)"},       // what floe made beside it, by a name of its own
		{"/a/bc", "cannot write /a/b/c (permission denied)"},              // not above it, though it starts alike
		{"/a/b/c/.check-1", "cannot write in /a/b/c (permission denied)"}, // what floe made in it, there already
	} {
		err := usererr.CannotWrite("/a/b/c", &fs.PathError{Op: "mkdir", Path: c.path, Err: syscall.EACCES})
		if got := err.Error(); !strings.HasPrefix(got, c.want+"; ") {
			t.Errorf("CannotWrite of an error at %s = %q, want %q", c.path, got, c.want)
		}
	}
}
