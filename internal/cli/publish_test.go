package cli_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/cli"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/servetest"
	"github.com/sukekyo26/floe/internal/store"
)

// served is a server of floe with a template of its own that reads images, and data files in home/p for it:
// the settings of a command that asks it, and that directory.
func served(t *testing.T) (env map[string]string, dir string) {
	t.Helper()

	s := servetest.Start(t, map[string]string{
		"templates/pic/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>" +
			"{{range .Data.i}}<img src=\"{{.}}\">{{end}}</body></html>\n",
		"templates/pic/schema.json":  `{"type": "object", "properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}}}`,
		"templates/pic/example.yaml": "lang: en\ntemplate: pic\n",
		"home/p/d.yaml":              "lang: en\ntemplate: pic\ntitle: D\ni: [img/a.png]\n",
		"home/p/img/a.png":           "\x89PNG\r\n\x1a\n0000",
		"home/p/gone.yaml":           "lang: en\ntemplate: pic\ni: [img/none.png]\n",
		"home/p/h.html":              "<title>H</title>",
		"home/p/docs/a.html":         "<title>A</title>",
		"home/p/w.yaml":              "lang: en\ntemplate: report\nsections:\n  - parts:\n      - text: \"```\\ncode\\n```\"\n",
		"home/q/w.yaml":              "lang: en\ntemplate: report\nsections:\n  - parts:\n      - text: \"```\\ncode\\n```\"\n",
	})
	return s.Env, filepath.Join(s.Dir, "home", "p")
}

// A data file is published through the server, its images sent when it asks for them; listed, drawn again,
// linked, and removed, each told as floe tells its mistakes.
func TestPublishThroughServer(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	url := "http://127.0.0.1:" + env["FLOE_PORT"]
	for _, step := range []struct {
		args      []string
		code      int
		out, errs string
	}{
		{[]string{"publish", "d.yaml"}, 0, url + "/a/d/\n", ""},
		{[]string{"publish", "d.yaml", "--title", "Kept"}, 0, url + "/a/d/\n", `, title "D")` + "\n" + `warning: the title changed from "D" to "Kept"` + "\n"},
		{[]string{"publish", "gone.yaml"}, 1, "", "img/none.png: no such file or directory\n"},
		{[]string{"publish", "--link", "h.html"}, 0, url + "/a/h/\n", ""},
		{[]string{"publish", "w.yaml"}, 0, url + "/a/w/\n", "w.yaml:5:15: warning: this text holds a fenced block of code on its line 1"},
		{[]string{"publish", "--link", "w.yaml", "--slug", "wl"}, 0, url + "/a/wl/\n", "w.yaml:5:15: warning: this text holds a fenced block of code on its line 1"},
		{[]string{"publish", "--link", "docs"}, 0, url + "/a/docs/\n", ""},
		{[]string{"publish", "--link", "docs", "--theme", "ink"}, 1, "", "floe: --theme picks the theme of a page drawn from a data file: a linked file is shown as it is\n"},
		{[]string{"publish", "--link", "h.html", "--theme", "ink"}, 1, "", "floe: --theme picks the theme of a page drawn from a data file: a linked file is shown as it is\n"},
		{[]string{"list"}, 0, "\th\tH\t", ""},
		{[]string{"list"}, 0, "\tdocs\tdocs\t" + dir + "/docs\n", ""},
		{[]string{"rebuild"}, 0, "", ""},
		{[]string{"rm", "d", "none"}, 1, "", "floe: no page \"none\" (see floe list)\n"},
		{[]string{"publish", "x.html"}, 1, "", "floe: x.html is not a data file: publish a page as a data file (.yaml or .yml) for a template such as report, or show HTML as it is with --link\n"},
	} {
		code, out, errs := runIn(dir, env, step.args...)
		if code != step.code || !strings.Contains(out, step.out) || !strings.Contains(errs, step.errs) {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q, %q", step.args, code, out, errs, step.code, step.out, step.errs)
		}
	}
	if code, out, _ := runIn(dir, env, "list"); code != 0 || strings.Contains(out, "\td\t") {
		t.Errorf("floe list after rm = %d, %q", code, out)
	}
	// Refused after it was drawn, as w is another project's, a page keeps its warnings.
	code, _, errs := runIn(filepath.Join(filepath.Dir(dir), "q"), env, "publish", "w.yaml")
	if code != 1 || !strings.Contains(errs, "w.yaml:5:15: warning: this text holds a fenced block of code on its line 1") {
		t.Errorf("floe publish of another project's slug = %d, %q", code, errs)
	}
}

