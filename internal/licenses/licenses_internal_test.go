package licenses

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/version"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "write NOTICES.txt anew from go list -deps")

// releaseTargets are the GOOS/GOARCH of the release binaries, the justfile's targets (TestReleaseTargets).
var releaseTargets = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"}

const root = "../.."

// NOTICES.txt is what go list -deps of every release target gives, made with the Go go.mod names, as Go's own
// LICENSE changes between releases. Run with go test alone, a result may be cached past a change of the modules, as
// the go list it runs is not counted; just's recipes run tests with -shuffle=on, which caches none.
func TestNotices(t *testing.T) {
	t.Parallel()

	release, err := goRelease(run(t, "go", "mod", "edit", "-json"))
	if err != nil {
		t.Fatal(err)
	}
	if running := strings.Fields(runtime.Version())[0]; running != release {
		msg := fmt.Sprintf("NOTICES.txt is made with %s; this test runs on %s: run it with GOTOOLCHAIN=%s", release, running, release)
		if *update {
			t.Fatal(msg)
		}
		t.Skip(msg)
	}
	got, err := build(t)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile("NOTICES.txt", got, 0o644); err != nil { //nolint:gosec // a file of the repository
			t.Fatal(err)
		}
		return
	}
	if !bytes.Equal(got, Notices) {
		t.Errorf("NOTICES.txt is not what go list -deps gives (%s): run just licenses and commit internal/licenses/NOTICES.txt",
			firstDiff(Notices, got))
	}
}

// goRelease is the release of Go that go.mod names, from go mod edit -json: its toolchain line, else its go line,
// which must then name a release, not a language version alone.
func goRelease(modJSON []byte) (string, error) {
	var mod struct{ Go, Toolchain string }
	if err := json.Unmarshal(modJSON, &mod); err != nil {
		return "", fmt.Errorf("go mod edit -json: %w", err)
	}
	if mod.Toolchain != "" {
		return mod.Toolchain, nil
	}
	v := "go" + mod.Go
	if !version.IsValid(v) || version.Lang(v) == v {
		return "", fmt.Errorf("go.mod's go line %q names no patch release of Go: add a toolchain line", mod.Go)
	}
	return v, nil
}

type module struct {
	dir     string
	pkgDirs []string
}

// build makes NOTICES.txt: floe's LICENSE, Go's, each module's by path, Lucide's and the management page's
// (webNotices), each file whole.
func build(t *testing.T) ([]byte, error) {
	t.Helper()

	var b bytes.Buffer
	floe, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		return nil, err
	}
	section(&b, "floe", []file{{"LICENSE", floe}})

	goroot := strings.TrimSpace(string(run(t, "go", "env", "GOROOT")))
	goFiles, err := read(goroot, []string{"."}, "the Go standard library")
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(goFiles, func(f file) bool { return f.name == "LICENSE" }) {
		return nil, fmt.Errorf("%s has no LICENSE: install Go from https://go.dev/dl, which ships it", goroot)
	}
	section(&b, "Go standard library", goFiles)

	mods, err := modules(t)
	if err != nil {
		return nil, err
	}
	for _, path := range slices.Sorted(maps.Keys(mods)) {
		m := mods[path]
		files, readErr := read(m.dir, m.pkgDirs, path)
		if readErr != nil {
			return nil, readErr
		}
		section(&b, path, files)
	}

	lucide, err := os.ReadFile(filepath.Join(root, "internal", "icons", "lucide", "LICENSE"))
	if err != nil {
		return nil, err
	}
	section(&b, "Lucide icons (internal/icons/lucide)", []file{{"LICENSE", lucide}})

	web, err := webNotices(filepath.Join(root, "internal", "ui", "web"))
	if err != nil {
		return nil, err
	}
	for _, n := range web {
		section(&b, n.title, []file{n.license})
	}
	return b.Bytes(), nil
}

// notice is a section of NOTICES.txt of one file.
type notice struct {
	title   string
	license file
}

