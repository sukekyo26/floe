package cli_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// floe part new and template new write a part and a template to fill in, which part check and template check take,
// of any name floe takes, YAML's null, true and 017 too, and one as long as a file's name may be; once there, a name is refused and nothing changes.
func TestNew(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"mine", "a-b_c", "null", "true", "017", strings.Repeat("a", 255)} {
		home := t.TempDir()
		env := map[string]string{"HOME": home}
		for _, kind := range []string{"part", "template"} {
			at := filepath.Join(home, ".config/floe", kind+"s", name)
			if code, stdout, stderr := runEnv(home, env, kind, "new", name); code != 0 || stdout != "made "+at+"\n" || stderr != "" {
				t.Errorf("floe %s new %s = %d, %q, %q", kind, name, code, stdout, stderr)
			}
			if code, stdout, stderr := runEnv(home, env, kind, "check"); code != 0 || stdout != "" || stderr != "" {
				t.Errorf("floe %s check after new %s = %d, %q, %q", kind, name, code, stdout, stderr)
			}
			before := texts(t, at)
			want := at + ": there is something of this name here already: pick another name, or remove it\n"
			if code, stdout, stderr := runEnv(home, env, kind, "new", name); code != 1 || stdout != "" || stderr != want {
				t.Errorf("floe %s new %s again = %d, %q, %q; want 1, %q", kind, name, code, stdout, stderr, want)
			}
			if after := texts(t, at); !maps.Equal(after, before) {
				t.Errorf("floe %s new %s again changed %v to %v", kind, name, before, after)
			}
		}
		if leftover, err := filepath.Glob(filepath.Join(home, ".config/floe/*/.*")); err != nil || leftover != nil {
			t.Errorf("new left %v, %v", leftover, err)
		}
	}
}