// A page renamed on the management page is published again from its file without --slug at its new slug, its images
// sent when asked for; a title made of the slug follows the rename, so that no title is told changed.
func TestPublishRenamed(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	url := "http://127.0.0.1:" + env["FLOE_PORT"]
	if err := os.WriteFile(filepath.Join(dir, "u.yaml"), []byte("lang: en\ntemplate: pic\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	en := i18n.EnglishCatalog()
	for _, c := range []struct{ file, slug, title string }{{"d.yaml", "d", "D"}, {"u.yaml", "u", "u-b"}} {
		if code, _, errs := runIn(dir, env, "publish", c.file); code != 0 {
			t.Fatalf("floe publish %s = %d, %q", c.file, code, errs)
		}
		body := `{"slug":"` + c.slug + `-b"}`
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPatch, url+"/api/artifacts/"+c.slug, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", url)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.Body.Close() != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("rename %s: %v, %v", c.slug, res, err)
		}
		code, out, errs := runIn(dir, env, "publish", c.file)
		told := strings.Split(en.Msg("cli_updated_titled", c.slug+"-b", "\x00", c.title), "\x00")
		_, tail, _ := strings.Cut(errs, ",")
		if code != 0 || out != url+"/a/"+c.slug+"-b/\n" || !strings.HasPrefix(errs, told[0]) || ","+tail != told[1]+"\n" {
			t.Errorf("floe publish %s again = %d, %q, %q; want %s-b, told %q", c.file, code, out, errs, c.slug, told)
		}
	}
}

// floe archive stops at the first page not there; an archived page is off floe list, and on floe list --archived
// with a fifth column on every line, until --undo.
func TestArchiveThroughServer(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	run := func(want int, args ...string) string {
		t.Helper()
		code, out, errs := runIn(dir, env, args...)
		if code != want {
			t.Fatalf("floe %v = %d, %q, %q", args, code, out, errs)
		}
		return out + errs
	}
	// rows returns the lines of floe list args, each checked to have tabs tabs, by slug.
	rows := func(tabs int, args ...string) map[string]string {
		t.Helper()
		by := map[string]string{}
		for line := range strings.Lines(run(0, append([]string{"list"}, args...)...)) {
			line = strings.TrimSuffix(line, "\n")
			if strings.Count(line, "\t") != tabs {
				t.Errorf("floe list %v: %q has not %d tabs", args, line, tabs)
			}
			by[strings.Split(line, "\t")[1]] = line
		}
		return by
	}
	run(0, "publish", "d.yaml")
	run(0, "publish", "--link", "h.html")
	if got := run(1, "archive", "none", "d"); got != "floe: no page \"none\" (see floe list)\n" {
		t.Errorf("floe archive none d: %q", got)
	}
	if _, ok := rows(3)["d"]; !ok {
		t.Error("floe archive went on past none")
	}
	run(0, "archive", "d")
	run(0, "archive", "d")
	if listed := rows(3); listed["d"] != "" || listed["h"] == "" {
		t.Errorf("floe list after archive: %v", listed)
	}
	all := rows(4, "--archived")
	if !strings.HasSuffix(all["d"], "\tD\t\tarchived") || !strings.HasSuffix(all["h"], "/h.html\t") {
		t.Errorf("floe list --archived: %v", all)
	}
	run(0, "archive", "--undo", "d")
	if _, ok := rows(3)["d"]; !ok {
		t.Error("floe archive --undo left d off the list")
	}
	if got := run(2, "archive"); got != "floe: requires at least 1 arg(s), only received 0: run floe --help\n" {
		t.Errorf("floe archive: %q", got)
	}
}

// floe tag puts tags on pages and takes them off in order, stopping at the first page not there; a name is one as
// written, commas and all, and a name to remove and to add stays on.
func TestTagThroughServer(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	run := func(want int, args ...string) string {
		t.Helper()
		code, out, errs := runIn(dir, env, args...)
		if code != want {
			t.Fatalf("floe %v = %d, %q, %q", args, code, out, errs)
		}
		return out + errs
	}
	tags := func(slug string) []string {
		t.Helper()
		return store.New(env["FLOE_DIR"]).ReadMeta(slug).Tags
	}
	run(0, "publish", "d.yaml")
	run(0, "publish", "--link", "h.html")
	run(0, "tag", "d", "h", "--add", "設計", "--add", "a,b")
	if !slices.Equal(tags("d"), []string{"設計", "a,b"}) || !slices.Equal(tags("h"), []string{"設計", "a,b"}) {
		t.Errorf("added: %q, %q", tags("d"), tags("h"))
	}
	run(0, "tag", "d", "--remove", "設計", "--add", "設計", "--remove", "a,b")
	if !slices.Equal(tags("d"), []string{"設計"}) {
		t.Errorf("removed and added: %q", tags("d"))
	}
	if got := run(1, "tag", "h", "none", "d", "--add", "x"); got != "floe: no page \"none\" (see floe list)\n" {
		t.Errorf("floe tag h none d: %q", got)
	}
	if !slices.Contains(tags("h"), "x") || slices.Contains(tags("d"), "x") {
		t.Errorf("floe tag went on past none, or stopped before it: %q, %q", tags("h"), tags("d"))
	}
	if got := run(1, "tag", "\xff", "--add", "x"); got != "floe: no page \"\xff\" (see floe list)\n" {
		t.Errorf("floe tag of a slug no UTF-8: %q", got)
	}
	if got := run(2, "tag", "d"); got != "floe: at least one of the flags in the group [add remove] is required: run floe --help\n" {
		t.Errorf("floe tag d: %q", got)
	}
	if got := run(2, "tag", "--add", "x"); got != "floe: requires at least 1 arg(s), only received 0: run floe --help\n" {
		t.Errorf("floe tag --add x: %q", got)
	}
	for _, flag := range []string{"--add", "--remove"} {
		if got := run(1, "tag", "d", flag, "a\xffb"); got != notUTF8(flag) {
			t.Errorf("floe tag d %s of no UTF-8: %q", flag, got)
		}
	}
}

// A tag that is no UTF-8 is refused before the server is started.
func TestTagNotUTF8NoServer(t *testing.T) {
	t.Parallel()

	env := map[string]string{"HOME": t.TempDir(), "FLOE_PORT": strconv.Itoa(freePort(t))}
	if code, _, errs := runIn("", env, "tag", "d", "--add", "a\xffb"); code != 1 || errs != notUTF8("--add") {
		t.Errorf("floe tag d --add of no UTF-8 = %d, %q", code, errs)
	}
	if code, out, _ := runIn("", env, "stop"); code != 0 || out != "server is not running\n" {
		t.Errorf("floe stop after = %d, %q", code, out)
	}
}

// Without a server, floe stop says so, and a setting that cannot be read stops every command of the server.
func TestServerCommandsAlone(t *testing.T) {
	t.Parallel()

	free := map[string]string{"HOME": t.TempDir(), "FLOE_PORT": "1"}
	if code, out, _ := runIn("", free, "stop"); code != 0 || out != "server is not running\n" {
		t.Errorf("floe stop = %d, %q", code, out)
	}
	for _, args := range [][]string{{"list"}, {"rm", "x"}, {"archive", "x"}, {"rebuild"}, {"serve"}, {"stop"}} {
		code, _, errs := runIn("/", map[string]string{"HOME": "/h", "FLOE_PORT": "0x10"}, args...)
		if code != 1 || errs != "floe: invalid FLOE_PORT \"0x10\": use a port number from 1 to 65535\n" {
			t.Errorf("floe %v = %d, %q", args, code, errs)
		}
	}
}

// notUTF8 is the line floe tells for a text sent that is no UTF-8, given at.
func notUTF8(at string) string {
	return "floe: " + at + " is not UTF-8; the server takes only UTF-8: rename it, or give it in UTF-8\n"
}

// made writes a file or makes a directory whose name is no UTF-8, skipping the test where the system takes none
// (APFS on macOS).
func made(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Skipf("the system takes no name that is no UTF-8: %v", err)
	}
}

