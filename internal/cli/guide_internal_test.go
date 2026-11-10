package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/docs"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// guides are the guide for agents and its translation; a test of them failing means: fix docs/guide.md and
// docs/guide.ja.md.
var guides = []string{"../../docs/guide.md", "../../docs/guide.ja.md"}

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
	vars := map[string]string{"HOME": tmp, "FLOE_DIR": filepath.Join(tmp, "store"), "FLOE_PORT": "1"}
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

// floe diagram guide prints docs/diagrams.md as it is, the file and the embedded text alike.
func TestDiagramGuide(t *testing.T) {
	t.Parallel()

	file, err := os.ReadFile("../../docs/diagrams.md")
	if err != nil {
		t.Fatal(err)
	}
	code, out, errs := runIn(guideEnv(t, t.TempDir()), "diagram", "guide")
	if code != 0 || errs != "" || out != string(file) || out != string(docs.DiagramGuide) || len(file) < 10000 {
		t.Errorf("floe diagram guide = %d, stderr %q, stdout the doc: %t (%d bytes)", code, errs, out == string(file), len(out))
	}
	if code, _, errs := runIn(guideEnv(t, t.TempDir()), "diagram", "guide", "x"); code == 0 || errs == "" {
		t.Errorf("floe diagram guide x = %d, %q: it takes no argument", code, errs)
	}
}

// floe guide prints guide.md as it is.
func TestGuide(t *testing.T) {
	t.Parallel()

	env := guideEnv(t, t.TempDir())
	if code, out, errs := runIn(env, "guide"); code != 0 || out != readGuide(t, guides[0]) || errs != "" {
		t.Errorf("floe guide = %d, stderr %q, stdout the guide: %t", code, errs, out == string(docs.Guide))
	}
	if code, _, _ := runIn(env, "guide", "x"); code != 2 {
		t.Errorf("floe guide x = %d, want 2", code)
	}
}

// placeholders are the words a command of the guide is run with for each <…> in it.
var placeholders = map[string]string{"<template>": "report", "<part>": "note", "<kind>": "flow", "<command>": "check", "<word>": "alb", "<語>": "alb"}

// readOnly are the commands a test runs as the guide writes them: they read, write nothing and ask no server.
var readOnly = [][]string{{"guide"}, {"template"}, {"part"}, {"diagram"}, {"theme", "default"}}

// Each floe command the guide names is one floe reads, its flags too; those that only read run and end well.
// LIMIT: the names of parts in the guide's prose (entries, steps, …) are not checked: a PR renaming a part changes
// any.json and docs, and greps the guide there.
func TestGuideCommands(t *testing.T) {
	t.Parallel()

	for _, path := range guides {
		guide := readGuide(t, path)
		named := regexp.MustCompile("`floe ([^`]*)`").FindAllStringSubmatch(guide, -1)
		if len(named) < 10 {
			t.Fatalf("%s: %d commands found", path, len(named))
		}
		for _, m := range named {
			args := strings.Fields(m[1])
			for i, word := range args {
				if strings.HasPrefix(word, "<") {
					w, ok := placeholders[word]
					if !ok {
						t.Fatalf("%s: floe %s: no word for %s in placeholders", path, m[1], word)
					}
					args[i] = w
				}
			}
			env := guideEnv(t, t.TempDir())
			if err := readable(env, args); err != nil {
				t.Errorf("%s: floe %s: %v (fix docs/guide.md and docs/guide.ja.md)", path, m[1], err)
				continue
			}
			root, _ := newRoot(env, io.Discard)
			if cmd, _, err := root.Find(args); err != nil || cmd == root && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
				t.Errorf("%s: floe %s names no command (fix docs/guide.md and docs/guide.ja.md)", path, m[1])
				continue
			}
			ro := slices.Contains(args, "--help") || slices.ContainsFunc(readOnly, func(c []string) bool {
				return len(args) >= len(c) && slices.Equal(args[:len(c)], c)
			})
			if !ro {
				continue
			}
			if code, _, errs := runIn(env, args...); code != 0 {
				t.Errorf("%s: floe %s = %d, %s (fix docs/guide.md and docs/guide.ja.md)", path, m[1], code, errs)
			}
		}
	}
}

// Each YAML block of the guide passes floe check and floe diagram inspect: a data file as it is, parts in a section of
// report.
func TestGuideExamples(t *testing.T) {
	t.Parallel()

	for _, path := range guides {
		blocks := fences(readGuide(t, path), "yaml")
		if len(blocks) != 4 {
			t.Fatalf("%s: %d YAML blocks, want 4", path, len(blocks))
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
				t.Errorf("%s: block %d: %d %q %q (fix docs/guide.md and docs/guide.ja.md)", path, i+1, code, out, errs)
			}
			if code, _, errs := runIn(guideEnv(t, dir), "diagram", "inspect", name); code != 0 || errs != "" {
				t.Errorf("%s: block %d: floe diagram inspect = %d, %q (fix docs/guide.md and docs/guide.ja.md)", path, i+1, code, errs)
			}
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
	for _, path := range guides {
		if quoted := fences(readGuide(t, path), "text"); !slices.Equal(quoted, got) {
			t.Errorf("%s quotes %q; floe check prints %q (fix docs/guide.md and docs/guide.ja.md)", path, quoted, got)
		}
	}
}

// The floe words the guide quotes outside its blocks are floe's: the head of a warning and where a mistake is.
func TestGuideQuotes(t *testing.T) {
	t.Parallel()

	placed := usererr.Tell(english, usererr.NewAt("f.yaml", 1, 2, "cli_skill_kept")).Render(named)
	whole := usererr.Tell(english, usererr.NewAt("f.yaml", 0, 0, "cli_skill_kept")).Render(named)
	warned := usererr.Report{Mistakes: nil, Failure: "", Warnings: usererr.Warn(english, []*usererr.Error{
		usererr.NewAt("f.yaml", 1, 2, "cli_skill_kept"),
	})}.Render(named)
	for quote, ok := range map[string]bool{
		"`warning:`":             strings.HasPrefix(warned, "f.yaml:1:2: warning: "),
		"`file:line:column:`":    strings.HasPrefix(placed, "f.yaml:1:2: "),
		"`file:`":                strings.HasPrefix(whole, "f.yaml: "),
		"`page in project floe`": english.Msg("cli_page_project", inProject("floe")) == "page in project floe",
	} {
		if !ok {
			t.Errorf("floe no longer prints %s as the guide says: %q %q %q", quote, placed, whole, warned)
		}
		for _, path := range guides {
			if !strings.Contains(readGuide(t, path), quote) {
				t.Errorf("%s does not say %s", path, quote)
			}
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
		t.Errorf("checks takes result %v; the guide says ok, ng and unchecked (fix docs/guide.md and docs/guide.ja.md)", got)
	}
	for path, says := range map[string]string{guides[0]: "`result: ok`, `ng` or `unchecked`", guides[1]: "`result: ok`・`ng`・`unchecked`"} {
		if !strings.Contains(readGuide(t, path), says) {
			t.Errorf("%s does not say %s", path, says)
		}
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
