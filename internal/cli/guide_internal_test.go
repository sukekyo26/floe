package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"floe/docs"
	"floe/internal/kit"
	"floe/internal/store"
	"floe/internal/testfs"
	"floe/internal/usererr"
	"floe/internal/yamlpos"
)

// guidePath is the guide for agents; a test of it failing means: fix docs/agents/guide.md.
const guidePath = "../../docs/agents/guide.md"

const taskGuidePath = "../../docs/agents/tasks.md"

// commandDocs are the docs whose commands TestGuideCommands runs, each a doc floe prints.
var commandDocs = []string{guidePath, taskGuidePath, "../../docs/agents/themes.md", "../../docs/agents/parts.md", "../../docs/agents/templates.md", "../../docs/agents/diagrams.md", "../../docs/agents/repositories.md"}

func readGuide(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fences is the text of each fenced block of lang in guide, its indentation taken off.
func fences(guide, lang string) []string {
	blocks := regexp.MustCompile("(?ms)^( *)```"+lang+"\n(.*?)^ *```").FindAllStringSubmatch(guide, -1)
	found := make([]string, 0, len(blocks))
	for _, m := range blocks {
		var lines []string
		for line := range strings.Lines(m[2]) {
			lines = append(lines, strings.TrimPrefix(line, m[1]))
		}
		found = append(found, strings.Join(lines, ""))
	}
	return found
}

// guideEnv runs floe in dir with a store and places of one's own that are empty, so that no command reads the
// developer's.
func guideEnv(t *testing.T, dir string) Env {
	t.Helper()
	tmp := t.TempDir()
	vars := HomeEnv(tmp, map[string]string{"FLOE_DIR": filepath.Join(tmp, "store"), "FLOE_PORT": "1"})
	for _, kind := range []string{"TEMPLATES", "PARTS", "THEMES", "ICONS"} {
		own := filepath.Join(tmp, strings.ToLower(kind))
		if err := os.Mkdir(own, 0o700); err != nil {
			t.Fatal(err)
		}
		vars["FLOE_"+kind+"_DIR"] = own
	}
	return Env{Stdout: nil, Stderr: nil, Dir: dir, Getenv: func(key string) string { return vars[key] }}
}

func runIn(env Env, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	env.Stdout, env.Stderr = &stdout, &stderr
	code := Run(args, env)
	return code, stdout.String(), stderr.String()
}

// floe diagram|part|task|template|theme guide print their docs as they are, the file and the embedded text alike, and take
// no argument.
func TestGuideDocs(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		kind, file string
		embedded   []byte
	}{
		{"diagram", "diagrams.md", docs.DiagramGuide},
		{"part", "parts.md", docs.PartGuide},
		{"repository", "repositories.md", docs.RepositoryGuide},
		{"task", "tasks.md", docs.TaskGuide},
		{"template", "templates.md", docs.TemplateGuide},
		{"theme", "themes.md", docs.ThemeGuide},
	} {
		file, err := os.ReadFile("../../docs/agents/" + tt.file)
		if err != nil {
			t.Fatal(err)
		}
		code, out, errs := runIn(guideEnv(t, t.TempDir()), tt.kind, "guide")
		if code != 0 || errs != "" || out != string(file) || out != string(tt.embedded) || len(file) < 5000 {
			t.Errorf("floe %s guide = %d, stderr %q, stdout %s: %t (%d bytes)", tt.kind, code, errs, tt.file, out == string(file), len(out))
		}
		if code, _, errs := runIn(guideEnv(t, t.TempDir()), tt.kind, "guide", "x"); code == 0 || errs == "" {
			t.Errorf("floe %s guide x = %d, %q: it takes no argument", tt.kind, code, errs)
		}
	}
}

// floe part|template|theme --help points to the kind's guide.
func TestGuidesInHelp(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"part", "template", "theme"} {
		code, out, errs := runIn(guideEnv(t, t.TempDir()), kind, "--help")
		if want := "floe " + kind + " guide prints how to make"; code != 0 || errs != "" || !strings.Contains(out, want) {
			t.Errorf("floe %s --help = %d, %q, %q; want it to hold %q", kind, code, out, errs, want)
		}
	}
}

