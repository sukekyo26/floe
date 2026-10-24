package usererr

import (
	"cmp"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Told is a mistake as told: worded in the reader's language, where it is, and the mistakes it stands for,
// Above when they are told above, under the first that stands for them, its text saying so. It is what the
// server sends the command line, which lays it out as it does its own.
type Told struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	At       string `json:"at,omitempty"`
	Text     string `json:"text"`
	Conflict bool   `json:"conflict,omitempty"`
	Under    []Told `json:"under,omitempty"`
	Above    bool   `json:"above,omitempty"`
}

// Report is an error as told: the user's mistakes, or what floe failed at, never both.
type Report struct {
	Mistakes []Told `json:"mistakes,omitempty"`
	Failure  string `json:"failure,omitempty"`
}

// Tell words err in the language of c: its mistakes, as List.Add takes them, those of a part or a theme
// that several stand for told under the first, or, for an error that is not the user's or holds no
// mistake, floe's failure. An error that carries a report already told (the server's, to the command line)
// is told as it is. It is the only place a mistake is worded. The same mistake, as told the first time (where
// it is, its words and those under it), is told once, where it first comes, wherever it was made.
func Tell(c *i18n.Catalog, err error) Report {
	var reported interface{ Report() Report }
	if errors.As(err, &reported) {
		return reported.Report()
	}
	var all List
	if failed := all.Add(err); failed != nil {
		return Report{Mistakes: nil, Failure: cmp.Or(failed.Error(), fmt.Sprintf("%T", failed))}
	}
	if len(all) == 0 {
		return Report{Mistakes: nil, Failure: fmt.Sprintf("%T with no mistake", err)}
	}
	told, seen := map[string]bool{}, map[string]bool{}
	r := Report{Mistakes: make([]Told, 0, len(all)), Failure: ""}
	for _, e := range all {
		if key := e.tell(c, map[string]bool{}).line(); !seen[key] {
			seen[key] = true
			r.Mistakes = append(r.Mistakes, e.tell(c, told))
		}
	}
	return r
}

// Status is the HTTP status r answers with: 500 for floe's failure, 409 when every mistake is a name in
// use, else 400.
func (r Report) Status() int {
	switch {
	case r.Failure != "":
		return http.StatusInternalServerError
	case len(r.Mistakes) > 0 && !slices.ContainsFunc(r.Mistakes, func(t Told) bool { return !t.Conflict }):
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

// Render lays r out, the only place that does: each mistake on a line, after where it is,
// file:line:column: at, or as much of it as it has, the mistakes it stands for under it, indented; name
// before each that is not at a place and stands for none that is, and before floe's failure. The command
// line names floe so; a page in the browser names nothing ("").
func (r Report) Render(name string) string {
	if r.Failure != "" {
		return name + r.Failure
	}
	lines := make([]string, len(r.Mistakes))
	for i, t := range r.Mistakes {
		lines[i] = t.line()
		if !t.atPlace() {
			lines[i] = name + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// line lays t out, after where it is, and the mistakes it stands for under it, indented.
func (t Told) line() string {
	var where []string
	switch {
	case t.File == "":
	case t.Line == 0:
		where = append(where, t.File)
	case t.Column == 0:
		where = append(where, fmt.Sprintf("%s:%d", t.File, t.Line))
	default:
		where = append(where, fmt.Sprintf("%s:%d:%d", t.File, t.Line, t.Column))
	}
	if t.At != "" {
		where = append(where, t.At)
	}
	text := strings.Join(append(where, t.Text), ": ")
	if t.Above {
		return text
	}
	for _, u := range t.Under {
		text += "\n  " + strings.ReplaceAll(u.line(), "\n", "\n  ")
	}
	return text
}

// atPlace reports whether t is at a place, or stands for a mistake that is.
func (t Told) atPlace() bool {
	return t.File != "" || slices.ContainsFunc(t.Under, Told.atPlace)
}
