package cli_test

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/cli"
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
	if err := os.Symlink(to, at); err != nil {
		t.Fatal(err)
	}
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
	for _, tt := range []struct {
		name string
		env  map[string]string
		code int
		errs string
	}{
		{"none set", nil, 0, ""},
		{"HOME a file", map[string]string{"HOME": "/dev/null"}, 0, ""},
		{"default not there", map[string]string{"XDG_CONFIG_HOME": filepath.Join(dir, "none")}, 0, ""},
		{"files linked out", map[string]string{"FLOE_PARTS_DIR": parts}, 0, ""},
		{"the directory a link", map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "plink")}, 0, ""},
		{"relative", map[string]string{"FLOE_PARTS_DIR": "parts"}, 1, "floe: FLOE_PARTS_DIR is parts, a relative path; set it to an absolute path\n"},
		{"set, not there", map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "prats")}, 1, "floe: FLOE_PARTS_DIR is " + filepath.Join(dir, "prats") + ", which cannot be used as a directory (no such file or directory); make it a directory floe can use, or set FLOE_PARTS_DIR to another directory\n"},
		{"set, a file", map[string]string{"FLOE_THEMES_DIR": filepath.Join(dir, "d.yaml")}, 1, "floe: FLOE_THEMES_DIR is " + filepath.Join(dir, "d.yaml") + ", which cannot be used as a directory (not a directory); make it a directory floe can use, or set FLOE_THEMES_DIR to another directory\n"},
		{"a link to a directory", map[string]string{"FLOE_PARTS_DIR": linked}, 1, filepath.Join(linked, "mine") + ": " + dirLink + "\n"},
		{"a link to nothing", map[string]string{"FLOE_PARTS_DIR": dangling}, 1, filepath.Join(dangling, "mine", "part.css") + ": no such file or directory\n"},
	} {
		if tt.name == "the directory a link" {
			link(t, parts, filepath.Join(dir, "plink"))
		}
		for _, args := range [][]string{{"part", "check"}, {"check", "d.yaml"}} {
			if code, stdout, stderr := runEnv(dir, tt.env, args...); code != tt.code || stdout != "" || stderr != tt.errs {
				t.Errorf("%s: floe %v = %d, %q, %q; want %d, %q", tt.name, args, code, stdout, stderr, tt.code, tt.errs)
			}
		}
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
	told := func(key, value, why string) string {
		return "floe: " + key + " is " + value + ", which cannot be used as a directory (" + why + "); make it a " +
			"directory floe can use, or set " + key + " to another directory\n"
	}
	for _, tt := range []struct {
		env    map[string]string
		args   []string
		code   int
		errs   string
		asRoot bool // root reads and writes any directory, so that the case is not one
	}{
		{map[string]string{"FLOE_PARTS_DIR": unread}, []string{"part", "check"}, 1, told("FLOE_PARTS_DIR", unread, "permission denied"), false},
		{map[string]string{"FLOE_PARTS_DIR": unwritten}, []string{"part", "new", "x"}, 1, told("FLOE_PARTS_DIR", unwritten, "permission denied"), false},
		{map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "none")}, []string{"part", "check"}, 0, "", true},
		{map[string]string{"FLOE_ICONS_DIR": file}, []string{"icons", "search", "x"}, 1, told("FLOE_ICONS_DIR", file, "not a directory"), true},
		{map[string]string{"FLOE_DIR": file}, []string{"serve"}, 1, told("FLOE_DIR", file, "not a directory"), true},
		{map[string]string{"FLOE_DIR": unwritten}, []string{"serve"}, 1, told("FLOE_DIR", unwritten, "permission denied"), false},
	} {
		if !tt.asRoot && os.Geteuid() == 0 {
			continue
		}
		env := map[string]string{"HOME": filepath.Join(dir, "home"), "FLOE_PORT": "1"}
		maps.Copy(env, tt.env)
		if code, _, stderr := runEnv(dir, env, tt.args...); code != tt.code || stderr != tt.errs {
			t.Errorf("%v floe %v = %d, %q; want %d, %q", tt.env, tt.args, code, stderr, tt.code, tt.errs)
		}
	}
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
	defaultParts := filepath.Join(home, ".config/floe/parts")
	mkdirs(t, filepath.Join(pp, "th"), filepath.Join(pp, ".th"), filepath.Join(dir, "pp2"), filepath.Join(dir, "pp22"), defaultParts)
	link(t, pp, filepath.Join(dir, "same"))
	link(t, filepath.Join(pp, "th"), filepath.Join(dir, "in"))
	link(t, filepath.Join(pp, "gone"), filepath.Join(dir, "dangling"))
	unusable := func(key, value string) string {
		return "floe: " + key + " is " + value + ", which cannot be used as a directory (no such file or directory); " +
			"make it a directory floe can use, or set " + key + " to another directory\n"
	}
	notDir := func(key, value string) string {
		return "floe: " + key + " is " + value + ", which cannot be used as a directory (not a directory); " +
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
			"a file and under it", map[string]string{parts: dir + "/d.yaml", themes: dir + "/d.yaml/th"}, nil,
			notDir(parts, dir+"/d.yaml") + notDir(themes, dir+"/d.yaml/th"),
		},
		{"not there, then ..", map[string]string{parts: pp, themes: pp + "/none/.."}, nil, unusable(themes, pp+"/none/..")},
		// A link to nothing is not followed (resolve), and is refused once what it leads to is there.
		{"a link to nothing", map[string]string{parts: pp, themes: dir + "/dangling"}, nil, unusable(themes, dir+"/dangling")},
	} {
		env := map[string]string{"HOME": home}
		maps.Copy(env, tt.env)
		args := tt.args
		if args == nil {
			args = []string{"part", "check"}
		}
		code, stdout, stderr := runEnv(dir, env, args...)
		if code != map[bool]int{true: 0, false: 1}[tt.errs == ""] || stdout != "" || stderr != tt.errs {
			t.Errorf("%s: floe %v = %d, %q, %q; want %q", tt.name, args, code, stdout, stderr, tt.errs)
		}
	}
}

