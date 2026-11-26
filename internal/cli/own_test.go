package cli_test

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"floe/internal/cli"
	"floe/internal/files"
	"floe/internal/i18n"
	"floe/internal/testfs"
	"floe/internal/timebox"
)

// runEnv runs floe with args in dir, with the environment env alone, and returns its exit code, stdout and
// stderr.
func runEnv(dir string, env map[string]string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	getenv := func(key string) string { return env[key] }
	code := cli.Run(args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir, Getenv: getenv})
	return code, stdout.String(), stderr.String()
}

const (
	ownSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "string"}`
	ownPage   = `<div data-floe-part="mine">{{.}}</div>`
	dirLink   = "this is a link to a directory, and floe reads only real directories here: make it a directory (links to files are read)"
)

func link(t *testing.T, to, at string) {
	t.Helper()
	testfs.Symlink(t, to, at)
}

// One's own directories: a setting must be an absolute directory that can be read; a default that is not there,
// or is under a file, is none. Files in them may be links to anywhere, which the system follows; a link to a
// directory is refused.
func TestOwnDirs(t *testing.T) {
	t.Parallel()

	repo := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": ownPage, "mine/part.css": ""})
	dir := write(t, map[string]string{"d.yaml": good})
	parts := filepath.Join(dir, "parts")
	if err := os.MkdirAll(filepath.Join(parts, "mine"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"schema.json", "part.html", "part.css"} {
		link(t, filepath.Join(repo, "mine", f), filepath.Join(parts, "mine", f))
	}
	linked := filepath.Join(dir, "linked")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	link(t, filepath.Join(repo, "mine"), filepath.Join(linked, "mine"))
	dangling := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": ownPage})
	link(t, "gone.css", filepath.Join(dangling, "mine", "part.css"))
	homeOfAFile := defaultAFile(t, "parts")
	words := func(err error) string { return testfs.Words(t, err) }
	_, notThere := files.ReadDir(filepath.Join(dir, "prats"))
	_, notDir := files.ReadDir(filepath.Join(dir, "d.yaml"))
	_, _, dangles := files.Read(filepath.Join(dangling, "mine", "part.css"))
	for _, tt := range []struct {
		name string
		env  map[string]string
		code int
		errs string
	}{
		{"none set", nil, 0, ""},
		{"the home a file", cli.HomeVars(filepath.Join(dir, "d.yaml")), 0, ""},
		{"default not there", cli.HomeVars(filepath.Join(dir, "none")), 0, ""},
		// XDG_CONFIG_HOME is read before the home, whose default (a file) would be refused.
		{"the XDG default not there", cli.HomeEnv(homeOfAFile, map[string]string{"XDG_CONFIG_HOME": filepath.Join(dir, "none")}), 0, ""},
		{"files linked out", map[string]string{"FLOE_PARTS_DIR": parts}, 0, ""},
		{"the directory a link", map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "plink")}, 0, ""},
		{"relative", map[string]string{"FLOE_PARTS_DIR": "parts"}, 1, "floe: FLOE_PARTS_DIR is parts, a relative path; set it to an absolute path\n"},
		{"set, not there", map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "prats")}, 1, "floe: FLOE_PARTS_DIR is " + filepath.Join(dir, "prats") + ", which cannot be used as a directory (" + words(notThere) + "); make it a directory floe can use, or set FLOE_PARTS_DIR to another directory\n"},
		{"set, a file", map[string]string{"FLOE_THEMES_DIR": filepath.Join(dir, "d.yaml")}, 1, "floe: FLOE_THEMES_DIR is " + filepath.Join(dir, "d.yaml") + ", which cannot be used as a directory (" + words(notDir) + "); make it a directory floe can use, or set FLOE_THEMES_DIR to another directory\n"},
		{"a link to a directory", map[string]string{"FLOE_PARTS_DIR": linked}, 1, filepath.Join(linked, "mine") + ": " + dirLink + "\n"},
		{"a link to nothing", map[string]string{"FLOE_PARTS_DIR": dangling}, 1, filepath.Join(dangling, "mine", "part.css") + ": " + words(dangles) + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.name == "the directory a link" {
				link(t, parts, filepath.Join(dir, "plink"))
			}
			for _, args := range [][]string{{"part", "check"}, {"check", "d.yaml"}} {
				if code, stdout, stderr := runEnv(dir, tt.env, args...); code != tt.code || stdout != "" || stderr != tt.errs {
					t.Errorf("floe %v = %d, %q, %q; want %d, %q", args, code, stdout, stderr, tt.code, tt.errs)
				}
			}
		})
	}
	if code, _, _ := runEnv(dir, nil, "part", "check", "x"); code != 2 {
		t.Errorf("floe part check x = %d, want 2", code)
	}
}

// A directory a setting names that cannot be read or written is told for the setting, where the command reads or
// writes it; a command that does not use it is not stopped.
func TestDirSettingTold(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"file": ""})
	file := filepath.Join(dir, "file")
	told := func(key, value, why string) string {
		return "floe: " + key + " is " + value + ", which cannot be used as a directory (" + why + "); make it a " +
			"directory floe can use, or set " + key + " to another directory\n"
	}
	tells := func(t *testing.T, set map[string]string, args []string, code int, errs string) {
		t.Helper()
		env := cli.HomeEnv(filepath.Join(dir, "home"), map[string]string{"FLOE_PORT": "1"})
		maps.Copy(env, set)
		if got, _, stderr := runEnv(dir, env, args...); got != code || stderr != errs {
			t.Errorf("%v floe %v = %d, %q; want %d, %q", set, args, got, stderr, code, errs)
		}
	}
	tells(t, map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "none")}, []string{"part", "check"}, 0, "")
	t.Run("the icons a file", func(t *testing.T) {
		testfs.UnixOnly(t, "a path under a file is read as not there on Windows (W12)")
		t.Parallel()
		tells(t, map[string]string{"FLOE_ICONS_DIR": file}, []string{"icons", "search", "aws", "x"}, 1, told("FLOE_ICONS_DIR", file, "not a directory"))
	})
	t.Run("the store a file", func(t *testing.T) {
		t.Parallel()
		tells(t, map[string]string{"FLOE_DIR": file}, []string{"serve"}, 1, told("FLOE_DIR", file, testfs.Words(t, files.MkdirAll(file, 0o700))))
	})
	t.Run("cannot be read or written", func(t *testing.T) {
		testfs.UnixOnly(t, "chmod keeps a directory from being read or written")
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root reads and writes any directory")
		}
		unread, unwritten := filepath.Join(dir, "unread"), filepath.Join(dir, "unwritten")
		for d, mode := range map[string]os.FileMode{unread: 0, unwritten: 0o500} {
			if err := os.Mkdir(d, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(d, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(d, 0o700) }) //nolint:errcheck // for TempDir to remove it
		}
		tells(t, map[string]string{"FLOE_PARTS_DIR": unread}, []string{"part", "check"}, 1, told("FLOE_PARTS_DIR", unread, "permission denied"))
		tells(t, map[string]string{"FLOE_PARTS_DIR": unwritten}, []string{"part", "new", "x"}, 1, told("FLOE_PARTS_DIR", unwritten, "permission denied"))
		tells(t, map[string]string{"FLOE_DIR": unwritten}, []string{"serve"}, 1, told("FLOE_DIR", unwritten, "permission denied"))
	})
}

// overlap is the mistake of the places of two kinds of one's own that are one directory, or one in the other.
func overlap(key1, dir1, key2, dir2 string) string {
	return "floe: " + key1 + " (" + dir1 + ") and " + key2 + " (" + dir2 + ") are one directory, or one is in the " +
		"other, where floe would read the files of one as those of the other; make " + key1 + " and " + key2 +
		" two directories, neither in the other\n"
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Places of one's own that are one directory, or one in the other, as the system takes them, are refused once,
// for their settings, before they are read: a setting not there yet too, and a default only when it is there.
func TestOwnDirsOverlap(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{
		"d.yaml": good, "pp/mine/schema.json": ownSchema, "pp/mine/part.html": ownPage, "pp/mine/part.css": "",
		"ok/mine/schema.json": ownSchema, "ok/mine/part.html": ownPage, "ok/mine/part.css": "",
	})
	pp, ok, home := filepath.Join(dir, "pp"), filepath.Join(dir, "ok"), filepath.Join(dir, "home")
	defaultParts := cli.DefaultPlace(home, "parts")
	mkdirs(t, filepath.Join(pp, "th"), filepath.Join(pp, ".th"), filepath.Join(dir, "pp2"), filepath.Join(dir, "pp22"), defaultParts)
	link(t, pp, filepath.Join(dir, "same"))
	link(t, filepath.Join(pp, "th"), filepath.Join(dir, "in"))
	link(t, filepath.Join(pp, "gone"), filepath.Join(dir, "dangling"))
	link(t, filepath.Join(dir, "elsewhere", "x"), filepath.Join(dir, "far"))
	mkdirs(t, filepath.Join(dir, "pq"))
	link(t, filepath.Join(dir, "elsewhere", "none"), filepath.Join(dir, "pq", "out"))
	// What the system says listing value, which floe tells in its words.
	unusable := func(key, value string) string {
		_, err := files.ReadDir(value)
		return "floe: " + key + " is " + value + ", which cannot be used as a directory (" + testfs.Words(t, err) + "); " +
			"make it a directory floe can use, or set " + key + " to another directory\n"
	}
	parts, themes, templates := "FLOE_PARTS_DIR", "FLOE_THEMES_DIR", "FLOE_TEMPLATES_DIR"
	for _, tt := range []struct {
		name string
		env  map[string]string
		args []string
		errs string
	}{
		{"one", map[string]string{parts: pp, themes: pp}, nil, overlap(parts, pp, themes, pp)},
		{"themes in parts", map[string]string{parts: pp, themes: pp + "/th"}, nil, overlap(parts, pp, themes, pp+"/th")},
		{"parts in themes", map[string]string{parts: pp, themes: dir}, nil, overlap(parts, pp, themes, dir)},
		{"a link to it", map[string]string{parts: pp, themes: dir + "/same"}, nil, overlap(parts, pp, themes, dir+"/same")},
		{"a link into it", map[string]string{parts: pp, themes: dir + "/in"}, nil, overlap(parts, pp, themes, dir+"/in")},
		{"..", map[string]string{parts: pp, themes: pp + "/th/.."}, nil, overlap(parts, pp, themes, pp+"/th/..")},
		{"a default", map[string]string{themes: defaultParts}, nil, overlap(parts, defaultParts, themes, defaultParts)},
		{"a dot directory", map[string]string{parts: pp, themes: pp + "/.th"}, nil, overlap(parts, pp, themes, pp+"/.th")},
		{"not there, in", map[string]string{parts: pp, themes: pp + "/none"}, nil, overlap(parts, pp, themes, pp+"/none")},
		{"not there, ending in /", map[string]string{parts: pp, themes: pp + "/none/"}, nil, overlap(parts, pp, themes, pp+"/none/")},
		{"not there, with //", map[string]string{parts: pp, themes: pp + "//none"}, nil, overlap(parts, pp, themes, pp+"//none")},
		{"both not there, one in the other", map[string]string{parts: dir + "/gone", themes: dir + "/gone/th"}, nil, overlap(parts, dir+"/gone", themes, dir+"/gone/th")},
		{"templates", map[string]string{parts: pp, templates: pp}, []string{"check", "d.yaml"}, overlap(parts, pp, templates, pp)},
		{
			"all three",
			map[string]string{parts: pp, themes: pp, templates: pp},
			[]string{"check", "d.yaml"},
			overlap(parts, pp, themes, pp) + overlap(parts, pp, templates, pp) + overlap(themes, pp, templates, pp),
		},
		{"templates, not read", map[string]string{parts: ok, templates: ok}, nil, ""},
		{"a name alike", map[string]string{parts: dir + "/pp2", themes: dir + "/pp22"}, nil, ""},
		{"not there, out", map[string]string{parts: pp, themes: dir + "/none"}, nil, unusable(themes, dir+"/none")},
		{
			"a file and under it",
			map[string]string{parts: dir + "/d.yaml", themes: dir + "/d.yaml/th"},
			nil,
			unusable(parts, dir+"/d.yaml") + unusable(themes, dir+"/d.yaml/th"),
		},
		// Its words are taken where the test runs (Unix): Windows folds none/.. as written.
		{"not there, then ..", map[string]string{parts: pp, themes: pp + "/none/.."}, nil, ""},
		// A link to nothing is compared where it leads, before what it leads to is there.
		{"a link to nothing", map[string]string{parts: pp, themes: dir + "/dangling"}, nil, overlap(parts, pp, themes, dir+"/dangling")},
		{"a link to nothing, out", map[string]string{parts: pp, themes: dir + "/far"}, nil, unusable(themes, dir+"/far")},
		// Compared where it leads alone, not where it is (RealAbove's LIMIT).
		{"a link to nothing in it, out", map[string]string{parts: dir + "/pq", themes: dir + "/pq/out"}, nil, unusable(themes, dir+"/pq/out")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.name == "not there, then .." {
				testfs.UnixOnly(t, "Windows folds none/.. as written, though none is not there")
				tt.errs = unusable(themes, pp+"/none/..")
			}
			env := cli.HomeEnv(home, tt.env)
			args := tt.args
			if args == nil {
				args = []string{"part", "check"}
			}
			code, stdout, stderr := runEnv(dir, env, args...)
			if code != map[bool]int{true: 0, false: 1}[tt.errs == ""] || stdout != "" || stderr != tt.errs {
				t.Errorf("floe %v = %d, %q, %q; want %q", args, code, stdout, stderr, tt.errs)
			}
		})
	}
}

// A mistake of the templates' setting stops the commands that read templates alone: part list, part new and theme new
// read and make as before, check is stopped.
func TestTemplatesDirNotRead(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good})
	home := filepath.Join(dir, "home")
	env := cli.HomeEnv(home, map[string]string{"FLOE_TEMPLATES_DIR": "tpl"})
	for _, tt := range []struct {
		args []string
		code int
		out  string   // in stdout
		told []string // in stderr, which is empty without them
	}{
		{[]string{"part", "new", "x"}, 0, filepath.Join(cli.DefaultPlace(home, "parts"), "x"), nil},
		{[]string{"theme", "new", "y"}, 0, filepath.Join(cli.DefaultPlace(home, "themes"), "y"), nil},
		{[]string{"check", "d.yaml"}, 1, "", []string{"FLOE_TEMPLATES_DIR", "tpl", "relative"}},
	} {
		code, stdout, stderr := runEnv(dir, env, tt.args...)
		ok := code == tt.code && strings.Contains(stdout, tt.out) && (len(tt.told) > 0) == (stderr != "")
		for _, w := range tt.told {
			ok = ok && strings.Contains(stderr, w)
		}
		if !ok {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q, told %v", tt.args, code, stdout, stderr, tt.code, tt.out, tt.told)
		}
	}
	mine := "\nx\t" + filepath.Join(cli.DefaultPlace(home, "parts"), "x") + "\t"
	if code, stdout, stderr := runEnv(dir, env, "part", "list"); code != 0 || !strings.Contains(stdout, mine) || stderr != "" {
		t.Errorf("floe part list = %d, %q, %q; want 0 with x", code, stdout, stderr)
	}
}

// What part new and template new make is compared with the places of the other kinds, set or not, there or not:
// what would overlap one is refused before it is made, after its name is checked; what would not is made.
func TestNewOwnOverlap(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good})
	home := filepath.Join(dir, "home")
	defaultParts, defaultThemes := cli.DefaultPlace(home, "parts"), cli.DefaultPlace(home, "themes")
	floe := filepath.Dir(defaultParts)
	mkdirs(t, floe, filepath.Join(dir, "parts"), filepath.Join(dir, "themes"))
	in := cli.HomeEnv(home, map[string]string{"FLOE_TEMPLATES_DIR": floe})
	byTemplates := overlap("FLOE_PARTS_DIR", defaultParts, "FLOE_TEMPLATES_DIR", floe)
	for _, tt := range []struct {
		env  map[string]string
		args []string
		code int
		errs string
	}{
		{cli.HomeEnv(home, map[string]string{"FLOE_THEMES_DIR": floe}), []string{"part", "new", "quote"}, 1, overlap("FLOE_PARTS_DIR", defaultParts, "FLOE_THEMES_DIR", floe)},
		{in, []string{"check", "d.yaml"}, 0, ""},
		{in, []string{"part", "new", "quote"}, 1, byTemplates},
		{in, []string{"template", "new", "parts"}, 1, byTemplates},
		{in, []string{"theme", "new", "calm"}, 1, overlap("FLOE_THEMES_DIR", defaultThemes, "FLOE_TEMPLATES_DIR", floe)},
		{in, []string{"template", "new", "tp2"}, 0, ""},
	} {
		if code, _, stderr := runEnv(dir, tt.env, tt.args...); code != tt.code || stderr != tt.errs {
			t.Errorf("%v floe %v = %d, %q; want %d, %q", tt.env, tt.args, code, stderr, tt.code, tt.errs)
		}
	}
	for _, d := range []string{defaultParts, defaultThemes} {
		if _, err := os.Stat(d); err == nil {
			t.Errorf("%s made; want nothing made where it overlaps", d)
		}
	}
	if _, err := os.Stat(filepath.Join(floe, "tp2", "page.html")); err != nil {
		t.Errorf("template new tp2 made no %s", filepath.Join(floe, "tp2", "page.html"))
	}
	t.Run("a name with ..", func(t *testing.T) {
		t.Parallel()
		env := map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "parts"), "FLOE_THEMES_DIR": filepath.Join(dir, "themes")}
		want := filepath.Join(dir, "themes") + ": floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it\n"
		if code, _, stderr := runEnv(dir, env, "part", "new", "../themes"); code != 1 || stderr != want {
			t.Errorf("floe part new ../themes = %d, %q; want 1, %q", code, stderr, want)
		}
	})
}

// A mistake of one's own stops floe check and render, though the page does not use it, and every command that
// reads one's own, told at the file it is in: a part's $ref to no part at the part's schema.json, not at a
// template's.
func TestOwnStops(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{
		"d.yaml":                 good,
		"parts/mine/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": "floe:///parts/gone.json"}`,
		"parts/mine/part.html":   ownPage, "parts/mine/part.css": "",
		"templates/memo/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", ` +
			`"properties": {"m": {"$ref": "floe:///parts/mine.json"}}}`,
		"templates/memo/page.html":    "<!doctype html><html><head><title>{{.Title}}</title></head><body></body></html>",
		"templates/memo/example.yaml": "lang: en\ntemplate: memo\n",
	})
	// A home for theme new, which makes in the default directory of themes.
	env := cli.HomeEnv(filepath.Join(dir, "home"), map[string]string{
		"FLOE_PARTS_DIR": filepath.Join(dir, "parts"), "FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates"),
	})
	want := filepath.Join(dir, "parts/mine/schema.json") + ": floe:///parts/gone.json is not a part; refer to one as floe:///parts/<name>.json\n"
	for _, args := range [][]string{
		{"check", "d.yaml"},
		{"render", "d.yaml"},
		{"part", "check"},
		{"template", "check"},
		{"part", "list"},
		{"part", "show", "note"},
		{"part", "new", "x"},
		{"template", "list"},
		{"template", "show", "report"},
		{"template", "new", "x"},
		{"theme", "check"},
		{"theme", "list"},
		{"theme", "show", "ink"},
	} {
		if code, stdout, stderr := runEnv(dir, env, args...); code != 1 || stdout != "" || stderr != want {
			t.Errorf("floe %v = %d, %q, %q; want 1, %q", args, code, stdout, stderr, want)
		}
	}
	t.Run("theme new", func(t *testing.T) {
		t.Parallel()
		if code, stdout, stderr := runEnv(dir, env, "theme", "new", "x"); code != 1 || stdout != "" || stderr != want {
			t.Errorf("floe theme new x = %d, %q, %q; want 1, %q", code, stdout, stderr, want)
		}
	})
}

// A part.html that does not parse is told at the line text/template's head names, as an editor counts it, without
// the head: a CR alone ends a line too, and the end of the file is a place after its last line break.
func TestOwnSyntax(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ page, want string }{
		{"<div data-floe-part=\"mine\">\n  {{if .}}\n</div>\n", ":4: unexpected EOF"},
		{"a\rb\n{{ bad }}\n", `:3: function "bad" not defined`},
	} {
		dir := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": tt.page, "mine/part.css": ""})
		want := filepath.Join(dir, "mine/part.html") + tt.want + "\n"
		if code, stdout, stderr := runEnv(dir, map[string]string{"FLOE_PARTS_DIR": dir}, "part", "check"); code != 1 || stdout != "" || stderr != want {
			t.Errorf("floe part check of %q = %d, %q, %q; want 1, %q", tt.page, code, stdout, stderr, want)
		}
	}
}

// floe template check draws the example.yaml of each template of one's own, its assets read when page.html
// names them, every mistake told together.
func TestTemplateCheck(t *testing.T) {
	t.Parallel()

	const page = "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body><img src=\"{{asset .Data.logo}}\">%s</body></html>\n"
	files := func(body, example, more string) map[string]string {
		f := map[string]string{
			"templates/memo/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", ` +
				`"additionalProperties": false, "properties": {"items": {"type": "array"}, "logo": {"type": "string"}}}`,
			"templates/memo/page.html":          strings.Replace(page, "%s", body, 1),
			"templates/memo/example.yaml":       example,
			"templates/memo/assets/img/a.svg":   `<svg xmlns="http://www.w3.org/2000/svg"/>`,
			"templates/memo/assets/img/b/c.png": "x",
		}
		if more != "" {
			f["templates/memo/checks.yaml"] = more
		}
		return f
	}
	example := "lang: en\ntemplate: memo\ntitle: t\nitems: [1]\nlogo: img/a.svg\n"
	at := func(dir, file string) string { return filepath.Join(dir, "templates/memo", file) }
	for _, tt := range []struct {
		name  string
		files map[string]string
		want  func(dir string) string
	}{
		{"right", files("<p>after</p>", example, ""), func(string) string { return "" }},
		{"ends in xmp", files("<xmp>x", example, ""), func(string) string { return "" }},
		{"another asset", files("", strings.Replace(example, "img/a.svg", "img/b/c.png", 1), ""), func(string) string { return "" }},
		{"no such asset", files("", strings.Replace(example, "img/a.svg", "img/z.svg", 1), ""), func(dir string) string {
			return at(dir, "page.html") + `:3:19: assets/ has no "img/z.svg": name a file under it, as a path from assets/ like img/logo.svg`
		}},
		{"for through a key the schema forbids", files("", example, "- for: itemz\n  expr: false\n  message: m\n"), func(dir string) string {
			return at(dir, "checks.yaml") + ":1:8: " + i18n.EnglishCatalog().Msg("kit_for_undeclared", "itemz", "itemz")
		}},
		{"an example of another template", files("", strings.Replace(example, "template: memo", "template: report", 1), ""), func(dir string) string {
			return at(dir, "example.yaml") + ":2:11: example.yaml names the template report; name it memo, after the template's directory"
		}},
		{"a page cut short", files("<svg><foreignObject><template>x</template></foreignObject></svg><p>after</p>", example, ""), func(dir string) string {
			return at(dir, "example.yaml") + ":2:11: the page drawn was cut short: HTML's reader leaves out the rest of a page past a <template> inside an <svg> or a <math>; take the <template> out of it (page.html, a part.html or an item of format html)"
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := write(t, tt.files)
			env := map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
			code, stdout, stderr := runEnv(dir, env, "template", "check")
			want := tt.want(dir)
			if want != "" {
				want += "\n"
			}
			if code != map[bool]int{true: 0, false: 1}[want == ""] || stdout != "" || stderr != want {
				t.Errorf("floe template check = %d, %q, %q; want %q", code, stdout, stderr, want)
			}
		})
	}
}