// publishes runs each publish of cases in dir with env, which tells errs and stores nothing.
func publishes(t *testing.T, env map[string]string, cases []struct {
	in, errs string
	args     []string
},
) {
	t.Helper()

	for _, c := range cases {
		if code, _, errs := runIn(c.in, env, c.args...); code != 1 || errs != c.errs {
			t.Errorf("floe %q = %d, %q, want %q", c.args, code, errs, c.errs)
		}
	}
	if _, out, _ := runIn("", env, "list"); out != "" {
		t.Errorf("stored:\n%s", out)
	}
}

// A publish whose title, description or theme is no UTF-8 is refused, named where it was given, with the other
// mistakes of the publish; a slug no UTF-8 is told once, as Accept tells it. A description sent empty clears it.
func TestPublishNotUTF8(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	notData := "floe: x.txt is not a data file: publish a page as a data file (.yaml or .yml) for a template such as " +
		"report, or show HTML as it is with --link\n"
	publishes(t, env, []struct {
		in, errs string
		args     []string
	}{
		{dir, notUTF8("--title"), []string{"publish", "d.yaml", "--title", "a\xffb"}},
		// The first that encoding/json/v2 meets.
		{dir, notUTF8("--description"), []string{"publish", "d.yaml", "--description", "a\xffb", "--theme", "t\xff"}},
		{dir, notUTF8("--title") + notData, []string{"publish", "x.txt", "--title", "a\xffb"}},
		{dir, "none.yaml: no such file or directory\n" + notUTF8("--title"), []string{"publish", "none.yaml", "--title", "a\xffb"}},
		{
			dir, "gone.html: no such file or directory\n" + notUTF8("--title"),
			[]string{"publish", "--link", "gone.html", "--title", "a\xffb"},
		},
		{dir, "floe: invalid slug \"a\xff\": use 1 to 64 lowercase letters, digits and hyphens, starting and ending with " +
			"a letter or digit\n", []string{"publish", "--link", "h.html", "--slug", "a\xff"}},
	})
	runIn(dir, env, "publish", "--link", "h.html", "--description", "d")
	runIn(dir, env, "publish", "--link", "h.html", "--description", "")
	if b := get(t, "http://127.0.0.1:"+env["FLOE_PORT"]+"/api/artifacts"); !strings.Contains(b, `"description":""`) {
		t.Errorf("a description sent empty is kept: %s", b)
	}
}

