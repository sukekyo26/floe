package kit_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/sukekyo26/floe/internal/cli"
)

// diagramGuides are the docs of how to write a diagram: the English one is what floe diagram guide prints, whole,
// and the Japanese one is its translation.
var diagramGuides = []string{"../../docs/diagrams.md", "../../docs/diagrams.ja.md"}

// guideBlock is a fenced block of code in a doc: its language, and its text.
type guideBlock struct{ lang, text string }

// readGuideDoc reads the Markdown of path into its tree, as floe reads Markdown, and returns its fenced blocks, its
// destinations of links and images, and what no doc for agents may hold: code indented rather than fenced, and raw
// HTML (which can hide a link).
func readGuideDoc(t *testing.T, path string) (blocks []guideBlock, dests, bad []string) {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.FencedCodeBlock:
			var body bytes.Buffer
			for i := range n.Lines().Len() {
				seg := n.Lines().At(i)
				body.Write(seg.Value(src))
			}
			blocks = append(blocks, guideBlock{lang: string(n.Language(src)), text: body.String()})
		case *ast.CodeBlock:
			bad = append(bad, "an indented block of code")
		case *ast.Link:
			dests = append(dests, string(n.Destination))
		case *ast.Image:
			dests = append(dests, string(n.Destination))
		case *ast.AutoLink:
			dests = append(dests, string(n.URL(src)))
		case *ast.RawHTML, *ast.HTMLBlock:
			bad = append(bad, "raw HTML")
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return blocks, dests, bad
}

// floe diagram guide prints docs/diagrams.md as it is to an agent, which cannot open another doc: no link leaves it
// but to a heading of its own. A link's destination is read from the Markdown's tree, so that a link of any form (a
// path, ../, ./, a heading of another doc, a reference, a URL) is told.
func TestDiagramGuideLinks(t *testing.T) {
	t.Parallel()

	_, dests, bad := readGuideDoc(t, diagramGuides[0])
	if len(dests) == 0 {
		t.Fatal("no link found: the doc links its headings")
	}
	for _, d := range dests {
		if !strings.HasPrefix(d, "#") {
			t.Errorf("%s links %q: floe diagram guide prints it, and an agent cannot open another doc (write what it says, or a command that prints it)", diagramGuides[0], d)
		}
	}
	for _, b := range bad {
		t.Errorf("%s holds %s: write fenced code, and no HTML", diagramGuides[0], b)
	}
}

// guideEnv runs floe with a store and places of one's own that are empty, so that no command reads the developer's.
func guideEnv(t *testing.T, stdout, stderr *bytes.Buffer) cli.Env {
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
	fakeAWSIcons(t, vars["FLOE_ICONS_DIR"])
	return cli.Env{Stdout: stdout, Stderr: stderr, Dir: tmp, Getenv: func(key string) string { return vars[key] }}
}

// fakeAWSIcons keeps in dir an AWS icon set of the icons the docs name, empty drawings: the set floe icons fetch
// would keep, which no test downloads. A doc naming another icon fails its example, to be added here.
func fakeAWSIcons(t *testing.T, dir string) {
	t.Helper()

	names := map[string]string{
		"res/elastic-load-balancing-application-load-balancer": "Networking-Content-Delivery",
		"amazon-elastic-container-service":                     "Containers",
		"amazon-rds":                                           "Databases",
	}
	index := make([]string, 0, len(names))
	for name, category := range names {
		file := filepath.Join(dir, "aws", filepath.FromSlash(name)+".svg")
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0o600); err != nil {
			t.Fatal(err)
		}
		index = append(index, fmt.Sprintf("%q:{\"category\":%q}", name, category))
	}
	body := `{"version":"01012030","source":"https://example.invalid/icons.zip","icons":{` + strings.Join(index, ",") + `}}`
	if err := os.WriteFile(filepath.Join(dir, "aws", "index.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Every block of code in the docs of how to write a diagram is a YAML block that is one of: a complete diagram of a
// page (a `- diagram:` item), a spec (a `title:` at its top), both of which floe diagram inspect passes, or a
// fragment, which says so in its first line (# fragment). A block of another kind, or of another language, fails, so
// that no block is left unchecked by a way of writing it (```yml, a fence with no language, ~~~yaml, four backticks
// all reach the tree as one fenced block).
func TestDiagramGuideExamples(t *testing.T) {
	t.Parallel()

	shapes := make([][]string, 0, len(diagramGuides))
	for _, path := range diagramGuides {
		blocks, _, bad := readGuideDoc(t, path)
		for _, b := range bad {
			t.Errorf("%s holds %s: write fenced code, and no HTML", path, b)
		}
		var shape []string
		dir := t.TempDir()
		for i, b := range blocks {
			where := fmt.Sprintf("%s: block %d", path, i+1)
			if b.lang != "yaml" {
				t.Errorf("%s is of language %q, not yaml", where, b.lang)
				continue
			}
			kind, file := guideBlockFile(b.text)
			shape = append(shape, kind)
			if kind == "fragment" {
				continue
			}
			if kind == "unknown" {
				t.Errorf("%s is neither a `- diagram:` item, a spec with a `title:` nor marked `# fragment`: %.40q", where, b.text)
				continue
			}
			name := filepath.Join(dir, fmt.Sprintf("block%d.yaml", i))
			if err := os.WriteFile(name, []byte(file), 0o600); err != nil {
				t.Fatal(err)
			}
			var out, errs bytes.Buffer
			if code := cli.Run([]string{"diagram", "inspect", name}, guideEnv(t, &out, &errs)); code != 0 || errs.Len() != 0 {
				t.Errorf("%s: floe diagram inspect = %d, %s", where, code, errs.String())
			}
		}
		if complete := len(shape) - strings.Count(strings.Join(shape, " "), "fragment"); complete < 2 {
			t.Errorf("%s: %d complete examples, want 2 or more (the blocks may not all be fragments)", path, complete)
		}
		shapes = append(shapes, shape)
	}
	if strings.Join(shapes[0], " ") != strings.Join(shapes[1], " ") {
		t.Errorf("the blocks of %s are %v, of %s %v: write the same in both", diagramGuides[0], shapes[0], diagramGuides[1], shapes[1])
	}
}

// guideBlockFile tells what a block of YAML is (fragment, page or spec, else unknown) and the file to inspect: a
// page puts the item in a section of report.
func guideBlockFile(body string) (kind, file string) {
	switch {
	case strings.HasPrefix(body, "# fragment\n"):
		return "fragment", ""
	case strings.HasPrefix(body, "- diagram:"):
		var parts strings.Builder
		for line := range strings.Lines(body) {
			parts.WriteString("      " + line)
		}
		return "page", "template: report\nlang: en\ntitle: T\nsections:\n  - heading: H\n    parts:\n" + parts.String()
	case strings.HasPrefix(body, "title:") || strings.Contains(body, "\ntitle:"):
		return "spec", body
	}
	return "unknown", ""
}
