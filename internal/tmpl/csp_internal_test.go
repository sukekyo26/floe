package tmpl

import (
	"io"
	"testing"
)

// The CSP goes into a download before anything it could load: after a doctype in any case, past a BOM
// and comments before it, which end where a browser ends them, or before the rest when there is none.
func TestWithCutOff(t *testing.T) {
	t.Parallel()

	meta := "\n<meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"" + CutOff("'none'") + "\">"
	for page, want := range map[string]string{
		"<!doctype html><html>":                       "<!doctype html>" + meta + "<html>",
		"\n <!DOCTYPE HTML>\n<head>":                  "\n <!DOCTYPE HTML>" + meta + "\n<head>",
		"\uFEFF<!-- a -->\n<!-- b --><!doctype html>": "\uFEFF<!-- a -->\n<!-- b --><!doctype html>" + meta,
		"<!-- a --><html>":                            "<!-- a -->" + meta + "<html>",
		"<!-- open <!doctype html>":                   meta + "<!-- open <!doctype html>",
		"<!-- a --!><!doctype html><p><!-- b -->":     "<!-- a --!><!doctype html>" + meta + "<p><!-- b -->",
		"<!--><!---><!doctype html>":                  "<!--><!---><!doctype html>" + meta,
		"<?xml version=\"1.0\"?>\f<!doctype html>":    "<?xml version=\"1.0\"?>\f<!doctype html>" + meta,
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
