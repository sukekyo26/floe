package cli_test

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"floe/internal/cli"
	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/testfs"
)

// floe part new, theme new and template new write a part, a theme (an empty palette.css) and a template to fill in,
// which their checks take, of any name floe takes, YAML's null, true and 017 too, and one as long as a file's name may
// be; once there, a name is refused and nothing changes.
func TestNew(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"mine", "a-b_c", "null", "true", "017", strings.Repeat("a", 255)} {
		home := t.TempDir()
		env := cli.HomeVars(home)
		for _, kind := range []string{"part", "theme", "template"} {
			at := filepath.Join(cli.DefaultPlace(home, kind+"s"), name)
			code, stdout, stderr := runEnv(home, env, kind, "new", name)
			if code != 0 || stderr != "" {
				t.Errorf("floe %s new %s = %d, %q, %q", kind, name, code, stdout, stderr)
			}
			wantMade(t, stdout, at, "floe "+kind, func(args ...string) (int, string, string) {
				return runEnv(home, env, args...)
			}, name == "mine")
			if code, stdout, stderr := runEnv(home, env, kind, "check"); code != 0 || stdout != "" || stderr != "" {
				t.Errorf("floe %s check after new %s = %d, %q, %q", kind, name, code, stdout, stderr)
			}
			before := texts(t, at)
			if want := map[string]string{"palette.css": ""}; kind == "theme" && !maps.Equal(before, want) {
				t.Errorf("floe theme new %s made %v; want %v", name, before, want)
			}
			want := at + ": there is something of this name here already: pick another name, or remove it\n"
			if code, stdout, stderr := runEnv(home, env, kind, "new", name); code != 1 || stdout != "" || stderr != want {
				t.Errorf("floe %s new %s again = %d, %q, %q; want 1, %q", kind, name, code, stdout, stderr, want)
			}
			if after := texts(t, at); !maps.Equal(after, before) {
				t.Errorf("floe %s new %s again changed %v to %v", kind, name, before, after)
			}
		}
		if leftover, err := filepath.Glob(filepath.Join(filepath.Dir(cli.DefaultPlace(home, "parts")), "*", ".*")); err != nil || leftover != nil {
			t.Errorf("new left %v, %v", leftover, err)
		}
	}
}

// nextLine is the line after made <path> of a new of one's own: the guide to read, and the command that prints the file
// of floe's own to start from.
var nextLine = regexp.MustCompile(`^next: read (.+) guide; (.+) prints floe's own to start from$`)

// wantMade checks stdout of a new of one's own made at, of the commands under head (floe theme): made <path>, then the
// guide and the file to start from, both of head. When ran, it runs both, which must print something, so that a
// model renamed or gone is not named on.
func wantMade(t *testing.T, stdout, at, head string, run func(args ...string) (int, string, string), ran bool) {
	t.Helper()

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 || lines[0] != "made "+at {
		t.Errorf("%s new printed %q, want made %s and the next line", head, stdout, at)
		return
	}
	m := nextLine.FindStringSubmatch(lines[1])
	if m == nil || m[1] != head || !strings.HasPrefix(m[2], head+" show ") {
		t.Errorf("%s new's next line = %q, want its guide and a show of %s", head, lines[1], head)
		return
	}
	if !ran {
		return
	}
	for _, c := range []string{m[1] + " guide", m[2]} {
		if code, out, stderr := run(strings.Fields(c)[1:]...); code != 0 || out == "" || stderr != "" {
			t.Errorf("%s, named by %s new, = %d, %d bytes, %q", c, head, code, len(out), stderr)
		}
	}
}

