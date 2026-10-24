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
	"github.com/sukekyo26/floe/internal/store"
)

var update = flag.Bool("update", false, "write the golden files of each sample anew")

// TestCorpus runs floe on each sample in testdata as an agent would, and compares the exit codes, what it
// printed and the pages it wrote with the sample's golden/. What a sample holds says what runs: its data
// files (those floe takes, .yaml and .yml) are checked together and rendered one at a time and together;
// templates/, parts/, themes/ and icons/ are its own, and templates/ is checked by floe template check,
// parts/ and themes/ by floe part check; the specs in diagrams/ are drawn by floe diagram render.
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
				// A mistake of the run stays told here, not as bytes that differ on the next machine.
				if !t.Failed() {
					writeGolden(t, golden, got)
				}
				return
			}
			compareGolden(t, golden, got)
		})
	}
}

// sample is a run of floe on the sample in dir: the settings of floe it runs under, and the files of its
// golden/ so far, commands.golden among them.
type sample struct {
	dir, tmp string
	vars     map[string]string
	commands strings.Builder
	files    map[string][]byte
}

// LIMIT: the golden files hold the English of Go's and other libraries' mistakes as floe passes them on
// (text/template's in pag of unusable-template). A PR that upgrades Go or a library checks that the change
// `just golden` makes is to those lines alone.
func runSample(t *testing.T, dir string) map[string][]byte {
	t.Helper()

	tmp := realPath(t, t.TempDir())
	s := &sample{dir: realPath(t, dir), tmp: tmp, files: map[string][]byte{}, vars: map[string]string{
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

	var data []string
	for _, file := range s.glob(t, "*", false) {
		if store.IsDataFile(file) {
			data = append(data, file)
		}
	}
	if len(data) > 0 {
		s.run(t, false, append([]string{"check"}, data...)...)
	}
	// One at a time, each into a directory of its own: floe render writes nothing while one of its files has a
	// mistake, and the page of a data file that passes is drawn beside one that does not.
	drawnBy := map[string]string{}
	for _, file := range data {
		out := filepath.Join(s.tmp, "pages", file)
		s.run(t, false, "render", "--out-dir", out, file)
		pages, err := filepath.Glob(filepath.Join(out, "*.html"))
		if err != nil {
			t.Fatal(err)
		}
		for _, page := range pages {
			name := filepath.Base(page)
			if other, ok := drawnBy[name]; ok {
				t.Errorf("%s and %s both write %s, which golden/ holds one of; rename one of them", other, file, name)
			}
			drawnBy[name] = file
			b, err := os.ReadFile(page)
			if err != nil {
				t.Fatal(err)
			}
			s.files[name] = s.drawn(t, "the page "+name+" of "+file, b)
		}
	}
	// And together, for what render does of several, as writing none while one has a mistake; its pages are
	// those above.
	if len(data) > 1 {
		s.run(t, false, append([]string{"render", "--out-dir", filepath.Join(s.tmp, "together")}, data...)...)
	}
	if templates := s.glob(t, "templates/*", true); len(templates) > 0 {
		// LIMIT: absolute, as floe template check reads a relative dir from the process's working directory,
		// not cli.Env.Dir (floe-usage-issues 58). Name them relative once it reads them from Env.Dir.
		for i, dir := range templates {
			templates[i] = filepath.Join(s.dir, dir)
		}
		s.run(t, false, append([]string{"template", "check"}, templates...)...)
	}
	if s.has(t, "parts") || s.has(t, "themes") {
		s.run(t, false, "part", "check")
	}
	if specs := s.glob(t, "diagrams/*.yaml", false); len(specs) > 0 {
		s.run(t, true, append([]string{"diagram", "render"}, specs...)...)
	}
	s.files["commands.golden"] = []byte(s.commands.String())
	return s.files
}

// run runs floe in process from the sample's directory and writes what it printed to commands.golden. The
// directories the test made stand as $SAMPLE and $TMP, so that the golden files are the same wherever the
// repository and the temporary directory are; but in what floe drew (drawn: the diagrams it printed), as in
// the pages, where a path of the machine would be a mistake.
func (s *sample) run(t *testing.T, drawn bool, args ...string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := cli.Run(t.Context(), args, cli.Env{
		Getenv: func(k string) string { return s.vars[k] }, Dir: s.dir, Stdout: &stdout, Stderr: &stderr, Environ: nil,
	})
	stand := strings.NewReplacer(s.tmp, "$TMP", s.dir, "$SAMPLE")
	out := stand.Replace(stdout.String())
	if drawn {
		out = string(s.drawn(t, "the stdout of floe "+args[0], stdout.Bytes()))
	}
	fmt.Fprintf(&s.commands, "$ floe %s\nexit %d\n", stand.Replace(strings.Join(args, " ")), code)
	for _, printed := range []struct{ name, text string }{{"stdout", out}, {"stderr", stand.Replace(stderr.String())}} {
		if printed.text != "" {
			fmt.Fprintf(&s.commands, "--- %s\n%s", printed.name, printed.text)
		}
	}
}

// drawn returns b, what floe drew (a page or the diagrams it printed), as it is, and fails the test when it
// holds a directory of the machine: here, on the machine that writes the golden files too, not on the next.
func (s *sample) drawn(t *testing.T, what string, b []byte) []byte {
	t.Helper()

	for _, dir := range []string{s.dir, s.tmp} {
		if bytes.Contains(b, []byte(dir)) {
			t.Errorf("%s holds the path %s of this machine", what, dir)
		}
	}
	return b
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

// glob returns the paths of the sample matching pattern, relative to it, in the order of their names: the
// directories alone when dirs, and none whose name starts with a dot (.DS_Store), which Glob's * matches.
func (s *sample) glob(t *testing.T, pattern string, dirs bool) []string {
	t.Helper()

	found, err := filepath.Glob(filepath.Join(s.dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	rel := make([]string, 0, len(found))
	for _, f := range found {
		if hidden(filepath.Base(f)) {
			continue
		}
		if dirs {
			info, err := os.Stat(f)
			if err != nil {
				t.Fatal(err)
			}
			if !info.IsDir() {
				continue
			}
		}
		r, err := filepath.Rel(s.dir, f)
		if err != nil {
			t.Fatal(err)
		}
		rel = append(rel, r)
	}
	slices.Sort(rel)
	return rel
}

// hidden reports whether name is one a system or an editor leaves, as macOS's .DS_Store.
func hidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// realPath resolves the symlinks of dir, as floe does when it prints a path (/var is /private/var on macOS).
func realPath(t *testing.T, dir string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
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
		if hidden(e.Name()) {
			continue
		}
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
