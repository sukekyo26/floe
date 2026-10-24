package tmpl

import (
	"errors"
	"html/template"
	"io"
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

// escapeMistake places a mistake at the line html/template says, counted as lineAt counts, and runs none of
// the page's functions.
func TestEscapeMistake(t *testing.T) {
	t.Parallel()

	called := false
	probe := template.FuncMap{"probe": func() string { called = true; return "" }}
	// By source, the line and column: html/template's line where it says one, else its node's place.
	for src, at := range map[string][2]int{
		"<p>\n<a href=\"{{if .a}}/x/{{else}}/s?q={{end}}{{probe}}{{.a}}\">x</a></p>":                                   {2, 44},
		"{{probe}}\n\n<script>{{if .a}}var a = '{{end}}</script>":                                                      {3, 14},
		"{{define \"row\"}}\n<a href=\"{{if .}}/x/{{else}}/s?q={{end}}{{.}}\">{{end}}{{probe}}{{template \"row\" .a}}": {2, 43},
		// A page's own names, those of floe's copy among them, are its own.
		"{{define \"floe-escape\"}}<a href=\"{{if .}}/x/{{else}}/s?q={{end}}{{.}}\">x</a>{{end}}{{probe}}<p>{{template \"floe-escape\" .a}}</p>": {1, 67},
		"<p>{{probe}}{{template \"floe-escape\" .}}</p>": {1, 24},
		// ErrOutputContext names the body of the template whose output it cannot compute, after its {{define}}.
		"<script>[{{template \"t\" .}}]</script>\n{{define \"t\"}}{{if .Tail}}{{template \"t\" .Tail}}{{end}}{{.Head}}\",{{end}}": {2, 15},
	} {
		page := template.Must(template.New("p.html").Funcs(probe).Parse(src))
		var want *template.Error
		if !errors.As(template.Must(page.Clone()).Execute(io.Discard, nil), &want) {
			t.Fatalf("%q: html/template refuses nothing", src)
		}
		var got *usererr.Error
		if err := escapeMistake(page, []byte(src), "p.html", true); !errors.As(err, &got) ||
			got.Line != at[0] || got.Column != at[1] || want.Line != 0 && want.Line != got.Line || got.Args[0] != want.Description {
			t.Errorf("%q: escapeMistake = %v, want %d:%d (html/template's line %d) and %q", src, err, at[0], at[1], want.Line, want.Description)
		}
	}
	if called {
		t.Error("escapeMistake ran a function of the page")
	}
	// Nothing of the page runs: a loop it starts with would hold the reading up, and a function read files.
	page := template.Must(template.New("p.html").Funcs(probe).Parse(`{{range 300000000}}{{end}}{{probe}}{{.a.b}}{{range .c}}{{.}}{{end}}`))
	if err := escapeMistake(page, nil, "p.html", true); err != nil || called {
		t.Errorf("escapeMistake of a page drawing fails on = %v, ran a function %v; want nil, false", err, called)
	}
	// floe's own page is floe's failure, html/template's error kept, no mistake of the maker's.
	floes := template.Must(template.New("p.html").Parse(`<a href="{{if .a}}/x/{{else}}/s?q={{end}}{{.a}}">`))
	var escape *template.Error
	var bad *usererr.Error
	if err := escapeMistake(floes, nil, "p.html", false); !errors.As(err, &escape) || errors.As(err, &bad) {
		t.Errorf("escapeMistake of floe's page = %v, want floe's failure holding html/template's error", err)
	}
	if _, err := page.Clone(); err != nil {
		t.Errorf("the page escapeMistake checked cannot be copied: %v", err)
	}
}