// An asset floe cannot read when page.html names it is told at the asset's file.
func TestAssetUnreadable(t *testing.T) {
	testfs.UnixOnly(t, "chmod keeps a file from being read")
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root reads a file of mode 000")
	}
	dir := write(t, map[string]string{
		"templates/memo/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"templates/memo/page.html":    "<!doctype html><html><head><title>t</title></head><body><img src=\"{{asset \"a.svg\"}}\"></body></html>",
		"templates/memo/example.yaml": "lang: en\ntemplate: memo\n",
		"templates/memo/assets/a.svg": "<svg/>",
	})
	file := filepath.Join(dir, "templates/memo/assets/a.svg")
	if err := os.Chmod(file, 0); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
	if code, _, stderr := runEnv(dir, env, "template", "check"); code != 1 || stderr != file+": the image \"a.svg\" cannot be read (permission denied)\n" {
		t.Errorf("floe template check = %d, %q", code, stderr)
	}
}

// A template's assets/ that is a FIFO is refused as what is no regular file nor directory, not listed for ever.
func TestAssetsFIFO(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{
		"templates/memo/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"templates/memo/page.html":    "<!doctype html><html><head><title>t</title></head><body></body></html>",
		"templates/memo/example.yaml": "lang: en\ntemplate: memo\n",
	})
	assets := filepath.Join(dir, "templates/memo/assets")
	testfs.Mkfifo(t, assets)
	env := map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
	timebox.Within(t, "floe template check", func() {
		if code, _, stderr := runEnv(dir, env, "template", "check"); code != 1 || !strings.HasPrefix(stderr, assets+": this is neither a regular file nor a directory") {
			t.Errorf("floe template check = %d, %q", code, stderr)
		}
	})
}

