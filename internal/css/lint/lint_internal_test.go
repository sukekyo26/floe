package lint

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tdewolff/parse/v2/css"

	floecss "floe/internal/css"
	"floe/internal/kit"
)

// floeInput returns floe's own files as a rule reads them: its kit and base.css, its docs, every-part as floe draws it,
// and the management page.
func floeInput(t *testing.T) *Input {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	ui, err := os.ReadFile("../../ui/ui.html")
	if err != nil {
		t.Fatal(err)
	}
	return &Input{
		kit: k, base: k.BaseCSS(), defaults: k.DefaultsCSS(), docs: os.DirFS("../../../docs"),
		pages: os.DirFS("../../corpus/testdata/every-part/golden"), ui: floecss.Preprocess(ui),
	}
}

// sub returns the directory name of dir as an fs.FS, an empty one when there is none.
func sub(dir, name string) fs.FS {
	if _, err := os.Stat(filepath.Join(dir, name)); errors.Is(err, fs.ErrNotExist) {
		return fstest.MapFS{}
	}
	return os.DirFS(filepath.Join(dir, name))
}

// dirInput returns an example of testdata as a rule reads it: dir holds a kit of its own (parts, themes, templates, read
// over none of floe's), base.css, defaults.css, docs, pages and ui.html, each when the rule reads it, and allow.txt, the exceptions in
// force: a line each, a quoted key and its reason.
func dirInput(t *testing.T, dir string) *Input {
	t.Helper()

	k, err := kit.Load(nil, kit.Sources{
		Parts:     kit.Dir{FS: sub(dir, "parts"), Name: "parts"},
		Themes:    kit.Dir{FS: sub(dir, "themes"), Name: "themes"},
		Templates: kit.Dir{FS: sub(dir, "templates"), Name: "templates"},
	})
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}
	in := &Input{kit: k, docs: sub(dir, "docs"), pages: sub(dir, "pages"), allow: map[string]string{}}
	if base := readIf(t, filepath.Join(dir, "base.css")); base != nil {
		in.base = floecss.Preprocess(base)
	}
	if defaults := readIf(t, filepath.Join(dir, "defaults.css")); defaults != nil {
		in.defaults = floecss.Preprocess(defaults)
	}
	if ui := readIf(t, filepath.Join(dir, "ui.html")); ui != nil {
		in.ui = floecss.Preprocess(ui)
	}
	for line := range strings.Lines(string(readIf(t, filepath.Join(dir, "allow.txt")))) {
		key, rest, err := quoted(strings.TrimSpace(line))
		if err != nil || strings.TrimSpace(rest) == "" {
			t.Fatalf("%s/allow.txt: %q: write a quoted key and its reason", dir, line)
		}
		in.allow[key] = strings.TrimSpace(rest)
	}
	return in
}

// readIf returns the file's bytes, nil when there is none.
func readIf(t *testing.T, file string) []byte {
	t.Helper()

	src, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return src
}

// quoted cuts the Go string at the start of s, and returns it and what follows.
func quoted(s string) (string, string, error) {
	q, err := strconv.QuotedPrefix(s)
	if err != nil {
		return "", "", err
	}
	v, err := strconv.Unquote(q)
	return v, s[len(q):], err
}

// wants returns what the bad example dir says its rule finds, each a finding's String: /* want "…" */ at the end of a line
// of a .css file says what is found on that line, a quoted text each, and want.txt says what is found elsewhere, a line
// each, "<file>:<line> "…"" or, for a finding of no line, "<file> "…"".
func wants(t *testing.T, dir, rule string) []string {
	t.Helper()

	var out []string
	files := os.DirFS(dir)
	err := fs.WalkDir(files, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(rel) != ".css" {
			return err
		}
		src, err := fs.ReadFile(files, rel)
		if err != nil {
			return err
		}
		for _, tok := range tokensOf(src) {
			body, ok := strings.CutPrefix(string(tok.Data), "/* want ")
			if tok.Type != css.CommentToken || !ok {
				continue
			}
			body = strings.TrimSpace(strings.TrimSuffix(body, "*/"))
			for body != "" {
				text, rest, err := quoted(body)
				if err != nil {
					return fmt.Errorf("%s:%d: %s: write each text quoted", rel, lineOf(src, tok.At), tok.Data)
				}
				out = append(out, Finding{rel, lineOf(src, tok.At), rule, text}.String())
				body = strings.TrimSpace(rest)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "want.txt"))
	if errors.Is(err, fs.ErrNotExist) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		where, rest, _ := strings.Cut(lines.Text(), " ")
		text, after, err := quoted(rest)
		if err != nil || strings.TrimSpace(after) != "" {
			t.Fatalf("%s/want.txt: %q: write where and a quoted text", dir, lines.Text())
		}
		file, line, numbered := strings.Cut(where, ":")
		n := 0
		if numbered {
			if n, err = strconv.Atoi(line); err != nil {
				t.Fatalf("%s/want.txt: %q: write the line as a number", dir, lines.Text())
			}
		}
		out = append(out, Finding{file, n, rule, text}.String())
	}
	return out
}

// written returns the findings written as a linter does, in order.
func written(found []Finding) []string {
	out := make([]string, 0, len(found))
	for _, f := range found {
		out = append(out, f.String())
	}
	slices.Sort(out)
	return out
}

