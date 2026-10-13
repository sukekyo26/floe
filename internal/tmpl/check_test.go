package tmpl_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
)

// The template of testdata/sizes uses every stage: its checks, its computed values, num and bytes.
func TestDrawEveryStage(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("testdata/sizes/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	page, err := draw(tmpl.NewLibrary("testdata"), string(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<p><b>made</b> for tests</p>", "<td>1.5 KiB</td>", "<p>2,049,536 bytes, 1 of them"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page has no %s:\n%s", want, page)
		}
	}
	twice := strings.Replace(string(src), "b.png", "a.txt", 1)
	if _, err := draw(tmpl.NewLibrary("testdata"), twice); err == nil || !strings.Contains(err.Error(), "data.yaml:7:11: files[1].name: give each file its own name") {
		t.Errorf("a check that fails = %v", err)
	}
}

// A right template passes, warned only that it does not name the floe it needs.
func TestCheck(t *testing.T) {
	t.Parallel()

	warnings, mistakes := tmpl.Check("testdata/sizes")
	if len(warnings) != 1 || len(mistakes) != 0 {
		t.Fatalf("Check = %v, %v", warnings, mistakes)
	}
	release := strings.SplitN(version.Version, ".", 3)
	if want := "testdata/sizes has no template.yaml: write floe: " + release[0] + "." + release[1] + " in it"; !strings.HasPrefix(warnings[0].Error(), want) {
		t.Errorf("the warning = %s, want %s", warnings[0], want)
	}

	dir := sizes(t, map[string]string{"template.yaml": "floe: " + release[0] + "." + release[1] + "\n"})
	if warnings, mistakes := tmpl.Check(dir); len(warnings) != 0 || len(mistakes) != 0 {
		t.Errorf("Check with template.yaml = %v, %v", warnings, mistakes)
	}
}

// Each mistake of a template is reported, at its line and column where it has one; one in its files
// stops its example from being checked, which would only fail.
func TestCheckMistakes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		files   map[string]string
		want    []string
		warning string
	}{
		{
			name:  "unknown format",
			files: map[string]string{"schema.json": `"format": "date"` + "|" + `"format": "datum"`},
			want:  []string{`sizes/schema.json: #/properties/files/items/properties/updated: floe does not know the format "datum"`},
		},
		{
			name:  "html out of reach",
			files: map[string]string{"schema.json": `"note": { "type": "string", "format": "html" }` + "|" + `"note": { "anyOf": [{ "type": "string", "format": "html" }] }`},
			want:  []string{"sizes/schema.json: #/properties/note/anyOf/0: the format html marks an item to draw"},
		},
		{
			name:    "open schema",
			files:   map[string]string{"schema.json": `"required": ["name", "bytes"],` + "\n" + `        "additionalProperties": false` + "|" + `"required": ["name", "bytes"]`},
			warning: `sizes/schema.json: #/properties/files/items declares properties without "additionalProperties": false`,
		},
		{
			name: "checks",
			files: map[string]string{"checks.yaml": "- for: file\n  expr: 'true'\n  message: m\n" +
				"- for: files.0\n  expr: 'true'\n  message: m\n" +
				"- for: files\n  field: size\n  expr: 'true'\n  message: m\n" +
				"- for: files\n  expr: itme\n  message: m\n"},
			want: []string{
				"sizes/checks.yaml:1:8: for: schema.json does not declare file",
				"sizes/checks.yaml:4:8: for: schema.json does not declare files.0",
				"sizes/checks.yaml:8:10: field: schema.json does not declare size under the items of files",
				"sizes/checks.yaml:12:9: itme cannot be read as an expression: unknown name itme",
			},
		},
		{
			name:  "not a list",
			files: map[string]string{"checks.yaml": "- for: note\n  expr: 'true'\n  message: m\n"},
			want:  []string{"sizes/checks.yaml:1:8: for: schema.json does not declare note a list"},
		},
		{
			name:  "computed",
			files: map[string]string{"computed.yaml": "total: sum(files, .bytes)\nfiles: 1\n"},
			want:  []string{"sizes/computed.yaml:2:1: files is an item of the data too"},
		},
		{
			name:  "page",
			files: map[string]string{"page.html": "{{if}}"},
			want:  []string{"parse the template sizes: "},
		},
		{
			name:  "example",
			files: map[string]string{"example.yaml": "template: report\nfiles:\n  - name: a\n"},
			want: []string{
				"sizes/example.yaml:1:11: example.yaml names the template report; name it sizes",
				"sizes/example.yaml:3:5: files[0].bytes is required but missing",
			},
		},
		{
			name:  "newer floe",
			files: map[string]string{"template.yaml": "floe: 999.0\n"},
			want:  []string{"template sizes needs floe 999.0.0 or later"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			warnings, mistakes := tmpl.Check(sizes(t, tt.files))
			lines := make([]string, 0, len(mistakes))
			for _, m := range mistakes {
				lines = append(lines, strings.Split(m.Error(), "\n")...)
			}
			if len(lines) != len(tt.want) {
				t.Fatalf("the mistakes =\n%s\nwant %d", strings.Join(lines, "\n"), len(tt.want))
			}
			for i, w := range tt.want {
				if !strings.Contains(lines[i], w) {
					t.Errorf("mistake %d = %s\nwant %s", i, lines[i], w)
				}
			}
			if tt.warning != "" && (len(warnings) < 2 || !strings.Contains(warnings[1].Error(), tt.warning)) {
				t.Errorf("the warnings = %v, want %s", warnings, tt.warning)
			}
		})
	}
}

// A directory is checked as a template of its name, which must be one.
func TestCheckName(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "Sizes")
	if err := os.CopyFS(dir, os.DirFS("testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	_, mistakes := tmpl.Check(dir)
	var e *usererr.Error
	if len(mistakes) != 1 || !errors.As(mistakes[0], &e) || e.Key != "tmpl_new_name" {
		t.Errorf("Check of %s = %v", dir, mistakes)
	}
}

// sizes copies testdata/sizes into a directory sizes, with files written over: a whole file, or old|new
// for old to be replaced with new.
func sizes(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "sizes")
	if err := os.CopyFS(dir, os.DirFS("testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	for name, src := range files {
		at := filepath.Join(dir, name)
		if old, repl, ok := strings.Cut(src, "|"); ok {
			b, err := os.ReadFile(at)
			if err != nil || !strings.Contains(string(b), old) {
				t.Fatalf("%s has no %s: %v", name, old, err)
			}
			src = strings.Replace(string(b), old, repl, 1)
		}
		if err := os.WriteFile(at, []byte(src), 0o600); err != nil { //nolint:gosec // G703: the test's own directory
			t.Fatal(err)
		}
	}
	return dir
}