// floe guide prints guide.md as it is when the task mode is on in settings.json, and without its Tasks section's Ask
// after publishing when it is off, there is no settings.json, or no store can be had (HOME not absolute, no FLOE_DIR); a settings.json that
// cannot be read, or a FLOE_DIR that cannot be used, is refused as floe theme default refuses it.
func TestGuide(t *testing.T) {
	t.Parallel()

	whole := readGuide(t, guidePath)
	off, err := kit.WithoutSection([]byte(whole), "Ask after publishing")
	if err != nil {
		t.Fatal(err)
	}
	env := guideEnv(t, t.TempDir())
	settings := func(text string) {
		t.Helper()
		if err := os.MkdirAll(env.Getenv("FLOE_DIR"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(env.Getenv("FLOE_DIR"), "settings.json"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	guide := func(what string, e Env, want string) {
		t.Helper()
		if code, out, errs := runIn(e, "guide"); code != 0 || out != want || errs != "" {
			t.Errorf("%s: floe guide = %d, stderr %q, stdout the guide: %t, without Tasks: %t", what, code, errs, out == whole, out == string(off))
		}
	}
	guide("no settings.json", env, string(off))
	settings(`{"theme":"app"}`)
	guide("off", env, string(off))
	settings(`{"theme":"app","taskMode":true}`)
	guide("on", env, whole)
	vars := homeVars("relative")
	guide("no store", Env{Stdout: nil, Stderr: nil, Dir: env.Dir, Getenv: func(key string) string { return vars[key] }}, string(off))
	settings(`{"taskMode":1}`)
	if code, _, errs := runIn(env, "guide"); code != 1 || !strings.Contains(errs, "settings.json: ") || !strings.Contains(errs, "ask the user to choose a setting again") {
		t.Errorf("a broken settings.json: floe guide = %d, %q", code, errs)
	}
	t.Run("a FLOE_DIR of a file", func(t *testing.T) {
		testfs.UnixOnly(t, "a path under a file is read as not there on Windows (W12)")
		t.Parallel()
		file := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		vars := HomeEnv(t.TempDir(), map[string]string{"FLOE_DIR": file})
		if code, _, errs := runIn(Env{Stdout: nil, Stderr: nil, Dir: env.Dir, Getenv: func(key string) string { return vars[key] }}, "guide"); code != 1 || errs == "" {
			t.Errorf("floe guide = %d, %q", code, errs)
		}
	})
	if code, _, _ := runIn(env, "guide", "x"); code != 2 {
		t.Errorf("floe guide x = %d, want 2", code)
	}
}

// placeholders are the words a command of the guide is run with for each <…> in it.
var placeholders = map[string]string{"<template>": "report", "<part>": "note", "<kind>": "flow", "<command>": "check", "<word>": "alb", "<slug>": "db-choice", "<ID>": "#130", "<project>": "app", "<name>": "default", "<file>": "palette.css", "<look>": "rows", "<set>": "aws", "<zip>": "icons.zip"}

// readOnly are the commands a test runs as the guide writes them: they read, write nothing and ask no server.
var readOnly = [][]string{{"guide"}, {"task", "guide"}, {"template"}, {"part", "guide"}, {"part", "list"}, {"part", "show"}, {"part", "check"}, {"diagram", "example"}, {"diagram", "guide"}, {"theme", "default"}, {"theme", "show"}, {"theme", "list"}, {"theme", "check"}}

// Each floe command the docs of commandDocs name, in code or as a line of an sh block, is one floe reads, its flags too;
// those that only read run and end well (themes.md names floe's themes' files, which this keeps real).
// LIMIT: a command's name and flags alone are read (readable): a positional argument it takes not, or one it lacks,
// passes. Read them if a doc comes to give a command a wrong one.
// LIMIT: the names of parts in the guide's prose (entries, steps, …) are not checked: a PR renaming a part changes
// any.json and docs, and greps the guide there.
func TestGuideCommands(t *testing.T) {
	t.Parallel()

	shLines := 0
	for _, path := range commandDocs {
		guide := readGuide(t, path)
		named := regexp.MustCompile("`floe ([^`]*)`").FindAllStringSubmatch(guide, -1)
		if len(named) < 10 {
			t.Fatalf("%s: %d commands found", path, len(named))
		}
		check := func(command string, args []string, run bool) {
			for i, word := range args {
				if strings.HasPrefix(word, "<") {
					w, ok := placeholders[word]
					if !ok {
						t.Fatalf("%s: floe %s: no word for %s in placeholders", path, command, word)
					}
					args[i] = w
				}
			}
			env := guideEnv(t, t.TempDir())
			if err := readable(env, args); err != nil {
				t.Errorf("%s: floe %s: %v (fix %s)", path, command, err, path)
				return
			}
			root, _ := newRoot(env, io.Discard)
			if cmd, _, err := root.Find(args); err != nil || cmd == root && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
				t.Errorf("%s: floe %s names no command (fix %s)", path, command, path)
				return
			}
			ro := slices.Contains(args, "--help") || slices.ContainsFunc(readOnly, func(c []string) bool {
				return len(args) >= len(c) && slices.Equal(args[:len(c)], c)
			})
			if !ro || !run {
				return
			}
			if code, _, errs := runIn(env, args...); code != 0 {
				t.Errorf("%s: floe %s = %d, %s (fix %s)", path, command, code, errs, path)
			}
		}
		for _, m := range named {
			check(m[1], strings.Fields(m[1]), true)
		}
		for _, command := range shCommands(guide) {
			shLines++
			check(command.text, shellWords(command.text), !command.settings)
		}
	}
	if shLines < 10 {
		t.Fatalf("the docs' sh blocks hold %d floe commands: read the blocks as the docs write them", shLines)
	}
}

// Each YAML block of the guide passes floe check: a data file as it is, parts in a section of report.
func TestGuideExamples(t *testing.T) {
	t.Parallel()

	blocks := fences(readGuide(t, guidePath), "yaml")
	if len(blocks) != 2 {
		t.Fatalf("%s: %d YAML blocks, want 2", guidePath, len(blocks))
	}
	dir := t.TempDir()
	for i, b := range blocks {
		if !strings.HasPrefix(b, "template:") {
			var parts strings.Builder
			for line := range strings.Lines(b) {
				parts.WriteString("      " + line)
			}
			b = "template: report\nlang: en\ntitle: T\nsections:\n  - heading: H\n    parts:\n" + parts.String()
		}
		name := filepath.Join(dir, "b"+string(rune('0'+i))+".yaml")
		if err := os.WriteFile(name, []byte(b), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, out, errs := runIn(guideEnv(t, dir), "check", name); code != 0 || out != "" || errs != "" {
			t.Errorf("%s: block %d: %d %q %q (fix docs/agents/guide.md)", guidePath, i+1, code, out, errs)
		}
	}
}

// The mistakes the guide quotes are what floe check prints for its fixtures: YAML's, yamlpos's and the schema's.
func TestGuideMistakes(t *testing.T) {
	t.Parallel()

	got := make([]string, 0, 2)
	for _, c := range []struct {
		dir   string
		files []string
	}{{"testdata/guide/yaml", []string{"h.yaml", "c.yaml", "t.yaml"}}, {"testdata/guide/shape", []string{"plan.yaml"}}} {
		dir, err := filepath.Abs(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		code, _, errs := runIn(guideEnv(t, dir), append([]string{"check"}, c.files...)...)
		if code != 1 {
			t.Fatalf("floe check %v = %d", c.files, code)
		}
		got = append(got, errs)
	}
	if quoted := fences(readGuide(t, guidePath), "text"); !slices.Equal(quoted, got) {
		t.Errorf("%s quotes %q; floe check prints %q (fix docs/agents/guide.md)", guidePath, quoted, got)
	}
}

// The floe words the guide and floe task guide quote outside their blocks are floe's: the head of a warning, where a
// mistake is, and the lines publish and task print.
func TestGuideQuotes(t *testing.T) {
	t.Parallel()

	placed := usererr.Tell(english, usererr.NewAt("f.yaml", 1, 2, "cli_skill_kept")).Render(named)
	whole := usererr.Tell(english, usererr.NewAt("f.yaml", 0, 0, "cli_skill_kept")).Render(named)
	warned := usererr.Report{Mistakes: nil, Failure: "", Warnings: usererr.Warn(english, []*usererr.Error{
		usererr.NewAt("f.yaml", 1, 2, "cli_skill_kept"),
	})}.Render(named)
	for quote, ok := range map[string]bool{
		"`warning:`":                                     strings.HasPrefix(warned, "f.yaml:1:2: warning: "),
		"`file:line:column:`":                            strings.HasPrefix(placed, "f.yaml:1:2: "),
		"`file:`":                                        strings.HasPrefix(whole, "f.yaml: "),
		"`page in project app`":                          english.Msg("cli_page_project", store.Project("app")) == "page in project app",
		"`page in project app, off task #9`":             english.Msg("cli_page_off_task", store.Project("app"), "#9") == "page in project app, off task #9",
		"`page in no project`":                           english.Msg("cli_page_project", store.Project(store.ProjectKey(""))) == "page in no project",
		"`no task #20 in project app`":                   strings.HasPrefix(english.Msg("cli_task_no_row", "#20", store.Project("app")), "no task #20 in project app:"),
		"`categories \"Designs\" \"Samples\" \"Ideas\"`": strings.HasSuffix(taskLine("#12", "app", store.Task{State: "todo", Categories: []string{"Designs", "Samples", "Ideas"}}), `, categories "Designs" "Samples" "Ideas"`),
		"`page <slug>: no task`":                         english.Msg("cli_task_page", "<slug>", english.Msg("cli_task_page_off")) == "page <slug>: no task",
		"`page old-notes: no task, in project floe`":     english.Msg("cli_task_page", "old-notes", english.Msg("cli_task_page_off_in", store.Project("floe"))) == "page old-notes: no task, in project floe",
		"`moved from task …`":                            english.Msg("cli_task_page_moved", "…") == "moved from task …",
		"`; moved from task #130, category \"designs\"`": "; "+english.Msg("cli_task_page_moved", "#130")+`, category "designs"` == `; moved from task #130, category "designs"`,
		"`; off task #130`":                              "; "+english.Msg("cli_task_page_left", "#130") == "; off task #130",
		"`slug \"db-choice\" was published from …: choose another slug`": english.Msg("store_slug_placed_alone", "db-choice", "…") == `slug "db-choice" was published from …: choose another slug`,
	} {
		if !ok {
			t.Errorf("floe no longer prints %s as the guide says: %q %q %q", quote, placed, whole, warned)
		}
		if !strings.Contains(readGuide(t, guidePath)+readGuide(t, taskGuidePath), quote) {
			t.Errorf("%s and %s do not say %s", guidePath, taskGuidePath, quote)
		}
	}
}

// The values of checks' result the guide names are those its schema takes.
func TestGuideCheckResults(t *testing.T) {
	t.Parallel()

	code, out, errs := runIn(guideEnv(t, t.TempDir()), "part", "show", "checks", "schema.json")
	if code != 0 {
		t.Fatalf("floe part show checks schema.json = %d, %s", code, errs)
	}
	var schema struct {
		Items struct {
			Properties struct {
				Result struct {
					Enum []string `json:"enum"`
				} `json:"result"`
			} `json:"properties"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &schema); err != nil {
		t.Fatal(err)
	}
	if got := schema.Items.Properties.Result.Enum; !slices.Equal(got, []string{"ok", "ng", "unchecked"}) {
		t.Errorf("checks takes result %v; the guide says ok, ng and unchecked (fix docs/agents/guide.md)", got)
	}
	if says := "`result: ok`, `ng` or `unchecked`"; !strings.Contains(readGuide(t, guidePath), says) {
		t.Errorf("%s does not say %s", guidePath, says)
	}
}

// The guide's "read floe part show <part> schema.json" holds for every part floe part list names but html.
func TestGuidePartShow(t *testing.T) {
	t.Parallel()

	env := guideEnv(t, t.TempDir())
	code, out, errs := runIn(env, "part", "list")
	if code != 0 {
		t.Fatalf("floe part list = %d, %s", code, errs)
	}
	for line := range strings.Lines(out) {
		name, _, _ := strings.Cut(line, "\t")
		if name == "html" {
			continue
		}
		if code, _, errs := runIn(env, "part", "show", name, "schema.json"); code != 0 {
			t.Errorf("floe part show %s schema.json = %d, %s", name, code, errs)
		}
	}
}

// The skill is a SKILL.md agentskills.io takes: named floe, described in 1 to 1024 characters.
func TestSkillFile(t *testing.T) {
	t.Parallel()

	head, _, ok := strings.Cut(strings.TrimPrefix(skill, "---\n"), "\n---\n")
	if !ok {
		t.Fatal("no head")
	}
	_, values, err := yamlpos.ParseMap("SKILL.md", []byte(head))
	if err != nil {
		t.Fatal(err)
	}
	desc, ok := values["description"].(string)
	if !ok || values["name"] != "floe" || len([]rune(desc)) < 1 || len([]rune(desc)) > 1024 {
		t.Errorf("head %v", values)
	}
}

// README gives the skill's line for AGENTS.md as the skill says it.
func TestSkillLineInReadme(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"../../README.md", "../../docs/README.ja.md"} {
		if !strings.Contains(readGuide(t, path), skillLine) {
			t.Errorf("%s does not hold the skill's line", path)
		}
	}
}

// overReader fails once more than n bytes are read from it.
type overReader struct {
	r    *strings.Reader
	n, i int
}

func (o *overReader) Read(p []byte) (int, error) {
	k, err := o.r.Read(p)
	o.i += k
	if o.i > o.n {
		return k, errors.New("read past the limit")
	}
	return k, err //nolint:wrapcheck // a test's reader
}

// sameSkill reads at most one byte past the skill, and tells the skill from one byte off and from the skill
// with more after it.
func TestSameSkill(t *testing.T) {
	t.Parallel()

	off := []byte(skill)
	off[len(off)-2]++
	for text, want := range map[string]bool{
		skill:                              true,
		string(off):                        false,
		skill + strings.Repeat("x", 1<<20): false,
	} {
		got, err := sameSkill(&overReader{r: strings.NewReader(text), n: len(skill) + 1, i: 0})
		if err != nil || got != want {
			t.Errorf("%d bytes: %t, %v; want %t", len(text), got, err, want)
		}
	}
}

// identifier is what a value may be while it stands bare: a word of floe (template: report, tone: warn), a name, a path
// or a date.
var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./-]*$`)

// unquoted is a walk of a YAML document that collects the text values not written as the guide teaches: in '...' on
// one line, or in a | block of two lines or more; or bare, as an identifier. A key is not a value; a value left out
// is no text.
type unquoted struct {
	file  string
	src   []byte
	told  map[yamlpos.Pos]bool
	found []string
}

// report tells what is wrong at n, once for each place.
func (u *unquoted) report(n *yamlpos.Node, what string) {
	at := yamlpos.Of(n)
	if u.told[at] {
		return
	}
	u.told[at] = true
	u.found = append(u.found, fmt.Sprintf("%s:%d:%d: %s", u.file, at.Line, at.Column, what))
}

func (u *unquoted) walk(n *yamlpos.Node) {
	switch n.Kind {
	case yamlpos.Mapping:
		for _, p := range n.Pairs {
			u.walk(p.Value)
		}
	case yamlpos.Sequence:
		for _, item := range n.Items {
			u.walk(item)
		}
	case yamlpos.Scalar:
		u.scalar(n)
	}
}

func (u *unquoted) scalar(n *yamlpos.Node) {
	text, isText := yamlpos.Text(n)
	switch {
	case n.Style == yamlpos.Folded:
		u.report(n, "write a text of several lines as a | block, not >")
	case n.Style == yamlpos.Literal:
		if !strings.Contains(strings.TrimSpace(text), "\n") {
			u.report(n, "write a text of one line in '...', not as a | block")
		}
	case n.Null:
		if n.Value != "" {
			u.report(n, "write '"+n.Value+"' in quotes, or leave the value out: a bare "+n.Value+" is no text")
		}
	case !isText:
	default:
		// Written tells a value on one line; one with an escape in double quotes is told as written over lines.
		// LIMIT: a value with two single quotes passes, written over lines too ('it''s, then "  over lines'), as
		// Written does not place it; tell its lines otherwise if the guide comes to show such a value.
		if _, oneLine := yamlpos.Written(u.src, n); !oneLine && !strings.Contains(text, "'") {
			u.report(n, "write a text on one line in '...', or over lines as a | block")
		}
		switch n.Style {
		case yamlpos.Double:
			u.report(n, "write the text in '...', not in double quotes: a \\ in them is read as an escape, mostly with no mistake")
		case yamlpos.Plain:
			if !identifier.MatchString(n.Value) {
				u.report(n, "write the text "+strconv.Quote(n.Value)+" in '...'")
			}
		case yamlpos.Single, yamlpos.Literal, yamlpos.Folded:
		}
	}
}

// notQuoted returns what is not written as the guide teaches in src, a YAML document, each at name's line and column.
func notQuoted(name string, src []byte) ([]string, error) {
	body, err := yamlpos.Parse(name, src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	u := &unquoted{file: name, src: src, told: map[yamlpos.Pos]bool{}, found: nil}
	u.walk(body)
	return u.found, nil
}

// copied is every YAML that floe shows an agent, which copies it: the blocks of the docs, the files of floe's templates,
// parts and themes (floe template show, part show, theme show), the diagrams' examples (floe diagram example) and the
// example a new template starts from.
func copied(t *testing.T) map[string][]byte {
	t.Helper()

	sources := map[string][]byte{}
	var docs []string
	for _, pattern := range []string{"../../docs/*.md", "../../docs/agents/*.md"} {
		found, err := filepath.Glob(pattern)
		if err != nil || len(found) == 0 {
			t.Fatalf("docs %s: %v, %d files", pattern, err, len(found))
		}
		docs = append(docs, found...)
	}
	for _, path := range docs {
		if strings.HasPrefix(filepath.Base(path), "CHANGELOG") {
			continue
		}
		for i, block := range fences(readGuide(t, path), "yaml") {
			sources[fmt.Sprintf("%s block %d", strings.TrimPrefix(filepath.ToSlash(path), "../../"), i+1)] = []byte(block)
		}
	}
	for _, root := range []string{"../kit/floe", "../diagram/examples"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".yaml" {
				return err
			}
			b, err := os.ReadFile(path) //nolint:gosec // G304: a file of floe's own, found by the walk
			sources[strings.TrimPrefix(filepath.ToSlash(path), "../")] = b
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sources["the example of floe template new"] = kit.TemplateSkeleton("memo")["example.yaml"]
	return sources
}

// Every YAML a reader of the guide would copy writes its text values as the guide says: in '...', as a | block of
// several lines, or bare as an identifier or a number.
// LIMIT: a text of one word (title: Steps) is an identifier and passes; telling it from a name would take the keys of
// every part, which would fall out of step as parts come. The guide teaches the rule in words, so that the words that
// pass are what an identifier is.
func TestGuideSentencesQuoted(t *testing.T) {
	t.Parallel()

	sources := copied(t)
	// A doc that holds a YAML block, and each of floe's kinds of file, is read: that none is left unread is the test's
	// own check.
	for _, name := range []string{"agents/guide.md", "pages.md", "pages.ja.md", "agents/templates.md", "agents/diagrams.md"} {
		if _, ok := sources["docs/"+name+" block 1"]; !ok {
			t.Errorf("docs/%s holds no YAML block: the test reads none of it", name)
		}
	}
	for _, dir := range []string{"kit/floe/templates/", "kit/floe/parts/", "kit/floe/themes/", "diagram/examples/"} {
		found := false
		for name := range sources {
			found = found || strings.HasPrefix(name, dir)
		}
		if !found {
			t.Errorf("no YAML file of %s: the test reads none of it", dir)
		}
	}
	for name, src := range sources {
		found, err := notQuoted(name, src)
		if err != nil {
			t.Error(err)
		}
		for _, f := range found {
			t.Error(f)
		}
	}
}

// The test of the sources stands on what notQuoted lets through and refuses.
func TestNotQuoted(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		src  string
		want int
	}{
		{"a: 'x y'\nb: 'it''s \\* \"q\"'\n", 0},
		{"a: |\n  one\n  two\n", 0},
		{"a: -x\n", 1},
		{"a: report\nb: 12\nc: true\nd: 2030-04-12\ne: a/b_c.d-e\nf: Steps\n", 0},
		{"- { a: 'x', b: ['y z', warn] }\n", 0},
		{"'a key with spaces': v\n", 0},
		{"a:\nb:\n", 0},
		{"a: x y\n", 1},
		{"a: \"x\"\n", 1},
		{"a: \"x\"\nb: PR 1\nc: 日本\n", 3},
		{"a: |\n  one\n", 1},
		{"a: |-\n  one\n", 1},
		{"a: |+\n  one\n\n", 1},
		{"a: >\n  one\n  two\n", 1},
		{"a: >\n  one\n\n  two\n", 1},
		{"a: null\nb: Null\nc: ~\n", 3},
		{"a: [x y, 'z']\n", 1},
		{"a: 'one\n  two'\n", 1},
		{"a: |\n\n  one\n", 1},
		{"? x y\n: v\n", 0},
		{"a: \"one\n  two\"\n", 1},
	} {
		found, err := notQuoted("c.yaml", []byte(c.src))
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if len(found) != c.want {
			t.Errorf("%q: %d found %q, want %d", c.src, len(found), found, c.want)
		}
	}
}