// floe part new makes the part as the umask says, its directory as its files.
func TestNewUmask(t *testing.T) { //nolint:paralleltest // sets the umask of the process
	for _, tt := range []struct{ umask, dir, file os.FileMode }{{0o022, 0o755, 0o644}, {0o077, 0o700, 0o600}} {
		old := syscall.Umask(int(tt.umask))
		home := t.TempDir()
		code, _, stderr := runEnv(home, map[string]string{"HOME": home}, "part", "new", "mine")
		syscall.Umask(old)
		if code != 0 {
			t.Fatalf("floe part new mine = %d, %q", code, stderr)
		}
		at := filepath.Join(home, ".config/floe/parts/mine")
		for path, mode := range map[string]os.FileMode{at: tt.dir, filepath.Join(at, "part.html"): tt.file} {
			if info, err := os.Stat(path); err != nil || info.Mode().Perm() != mode {
				t.Errorf("umask %o: %s: %v, %v; want mode %o", tt.umask, path, info, err, mode)
			}
		}
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
func TestNewRefused(t *testing.T) {
	t.Parallel()

	home := write(t, map[string]string{".config/floe/parts/shr.json": "{}"})
	parts := filepath.Join(home, ".config/floe/parts")
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
		{nil, []string{"part", "new"}, 2, "floe: accepts 1 arg(s), received 0: run floe --help\n"},
		{nil, []string{"part", "new", "x", "--from", "note"}, 2, "floe: unknown flag: --from: run floe --help\n"},
		{nil, []string{"template", "new", "a", "b"}, 2, "floe: accepts 1 arg(s), received 2: run floe --help\n"},
		{map[string]string{"FLOE_PARTS_DIR": "/nowhere"}, []string{"part", "new", "x"}, 1, "floe: FLOE_PARTS_DIR is /nowhere, which cannot be used as a directory (no such file or directory); make it a directory floe can use, or set FLOE_PARTS_DIR to another directory\n"},
		{map[string]string{}, []string{"part", "new", "x"}, 1, "floe: there is no default directory, as HOME is not an absolute path; set FLOE_PARTS_DIR to an absolute path\n"},
		{map[string]string{}, []string{"template", "new", "x"}, 1, "floe: there is no default directory, as HOME is not an absolute path; set FLOE_TEMPLATES_DIR to an absolute path\n"},
		{
			map[string]string{"HOME": "/dev/null"},
			[]string{"part", "new", "x"},
			1,
			"/dev/null/.config/floe/parts: cannot be written (not a directory): check that its directory exists and can be written to, and that its name is free and not too long\n",
		},
	} {
		env := tt.env
		if env == nil {
			env = map[string]string{"HOME": home}
		}
		if code, stdout, stderr := runEnv(home, env, tt.args...); code != tt.code || stdout != "" || stderr != tt.errs {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q", tt.args, code, stdout, stderr, tt.code, tt.errs)
		}
	}
	if got := texts(t, parts); len(got) != 1 {
		t.Errorf("refused names wrote in %s: %v", parts, got)
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
	allThemes := "app, aurora, blueprint, brutal, default, editorial, empty, ink, mine, notebook, retro, soft, swiss, washi"
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
		{[]string{"part", "show", "nope"}, 1, func(out string) bool { return out == "" }, `floe: unknown part "nope"; use any, asks, band, bars, cards, cell, chart, checklist, checks, code, columns, compare, decision, details, diagram, diff, entries, facts, fields, findings, html, image, keys, links, marked, mine, note, numbered, options, panel, rows, script, stack, stats, steps, table, tabs, tags, terminal, text, timeline, toc, tone, tree, zoom` + "\n"},
		{[]string{"template", "list"}, 0, func(out string) bool {
			return strings.HasPrefix(out, "adr\tfloe\t-\tA decision record") &&
				strings.Contains(out, "\nmemo\t"+filepath.Join(templates, "memo")+"\tlang,template,theme,title\ta b c\npostmortem\tfloe\t")
		}, ""},
		{[]string{"template", "show", "memo"}, 0, func(out string) bool {
			return out == "memo: a b c\nwhere: "+filepath.Join(templates, "memo")+"\nthemes: "+allThemes+"\n"+
				"files: schema.json, computed.yaml, example.yaml, page.html, page.css\n"
		}, ""},
		{[]string{"template", "show", "memo", "computed.yaml"}, 0, func(out string) bool { return out == "n: 1\n" }, ""},
		{[]string{"template", "show", "nope"}, 1, func(out string) bool { return out == "" }, `floe: unknown template "nope"; use adr, design, handoff, memo, postmortem, report, runbook, slides` + "\n"},
		{[]string{"theme", "list"}, 0, func(out string) bool {
			return strings.HasPrefix(out, "app\tfloe\t-\tAn app's screen: cool grey ground, white cards raised by soft shadows, a raspberry accent\n") && strings.Contains(out, "\nmine\t"+filepath.Join(themes, "mine")+"\t-\t\n")
		}, ""},
		{[]string{"theme", "show", "mine"}, 0, func(out string) bool {
			return out == "mine\nwhere: "+filepath.Join(themes, "mine")+"\nfiles: palette.css, base.css, code.css\n"
		}, ""},
		{[]string{"theme", "show", "mine", "code.css"}, 0, func(out string) bool { return out == ":scope { color: red; }\n" }, ""},
		{[]string{"theme", "show", "empty"}, 0, func(out string) bool { return out == "empty\nwhere: "+filepath.Join(themes, "empty")+"\n" }, ""},
		{[]string{"theme", "show", "empty", "x.css"}, 1, func(out string) bool { return out == "" }, `floe: empty has no file "x.css" (its files: )` + "\n"},
		{[]string{"theme", "show", "nope"}, 1, func(out string) bool { return out == "" }, `floe: there is no theme "nope"; use ` + allThemes + "\n"},
	} {
		if code, stdout, stderr := runEnv(dir, env, tt.args...); code != tt.code || !tt.out(stdout) || stderr != tt.errs {
			t.Errorf("floe %v = %d, %q, %q; want %d, %q", tt.args, code, stdout, stderr, tt.code, tt.errs)
		}
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
	env := map[string]string{"HOME": dir}
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