// A publish of a file, a link or a project whose name is no UTF-8 is refused, named where it was given: argument 1,
// the working directory a link is under, or the project. A file so named is not read.
func TestPublishNameNotUTF8(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	src := "lang: en\ntemplate: pic\n"
	made(t, os.WriteFile(filepath.Join(dir, "a\xffb.yaml"), []byte(src), 0o600))
	made(t, os.WriteFile(filepath.Join(dir, "k\xff.html"), []byte("<title>K</title>"), 0o600))
	made(t, os.Mkdir(filepath.Join(dir, "p\xffq"), 0o700))
	made(t, os.WriteFile(filepath.Join(dir, "p\xffq", "q.yaml"), []byte(src), 0o600))
	made(t, os.WriteFile(filepath.Join(dir, "p\xffq", "h.html"), []byte("<title>H</title>"), 0o600))
	in := filepath.Join(dir, "p\xffq")
	publishes(t, env, []struct {
		in, errs string
		args     []string
	}{
		{dir, notUTF8("argument 1"), []string{"publish", "a\xffb.yaml", "--slug", "ab"}},
		{dir, notUTF8("argument 1"), []string{"publish", "--link", filepath.Join(dir, "k\xff.html")}},
		// Not read: no second mistake that it is not there.
		{dir, notUTF8("argument 1"), []string{"publish", "gone\xff.yaml"}},
		{in, notUTF8("the working directory"), []string{"publish", "--link", "h.html"}},
		{
			in, notUTF8("the project name (of the git remote, the repository's directory or the working directory)"),
			[]string{"publish", "q.yaml"},
		},
	})
}