// In assets/ too, a link to a file is read and a link to a directory refused.
func TestAssetLinks(t *testing.T) {
	t.Parallel()

	shared := write(t, map[string]string{"logo.svg": `<svg xmlns="http://www.w3.org/2000/svg"/>`})
	dir := write(t, map[string]string{
		"templates/memo/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"templates/memo/page.html":    "<!doctype html><html><head><title>t</title></head><body><img src=\"{{asset \"img/logo.svg\"}}\"></body></html>",
		"templates/memo/example.yaml": "lang: en\ntemplate: memo\n",
	})
	img := filepath.Join(dir, "templates/memo/assets/img")
	if err := os.MkdirAll(img, 0o755); err != nil {
		t.Fatal(err)
	}
	link(t, filepath.Join(shared, "logo.svg"), filepath.Join(img, "logo.svg"))
	env := map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
	if code, _, stderr := runEnv(dir, env, "template", "check"); code != 0 || stderr != "" {
		t.Errorf("a file linked in = %d, %q", code, stderr)
	}
	t.Run("a directory linked in", func(t *testing.T) {
		t.Parallel()
		link(t, shared, filepath.Join(img, "more"))
		want := filepath.Join(img, "more") + ": " + dirLink + "\n"
		if code, _, stderr := runEnv(dir, env, "template", "check"); code != 1 || stderr != want {
			t.Errorf("floe template check = %d, %q; want %q", code, stderr, want)
		}
		// The files and directories beside the link are told too.
		if err := os.MkdirAll(filepath.Join(img, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, note := range []string{filepath.Join(img, "note.txt"), filepath.Join(img, "sub", "note.txt")} {
			if err := os.WriteFile(note, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if code, _, stderr := runEnv(dir, env, "template", "check"); code != 1 || !strings.HasPrefix(stderr, want) || !strings.Contains(stderr, note+": ") {
				t.Errorf("floe template check = %d, %q; want %s told after the link", code, stderr, note)
			}
		}
	})
}

// What is neither a regular file nor a directory, a FIFO or a link to a device, is refused, not read for ever;
// and a default directory that is a file is told, not taken for none.
func TestOwnNotFiles(t *testing.T) {
	t.Parallel()

	const notFile = ": this is neither a regular file nor a directory (a FIFO, a device or the like), which floe may never finish reading: put a regular file here\n"
	refused := func(t *testing.T, env map[string]string, errs string) {
		t.Helper()
		if code, _, stderr := runEnv(t.TempDir(), env, "part", "check"); code != 1 || stderr != errs {
			t.Errorf("floe part check = %d, %q; want 1, %q", code, stderr, errs)
		}
	}
	t.Run("a FIFO", func(t *testing.T) {
		t.Parallel()
		fifo := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": ownPage})
		testfs.Mkfifo(t, filepath.Join(fifo, "mine", "part.css"))
		refused(t, map[string]string{"FLOE_PARTS_DIR": fifo}, filepath.Join(fifo, "mine", "part.css")+notFile)
	})
	t.Run("a link to a device", func(t *testing.T) {
		testfs.UnixOnly(t, "a device told by its kind (Windows: TestNotLocalWindows)")
		t.Parallel()
		zero := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": ownPage})
		link(t, "/dev/zero", filepath.Join(zero, "mine", "part.css"))
		refused(t, map[string]string{"FLOE_PARTS_DIR": zero}, filepath.Join(zero, "mine", "part.css")+notFile)
	})
	t.Run("the default a file", func(t *testing.T) {
		testfs.UnixOnly(t, "a default place that is a file is read as none on Windows (W12)")
		t.Parallel()
		home := defaultAFile(t, "parts")
		refused(t, cli.HomeVars(home), cli.DefaultPlace(home, "parts")+": not a directory\n")
		// The XDG base directory's default, which configBase reads before the home.
		config := write(t, map[string]string{"floe/parts": "a file"})
		refused(t, map[string]string{"XDG_CONFIG_HOME": config}, filepath.Join(config, "floe", "parts")+": not a directory\n")
	})
}

// defaultAFile is a home whose default place of what is a file.
func defaultAFile(t *testing.T, what string) string {
	t.Helper()
	home := t.TempDir()
	place := cli.DefaultPlace(home, what)
	if err := os.MkdirAll(filepath.Dir(place), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(place, []byte("a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// A reads.json that cannot be set aside keeps floe serve from starting, told before it says it serves.
func TestServeCannotBegin(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := filepath.Join(dir, "store")
	for _, name := range []string{"reads.json", filepath.Join("reads.json.broken", "x")} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	env := cli.HomeEnv(filepath.Join(dir, "home"), map[string]string{"FLOE_DIR": root, "FLOE_PORT": strconv.Itoa(freePort(t))})
	code, stdout, stderr := runEnv(dir, env, "serve")
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, filepath.Join(root, "reads.json.broken")+": cannot be written (") {
		t.Errorf("floe serve = %d, %q, %q", code, stdout, stderr)
	}
}

// Parts and themes rely on each other, so floe theme check checks what floe part check does, and says the same:
// a theme's mistake, its <part>.css of no part, a part's mistake, and their places overlapping.
func TestThemeCheckIsPartCheck(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{
		"broken/mine/base.css":  "a { color: red",
		"stray/mine/quote.css":  ":scope { color: red; }",
		"good/mine/palette.css": ":scope { --accent: #123456; }",
		"parts/bad/schema.json": ownSchema,
		"ok/mine/schema.json":   ownSchema, "ok/mine/part.html": ownPage, "ok/mine/part.css": "",
	})
	mkdirs(t, filepath.Join(dir, "ok", "th"))
	// Each wants a piece of its own mistake, which a check that reads neither parts nor themes, or compares no
	// places, does not print.
	for _, tt := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{"right", map[string]string{"FLOE_THEMES_DIR": filepath.Join(dir, "good")}, ""},
		{"a theme's mistake", map[string]string{"FLOE_THEMES_DIR": filepath.Join(dir, "broken")}, filepath.Join(dir, "broken/mine/base.css") + ":1:3: "},
		{"a file of no part", map[string]string{"FLOE_THEMES_DIR": filepath.Join(dir, "stray")}, filepath.Join(dir, "stray/mine/quote.css") + ": floe reads no file of this name"},
		{"a part's mistake", map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "parts"), "FLOE_THEMES_DIR": filepath.Join(dir, "good")}, filepath.Join(dir, "parts/bad") + ": a part holds"},
		{
			"overlapping",
			map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "ok"), "FLOE_THEMES_DIR": filepath.Join(dir, "ok", "th")},
			"and FLOE_THEMES_DIR (" + filepath.Join(dir, "ok", "th") + ") are one directory, or one is in the other",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := cli.HomeEnv(filepath.Join(dir, "home"), tt.env)
			code, stdout, stderr := runEnv(dir, env, "part", "check")
			if code != map[bool]int{true: 0, false: 1}[tt.want == ""] || stdout != "" || !strings.Contains(stderr, tt.want) || (stderr == "") != (tt.want == "") {
				t.Errorf("floe part check = %d, %q, %q; want one with %q", code, stdout, stderr, tt.want)
			}
			if themeCode, themeOut, themeErr := runEnv(dir, env, "theme", "check"); themeCode != code || themeOut != stdout || themeErr != stderr {
				t.Errorf("floe theme check = %d, %q, %q; want floe part check's %d, %q, %q", themeCode, themeOut, themeErr, code, stdout, stderr)
			}
		})
	}
}
