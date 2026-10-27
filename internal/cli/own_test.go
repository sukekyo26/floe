package cli_test

import (
	"bytes"
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
		{"set, not there", map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "prats")}, 1, filepath.Join(dir, "prats") + ": no such file or directory\n"},
		{"set, a file", map[string]string{"FLOE_THEMES_DIR": filepath.Join(dir, "d.yaml")}, 1, filepath.Join(dir, "d.yaml") + ": not a directory\n"},
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