// new reads what its kind relies on before it makes one: a part and a theme the parts and themes, so that a broken
// template does not stop them; a template all three. A broken part stops all three.
func TestNewReads(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"templates/memo/schema.json": "{}", "parts/bad/schema.json": ownSchema})
	brokenTemplate := cli.HomeEnv(filepath.Join(dir, "home"), map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates")})
	brokenPart := cli.HomeEnv(filepath.Join(dir, "home2"), map[string]string{"FLOE_PARTS_DIR": filepath.Join(dir, "parts")})
	for _, tt := range []struct {
		name string
		env  map[string]string
		args []string
		code int
		errs string
	}{
		{"part, a template broken", brokenTemplate, []string{"part", "new", "x"}, 0, ""},
		{"theme, a template broken", brokenTemplate, []string{"theme", "new", "x"}, 0, ""},
		{"template, a template broken", brokenTemplate, []string{"template", "new", "x"}, 1, filepath.Join(dir, "templates/memo")},
		{"part, a part broken", brokenPart, []string{"part", "new", "x"}, 1, filepath.Join(dir, "parts/bad")},
		{"theme, a part broken", brokenPart, []string{"theme", "new", "x"}, 1, filepath.Join(dir, "parts/bad")},
		{"template, a part broken", brokenPart, []string{"template", "new", "x"}, 1, filepath.Join(dir, "parts/bad")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _, stderr := runEnv(dir, tt.env, tt.args...)
			if code != tt.code || !strings.HasPrefix(stderr, tt.errs) || (stderr == "") != (tt.errs == "") {
				t.Errorf("%v floe %v = %d, %q; want %d, beginning %q", tt.env, tt.args, code, stderr, tt.code, tt.errs)
			}
		})
	}
}

// texts returns the files in dir by name, with their texts.
func texts(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range entries {
		text, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		got[e.Name()] = string(text)
	}
	return got
}

// floe part new refuses a name as the kit of one's own would, and writes nothing; and a directory it cannot
// make or write in is told.
func TestNewRefused(t *testing.T) { //nolint:tparallel // what was written is looked at after the commands
	t.Parallel()

	home := t.TempDir()
	parts, themes := cli.DefaultPlace(home, "parts"), cli.DefaultPlace(home, "themes")
	if err := os.MkdirAll(parts, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parts, "shr.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	nowhere, file := filepath.Join(t.TempDir(), "nowhere"), fileIn(t)
	_, notThere := files.ReadDir(nowhere)
	//nolint:paralleltest // as the function
	for _, tt := range []struct {
		env  map[string]string
		args []string
		code int
		errs string
	}{
		{nil, []string{"part", "new", "note"}, 1, filepath.Join(parts, "note") + ": note is the name of floe's parts/note: give yours another name\n"},
		{nil, []string{"part", "new", "Bad"}, 1, filepath.Join(parts, "Bad") + ": floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it\n"},
		{nil, []string{"part", "new", "palette"}, 1, filepath.Join(parts, "palette") + ": palette names a file of every theme (palette.css), which a theme's CSS for this part would be read as: give it another name\n"},
		{nil, []string{"part", "new", "shr"}, 1, filepath.Join(parts, "shr") + ": shr names both " + filepath.Join(parts, "shr") + " and " + filepath.Join(parts, "shr.json") + ": give one of them another name\n"},
		{nil, []string{"theme", "new", "default"}, 1, filepath.Join(themes, "default") + ": default is the name of floe's themes/default: give yours another name\n"},
		{nil, []string{"theme", "new", "Bad"}, 1, filepath.Join(themes, "Bad") + ": floe takes no such name: write lower case letters, digits, - and _, starting with a letter or a digit; rename it\n"},
		{nil, []string{"part", "new"}, 2, "floe: accepts 1 arg(s), received 0: run floe --help\n"},
		{nil, []string{"part", "new", "x", "--from", "note"}, 2, "floe: unknown flag: --from: run floe --help\n"},
		{nil, []string{"template", "new", "a", "b"}, 2, "floe: accepts 1 arg(s), received 2: run floe --help\n"},
		{map[string]string{"FLOE_PARTS_DIR": nowhere}, []string{"part", "new", "x"}, 1, "floe: FLOE_PARTS_DIR is " + nowhere + ", which cannot be used as a directory (" + testfs.Words(t, notThere) + "); make it a directory floe can use, or set FLOE_PARTS_DIR to another directory\n"},
		{map[string]string{}, []string{"part", "new", "x"}, 1, "floe: there is no default directory, as " + cli.PlaceVar("parts") + " is not an absolute path; set FLOE_PARTS_DIR to an absolute path\n"},
		{map[string]string{}, []string{"template", "new", "x"}, 1, "floe: there is no default directory, as " + cli.PlaceVar("templates") + " is not an absolute path; set FLOE_TEMPLATES_DIR to an absolute path\n"},
		{
			cli.HomeVars(file),
			[]string{"part", "new", "x"},
			1,
			cli.DefaultPlace(file, "parts") + ": cannot be written (not a directory): check that its directory exists and can be written to, and that its name is free and not too long\n",
		},
	} {
		env := tt.env
		if env == nil {
			env = cli.HomeVars(home)
		}
		if code, stdout, stderr := runEnv(home, env, tt.args...); code != tt.code || stdout != "" || stderr != tt.errs {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q", tt.args, code, stdout, stderr, tt.code, tt.errs)
		}
	}
	if got := texts(t, parts); len(got) != 1 {
		t.Errorf("refused names wrote in %s: %v", parts, got)
	}
	if _, err := os.Stat(themes); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("refused names made %s: %v", themes, err)
	}
}

