package kit_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"floe/internal/cli"
	"floe/internal/icons"
	"floe/internal/kit"
)

// diagramGuide is the doc of how to write a diagram, what floe diagram guide prints, whole.
const diagramGuide = "../../docs/agents/diagrams.md"

// guideBlock is a fenced block of code in a doc: its language, and its text.
type guideBlock struct{ lang, text string }

// readGuideDoc reads the Markdown of path into its tree, as floe reads Markdown, and returns its fenced blocks, its
// destinations of links and images, and what no doc for agents may hold: code indented rather than fenced, and raw
// HTML (which can hide a link).
func readGuideDoc(t *testing.T, path string) (blocks []guideBlock, dests, headings, bad []string) {
	t.Helper()

	src, err := os.ReadFile(path) //nolint:gosec // G304: a doc of this repository
	if err != nil {
		t.Fatal(err)
	}
	// Links are read as the page reads Markdown (Linkify among them: a bare URL is a link); its headings' ids by
	// goldmark's own parser, which no heading of the page's depends on.
	doc := kit.PageMarkdown.Parser().Parse(text.NewReader(src))
	headings = headingIDs(src)
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
	return blocks, dests, headings, bad
}

// headingIDs are the ids goldmark gives the headings of src, with their #, which a link to a heading of the doc
// is written as.
func headingIDs(src []byte) []string {
	var ids []string
	doc := goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID())).Parser().Parse(text.NewReader(src))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the walk fails only as its func does
		if h, ok := n.(*ast.Heading); ok && entering {
			if id, found := h.AttributeString("id"); found {
				if raw, isBytes := id.([]byte); isBytes {
					ids = append(ids, "#"+string(raw))
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return ids
}

// The docs floe prints (floe guide, floe diagram|part|template|theme guide) reach an agent, which cannot open
// another doc: no link leaves one but to a heading of its own. A link's destination is read from the Markdown's
// tree, so that a link of any form (a path, ../, ./, a heading of another doc, a reference, a URL) is told. The docs
// are those docs/docs.go embeds, so that a doc printed anew is checked too.
func TestGuideDocLinks(t *testing.T) {
	t.Parallel()

	embedded := embeddedDocs(t)
	if len(embedded) < 5 {
		t.Fatalf("docs.go embeds %d docs, want guide.md, diagrams.md, parts.md, templates.md and themes.md", len(embedded))
	}
	for _, rel := range embedded {
		path := "../../docs/" + rel
		_, dests, headings, bad := readGuideDoc(t, path)
		for _, d := range dests {
			if !strings.HasPrefix(d, "#") {
				t.Errorf("%s links %q: floe prints it, and an agent cannot open another doc (write what it says, or a command that prints it)", path, d)
			} else if !slices.Contains(headings, d) {
				t.Errorf("%s links %q, which no heading of it has (its headings: %v)", path, d, headings)
			}
		}
		for _, b := range bad {
			t.Errorf("%s holds %s: write fenced code, and no HTML", path, b)
		}
	}
}

// readGuideDoc finds a link of every form, so that TestGuideDocLinks finding none in a doc means it holds none.
func TestReadGuideDocLinks(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "d.md")
	src := "# D\n\n## A b\n\n[a](#a-b), [o](other.md#x), ![i](i.png), <https://a.example>, https://b.example and [r][1].\n\n[1]: ../r.md\n"
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	_, dests, headings, _ := readGuideDoc(t, path)
	want := []string{"#a-b", "other.md#x", "i.png", "https://a.example", "https://b.example", "../r.md"}
	if !slices.Equal(dests, want) || !slices.Contains(headings, "#a-b") {
		t.Errorf("links %q, headings %q; want links %q and the heading #a-b", dests, headings, want)
	}
}

// The table of icon sets in diagrams.md, the table whose first heading is Set, names the sets floe icons fetch keeps,
// in its order. The table is read from the Markdown's tree, as the page reads it.
func TestGuideIconSets(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile(diagramGuide)
	if err != nil {
		t.Fatal(err)
	}
	var tables [][]string
	err = ast.Walk(kit.PageMarkdown.Parser().Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		table, ok := n.(*extast.Table)
		if !entering || !ok || cellText(table.FirstChild().FirstChild(), src) != "Set" {
			return ast.WalkContinue, nil
		}
		var sets []string
		for row := table.FirstChild().NextSibling(); row != nil; row = row.NextSibling() {
			sets = append(sets, cellText(row.FirstChild(), src))
		}
		tables = append(tables, sets)
		return ast.WalkSkipChildren, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || !slices.Equal(tables[0], icons.Sets()) {
		t.Errorf("%s: its tables of icon sets name %q; want one naming %q, the sets floe icons fetch keeps", diagramGuide, tables, icons.Sets())
	}
}

// cellText is the text n holds, its code spans' among it.
func cellText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the walk returns no error
		if t, ok := n.(*ast.Text); ok && entering {
			b.Write(t.Segment.Value(src))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// embeddedDocs returns the docs docs/docs.go embeds, as //go:embed gives them (agents/guide.md), so that a doc
// printed anew is read too.
func embeddedDocs(t *testing.T) []string {
	t.Helper()

	src, err := os.ReadFile("../../docs/docs.go")
	if err != nil {
		t.Fatal(err)
	}
	embedded := regexp.MustCompile(`(?m)^//go:embed (\S+)$`).FindAllStringSubmatch(string(src), -1)
	paths := make([]string, 0, len(embedded))
	for _, m := range embedded {
		paths = append(paths, m[1])
	}
	return paths
}

// The docs floe prints are in docs/agents/ alone, and in English alone: the docs docs.go embeds are those of
// docs/agents/, and it holds no md that is not embedded (TestGuideDocLinks reads only what is) and no Japanese
// version, which the docs for people have; no embedded doc has a copy or a Japanese version in docs/ either.
func TestAgentDocs(t *testing.T) {
	t.Parallel()

	var want []string
	for _, rel := range embeddedDocs(t) {
		name, ok := strings.CutPrefix(rel, "agents/")
		if !ok || strings.Contains(name, "/") || strings.HasSuffix(name, ".ja.md") {
			t.Errorf("docs.go embeds %s: the docs floe prints are the English ones of docs/agents/", rel)
			continue
		}
		want = append(want, name)
		for _, beside := range []string{name, strings.TrimSuffix(name, ".md") + ".ja.md"} {
			if _, err := os.Stat("../../docs/" + beside); err == nil {
				t.Errorf("docs/%s exists beside docs/agents/%s, which floe prints: keep it in docs/agents/ alone, in English alone", beside, name)
			}
		}
	}
	files, err := filepath.Glob("../../docs/agents/*")
	if err != nil {
		t.Fatal(err)
	}
	have := make([]string, 0, len(files))
	for _, f := range files {
		have = append(have, filepath.Base(f))
	}
	slices.Sort(want)
	slices.Sort(have)
	if !slices.Equal(have, want) {
		t.Errorf("docs/agents/ holds %v, docs.go embeds %v: embed every md there in docs.go, and write no Japanese version", have, want)
	}
}

// guideEnv runs floe with a store and places of one's own that are empty, so that no command reads the developer's.
// LIMIT: a copy of guideEnv of internal/cli's tests. A helper both use would live where internal/cli's own tests
// cannot import it (a package that imports cli, a cycle), and in a file that is not a test, for a test alone.
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
		"res/amazon-vpc-nat-gateway":                           "Networking-Content-Delivery",
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
	body := `{"version":"01012030","sources":["https://example.invalid/icons.zip"],"icons":{` + strings.Join(index, ",") + `}}`
	if err := os.WriteFile(filepath.Join(dir, "aws", "index.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Every block of code in the docs of how to write a diagram is a complete YAML example floe diagram inspect passes:
// a diagram of a page (a `- diagram:` item), or a spec (a `title:` at its top). A block of another kind or of another
// language fails, so that no block is left unchecked by a way of writing it (```yml, a fence with no language,
// ~~~yaml, four backticks all reach the tree as one fenced block); an agent copies each block, so none is a piece.
func TestDiagramGuideExamples(t *testing.T) {
	t.Parallel()

	blocks, _, _, bad := readGuideDoc(t, diagramGuide)
	for _, b := range bad {
		t.Errorf("%s holds %s: write fenced code, and no HTML", diagramGuide, b)
	}
	examples := 0
	dir := t.TempDir()
	for i, b := range blocks {
		where := fmt.Sprintf("%s: block %d", diagramGuide, i+1)
		if b.lang != "yaml" {
			t.Errorf("%s is of language %q, not yaml", where, b.lang)
			continue
		}
		kind, file := guideBlockFile(b.text)
		examples++
		if kind == "unknown" {
			t.Errorf("%s is neither a `- diagram:` item nor a spec with a `title:`: write a complete example, %.40q", where, b.text)
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
	if examples < 2 {
		t.Errorf("%s: %d examples, want 2 or more", diagramGuide, examples)
	}
}

// guideBlockFile tells what a block of YAML is (page or spec, else unknown) and the file to inspect: a
// page puts the item in a section of report.
func guideBlockFile(body string) (kind, file string) {
	switch {
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

// The variables the doc names for the colours of raw SVG are ones default's palette.css defines, which an agent reads
// with floe theme show default palette.css (those of a tone, and the --*-fill it says to leave, aside).
func TestDiagramGuidePalette(t *testing.T) {
	t.Parallel()

	palette, err := os.ReadFile("floe/themes/default/palette.css")
	if err != nil {
		t.Fatal(err)
	}
	checkRawRow(t, diagramGuide, string(palette))
}

// checkRawRow checks the section of the doc at path that is about the part raw: every variable it writes is one default's
// palette.css defines, bar the theme's fills and a tone's. The table of colours is held to the palette in both directions
// by the lint rule raw-in-docs (internal/css/lint).
func checkRawRow(t *testing.T, path, palette string) {
	t.Helper()

	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(doc), "\n### raw\n")
	if !ok {
		t.Fatalf("%s has no section \"raw\"", path)
	}
	section, _, _ := strings.Cut(after, "\n## ")
	if strings.Contains(section, "`-soft`") {
		t.Errorf("%s: the raw section writes \"their `-soft`\": only some colours have one, name each", path)
	}
	for _, v := range regexp.MustCompile("--[a-z]+(?:-[a-z]+)*").FindAllString(section, -1) {
		defined := strings.Contains(palette, " "+v+":")
		switch {
		case strings.HasPrefix(v, "--tone"):
			// The colours of a tone are made by floe from tone.json, not set in palette.css.
		case strings.HasSuffix(v, "-fill") && v != "--on-fill":
			// A theme's own fill, named to be left: palette.css does not define it.
			if defined {
				t.Errorf("%s: the raw section says %s is not in default's palette.css, which defines it", path, v)
			}
		case !defined:
			t.Errorf("%s: the raw section names %s, which default's palette.css does not define", path, v)
		}
	}
}
