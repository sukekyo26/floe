package tmpl

import (
	"io"
	"testing"
)

// The CSP goes into a download before anything it could load: past a BOM, whitespace and the comments an
// HTML tokenizer reads, after a doctype, or before the rest when there is none. A comment or a doctype that
// runs to the end of the page is taken as open, and the CSP goes before it, not into it.
func TestWithCutOff(t *testing.T) {
	t.Parallel()

	meta := "\n<meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"" + CutOff("'none'") + "\">"
	for page, want := range map[string]string{
		"<!doctype html><html>":                       "<!doctype html>" + meta + "<html>",
		"\n <!DOCTYPE HTML>\n<head>":                  "\n <!DOCTYPE HTML>" + meta + "\n<head>",
		"\uFEFF<!-- a -->\n<!-- b --><!doctype html>": "\uFEFF<!-- a -->\n<!-- b -->" + meta + "<!doctype html>",
		"<!-- a --><html>":                            "<!-- a -->" + meta + "<html>",
		"<!-- open <!doctype html>":                   meta + "<!-- open <!doctype html>",
		"<!-- a --!><!doctype html><p><!-- b -->":     "<!-- a --!><!doctype html>" + meta + "<p><!-- b -->",
		"<!--><!---><!doctype html>":                  "<!--><!--->" + meta + "<!doctype html>",
		"<?xml version=\"1.0\"?>\f<!doctype html>":    "<?xml version=\"1.0\"?>\f" + meta + "<!doctype html>",
		"<?xml version=\"1.0\"?>\f<!doctype html><p>": "<?xml version=\"1.0\"?>\f<!doctype html>" + meta + "<p>",
		"<!x><!doctype html><p>":                      "<!x><!doctype html>" + meta + "<p>",
		"  hello<!doctype html>":                      "  " + meta + "hello<!doctype html>",
		"<!doctype html PUBLIC \"a>b\"><p>":           "<!doctype html PUBLIC \"a>" + meta + "b\"><p>",
		"<html><head><title>x</title>":                meta + "<html><head><title>x</title>",
		"<!doctype":                                   meta + "<!doctype",
		"":                                            meta,
	} {
		got, err := io.ReadAll(WithCutOff([]byte(page)))
		if err != nil || string(got) != want || string(CutOffPage([]byte(page))) != want {
			t.Errorf("WithCutOff(%q) = %q, %v; CutOffPage = %q; want %q", page, got, err, CutOffPage([]byte(page)), want)
		}
	}
}