// TestLintRule runs each rule of Rules on floe's own files, where it finds nothing and every exception is used and has a
// reason, on its good example (testdata/<name>/good), where it finds nothing, and on its bad one (testdata/<name>/bad),
// where it finds what the example's wants say, each once, and nothing else. A good or a bad example takes no exception of
// the rule's own: those of its allow.txt alone.
func TestLintRule(t *testing.T) {
	t.Parallel()

	for _, rule := range Rules {
		t.Run(rule.Name, func(t *testing.T) {
			t.Parallel()

			in := floeInput(t)
			in.allow = map[string]string{}
			for _, a := range rule.Allow {
				if a.Reason == "" {
					t.Errorf("the exception %q has no reason: write why", a.Key)
				}
				in.allow[a.Key] = a.Reason
			}
			for _, f := range written(rule.Check(in)) {
				t.Error(f)
			}
			if in.seen == 0 {
				t.Error("saw nothing it checks in floe's files: what it checks is renamed or gone, or the rule reads it no more")
			}
			for _, key := range slices.Sorted(maps.Keys(in.allow)) {
				if !in.used[key] {
					t.Errorf("the exception %q is not used on floe's own files: remove it", key)
				}
			}

			dir := filepath.Join("testdata", rule.Name)
			good := dirInput(t, filepath.Join(dir, "good"))
			for _, f := range written(rule.Check(good)) {
				t.Errorf("good: %s", f)
			}
			if good.seen == 0 {
				t.Errorf("%s/good: the rule saw nothing it checks: write what it checks, kept", dir)
			}

			bad := dirInput(t, filepath.Join(dir, "bad"))
			want := wants(t, filepath.Join(dir, "bad"), rule.Name)
			if len(want) == 0 {
				t.Errorf("%s/bad wants nothing: write CSS that breaks the rule, and what the rule says of it", dir)
			}
			got := written(rule.Check(bad))
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("bad:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
			}
		})
	}
}

// TestRules keeps the lines of the rules files and Rules to one another: each line of css.md names itself, [name], and no
// two lines one name, and ends in its test, `TestLintRule/<name>` or `TestLookRule/<name>`, or in （判断）, a judgement no
// test keeps; each rule of Rules is named by one line of a rules file alone, which ends in TestLintRule/<name>; and a line
// that ends so names that rule. TestLookRule holds its own list, looks, to the lines that name it.
func TestRules(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("../../../.claude/rules/*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no rules files: %v", err)
	}
	ruled := map[string]string{} // a name: the line that names it
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for n, line := range strings.Split(string(src), "\n") {
			if !strings.HasPrefix(line, "- **") {
				continue
			}
			at := fmt.Sprintf("%s:%d", filepath.Base(file), n+1)
			name := nameOf(line)
			if name == "" {
				if filepath.Base(file) == "css.md" {
					t.Errorf("%s: names no rule: write [name] after its title", at)
				}
				continue
			}
			if prev, ok := ruled[name]; ok {
				t.Errorf("%s: names %s, as %s does: give each rule a name of its own", at, name, prev)
			}
			ruled[name] = at
			if filepath.Base(file) == "css.md" && !strings.HasSuffix(line, "（判断）") &&
				!strings.HasSuffix(line, "`TestLintRule/"+name+"`") && !strings.HasSuffix(line, "`TestLookRule/"+name+"`") {
				t.Errorf("%s: ends in neither its test, `TestLintRule/%s` or `TestLookRule/%s`, nor （判断）: end it in one", at, name, name)
			}
			end, lints := lintEnd(line)
			switch {
			case !lints:
			case end != name:
				t.Errorf("%s: ends in TestLintRule/%s, not its own %s: end it in its own", at, end, name)
			case !slices.ContainsFunc(Rules, func(r Rule) bool { return r.Name == name }):
				t.Errorf("%s: ends in TestLintRule/%s, which Rules does not hold: add the rule, or end the line otherwise", at, name)
			}
		}
	}
	for _, r := range Rules {
		at, ok := ruled[r.Name]
		if !ok {
			t.Errorf("Rules holds %s, which no line of .claude/rules names: write its line", r.Name)
			continue
		}
		file, n, _ := strings.Cut(at, ":")
		if end, _ := lintEnd(lineAt(t, "../../../.claude/rules/"+file, n)); end != r.Name {
			t.Errorf("%s: names %s, which Rules holds: end the line in `TestLintRule/%s`", at, r.Name, r.Name)
		}
	}
}

// lintEnd returns the rule a line of a rules file ends in, `TestLintRule/<name>`, and whether it ends so.
func lintEnd(line string) (string, bool) {
	rest, ok := strings.CutSuffix(strings.TrimRight(line, " "), "`")
	if !ok {
		return "", false
	}
	i := strings.LastIndex(rest, "`TestLintRule/")
	if i < 0 || strings.Contains(rest[i+1:], "`") {
		return "", false
	}
	return rest[i+len("`TestLintRule/"):], true
}

// nameOf returns the name a line of a rules file gives itself: [name] right after its bold title, "" for none.
func nameOf(line string) string {
	_, after, ok := strings.Cut(line[len("- **"):], "**")
	if !ok {
		return ""
	}
	after = strings.TrimLeft(after, " ")
	if !strings.HasPrefix(after, "[") {
		return ""
	}
	name, _, ok := strings.Cut(after[1:], "]")
	if !ok {
		return ""
	}
	return name
}

// lineAt returns the line n (from 1, written) of file.
func lineAt(t *testing.T, file, n string) string {
	t.Helper()

	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	i, err := strconv.Atoi(n)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(src), "\n")[i-1]
}