// floe part list prints what a section of floe's templates takes, and with --all, as template list and theme list do,
// floe's and one's own, a line each, the description on one line;
// show lists the files it reads on a line, or prints the file named after alone, as it is; and refuses a name there
// is not.
func TestListShow(t *testing.T) {
	t.Parallel()

	const schema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "description": "a\n\tb   c", "type": "object"}`
	const page = "<!doctype html><html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body></body></html>"
	dir := write(t, map[string]string{
		"parts/mine/schema.json":       schema,
		"parts/mine/checks.yaml":       "- expr: true\n  message: m\n",
		"parts/mine/part.yaml":         "looks: [panel]\n",
		"parts/mine/part.html":         ownPage,
		"parts/mine/part.css":          "",
		"parts/mine/words.yaml":        "en: {w: x}\n",
		"templates/memo/page.html":     page,
		"templates/memo/page.css":      "p { color: red; }\n",
		"templates/memo/schema.json":   schema,
		"templates/memo/computed.yaml": "n: 1\n",
		"templates/memo/example.yaml":  "template: memo\nlang: en\n",
		"themes/mine/palette.css":      ":root { --accent: red; }\n",
		"themes/mine/code.css":         ":scope { color: red; }\n",
		"themes/mine/base.css":         "body { margin: 0; }\n",
	})
	if err := os.Mkdir(filepath.Join(dir, "themes", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	parts, templates, themes := filepath.Join(dir, "parts"), filepath.Join(dir, "templates"), filepath.Join(dir, "themes")
	env := map[string]string{"FLOE_PARTS_DIR": parts, "FLOE_TEMPLATES_DIR": templates, "FLOE_THEMES_DIR": themes}
	floeFile := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "kit", name))
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSuffix(string(b), "\n")
	}
	allThemes := "app, aurora, blueprint, cyber, default, editorial, empty, ink, mine, notebook, pebble, retro, soft, swiss, washi"
	for _, tt := range []struct {
		args []string
		code int
		out  func(string) bool
		errs string
	}{
		{[]string{"part", "list"}, 0, func(out string) bool {
			// The whole list is TestSectionParts's; here, that it is the one floe part list prints, one's own part in it.
			return strings.Contains(out, "\nhtml\tfloe\thtml\tHTML that goes in as it is\n") &&
				strings.Contains(out, "\nmine\t") && !strings.Contains(out, "\ncards\t") && !strings.Contains(out, "\ntone\t")
		}, ""},
		{[]string{"part", "list", "--all"}, 0, func(out string) bool {
			return strings.HasPrefix(out, "any\tfloe\tone of asks,bars,") && strings.Contains(out, ",tree\tOne key naming the kind of the part\n") &&
				strings.Contains(out, "\nmine\t"+filepath.Join(parts, "mine")+"\t-\ta b c\n") &&
				strings.Contains(out, "\npanel\tfloe\t-\tA box most parts are drawn in") &&
				!strings.Contains(out, "\nhtml\t")
		}, ""},
		{[]string{"part", "show", "mine"}, 0, func(out string) bool {
			return out == "mine: a b c\nwhere: "+filepath.Join(parts, "mine")+"\nlooks: panel\n"+
				"files: schema.json, checks.yaml, part.yaml, part.html, part.css, words.yaml\n"
		}, ""},
		{[]string{"part", "show", "mine", "part.html"}, 0, func(out string) bool { return out == ownPage }, ""},
		{[]string{"part", "show", "mine", "part.css"}, 0, func(out string) bool { return out == "" }, ""},
		{
			[]string{"part", "show", "mine", "nope.css"},
			1, func(out string) bool { return out == "" },
			`floe: mine has no file "nope.css" (its files: schema.json, checks.yaml, part.yaml, part.html, part.css, words.yaml)` + "\n",
		},
		{[]string{"part", "show", "tone"}, 0, func(out string) bool {
			return out == "tone: The colour of a part: accent, good, warn, bad or muted (quiet), from the theme's colour variables\nwhere: floe\nfiles: tone.json\n"
		}, ""},
		{[]string{"part", "show", "rows"}, 0, func(out string) bool {
			return out == "rows: Rows in a panel, a rule between each and the next; take it by looks or data-floe-look\nwhere: floe\nlooks: panel\n"+
				"files: part.yaml, part.css\n"
		}, ""},
		{[]string{"part", "show", "html"}, 0, func(out string) bool {
			return out == "html: HTML that goes in as it is\nwhere: floe\nfiles: any.json\n"
		}, ""},
		{[]string{"part", "show", "html", "any.json"}, 0, func(out string) bool { return out == floeFile("floe/parts/any.json")+"\n" }, ""},
		{
			[]string{"part", "show", "html", "schema.json"},
			1, func(out string) bool { return out == "" },
			`floe: html has no file "schema.json" (its files: any.json)` + "\n",
		},
		{[]string{"part", "show", "nope"}, 1, func(out string) bool { return out == "" }, `floe: unknown part "nope"; use any, asks, band, bars, cards, cell, chart, checklist, checks, code, columns, decision, details, diagram, diff, entries, facts, fields, html, image, keys, links, marked, mine, note, numbered, options, panel, rows, script, stack, stats, steps, table, tabs, tags, terminal, text, timeline, toc, tone, tree, zoom` + "\n"},
		{[]string{"template", "list"}, 0, func(out string) bool {
			return strings.HasPrefix(out, "adr\tfloe\t-\tA decision record") &&
				strings.Contains(out, "\nmemo\t"+filepath.Join(templates, "memo")+"\tlang,template,theme,title\ta b c\npostmortem\tfloe\t")
		}, ""},
		{[]string{"template", "show", "memo"}, 0, func(out string) bool {
			return out == "memo: a b c\nwhere: "+filepath.Join(templates, "memo")+"\nthemes: "+allThemes+"\n"+
				"files: schema.json, computed.yaml, example.yaml, page.html, page.css\n"
		}, ""},
		{[]string{"template", "show", "memo", "computed.yaml"}, 0, func(out string) bool { return out == "n: 1\n" }, ""},
		{[]string{"template", "show", "nope"}, 1, func(out string) bool { return out == "" }, `floe: unknown template "nope"; use adr, handoff, meeting, memo, postmortem, prd, report, runbook, slides, tech-spec` + "\n"},
		{[]string{"theme", "list"}, 0, func(out string) bool {
			return strings.HasPrefix(out, "app\tfloe\t-\tAn app's screen: cool grey ground, white cards raised by soft shadows, a raspberry accent\n") && strings.Contains(out, "\nmine\t"+filepath.Join(themes, "mine")+"\t-\t\n")
		}, ""},
		{[]string{"theme", "show", "mine"}, 0, func(out string) bool {
			return out == "mine\nwhere: "+filepath.Join(themes, "mine")+"\nfiles: palette.css, base.css, code.css\n"
		}, ""},
		{[]string{"theme", "show", "mine", "code.css"}, 0, func(out string) bool { return out == ":scope { color: red; }\n" }, ""},
		{[]string{"theme", "show", "empty"}, 0, func(out string) bool { return out == "empty\nwhere: "+filepath.Join(themes, "empty")+"\n" }, ""},
		{[]string{"theme", "show", "empty", "x.css"}, 1, func(out string) bool { return out == "" }, `floe: empty has no file "x.css" (its files: )` + "\n"},
		{[]string{"theme", "show", "nope"}, 1, func(out string) bool { return out == "" }, `floe: unknown theme "nope"; use ` + allThemes + "\n"},
	} {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			t.Parallel()
			if code, stdout, stderr := runEnv(dir, env, tt.args...); code != tt.code || !tt.out(stdout) || stderr != tt.errs {
				t.Errorf("floe %v = %d, %q, %q; want %d, %q", tt.args, code, stdout, stderr, tt.code, tt.errs)
			}
		})
	}
}

// Each of floe's parts, looks, shared schemas, templates and themes lists on its files: line the files it reads, and
// each of them named after prints it as it reads it.
func TestShowEachFile(t *testing.T) {
	t.Parallel()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	section, err := k.SectionParts()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	env := cli.HomeVars(dir)
	for _, kind := range []struct {
		name    string
		entries []kit.Entry
	}{{"part", k.Parts()}, {"part", section}, {"template", k.Templates()}, {"theme", k.Themes()}} {
		for _, e := range kind.entries {
			names := make([]string, len(e.Files))
			for i, f := range e.Files {
				names[i] = f.Name
			}
			// show writes no files: line for an entry of no file.
			if code, stdout, stderr := runEnv(dir, env, kind.name, "show", e.Name); code != 0 || stderr != "" ||
				(len(names) > 0 && !strings.HasSuffix(stdout, "\nfiles: "+strings.Join(names, ", ")+"\n")) ||
				(len(names) == 0 && strings.Contains(stdout, "\nfiles:")) {
				t.Errorf("floe %s show %s = %d, %q, %q; want its files: %v", kind.name, e.Name, code, stdout, stderr, names)
			}
			for _, f := range e.Files {
				if code, stdout, stderr := runEnv(dir, env, kind.name, "show", e.Name, f.Name); code != 0 || stdout != string(f.Text) || stderr != "" {
					t.Errorf("floe %s show %s %s = %d, %d bytes, %q; want 0 and the %d bytes it reads",
						kind.name, e.Name, f.Name, code, len(stdout), stderr, len(f.Text))
				}
			}
		}
	}
}

// A part of one's own named html, a key a section of floe's templates takes, is refused as one named as floe's
// parts are: floe part list and show would tell of floe's html and one's own apart.
func TestPartListOwnHTML(t *testing.T) {
	t.Parallel()

	const schema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "description": "mine", "type": "object"}`
	dir := write(t, map[string]string{
		"parts/html/schema.json": schema,
		"parts/html/part.html":   strings.ReplaceAll(ownPage, `"mine"`, `"html"`),
		"parts/html/part.css":    "",
	})
	parts := filepath.Join(dir, "parts")
	want := filepath.Join(parts, "html") + ": html is the name of floe's parts/any.json: give yours another name\n"
	for _, args := range [][]string{{"part", "list"}, {"part", "show", "html"}} {
		if code, stdout, stderr := runEnv(dir, map[string]string{"FLOE_PARTS_DIR": parts}, args...); code != 1 || stdout != "" || stderr != want {
			t.Errorf("floe %v = %d, %q, %q; want 1 and %q", args, code, stdout, stderr, want)
		}
	}
}