// webNotices are the notices of the management page's build in the directory web: each package dist/licenses.json
// names, as Vite wrote it, by name, then Tailwind's, which makes CSS and so is in no licenses.json: the copy
// tailwindcss.LICENSE under the version package.json pins.
func webNotices(web string) ([]notice, error) {
	b, err := os.ReadFile(filepath.Join(web, "dist", "licenses.json")) //nolint:gosec // G304: a file of the repository
	if err != nil {
		return nil, err
	}
	var built []struct{ Name, Version, Text string }
	if err = json.Unmarshal(b, &built); err != nil {
		return nil, fmt.Errorf("dist/licenses.json: %w", err)
	}
	slices.SortFunc(built, func(a, b struct{ Name, Version, Text string }) int { return strings.Compare(a.Name, b.Name) })
	var notices []notice
	for _, p := range built {
		if p.Name == "" || p.Version == "" || p.Text == "" {
			return nil, fmt.Errorf("dist/licenses.json has a package of no name, version or text: %q %q", p.Name, p.Version)
		}
		notices = append(notices, notice{p.Name + " " + p.Version + " (management page)", file{"LICENSE", []byte(p.Text)}})
	}

	b, err = os.ReadFile(filepath.Join(web, "package.json")) //nolint:gosec // G304: a file of the repository
	if err != nil {
		return nil, err
	}
	var pkg struct {
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err = json.Unmarshal(b, &pkg); err != nil {
		return nil, fmt.Errorf("package.json: %w", err)
	}
	tailwind, err := os.ReadFile(filepath.Join(web, "tailwindcss.LICENSE")) //nolint:gosec // G304: as above
	if err != nil {
		return nil, err
	}
	v := pkg.DevDependencies["tailwindcss"]
	if v == "" || len(tailwind) == 0 {
		return nil, errors.New("package.json pins no tailwindcss, or tailwindcss.LICENSE is empty")
	}
	return append(notices, notice{"tailwindcss " + v + " (management page)", file{"LICENSE", tailwind}}), nil
}

// modules are the modules but floe's of the packages built into floe for any release target.
func modules(t *testing.T) (map[string]*module, error) {
	t.Helper()

	mods := map[string]*module{}
	for _, target := range releaseTargets {
		goos, goarch, _ := strings.Cut(target, "/")
		cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "-json", "./cmd/floe")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list for %s: %w", target, err)
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for {
			var p struct {
				Dir      string
				Standard bool
				Module   *struct {
					Path, Dir string
					Main      bool
				}
			}
			if err := dec.Decode(&p); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil, fmt.Errorf("go list for %s: %w", target, err)
			}
			if p.Standard || p.Module == nil || p.Module.Main {
				continue
			}
			m := mods[p.Module.Path]
			if m == nil {
				m = &module{dir: p.Module.Dir, pkgDirs: nil}
				mods[p.Module.Path] = m
			}
			rel, err := filepath.Rel(m.dir, p.Dir)
			if err != nil {
				return nil, err
			}
			if !slices.Contains(m.pkgDirs, rel) {
				m.pkgDirs = append(m.pkgDirs, rel)
			}
		}
	}
	return mods, nil
}

type file struct {
	name string
	body []byte
}

// read reads the license files of a module: those of each package's directory and of every directory above it, up
// to the module's root, named by their path from the root, in byte order.
func read(dir string, pkgDirs []string, what string) ([]file, error) {
	var names []string
	for _, rel := range pkgDirs {
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("%s: a package at %s is outside the module", what, rel)
		}
		for d := filepath.Clean(rel); ; d = filepath.Dir(d) {
			entries, err := os.ReadDir(filepath.Join(dir, d))
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				if name := filepath.ToSlash(filepath.Join(d, e.Name())); e.Type().IsRegular() && isLicense(e.Name()) &&
					!slices.Contains(names, name) {
					names = append(names, name)
				}
			}
			if d == "." {
				break
			}
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s has no license file in %s", what, dir)
	}
	slices.Sort(names)
	files := make([]file, 0, len(names))
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		files = append(files, file{name, body})
	}
	return files, nil
}

// isLicense takes more names than licenses have rather than fewer: any but Go's with a word of one.
// LIMIT: a file named COPYRIGHT alone, and the files in a REUSE LICENSES/ directory, are not taken; no module built
// into floe has them. Add their names when one does.
func isLicense(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".go") {
		return false
	}
	return slices.ContainsFunc([]string{"licen", "copying", "notice", "patents"}, func(w string) bool {
		return strings.Contains(lower, w)
	})
}

func section(b *bytes.Buffer, title string, files []file) {
	rule := strings.Repeat("=", 64)
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	fmt.Fprintf(b, "%s\n%s\n%s\n", rule, title, rule)
	for _, f := range files {
		fmt.Fprintf(b, "\n---- %s ----\n\n", f.name)
		b.Write(f.body)
		if !bytes.HasSuffix(f.body, []byte("\n")) {
			b.WriteString("\n")
		}
	}
}

func firstDiff(old, made []byte) string {
	a, b := bytes.Split(old, []byte("\n")), bytes.Split(made, []byte("\n"))
	for n := 0; ; n++ {
		if n == len(a) || n == len(b) || !bytes.Equal(a[n], b[n]) {
			return fmt.Sprintf("line %d is %q, go list gives %q", n+1, line(a, n), line(b, n))
		}
	}
}

func line(lines [][]byte, n int) []byte {
	if n < len(lines) {
		return lines[n]
	}
	return nil
}

func run(t *testing.T, name string, args ...string) []byte {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("%s %v: %v: %s", name, args, err, exit.Stderr)
		}
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return out
}

