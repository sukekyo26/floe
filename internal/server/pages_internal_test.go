package server

import "testing"

// The CSP goes into a download before anything it could load: after a doctype in any case, or first
// when there is none.
func TestWithCutOff(t *testing.T) {
	t.Parallel()

	meta := "\n<meta http-equiv=\"Content-Security-Policy\" content=\"" + cutOff("'none'") + "\">"
	for page, want := range map[string]string{
		"<!doctype html><html>":        "<!doctype html>" + meta + "<html>",
		"\n <!DOCTYPE HTML>\n<head>":   "\n <!DOCTYPE HTML>" + meta + "\n<head>",
		"<html><head><title>x</title>": meta + "<html><head><title>x</title>",
		"<!doctype":                    meta + "<!doctype",
		"":                             meta,
	} {
		if got := string(withCutOff([]byte(page))); got != want {
			t.Errorf("withCutOff(%q) = %q, want %q", page, got, want)
		}
	}
}