// The four lists print each line as name, where, shape and description, tab-separated, by a.list alone: floe's
// entries and one's own, and diagram example's too.
func TestListShape(t *testing.T) {
	t.Parallel()

	const schema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "description": "mine", "type": "object", "properties": {"b": {"type": "string"}, "a": {"type": "string"}}}`
	dir := write(t, map[string]string{
		"parts/mine/schema.json":      schema,
		"parts/mine/part.html":        ownPage,
		"parts/mine/part.css":         "",
		"templates/memo/page.html":    "<!doctype html><html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body></body></html>",
		"templates/memo/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"note": {"type": "string"}}}`,
		"templates/memo/example.yaml": "template: memo\nlang: en\n",
		"themes/mine/palette.css":     ":root { --accent: red; }\n",
	})
	env := map[string]string{
		"FLOE_PARTS_DIR": filepath.Join(dir, "parts"), "FLOE_TEMPLATES_DIR": filepath.Join(dir, "templates"),
		"FLOE_THEMES_DIR": filepath.Join(dir, "themes"),
	}
	shapes := map[string]map[string]string{
		"part list":       {"table": "columns,rows", "chart": "-", "html": "html", "facts": "[] name,value", "text": "markdown"},
		"part list --all": {"cards": "-", "mine": "a,b", "tone": "-"},
		"template list":   {"report": "eyebrow,lang,lead,sections,template,theme,title,toc", "adr": "-", "memo": "lang,note,template,theme,title"},
		"theme list":      {"app": "-", "mine": "-"},
		"diagram example": {"architecture": "-", "flow": "-"},
	}
	for _, args := range []string{"part list", "part list --all", "template list", "theme list", "diagram example"} {
		code, stdout, stderr := runEnv(dir, env, strings.Fields(args)...)
		if code != 0 || stderr != "" {
			t.Fatalf("floe %s = %d, %q", args, code, stderr)
		}
		var names []string
		for line := range strings.Lines(stdout) {
			cols := strings.Split(strings.TrimSuffix(line, "\n"), "\t")
			if len(cols) != 4 {
				t.Errorf("floe %s printed %q: the four lists are printed by a.list alone, as name, where, shape and description", args, line)
				continue
			}
			names = append(names, cols[0])
			if want, ok := shapes[args][cols[0]]; ok && cols[2] != want {
				t.Errorf("floe %s: %s has the shape %q, want %q", args, cols[0], cols[2], want)
			}
		}
		if !slices.IsSorted(names) {
			t.Errorf("floe %s lists %v, not by name", args, names)
		}
		for name := range shapes[args] {
			if !slices.Contains(names, name) {
				t.Errorf("floe %s lists no %s", args, name)
			}
		}
	}
}

