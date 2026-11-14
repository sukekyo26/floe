package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// setMeta sets the key of the page slug's meta.json to value, as a hand edit does.
func setMeta(t *testing.T, env map[string]string, slug, key string, value any) {
	t.Helper()

	path := filepath.Join(env["FLOE_DIR"], slug, "meta.json")
	b, err := os.ReadFile(path) //nolint:gosec // G304: the test's own store
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	m[key] = value
	if b, err = json.Marshal(m); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// floe info prints the same names in the same order for every kind of page, text values quoted and none empty,
// the project as floe task takes it, and writes nothing.
func TestInfo(t *testing.T) {
	t.Parallel()

	env, home := served(t)
	repo := filepath.Join(filepath.Dir(home), "r")
	if err := os.MkdirAll(filepath.Join(repo, "docs"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "docs", "d.yaml"), []byte("lang: en\ntemplate: pic\ntitle: \"D\\tE\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://example.com/o/named.git"}} {
		git := exec.CommandContext(t.Context(), "git", args...)
		git.Dir = repo
		git.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := git.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run := func(dir string, want int, args ...string) string {
		t.Helper()
		code, out, errs := runIn(dir, env, args...)
		if code != want {
			t.Fatalf("floe %v = %d, %q, %q", args, code, out, errs)
		}
		return out + errs
	}
	run(repo, 0, "publish", "docs/d.yaml", "--slug", "a")
	run(repo, 0, "task", "--slug", "a", "--task", "#1", "--category", "designs")
	run(repo, 0, "tag", "a", "--add", "plans", "--add", "a, b")
	run(home, 0, "publish", "--link", "h.html", "--slug", "l")
	run(home, 0, "task", "--slug", "l", "--pin")
	run(home, 0, "publish", "--link", "docs", "--slug", "f")
	run(home, 0, "archive", "f")
	run(home, 0, "publish", "w.yaml", "--slug", "n")
	setMeta(t, env, "n", "project", "")
	run(home, 0, "publish", "d.yaml", "--slug", "s")
	setMeta(t, env, "s", "project", " p ")
	actual, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	before := storeWrites(t, env)
	base := "http://127.0.0.1:" + env["FLOE_PORT"] + "/a/"
	data := func(slug, title, project, repository, source string) string {
		return "slug\t" + slug + "\ntitle\t" + title + "\nurl\t" + base + slug + "/\nkind\tdata\nproject\t" + project +
			"\ntask\t\ncategory\t\npinned\tfalse\ntags\t\narchived\tfalse\nrepository\t" + repository +
			"\nsource\t" + source + "\nversion\t1\n"
	}
	for slug, want := range map[string]string{
		"a": "slug\ta\ntitle\t\"D\\tE\"\nurl\t" + base + "a/\nkind\tdata\nproject\t\"named\"\ntask\t\"#1\"\n" +
			"category\t\"designs\"\npinned\tfalse\ntags\t\"plans\" \"a, b\"\narchived\tfalse\nrepository\t\"named\"\n" +
			"source\t\"docs/d.yaml\"\nversion\t1\n",
		"l": "slug\tl\ntitle\t\"H\"\nurl\t" + base + "l/\nkind\tlink\nproject\t\"p\"\ntask\t\ncategory\t\npinned\ttrue\n" +
			"tags\t\narchived\tfalse\nrepository\t\nsource\t\"" + filepath.Join(actual, "h.html") + "\"\nversion\t\n",
		// A folder outside a repository is in the project of its own name.
		"f": "slug\tf\ntitle\t\"docs\"\nurl\t" + base + "f/\nkind\tfolder\nproject\t\"docs\"\ntask\t\ncategory\t\npinned\tfalse\n" +
			"tags\t\narchived\ttrue\nrepository\t\nsource\t\"" + filepath.Join(actual, "docs") + "\"\nversion\t\n",
		"n": data("n", `"n"`, "", "", `"`+filepath.Join(actual, "w.yaml")+`"`),
		"s": data("s", `"D"`, `"p"`, "", `"`+filepath.Join(actual, "d.yaml")+`"`),
	} {
		got := run(home, 0, "info", slug)
		// The time is the server's own: told as floe list tells it.
		at := strings.LastIndex(got, "updated\t")
		if at < 0 || !regexp.MustCompile(`^updated\t\d{4}-\d\d-\d\d \d\d:\d\d\n$`).MatchString(got[at:]) {
			t.Errorf("floe info %s = %q: no updated line at its end", slug, got)
			continue
		}
		if got[:at] != want {
			t.Errorf("floe info %s =\n%s\nwant\n%s", slug, got[:at], want)
		}
	}
	if got := run(home, 1, "info", "nope"); !strings.Contains(got, `"nope"`) {
		t.Errorf("floe info nope = %q", got)
	}
	run(home, 2, "info")
	run(home, 2, "info", "a", "l")
	if after := storeWrites(t, env); after != before {
		t.Errorf("floe info wrote:\n%s\nwant\n%s", after, before)
	}
}
