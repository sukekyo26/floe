package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The server draws in floe __draw, watched: a publish of a page that holds more than the limit is refused at its file,
// and so is one that takes longer than the time; a rebuild that meets such a page keeps it and draws the others; and
// the server serves on. The pages make 500 MB at most, and the slow one sleeps no longer than the test's timeout.
func TestServerDrawsApart(t *testing.T) {
	t.Parallel()

	e := newDenv(t)
	for name, text := range map[string]string{
		"templates/many/schema.json":   repeatTemplate,
		"templates/many/page.html":     "<!doctype html><html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body></body></html>",
		"templates/many/computed.yaml": repeatComputed,
		"templates/many/example.yaml":  "template: many\nlang: en\nn: 1\n",
		"templates/slow/schema.json":   repeatTemplate,
		"templates/slow/page.html":     "<!doctype html><html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body></body></html>",
		// About 2 seconds an item, each run within expr's budget.
		"templates/slow/checks.yaml":  "- for: rows\n  expr: 'let big = repeat(\"x\", 400000); count(1..500000, big contains \"y\") == 0'\n  message: m\n",
		"templates/slow/example.yaml": "template: slow\nlang: en\nn: 1\n",
		"heavy.yaml":                  "template: many\nlang: en\ntitle: H\nn: 5000\n",
		"light.yaml":                  "template: many\nlang: en\ntitle: L\nn: 1\n",
		"slow.yaml":                   "template: slow\nlang: en\ntitle: S\nrows: [" + strings.TrimSuffix(strings.Repeat("1, ", 30), ", ") + "]\n",
		"parts/.keep":                 "",
		"themes/.keep":                "",
	} {
		path := filepath.Join(e.tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e.vars["FLOE_TEMPLATES_DIR"] = filepath.Join(e.tmp, "templates")
	e.vars["FLOE_PARTS_DIR"] = filepath.Join(e.tmp, "parts")
	e.vars["FLOE_THEMES_DIR"] = filepath.Join(e.tmp, "themes")
	e.vars[guarded] = strconv.Itoa(testLimit)
	// Far past what the pages stopped by their memory take on a slow CI runner (over 1 second once), far below the slow
	// page's 30 rows of about 2 seconds.
	e.vars[drawSeconds] = "10"

	for _, c := range []struct {
		file, told string
	}{
		{"heavy.yaml", "heavy.yaml: drawing it needed more than 512 MiB of memory"},
		{"slow.yaml", "slow.yaml: drawing it took more than 10 seconds"},
	} {
		if r := e.exe(t, selfBin, "publish", c.file); r.code != 1 || !strings.Contains(r.stderr, c.told) {
			t.Errorf("floe publish %s = %d, %q; want 1 and %q (%s)", c.file, r.code, r.stderr, c.told, r.why())
		}
	}
	for _, file := range []string{"light.yaml", "page.yaml"} {
		if r := e.exe(t, selfBin, "publish", file); r.code != 0 {
			t.Fatalf("floe publish %s = %d, %q (%s)", file, r.code, r.stderr, r.why())
		}
	}
	// light.yaml now makes 500 MB, as heavy.yaml does.
	heavier := `copies: 'let s = repeat("x", 100000); len(map(1..((n ?? 0) * 5000), s + "x"))'` + "\n"
	if err := os.WriteFile(filepath.Join(e.tmp, "templates", "many", "computed.yaml"), []byte(heavier), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := e.exe(t, selfBin, "rebuild"); r.code != 1 || !strings.Contains(r.stderr, "drawing it needed more than") ||
		strings.Contains(r.stderr, "page.yaml") {
		t.Errorf("floe rebuild = %d, %q; want 1 and the memory of light alone (%s)", r.code, r.stderr, r.why())
	}
	if r := e.exe(t, selfBin, "list"); r.code != 0 || !strings.Contains(r.stdout, "light") {
		t.Errorf("floe list after = %d, %q, %q (%s)", r.code, r.stdout, r.stderr, r.why())
	}
}
