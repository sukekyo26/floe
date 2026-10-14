package server

import (
	"io"
	"testing"
)

// The CSP goes into a download before anything it could load: after a doctype in any case, past a BOM
// and comments before it, or before the rest when there is none.
func TestWithCutOff(t *testing.T) {
	t.Parallel()

	meta := "\n<meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"" + cutOff("'none'") + "\">"
	for page, want := range map[string]string{
		"<!doctype html><html>":                       "<!doctype html>" + meta + "<html>",
		"\n <!DOCTYPE HTML>\n<head>":                  "\n <!DOCTYPE HTML>" + meta + "\n<head>",
		"\uFEFF<!-- a -->\n<!-- b --><!doctype html>": "\uFEFF<!-- a -->\n<!-- b --><!doctype html>" + meta,
		"<!-- a --><html>":                            "<!-- a -->" + meta + "<html>",
		"<!-- open <!doctype html>":                   meta + "<!-- open <!doctype html>",
		"<html><head><title>x</title>":                meta + "<html><head><title>x</title>",
		"<!doctype":                                   meta + "<!doctype",
		"":                                            meta,
	} {
		got, err := io.ReadAll(withCutOff([]byte(page)))
		if err != nil || string(got) != want {
			t.Errorf("withCutOff(%q) = %q, %v, want %q", page, got, err, want)
		}
	}
}
