package corpus_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/cli"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/kit"
)

var update = flag.Bool("update", false, "write the golden files of each sample anew")

// TestCorpus runs floe on each sample in testdata as an agent would, and compares the exit codes, what it
// printed and the pages it wrote with the sample's golden/: its data files (those floe takes, .yaml and .yml)
// are checked together and rendered one at a time and together. The samples in testdata/mistakes are each
// the least input of a mistake floe tells, and run alike; render-same-page there shows floe refusing two data
// files that write one page, kept there as it has no page.
func TestCorpus(t *testing.T) {
	t.Parallel()

	for _, name := range samples(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir, err := filepath.Abs(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			got := runSample(t, name, dir)
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

// mistakes is the directory in testdata that holds samples, not one.
const mistakes = "mistakes"

// samples returns the samples in testdata, relative to it: those of testdata, then those of
// testdata/mistakes, each in the order of their names.
func samples(t *testing.T) []string {
	t.Helper()

	var found []string
	for _, parent := range []string{".", mistakes} {
		entries, err := os.ReadDir(filepath.Join("testdata", parent))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() && !hidden(e.Name()) && (parent != "." || e.Name() != mistakes) {
				found = append(found, filepath.Join(parent, e.Name()))
			}
		}
	}
	return found
}

// sample is a run of floe on the sample in dir: whether its golden/ keeps the pages it draws, and the files
// of its golden/ so far, commands.golden among them.
type sample struct {
	dir, tmp   string
	keepsPages bool
	env        map[string]string // FLOE_ICONS_DIR, and FLOE_PARTS_DIR and the like for the directories of one's own
	commands   strings.Builder
	files      map[string][]byte
}

// LIMIT: the golden files hold the English of Go's and other libraries' mistakes as floe passes them on. A PR
// that upgrades Go or a library checks that the change `just golden` makes is to those lines alone.
func runSample(t *testing.T, name, dir string) map[string][]byte {
	t.Helper()

	tmp := realPath(t, t.TempDir())
	// A sample of mistakes is for what floe tells: its golden/ holds commands.golden alone.
	s := &sample{
		dir: realPath(t, dir), tmp: tmp, keepsPages: filepath.Dir(name) != mistakes, env: nil, files: map[string][]byte{},
	}
	s.env = map[string]string{"FLOE_ICONS_DIR": filepath.Join(s.dir, ".icons")}
	// One's own parts, themes and templates, in own/: checked first, as an agent checks what it wrote.
	own := false
	for _, kind := range []string{"parts", "themes", "templates"} {
		if fi, err := os.Stat(filepath.Join(s.dir, "own", kind)); err == nil && fi.IsDir() {
			s.env["FLOE_"+strings.ToUpper(kind)+"_DIR"] = filepath.Join(s.dir, "own", kind)
			own = true
		}
	}
	if own {
		s.run(t, "part", "check")
	}
	if _, ok := s.env["FLOE_TEMPLATES_DIR"]; ok {
		s.run(t, "template", "check")
	}

	var data []string
	for _, file := range s.glob(t, "*", isFile) {
		if files.IsDataFile(file) {
			data = append(data, file)
		}
	}
	if len(data) > 0 {
		s.run(t, append([]string{"check"}, data...)...)
	}
	each := s.renderEach(t, data)
	if len(data) > 1 {
		s.renderTogether(t, data, each)
	}
	s.files["commands.golden"] = []byte(s.commands.String())
	return s.files
}

// alone is what renderEach found of the data files, each rendered on its own.
type alone struct {
	pages   map[string][]byte // the pages drawn, by name; of two of one name, the first
	drawnBy map[string]string // the data file each page was drawn from
	failed  []string          // the data files whose render failed
	clash   bool              // two data files wrote one page, but for case
}

// renderEach renders each data file on its own, into a directory of its own, as floe render writes nothing
// while one of its files has a mistake: the page of one that passes is drawn beside one that does not. It
// puts the pages in s.files when the sample keeps them.
func (s *sample) renderEach(t *testing.T, data []string) alone {
	t.Helper()

	each := alone{pages: map[string][]byte{}, drawnBy: map[string]string{}, failed: nil, clash: false}
	// By the name in lower case, as floe render refuses two pages of one name but for case: one file to a
	// filesystem that ignores case (macOS), where golden/ is checked out too.
	named := map[string]string{}
	for _, file := range data {
		out := filepath.Join(s.tmp, "pages", file)
		if s.run(t, "render", "--out-dir", out, file) != 0 {
			each.failed = append(each.failed, file)
		}
		for name, b := range s.pages(t, out, "the page %s of "+file) {
			if other, ok := named[strings.ToLower(name)]; ok {
				each.clash = true
				// A sample of mistakes keeps no page: two of one name are floe's refusal it shows.
				if s.keepsPages {
					but := ""
					if other != name {
						but = " but for case"
					}
					t.Errorf("%s and %s both write %s%s, which golden/ holds one of; rename one of them",
						each.drawnBy[other], file, name, but)
				}
				continue
			}
			named[strings.ToLower(name)] = name
			each.drawnBy[name] = file
			each.pages[name] = b
			if s.keepsPages {
				s.files[name] = b
			}
		}
	}
	return each
}

// renderTogether renders the data files together, for what floe render does of several: it passes when, and
// only when, each passes alone and no two write one page, and then draws each page as renderEach drew it, so
// that what one file read is not drawn into another's page; else it writes none.
func (s *sample) renderTogether(t *testing.T, data []string, each alone) {
	t.Helper()

	out := filepath.Join(s.tmp, "together")
	code := s.run(t, append([]string{"render", "--out-dir", out}, data...)...)
	pages := s.pages(t, out, "the page %s that render together drew")
	passes := len(each.failed) == 0 && !each.clash
	switch {
	case code != 0 && passes:
		t.Errorf("render together failed, though each data file passed alone and no two write one page")
	case code == 0 && len(each.failed) > 0:
		t.Errorf("render together passed, though %s failed alone", strings.Join(each.failed, ", "))
	case code == 0 && each.clash:
		t.Errorf("render together passed, though two data files write one page")
	}
	if code != 0 {
		for _, name := range slices.Sorted(maps.Keys(pages)) {
			t.Errorf("render together failed but wrote %s", name)
		}
		return
	}
	for _, name := range slices.Sorted(maps.Keys(each.drawnBy)) {
		if _, ok := pages[name]; !ok {
			t.Errorf("render together did not write %s, which %s alone writes", name, each.drawnBy[name])
		}
	}
	for _, name := range slices.Sorted(maps.Keys(pages)) {
		switch drawn, ok := each.pages[name]; {
		case !ok:
			t.Errorf("render together wrote %s, which no data file alone writes", name)
		case !bytes.Equal(pages[name], drawn):
			t.Errorf("render together drew %s otherwise than %s alone", name, each.drawnBy[name])
		}
	}
}

// pages reads the pages floe render wrote in out, by name, each checked by drawn as what (with its name for
// the %s).
func (s *sample) pages(t *testing.T, out, what string) map[string][]byte {
	t.Helper()

	found, err := filepath.Glob(filepath.Join(out, "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string][]byte{}
	for _, page := range found {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(page)
		pages[name] = s.drawn(t, fmt.Sprintf(what, name), b)
	}
	return pages
}

// run runs floe in process from the sample's directory, writes what it printed to commands.golden and
// returns its exit code. In stdout and stderr the directories the test made stand as $SAMPLE and $TMP, so
// that the golden files are the same wherever the repository and the temporary directory are. Its icons are
// those of the sample's .icons, none kept for a sample without one.
func (s *sample) run(t *testing.T, args ...string) int {
	t.Helper()

	var stdout, stderr bytes.Buffer
	getenv := func(key string) string { return s.env[key] }
	code := cli.Run(args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: s.dir, Getenv: getenv})
	stand := strings.NewReplacer(s.tmp, "$TMP", s.dir, "$SAMPLE")
	out := stand.Replace(stdout.String())
	fmt.Fprintf(&s.commands, "$ floe %s\nexit %d\n", stand.Replace(strings.Join(args, " ")), code)
	for _, printed := range []struct{ name, text string }{{"stdout", out}, {"stderr", stand.Replace(stderr.String())}} {
		if printed.text != "" {
			fmt.Fprintf(&s.commands, "--- %s\n%s", printed.name, printed.text)
		}
	}
	return code
}

// drawn returns b, what floe drew (a page), as it is, and fails the test when it holds a directory of the
// machine: here, on the machine that writes the golden files too, not on the next.
func (s *sample) drawn(t *testing.T, what string, b []byte) []byte {
	t.Helper()

	for _, dir := range []string{s.dir, s.tmp} {
		if bytes.Contains(b, []byte(dir)) {
			t.Errorf("%s holds the path %s of this machine", what, dir)
		}
	}
	return b
}

// isFile picks the files glob returns.
var isFile = func(info fs.FileInfo) bool { return !info.IsDir() }

// glob returns the paths of the sample matching pattern whose kind is one keep takes, relative to the sample,
// in the order of their names, but none whose name starts with a dot (.DS_Store), which Glob's * matches.
func (s *sample) glob(t *testing.T, pattern string, keep func(fs.FileInfo) bool) []string {
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
		info, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if !keep(info) {
			continue
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

// The sample example is the example.yaml of floe's report, as it is: its page is the one develop drew of it.
func TestExampleIsReport(t *testing.T) {
	t.Parallel()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	report, _ := k.Template("report")
	b, err := os.ReadFile(filepath.Join("testdata", "example", "example.yaml"))
	if err != nil || !bytes.Equal(b, report.Example()) {
		t.Errorf("testdata/example/example.yaml is not report's example.yaml (%v): copy it again", err)
	}
}