// new refuses to make what a link to nothing among the other places leads into, a setting's or a default's own, the
// parts' default there or not, as a setting not there yet is: once made, each command reading both places would stop
// (dir_overlap). template new tells it of its templates setting, reading them first.
func TestNewIntoLinkToNothing(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		setup   func(home, tmp string) (env map[string]string, made string)
		args    []string
		told    []string // in stderr
		noParts bool
	}{
		{
			name: "a templates setting into parts", args: []string{"part", "new", "x"},
			setup: func(home, tmp string) (map[string]string, string) {
				made := filepath.Join(cli.DefaultPlace(home, "parts"), "x")
				link(t, made, filepath.Join(tmp, "tpl"))
				return map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(tmp, "tpl")}, made
			},
			told: []string{"FLOE_PARTS_DIR", "FLOE_TEMPLATES_DIR", "one is in the other"},
		},
		{
			name: "a templates setting into parts not there", args: []string{"part", "new", "x"}, noParts: true,
			setup: func(home, tmp string) (map[string]string, string) {
				made := filepath.Join(cli.DefaultPlace(home, "parts"), "x")
				link(t, made, filepath.Join(tmp, "tpl"))
				return map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(tmp, "tpl")}, made
			},
			told: []string{"FLOE_PARTS_DIR", "FLOE_TEMPLATES_DIR", "one is in the other"},
		},
		{
			name: "a templates setting into themes", args: []string{"theme", "new", "y"},
			setup: func(home, tmp string) (map[string]string, string) {
				made := filepath.Join(cli.DefaultPlace(home, "themes"), "y")
				mkdirs(t, filepath.Dir(made))
				link(t, made, filepath.Join(tmp, "tpl"))
				return map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(tmp, "tpl")}, made
			},
			told: []string{"FLOE_THEMES_DIR", "FLOE_TEMPLATES_DIR", "one is in the other"},
		},
		{
			name: "a templates setting, template new", args: []string{"template", "new", "z"},
			setup: func(home, tmp string) (map[string]string, string) {
				link(t, filepath.Join(cli.DefaultPlace(home, "parts"), "x"), filepath.Join(tmp, "tpl"))
				return map[string]string{"FLOE_TEMPLATES_DIR": filepath.Join(tmp, "tpl")}, filepath.Join(tmp, "tpl", "z")
			},
			told: []string{"FLOE_PARTS_DIR", "FLOE_TEMPLATES_DIR", "one is in the other"},
		},
		{
			name: "the templates default into parts", args: []string{"part", "new", "x"},
			setup: func(home, _ string) (map[string]string, string) {
				made := filepath.Join(cli.DefaultPlace(home, "parts"), "x")
				link(t, made, cli.DefaultPlace(home, "templates"))
				return nil, made
			},
			told: []string{"FLOE_PARTS_DIR", "FLOE_TEMPLATES_DIR", "one is in the other"},
		},
		{
			name: "the themes default into templates", args: []string{"template", "new", "x"},
			setup: func(home, _ string) (map[string]string, string) {
				made := filepath.Join(cli.DefaultPlace(home, "templates"), "x")
				mkdirs(t, filepath.Dir(made))
				link(t, made, cli.DefaultPlace(home, "themes"))
				return nil, made
			},
			told: []string{"FLOE_THEMES_DIR", "FLOE_TEMPLATES_DIR", "one is in the other"},
		},
		{
			name: "the parts default into themes", args: []string{"theme", "new", "x"}, noParts: true,
			setup: func(home, _ string) (map[string]string, string) {
				made := filepath.Join(cli.DefaultPlace(home, "themes"), "x")
				mkdirs(t, filepath.Dir(made), filepath.Dir(cli.DefaultPlace(home, "parts")))
				link(t, made, cli.DefaultPlace(home, "parts"))
				return nil, made
			},
			told: []string{"FLOE_PARTS_DIR", "FLOE_THEMES_DIR", "one is in the other"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			home, tmp := t.TempDir(), t.TempDir()
			if !tt.noParts {
				mkdirs(t, cli.DefaultPlace(home, "parts"))
			}
			env, made := tt.setup(home, tmp)
			code, stdout, stderr := runEnv(home, cli.HomeEnv(home, env), tt.args...)
			ok := code == 1 && stdout == ""
			for _, w := range tt.told {
				ok = ok && strings.Contains(stderr, w)
			}
			if !ok {
				t.Errorf("floe %v = %d, %q, %q; want 1, told %v", tt.args, code, stdout, stderr, tt.told)
			}
			if _, err := os.Stat(made); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("floe %v made %s: %v", tt.args, made, err)
			}
		})
	}
}