// A text that is no UTF-8 is refused before the server starts: its store is not made.
func TestPublishNotUTF8Alone(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "d.yaml"), []byte("lang: en\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A port no test serves at: the server a mistake would start is refused there, as the store is made first.
	env := map[string]string{"HOME": tmp, "FLOE_DIR": filepath.Join(tmp, "store"), "FLOE_PORT": "1", "BROWSER": "true"}
	if code, _, errs := runIn(tmp, env, "publish", "d.yaml", "--title", "a\xffb"); code != 1 || errs != notUTF8("--title") {
		t.Errorf("floe publish = %d, %q", code, errs)
	}
	if _, err := os.Stat(env["FLOE_DIR"]); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the store: %v", err)
	}
}

// get is the body that a GET of u answers.
func get(t *testing.T, u string) string {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, u, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(res.Body)
	if err = errors.Join(err, res.Body.Close()); err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// publishedTogether is what floe publish writes to stdout and stderr into one buffer, in the order written.
func publishedTogether(t *testing.T, dir string, env map[string]string, args ...string) string {
	t.Helper()

	var both bytes.Buffer
	code := cli.Run(append([]string{"publish"}, args...), cli.Env{Stdout: &both, Stderr: &both, Dir: dir, Getenv: func(key string) string { return env[key] }})
	if code != 0 {
		t.Fatalf("floe publish %v = %d, %q", args, code, both.String())
	}
	return both.String()
}

// A page the directory floe publish runs in gives its project is told after its URL, on stderr: when it is new, when
// it replaces another file's (--force) and when it was emptied of its project; not when it keeps its project, nor for a
// link, which is in its target's directory's project.
func TestPublishSaysProject(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	url := "http://127.0.0.1:" + env["FLOE_PORT"] + "/a/"
	other := filepath.Join(filepath.Dir(dir), "q")
	const page = "lang: en\ntemplate: report\n"
	for _, name := range []string{"n.yaml", "m.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(page), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(other, "v.yaml"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	s := store.New(env["FLOE_DIR"])
	for _, step := range []struct {
		what string
		dir  string
		args []string
		told string // what ends the output, after the URL; "" for none
	}{
		{"a new page", dir, []string{"n.yaml"}, "\npage in project p\n"},
		{"a new page of no project", "/", []string{filepath.Join(dir, "m.yaml")}, "\npage in no project\n"},
		{"a page keeping its project", dir, []string{"n.yaml"}, ""},
		{"a link", dir, []string{"--link", "h.html"}, ""},
		{"another file's page, of another project (--force)", other, []string{"v.yaml", "--slug", "n", "--force"}, "\npage in project q\n"},
		{"another file's page, of the same project (--force)", other, []string{"w.yaml", "--slug", "n", "--force"}, ""},
	} {
		got := publishedTogether(t, step.dir, env, step.args...)
		if !strings.HasPrefix(got, url) || strings.Contains(got, "page in ") != (step.told != "") || !strings.HasSuffix(got, step.told) {
			t.Errorf("%s: floe publish %v wrote %q, want the URL first and then %q", step.what, step.args, got, step.told)
		}
	}
	// Emptied of its project, a page takes the one published from again; from where there is none, it has none still.
	if _, err := s.Edit("m", store.Edit{Project: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if got := publishedTogether(t, "/", env, filepath.Join(dir, "m.yaml")); strings.Contains(got, "page in ") {
		t.Errorf("a page of no project, published from where there is none, wrote %q", got)
	}
	if got := publishedTogether(t, dir, env, "m.yaml"); !strings.Contains(got, "\npage in project p\n") {
		t.Errorf("a page emptied of its project wrote %q, want its project told", got)
	}
	// Moved on the management page, a page keeps its project.
	if _, err := s.Edit("m", store.Edit{Project: ptr("moved")}); err != nil {
		t.Fatal(err)
	}
	if got := publishedTogether(t, dir, env, "m.yaml"); strings.Contains(got, "page in ") {
		t.Errorf("a page moved to another project wrote %q, want its project kept and not told", got)
	}
}

// --move-project puts a page in the project of the directory floe publish runs in, though it is in another; no other
// publish moves it, and it moves none of the page's task. Not with a link, nor from a place with no project.
func TestPublishMoveProject(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	other := filepath.Join(filepath.Dir(dir), "q")
	file := filepath.Join(dir, "n.yaml")
	if err := os.WriteFile(file, []byte("lang: en\ntemplate: report\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := store.New(env["FLOE_DIR"])
	projectOfPage := func() string {
		t.Helper()
		m := s.ReadMeta("n")
		if m == nil {
			t.Fatal("no page n")
		}
		return m.Project
	}
	said := func(at string, args ...string) string {
		t.Helper()
		return publishedTogether(t, at, env, append([]string{file}, args...)...)
	}
	if got := said(dir); !strings.HasSuffix(got, "\npage in project p\n") {
		t.Fatalf("a new page wrote %q", got)
	}
	if _, err := s.Edit("n", store.Edit{Work: &store.WorkEdit{Task: ptr("t1")}}); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		what string
		at   string
		args []string
		told string // what ends the output after the URL; "" for none
		in   string // the project of the page after it
	}{
		{"another place, without the flag", other, nil, "", "p"},
		{"another place, with the flag", other, []string{"--move-project"}, "\npage in project q\n", "q"},
		{"the same place again", other, []string{"--move-project"}, "", "q"},
		{"the first place, without the flag", dir, nil, "", "q"},
		{"the first place, with the flag", dir, []string{"--move-project"}, "\npage in project p\n", "p"},
	} {
		got := said(step.at, step.args...)
		if strings.Contains(got, "page in ") != (step.told != "") || !strings.HasSuffix(got, step.told) || projectOfPage() != step.in {
			t.Errorf("%s: wrote %q, the page is in %q; want it to end with %q and be in %q", step.what, got, projectOfPage(), step.told, step.in)
		}
	}
	if m := s.ReadMeta("n"); m == nil || m.Work == nil || m.Work.Task != "t1" {
		t.Errorf("moved, the page's work is %+v, want its task t1 kept", m)
	}
	if got, err := s.ReadTasks(); err != nil || !reflect.DeepEqual(got, tasks) {
		t.Errorf("moved, the tasks are %v (%v), want them as they were, %v", got, err, tasks)
	}
	for _, step := range []struct {
		what, at string
		args     []string
		want     string
	}{
		{"a place with no project", "/", []string{"--move-project", file}, "has no project, which --move-project puts the page in"},
		{"a link", dir, []string{"--link", "h.html", "--move-project"}, "none of the others can be"},
	} {
		code, out, errs := runIn(step.at, env, append([]string{"publish"}, step.args...)...)
		if code != 1 || out != "" || !strings.Contains(errs, step.want) {
			t.Errorf("%s: floe publish %v = %d, %q, %q; want 1, nothing and %q", step.what, step.args, code, out, errs, step.want)
		}
	}
	if projectOfPage() != "p" || s.ReadMeta("h") != nil {
		t.Errorf("refused, the page is in %q and h is %v, want p and none", projectOfPage(), s.ReadMeta("h"))
	}
	// Replacing another file's page (--force) puts the page in the place's project, with the flag as without it.
	if got := publishedTogether(t, other, env, "w.yaml", "--slug", "n", "--force", "--move-project"); !strings.Contains(got, "\npage in project q\n") || projectOfPage() != "q" {
		t.Errorf("another file's page replaced with the flag wrote %q and is in %q, want it in q", got, projectOfPage())
	}
}
