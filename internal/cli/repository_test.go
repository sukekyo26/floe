package cli_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"floe/internal/testfs"
	"floe/internal/version"
	"floe/internal/yamlpos"
)

// setting is a .floe/repository.yaml of this floe's version and the documents' directories docs.
func setting(docs ...string) string {
	s := "version: " + strconv.Quote(version.Version) + "\ndocuments:\n"
	for _, d := range docs {
		s += "  - " + strconv.Quote(d) + "\n"
	}
	return s
}

// repo writes files in a new repository: a .git, so that no .floe above the test's directory is found, and the
// setting of the documents' directories docs.
func repo(t *testing.T, files map[string]string, docs ...string) string {
	t.Helper()
	files[".git/HEAD"] = ""
	files[".floe/repository.yaml"] = setting(docs...)
	return write(t, files)
}

// The repository is found from a directory under it too, and its documents are named from its root: the same
// output wherever it runs, through a link to it too. The look stops at a .git, a directory or a file.
func TestRepositoryFindsRoot(t *testing.T) {
	t.Parallel()

	dir := repo(t, map[string]string{"docs/a.yaml": good}, "docs")
	page := filepath.Join("docs", "a.html")
	if code, stdout, stderr := run(dir, "repository", "render"); code != 0 || stdout != page+"\n" || stderr != "" {
		t.Fatalf("floe repository render = %d, %q, %q", code, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "a.yaml"), []byte(good+"lead: changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	testfs.Symlink(t, dir, link)
	_, _, want := run(dir, "repository", "check")
	if !strings.Contains(want, page+" is not what "+filepath.Join("docs", "a.yaml")) {
		t.Errorf("floe repository check at the root = %q", want)
	}
	for _, at := range []string{filepath.Join(dir, "docs"), link, filepath.Join(link, "docs")} {
		if code, _, stderr := run(at, "repository", "check"); code != 1 || stderr != want {
			t.Errorf("floe repository check in %s = %d, %q, want %q", at, code, stderr, want)
		}
	}
	for _, git := range []string{".git/HEAD", ".git"} {
		inner := filepath.Join(dir, "inner")
		if err := os.MkdirAll(filepath.Dir(filepath.Join(inner, git)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(inner, git), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr := run(inner, "repository", "check"); code != 1 || !strings.Contains(stderr, "no .floe/repository.yaml in") {
			t.Errorf("floe repository check under a %s = %d, %q", git, code, stderr)
		}
		if err := os.RemoveAll(inner); err != nil {
			t.Fatal(err)
		}
	}
	if code, _, stderr := run("", "repository", "check"); code != 1 || !strings.Contains(stderr, "working directory cannot be read") {
		t.Errorf("floe repository check with no working directory = %d, %q", code, stderr)
	}
}

// Another version is refused before anything is read, its keys too, and nothing is written; the guides are
// printed of any version, and out of a repository.
func TestRepositoryVersion(t *testing.T) {
	t.Parallel()

	dir := repo(t, map[string]string{"docs/a.yaml": good}, "docs")
	if err := os.WriteFile(filepath.Join(dir, ".floe", "repository.yaml"),
		[]byte("version: \"0.0.1\"\nlater: true\ndocuments: [docs]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"render", "check"} {
		code, _, stderr := run(dir, "repository", command)
		if code != 1 || !strings.Contains(stderr, "floe 0.0.1, and this is floe "+version.Version) || strings.Contains(stderr, "later") {
			t.Errorf("floe repository %s of another version = %d, %q", command, code, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "a.html")); err == nil {
		t.Error("floe repository render of another version wrote the page")
	}
	for _, at := range []string{dir, t.TempDir()} {
		for _, args := range [][]string{{"guide"}, {"template", "guide"}, {"part", "guide"}, {"theme", "guide"}} {
			if code, stdout, stderr := run(at, append([]string{"repository"}, args...)...); code != 0 || stdout == "" || stderr != "" {
				t.Errorf("floe repository %v = %d, %q", args, code, stderr)
			}
		}
	}
}

// check tells every document's mistakes and the pages not drawn again, in folders, .yml too, from any directory
// the setting names (., a dot name), each once; an HTML file with no data file is left alone.
func TestRepositoryCheck(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name  string
		docs  []string
		pages []string
	}{
		{"folders", []string{"docs", "docs/b"}, []string{"docs/a.html", "docs/b/c.html"}},
		{"the root", []string{"."}, []string{"docs/a.html", "docs/b/c.html"}},
		{"a dot name", []string{".github"}, []string{".github/w.html"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := repo(t, map[string]string{
				"docs/a.yaml": good, "docs/b/c.yml": good, "docs/note.html": "<p>by hand</p>",
				"docs/.drafts/x.yaml": "x: [\n", ".github/w.yaml": good,
			}, c.docs...)
			code, stdout, stderr := run(dir, "repository", "render")
			var want []string
			for _, p := range c.pages {
				want = append(want, filepath.FromSlash(p))
			}
			if code != 0 || stdout != strings.Join(want, "\n")+"\n" || stderr != "" {
				t.Fatalf("floe repository render = %d, %q, %q", code, stdout, stderr)
			}
			if code, _, stderr := run(dir, "repository", "check"); code != 0 || stderr != "" {
				t.Errorf("floe repository check = %d, %q", code, stderr)
			}
			if err := os.Remove(filepath.Join(dir, want[0])); err != nil {
				t.Fatal(err)
			}
			code, _, stderr = run(dir, "repository", "check")
			if code != 1 || strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, want[0]+" is missing") ||
				!strings.Contains(stderr, "with floe repository render") {
				t.Errorf("floe repository check without %s = %d, %q", want[0], code, stderr)
			}
		})
	}
}

// The repository's templates, parts and themes are in .floe alone, the settings of one's own read not; one not
// there is none; two that are one are refused; they show as absolute paths of the root as found, and a template's
// example reads its images wherever the command runs.
func TestRepositoryOwnPlaces(t *testing.T) {
	t.Parallel()

	dir := repo(t, map[string]string{
		".floe/templates/pic/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>" +
			"{{range .Data.i}}<img src=\"{{.}}\">{{end}}</body></html>\n",
		".floe/templates/pic/schema.json":  `{"type": "object", "properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}}}`,
		".floe/templates/pic/example.yaml": "lang: en\ntemplate: pic\ni: [assets/a.png]\n",
		".floe/templates/pic/assets/a.png": "\x89PNG\r\n\x1a\n0000",
	}, "docs")
	env := map[string]string{"FLOE_TEMPLATES_DIR": "relative", "FLOE_PARTS_DIR": "relative"}
	for _, at := range []string{dir, filepath.Join(dir, ".floe")} {
		if code, _, stderr := runIn(at, env, "repository", "template", "check"); code != 0 || stderr != "" {
			t.Errorf("floe repository template check in %s = %d, %q", at, code, stderr)
		}
	}
	if code, stdout, stderr := runIn(dir, env, "repository", "template", "new", "memo"); code != 0 ||
		stdout != "made "+filepath.Join(dir, ".floe", "templates", "memo")+"\n" || stderr != "" {
		t.Errorf("floe repository template new = %d, %q, %q", code, stdout, stderr)
	}
	_, stdout, _ := runIn(dir, env, "repository", "template", "list")
	if !strings.Contains(stdout, "pic\t"+filepath.Join(dir, ".floe", "templates", "pic")+"\t") || strings.Contains(stdout, `\\?\`) {
		t.Errorf("floe repository template list = %q", stdout)
	}
	t.Run("one place in another", func(t *testing.T) {
		t.Parallel()
		dir := repo(t, map[string]string{".floe/templates/x": ""}, "docs")
		testfs.Symlink(t, filepath.Join(dir, ".floe", "templates"), filepath.Join(dir, ".floe", "parts"))
		if code, _, stderr := run(dir, "repository", "check"); code != 1 || !strings.Contains(stderr, filepath.Join(".floe", "parts")+" (") ||
			!strings.Contains(stderr, "are one directory") {
			t.Errorf("floe repository check of .floe/parts in .floe/templates = %d, %q", code, stderr)
		}
	})
}

// Nothing out of the repository is read or written: a page, a data file, the setting or .floe that is a link out
// of it, or one that leads nowhere it can tell; a file in a place that leads out of the place. A link in it is
// followed.
func TestRepositoryOutside(t *testing.T) {
	t.Parallel()

	out := write(t, map[string]string{"x.html": "theirs", "x.yaml": good, "repository.yaml": "secret: [\n"})
	cases := []struct {
		name  string
		links map[string]string
		args  []string
		told  string
	}{
		{"a page", map[string]string{"docs/p.html": filepath.Join(out, "x.html")}, []string{"render"}, filepath.Join("docs", "p.html")},
		{"a data file", map[string]string{"docs/q.yaml": filepath.Join(out, "x.yaml")}, []string{"render"}, filepath.Join("docs", "q.yaml")},
		{"a link that leads nowhere", map[string]string{"docs/q.yaml": filepath.Join("gone", "..", "..", "..", "x.yaml")}, []string{"render"}, filepath.Join("docs", "q.yaml")},
		{"the setting", map[string]string{".floe/repository.yaml": filepath.Join(out, "repository.yaml")}, []string{"check"}, filepath.Join(".floe", "repository.yaml")},
		{".floe", map[string]string{".floe/templates": out}, []string{"template", "new", "t"}, filepath.Join(".floe", "templates")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := repo(t, map[string]string{"docs/p.yaml": good}, "docs")
			for at, to := range c.links {
				_ = os.Remove(filepath.Join(dir, at))
				testfs.Symlink(t, to, filepath.Join(dir, at))
			}
			code, _, stderr := run(dir, append([]string{"repository"}, c.args...)...)
			if code != 1 || !strings.HasPrefix(stderr, c.told+": this leads out of the repository") || strings.Contains(stderr, "secret") {
				t.Errorf("floe repository %v = %d, %q", c.args, code, stderr)
			}
		})
	}
	t.Run("new under a link out", func(t *testing.T) {
		t.Parallel()
		dir := write(t, map[string]string{".git/HEAD": ""})
		elsewhere := t.TempDir()
		testfs.Symlink(t, elsewhere, filepath.Join(dir, ".floe"))
		if code, _, stderr := run(dir, "repository", "new", "docs"); code != 1 || !strings.Contains(stderr, "leads out of the repository") {
			t.Errorf("floe repository new = %d, %q", code, stderr)
		}
		if names, _ := os.ReadDir(elsewhere); len(names) != 0 {
			t.Errorf("floe repository new wrote %v out of the repository", names)
		}
	})
	t.Run("a file of a place out of it", func(t *testing.T) {
		t.Parallel()
		dir := repo(t, map[string]string{
			".floe/templates/memo/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
			".floe/templates/memo/page.html":    "<!doctype html><html><head><title>t</title></head><body><img src=\"{{asset \"a.svg\"}}\"></body></html>",
			".floe/templates/memo/example.yaml": "lang: en\ntemplate: memo\n",
			".floe/templates/memo/assets/b.svg": "<svg/>",
			"docs/a.svg":                        "<svg/>",
		}, "docs")
		testfs.Symlink(t, filepath.Join("..", "..", "..", "..", "docs", "a.svg"), filepath.Join(dir, ".floe", "templates", "memo", "assets", "a.svg"))
		if code, _, stderr := run(dir, "repository", "template", "check"); code != 1 || !strings.Contains(stderr, "a.svg: path escapes from parent") {
			t.Errorf("floe repository template check = %d, %q", code, stderr)
		}
	})
	t.Run("a place in it through a link", func(t *testing.T) {
		t.Parallel()
		dir := repo(t, map[string]string{"shared/templates/memo/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
			"shared/templates/memo/page.html":    "<!doctype html><html><head><title>t</title></head><body></body></html>",
			"shared/templates/memo/example.yaml": "lang: en\ntemplate: memo\n"}, "docs")
		testfs.Symlink(t, filepath.Join("..", "shared", "templates"), filepath.Join(dir, ".floe", "templates"))
		if code, _, stderr := run(dir, "repository", "template", "check"); code != 0 || stderr != "" {
			t.Errorf("floe repository template check = %d, %q", code, stderr)
		}
	})
}

// The documents' directories are written from the root, slash-separated, and the setting by its schema.
func TestRepositoryPaths(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ setting, told string }{
		{"documents: [../x]", "../x is no directory"},
		{"documents: [/etc]", "/etc is no directory"},
		{"documents: [docs/]", "docs/ is no directory"},
		{"documents: [./docs]", "./docs is no directory"},
		{"documents: []", "minItems"},
		{"documents: [docs]\nmore: 1", "more"},
	} {
		dir := repo(t, map[string]string{}, "docs")
		src := "version: " + strconv.Quote(version.Version) + "\n" + c.setting + "\n"
		if err := os.WriteFile(filepath.Join(dir, ".floe", "repository.yaml"), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr := run(dir, "repository", "check"); code != 1 || !strings.Contains(stderr, c.told) {
			t.Errorf("floe repository check of %q = %d, %q", c.setting, code, stderr)
		}
	}
}

// new writes the setting once, of names YAML reads as written, and refuses one there or above, and a name that is
// not UTF-8; a repository within another, of its own .git, is made.
func TestRepositoryNew(t *testing.T) {
	t.Parallel()

	names := []string{"a\u0085b", "a b", "a b", "\xef\xbb\xbfx", "a\x7fb", "a\u009bb", "a b", "a\x01b",
		"設計/仕様", "a\U0001F600b", "a #b", `a"b\c`, "2026", "null"}
	dir := write(t, map[string]string{".git/HEAD": ""})
	if code, stdout, stderr := run(dir, append([]string{"repository", "new"}, names...)...); code != 0 ||
		stdout != "made "+filepath.Join(".floe", "repository.yaml")+"\n" || stderr != "" {
		t.Fatalf("floe repository new = %d, %q, %q", code, stdout, stderr)
	}
	src, err := os.ReadFile(filepath.Join(dir, ".floe", "repository.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_, values, err := yamlpos.ParseMap("repository.yaml", src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	got, _ := values["documents"].([]any)
	if values["version"] != version.Version || len(got) != len(names) {
		t.Fatalf("floe repository new wrote %v", values)
	}
	for i, name := range names {
		if got[i] != name {
			t.Errorf("floe repository new wrote %q as %q", name, got[i])
		}
	}
	if code, _, stderr := run(filepath.Join(dir, ".floe"), "repository", "new", "docs"); code != 1 || !strings.Contains(stderr, "is there already") {
		t.Errorf("floe repository new again = %d, %q", code, stderr)
	}
	inner := filepath.Join(dir, "inner")
	if err := os.MkdirAll(filepath.Join(inner, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(inner, "repository", "new", "docs"); code != 0 || stderr != "" {
		t.Errorf("floe repository new within another = %d, %q", code, stderr)
	}
	t.Run("checks what it wrote", func(t *testing.T) {
		t.Parallel()
		dir := write(t, map[string]string{".git/HEAD": "", "2026/a.yaml": good, "a #b/c.yaml": good})
		for _, args := range [][]string{{"new", "2026", "a #b"}, {"render"}, {"check"}} {
			if code, _, stderr := run(dir, append([]string{"repository"}, args...)...); code != 0 || stderr != "" {
				t.Errorf("floe repository %v = %d, %q", args, code, stderr)
			}
		}
	})
	t.Run("not UTF-8", func(t *testing.T) {
		t.Parallel()
		dir := write(t, map[string]string{".git/HEAD": ""})
		if code, _, stderr := run(dir, "repository", "new", "a\xffb"); code != 1 || !strings.Contains(stderr, "in UTF-8") {
			t.Errorf("floe repository new of a name not UTF-8 = %d, %q", code, stderr)
		}
		if _, err := os.Stat(filepath.Join(dir, ".floe")); err == nil {
			t.Error("floe repository new of a name not UTF-8 made .floe")
		}
	})
}

// floeCommand is a command of floe's that floe repository has one of its own for.
var floeCommand = regexp.MustCompile(`\bfloe (part|template|theme|check|render)\b`)

// The commands of one's own under floe repository have no theme default, which reads the store, and their help
// names neither the settings of one's own nor floe's commands for them.
func TestRepositoryCommands(t *testing.T) {
	t.Parallel()

	dir := repo(t, map[string]string{}, "docs")
	if code, _, _ := run(dir, "repository", "theme", "default"); code == 0 {
		t.Error("floe repository theme default ran")
	}
	for _, kind := range []string{"part", "template", "theme"} {
		for _, sub := range []string{"", "list", "show", "new", "check"} {
			args := []string{"repository", kind}
			if sub != "" {
				args = append(args, sub)
			}
			_, help, _ := run(dir, append(args, "--help")...)
			for _, line := range strings.Split(help, "\n") {
				if strings.Contains(line, "FLOE_") || floeCommand.MatchString(strings.ReplaceAll(line, "floe repository ", "")) {
					t.Errorf("floe %v --help names %q", args, line)
				}
			}
		}
	}
}

// No command of floe repository starts the server or makes the store.
func TestRepositoryNoServer(t *testing.T) {
	t.Parallel()

	store := filepath.Join(t.TempDir(), "store")
	dir := repo(t, map[string]string{"docs/a.yaml": good}, "docs")
	env := map[string]string{"FLOE_DIR": store, "FLOE_PORT": "1"}
	for _, args := range [][]string{{"render"}, {"check"}, {"template", "list"}, {"template", "new", "t"}, {"part", "check"}, {"theme", "list"}} {
		if code, _, stderr := runIn(dir, env, append([]string{"repository"}, args...)...); code != 0 {
			t.Errorf("floe repository %v = %d, %q", args, code, stderr)
		}
	}
	if _, err := os.Stat(store); err == nil {
		t.Error("floe repository made the store")
	}
}