// A license file is any but Go's whose name holds a word of one, in the package's directory and every one above it to
// the module's root, and never above the root; a module with none is refused.
func TestNoticesPick(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{
		"LICENSE", "APACHE-LICENSE-2.0", "PATENTS", "NOTICE", "license.go", "README.md",
		"a/LICENSE", "a/b/LICENSE", "a/b/code.go", "c/LICENSE",
	} {
		write("m/" + name)
	}
	write("LICENSE") // above the module's root
	if err := os.Mkdir(filepath.Join(dir, "m", "a", "b", "COPYING"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, err := read(filepath.Join(dir, "m"), []string{filepath.Join("a", "b")}, "m")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		if string(f.body) != "m/"+f.name {
			t.Errorf("%s holds %q", f.name, f.body)
		}
		names = append(names, f.name)
	}
	if want := []string{"APACHE-LICENSE-2.0", "LICENSE", "NOTICE", "PATENTS", "a/LICENSE", "a/b/LICENSE"}; !slices.Equal(names, want) {
		t.Errorf("read picked %q, want %q", names, want)
	}
	if _, err := read(filepath.Join(dir, "m", "c"), []string{"."}, "c"); err != nil {
		t.Errorf("a module of one LICENSE: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "empty", "p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := read(filepath.Join(dir, "empty"), []string{"p"}, "empty"); err == nil {
		t.Error("a module with no license file was read")
	}
}

// The release of Go is go.mod's toolchain line, else its go line, which must name a patch release.
func TestNoticesVersion(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ mod, want string }{
		{`{"Go": "1.27.4"}`, "go1.27.4"},
		{`{"Go": "1.27", "Toolchain": "go1.27.6"}`, "go1.27.6"},
		{`{"Go": "1.27.4", "Toolchain": "go1.28.1"}`, "go1.28.1"},
		{`{"Go": "1.28rc1"}`, "go1.28rc1"},
		{`{"Go": "1.27"}`, ""},
		{`{"Go": "x"}`, ""},
	} {
		got, err := goRelease([]byte(c.mod))
		if got != c.want || (err == nil) != (c.want != "") {
			t.Errorf("goRelease(%s) = %q, %v; want %q", c.mod, got, err, c.want)
		}
	}
}

// releaseTargets are the justfile's targets, which build-all builds; skipped without just, which CI has.
func TestReleaseTargets(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("just"); err != nil {
		t.Skip("just is not on PATH")
	}
	got := strings.Fields(string(run(t, "just", "--evaluate", "targets")))
	want := make([]string, 0, len(releaseTargets))
	for _, target := range releaseTargets {
		want = append(want, strings.Replace(target, "/", "-", 1))
	}
	if !slices.Equal(got, want) {
		t.Errorf("the justfile's targets are %q, releaseTargets %q: make them the same", got, want)
	}
}

// The management page's notices name every package of its build and Tailwind, and a package told only in part is
// refused.
func TestNoticesWeb(t *testing.T) {
	t.Parallel()

	web := filepath.Join(root, "internal", "ui", "web")
	notices, err := webNotices(web)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(web, "dist", "licenses.json"))
	if err != nil {
		t.Fatal(err)
	}
	var built []struct{ Name string }
	if err := json.Unmarshal(b, &built); err != nil {
		t.Fatal(err)
	}
	if len(built) == 0 {
		t.Fatal("dist/licenses.json names no package")
	}
	titles := make([]string, 0, len(notices))
	for _, n := range notices {
		titles = append(titles, n.title)
	}
	names := make([]string, 0, 1+len(built))
	names = append(names, "tailwindcss")
	for _, p := range built {
		names = append(names, p.Name)
	}
	for _, name := range names {
		if !slices.ContainsFunc(titles, func(title string) bool { return strings.HasPrefix(title, name+" ") }) {
			t.Errorf("no notice of %s in %q", name, titles)
		}
	}

	// Each case changes one file of a build that is right
	for name, changed := range map[string]map[string]string{
		"no name":        {"dist/licenses.json": `[{"version": "1", "text": "MIT"}]`},
		"no version":     {"dist/licenses.json": `[{"name": "a", "text": "MIT"}]`},
		"no text":        {"dist/licenses.json": `[{"name": "a", "version": "1"}]`},
		"no tailwindcss": {"package.json": `{"devDependencies": {}}`},
		"empty license":  {"tailwindcss.LICENSE": ""},
	} {
		files := map[string]string{
			"dist/licenses.json":  `[{"name": "a", "version": "1", "text": "MIT"}]`,
			"package.json":        `{"devDependencies": {"tailwindcss": "4.3.5"}}`,
			"tailwindcss.LICENSE": "MIT",
		}
		maps.Copy(files, changed)
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "dist"), 0o755); err != nil {
			t.Fatal(err)
		}
		for path, body := range files {
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := webNotices(dir); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