// What part new and template new make is compared with the places of the other kinds, set or not, there or not:
// what would overlap one is refused before it is made, after its name is checked; what would not is made.
func TestNewOwnOverlap(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good})
	home := filepath.Join(dir, "home")
	floe := filepath.Join(home, ".config/floe")
	defaultParts := filepath.Join(floe, "parts")
	mkdirs(t, floe, filepath.Join(dir, "parts"), filepath.Join(dir, "themes"))
	in := map[string]string{"HOME": home, "FLOE_TEMPLATES_DIR": floe}
	byTemplates := overlap("FLOE_PARTS_DIR", defaultParts, "FLOE_TEMPLATES_DIR", floe)
	for _, tt := range []struct {
		env  map[string]string
		args []string
		code int
		errs string
	}{
		{map[string]string{"HOME": home, "FLOE_THEMES_DIR": floe}, []string{"part", "new", "quote"}, 1, overlap("FLOE_PARTS_DIR", defaultParts, "FLOE_THEMES_DIR", floe)},
		{in, []string{"check", "d.yaml"}, 0, ""},
		{in, []string{"part", "new", "quote"}, 1, byTemplates},
		{in, []string{"template", "new", "parts"}, 1, byTemplates},
		{in, []string{"template", "new", "tp2"}, 0, ""},
		{
			map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "parts"), "FLOE_THEMES_DIR": filepath.Join(dir, "themes")},
			[]string{"part", "new", "../themes"},
			1,
			filepath.Join(dir, "themes") + ": floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it\n",
		},
	} {
		if code, _, stderr := runEnv(dir, tt.env, tt.args...); code != tt.code || stderr != tt.errs {
			t.Errorf("%v floe %v = %d, %q; want %d, %q", tt.env, tt.args, code, stderr, tt.code, tt.errs)
		}
	}
	if _, err := os.Stat(defaultParts); err == nil {
		t.Errorf("%s made; want nothing made where it overlaps", defaultParts)
	}
	if _, err := os.Stat(filepath.Join(floe, "tp2", "page.html")); err != nil {
		t.Errorf("template new tp2 made no %s", filepath.Join(floe, "tp2", "page.html"))
	}
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
	env := map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "parts"), "FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
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
	} {
		if code, stdout, stderr := runEnv(dir, env, args...); code != 1 || stdout != "" || stderr != want {
			t.Errorf("floe %v = %d, %q, %q; want 1, %q", args, code, stdout, stderr, want)
		}
	}
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
			return at(dir, "checks.yaml") + ":1:8: for itemz: the schema allows no itemz there; write a path of keys its schema declares"
		}},
		{"an example of another template", files("", strings.Replace(example, "template: memo", "template: report", 1), ""), func(dir string) string {
			return at(dir, "example.yaml") + ":2:11: example.yaml names the template report; name it memo, after the template's directory"
		}},
		{"a page cut short", files("<svg><foreignObject><template>x</template></foreignObject></svg><p>after</p>", example, ""), func(dir string) string {
			return at(dir, "example.yaml") + ":2:11: the page drawn was cut short: HTML's reader leaves out the rest of a page past a <template> inside an <svg> or a <math>; take the <template> out of it (page.html, a part.html or an item of format html)"
		}},
	} {
		dir := write(t, tt.files)
		env := map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")}
		code, stdout, stderr := runEnv(dir, env, "template", "check")
		want := tt.want(dir)
		if want != "" {
			want += "\n"
		}
		if code != map[bool]int{true: 0, false: 1}[want == ""] || stdout != "" || stderr != want {
			t.Errorf("%s: floe template check = %d, %q, %q; want %q", tt.name, code, stdout, stderr, want)
		}
	}
}

