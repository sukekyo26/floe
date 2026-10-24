package corpus_test

import (
	"bytes"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/cli"
)

var update = flag.Bool("update", false, "write the golden files of each sample anew")

// TestCorpus runs floe on each sample in testdata as an agent would, and compares the exit codes, what it
// printed and the pages it wrote with the sample's golden/. What a sample holds says what runs: its data
// files (*.yaml) are checked, and rendered when they pass; templates/, parts/, themes/ and icons/ are its
// own, and templates/ is checked by floe template check, parts/ and themes/ by floe part check; the specs
// in diagrams/ are drawn by floe diagram render.
func TestCorpus(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			t.Parallel()

			dir, err := filepath.Abs(filepath.Join("testdata", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			got := runSample(t, dir)
			golden := filepath.Join(dir, "golden")
			if *update {
				writeGolden(t, golden, got)
				return
			}
			compareGolden(t, golden, got)
		})
	}
}

// sample is a run of floe on the sample in dir: the settings of floe it runs under, and the files of its
// golden/ so far, commands.golden among them.
type sample struct {
	dir, out string
	vars     map[string]string
	commands strings.Builder
	files    map[string][]byte
}

func runSample(t *testing.T, dir string) map[string][]byte {
	t.Helper()

	tmp := realPath(t, t.TempDir())
	s := &sample{dir: realPath(t, dir), out: filepath.Join(tmp, "out"), files: map[string][]byte{}, vars: map[string]string{
		"HOME":     filepath.Join(tmp, "home"),
		"FLOE_DIR": filepath.Join(tmp, "store"),
	}}
	// One's own of each kind is the sample's, else none: a directory that is not there.
	for key, sub := range map[string]string{
		"FLOE_TEMPLATES_DIR": "templates", "FLOE_PARTS_DIR": "parts", "FLOE_THEMES_DIR": "themes", "FLOE_ICONS_DIR": "icons",
	} {
		s.vars[key] = filepath.Join(tmp, "none", sub)
		if s.has(t, sub) {
			s.vars[key] = filepath.Join(s.dir, sub)
		}
	}

	data := s.glob(t, "*.yaml")
	if len(data) > 0 && s.run(t, append([]string{"check"}, data...)...) == 0 {
		s.run(t, append([]string{"render", "--out-dir", s.out}, data...)...)
		pages, err := filepath.Glob(filepath.Join(s.out, "*.html"))
		if err != nil {
			t.Fatal(err)
		}
		for _, page := range pages {
			b, err := os.ReadFile(page)
			if err != nil {
				t.Fatal(err)
			}
			s.files[filepath.Base(page)] = b
		}
	}
	if templates := s.glob(t, "templates/*"); len(templates) > 0 {
		// LIMIT: absolute, as floe template check reads a relative dir from the process's working directory,
		// not cli.Env.Dir (floe-usage-issues 58). Name them relative once it reads them from Env.Dir.
		for i, dir := range templates {
			templates[i] = filepath.Join(s.dir, dir)
		}
		s.run(t, append([]string{"template", "check"}, templates...)...)
	}
	if s.has(t, "parts") || s.has(t, "themes") {
		s.run(t, "part", "check")
	}
	if specs := s.glob(t, "diagrams/*.yaml"); len(specs) > 0 {
		s.run(t, append([]string{"diagram", "render"}, specs...)...)
	}
	s.files["commands.golden"] = []byte(s.commands.String())
	return s.files
}

// run runs floe in process from the sample's directory, writes what it printed to commands.golden, and
// returns its exit code. The directories the test made stand as $SAMPLE and $OUT, so that the golden
// files are the same wherever the repository and the temporary directory are; the pages are written as
// they are, where a path of the machine would be a mistake.
func (s *sample) run(t *testing.T, args ...string) int {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := cli.Run(t.Context(), args, cli.Env{
		Getenv: func(k string) string { return s.vars[k] }, Dir: s.dir, Stdout: &stdout, Stderr: &stderr, Environ: nil,
	})
	stand := strings.NewReplacer(s.out, "$OUT", s.dir, "$SAMPLE")
	fmt.Fprintf(&s.commands, "$ floe %s\nexit %d\n", stand.Replace(strings.Join(args, " ")), code)
	for _, printed := range []struct{ name, text string }{{"stdout", stdout.String()}, {"stderr", stderr.String()}} {
		if printed.text != "" {
			fmt.Fprintf(&s.commands, "--- %s\n%s", printed.name, stand.Replace(printed.text))
		}
	}
	return code
}

// has reports whether the sample has the directory sub.
func (s *sample) has(t *testing.T, sub string) bool {
	t.Helper()

	info, err := os.Stat(filepath.Join(s.dir, sub))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return info.IsDir()
}

// glob returns the paths of the sample matching pattern, relative to it, in the order of their names.
func (s *sample) glob(t *testing.T, pattern string) []string {
	t.Helper()

	found, err := filepath.Glob(filepath.Join(s.dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	rel := make([]string, 0, len(found))
	for _, f := range found {
		r, err := filepath.Rel(s.dir, f)
		if err != nil {
			t.Fatal(err)
		}
		rel = append(rel, r)
	}
	slices.Sort(rel)
	return rel
}

// realPath resolves the symlinks of dir, as floe does when it prints a path (/var is /private/var on macOS).
func realPath(t *testing.T, dir string) string {
	t.Helper()

	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// writeGolden writes golden anew with got alone, so that a page no longer drawn leaves no file behind.
func writeGolden(t *testing.T, golden string, got map[string][]byte) {
	t.Helper()

	if err := os.RemoveAll(golden); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(golden, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, b := range got {
		if err := os.WriteFile(filepath.Join(golden, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func compareGolden(t *testing.T, golden string, got map[string][]byte) {
	t.Helper()

	const fix = "run `just golden` and read the change in git diff"
	entries, err := os.ReadDir(golden)
	if err != nil {
		t.Fatalf("%v; %s", err, fix)
	}
	want := map[string]bool{}
	for _, e := range entries {
		want[e.Name()] = true
		if _, ok := got[e.Name()]; !ok {
			t.Errorf("%s: no longer written; %s", filepath.Join(golden, e.Name()), fix)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(got)) {
		path := filepath.Join(golden, name)
		if !want[name] {
			t.Errorf("%s: written, but not in golden/; %s", path, fix)
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if line, g, w, differ := firstDiff(got[name], b); differ {
			t.Errorf("%s:%d: differs; %s\n got: %q\nwant: %q", path, line, fix, g, w)
		}
	}
}

// firstDiff returns the number of the first line in which got and want differ, and that line of each.
func firstDiff(got, want []byte) (int, string, string, bool) {
	if bytes.Equal(got, want) {
		return 0, "", "", false
	}
	g, w := strings.SplitAfter(string(got), "\n"), strings.SplitAfter(string(want), "\n")
	for i := range max(len(g), len(w)) {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return i + 1, gl, wl, true
		}
	}
	return 0, "", "", false
}
