package licenses

import (
	"bufio"
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
var releaseTargets = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"}

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
	version string
	dir     string
	pkgDirs []string
}

// build makes NOTICES.txt: floe's LICENSE, Go's, each module's by path and Lucide's, each file whole.
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
		section(&b, path+" "+m.version, files)
	}

	lucide, err := os.ReadFile(filepath.Join(root, "internal", "icons", "lucide", "LICENSE"))
	if err != nil {
		return nil, err
	}
	section(&b, "Lucide icons (internal/icons/lucide)", []file{{"LICENSE", lucide}})
	return b.Bytes(), nil
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
					Path, Version, Dir string
					Main               bool
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
				m = &module{version: p.Module.Version, dir: p.Module.Dir, pkgDirs: nil}
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
	a, b := bufio.NewScanner(bytes.NewReader(old)), bufio.NewScanner(bytes.NewReader(made))
	for n := 1; ; n++ {
		okA, okB := a.Scan(), b.Scan()
		if !okA && !okB {
			return "the same lines"
		}
		if okA != okB || a.Text() != b.Text() {
			return fmt.Sprintf("line %d is %q, go list gives %q", n, a.Text(), b.Text())
		}
	}
}

func run(t *testing.T, name string, args ...string) []byte {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
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