// An asset floe cannot read when page.html names it is told at the asset's file.
func TestAssetUnreadable(t *testing.T) {
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
	if code, _, stderr := runEnv(dir, env, "template", "check"); code != 1 || stderr != file+": permission denied\n" {
		t.Errorf("floe template check = %d, %q", code, stderr)
	}
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
	link(t, shared, filepath.Join(img, "more"))
	want := filepath.Join(img, "more") + ": " + dirLink + "\n"
	if code, _, stderr := runEnv(dir, env, "template", "check"); code != 1 || stderr != want {
		t.Errorf("a directory linked in = %d, %q; want %q", code, stderr, want)
	}
}

// What is neither a regular file nor a directory, a FIFO or a link to a device, is refused, not read for ever;
// and a default directory that is a file is told, not taken for none.
func TestOwnNotFiles(t *testing.T) {
	t.Parallel()

	fifo := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": ownPage})
	if err := syscall.Mkfifo(filepath.Join(fifo, "mine", "part.css"), 0o600); err != nil {
		t.Fatal(err)
	}
	zero := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": ownPage})
	link(t, "/dev/zero", filepath.Join(zero, "mine", "part.css"))
	const notFile = ": this is neither a regular file nor a directory (a FIFO, a device or the like), which floe may never finish reading: put a regular file here\n"
	config := write(t, map[string]string{"floe/parts": "a file"})
	for _, tt := range []struct {
		name string
		env  map[string]string
		errs string
	}{
		{"a FIFO", map[string]string{"FLOE_PARTS_DIR": fifo}, filepath.Join(fifo, "mine", "part.css") + notFile},
		{"a link to a device", map[string]string{"FLOE_PARTS_DIR": zero}, filepath.Join(zero, "mine", "part.css") + notFile},
		{"the default a file", map[string]string{"XDG_CONFIG_HOME": config}, filepath.Join(config, "floe", "parts") + ": not a directory\n"},
	} {
		if code, _, stderr := runEnv(t.TempDir(), tt.env, "part", "check"); code != 1 || stderr != tt.errs {
			t.Errorf("%s: floe part check = %d, %q; want 1, %q", tt.name, code, stderr, tt.errs)
		}
	}
}
