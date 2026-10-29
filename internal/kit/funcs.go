package kit

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmtext "github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// funcs are the functions page.html and part.html draw with; word, which each words.yaml gives, is added where
// they are read.
var funcs = template.FuncMap{
	"bytes":    formatBytes,
	"num":      formatNum,
	"markdown": markdownHTML,
	// markdownLine writes a line of Markdown, such as a table cell, as HTML without a paragraph around it.
	"markdownLine": func(text string) (template.HTML, error) {
		// The text is made one paragraph of unbroken lines: a blank line would split it, an indented line
		// would open no block, as markdownLine reads no indented code, and two spaces at the end of a line
		// would break it.
		var lines []string
		for line := range strings.Lines(text) {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
		html, _, err := convert(markdownLine, "markdownLine", strings.Join(lines, "\n"))
		if err != nil {
			return "", err
		}
		html = strings.TrimSuffix(strings.TrimPrefix(html, "<p>"), "</p>\n")
		return template.HTML(html), nil //nolint:gosec // G203: goldmark leaves raw HTML out
	},
}

var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// markdownLine reads paragraphs alone, without the paragraph transformers that make a line a table or a link
// reference definition, so that a line like "-", "1." or "# x" stays text.
var markdownLine = goldmark.New(
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(util.Prioritized(parser.NewParagraphParser(), 1000)),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
	)),
	goldmark.WithExtensions(extension.Strikethrough, extension.Linkify),
)

// markdownPart is the part whose look Markdown's tables take, the one part Markdown names.
const markdownPart = "table"

// markdownHTML writes Markdown text as HTML, a table in the table part's look. Raw HTML in it is left out:
// a data item takes HTML only when its schema says format: html.
func markdownHTML(text string) (template.HTML, error) {
	html, _, err := Markdown(text)
	return html, err
}

// Markdown writes text, an item of format markdown, as HTML, as markdown in page.html does, and reports
// whether it holds a fenced code block.
func Markdown(text string) (template.HTML, bool, error) {
	html, fenced, err := convert(markdown, "markdown", text)
	if err != nil {
		return "", false, err
	}
	// With raw HTML left out, every <table> in the output is one goldmark drew.
	html = strings.ReplaceAll(html, "<table>", `<div data-floe-part="`+markdownPart+`"><table>`)
	html = strings.ReplaceAll(html, "</table>", "</table></div>")
	return template.HTML(html), fenced, nil //nolint:gosec // G203: goldmark leaves raw HTML out
}

// TextPart is the part of Markdown that stands among other parts (any.json, stack), where a code part can stand
// beside it: the one whose fences are warned of.
// LIMIT: a fence in a template's or a part's own Markdown is not warned of, as no code part can stand there; warn
// there too if one comes to take a code part beside it. A schema of one's own that names text where no code part
// stands is warned of all the same; its description tells to name format markdown there.
const TextPart = "text"

// ErrFunc marks floe's own failure in a function of page.html or part.html: text/template wraps what a function
// returns as it wraps a template's mistake (index out of range), and drawing tells this one as floe's.
var ErrFunc = errors.New("a function of floe failed")

// convert writes text as HTML with md, the function of page.html called name, read once, as md.Convert does, and
// reports whether it holds a fenced code block.
// LIMIT: Markdown images show only as data URIs (png, gif, jpeg, webp); refuse them if agents keep writing them.
func convert(md goldmark.Markdown, name, text string) (string, bool, error) {
	src := []byte(text)
	doc := md.Parser().Parse(gmtext.NewReader(src))
	var b bytes.Buffer
	if err := md.Renderer().Render(&b, src, doc); err != nil {
		return "", false, fmt.Errorf("%w: %s: %w", ErrFunc, name, err)
	}
	fenced := false
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // fn fails nothing
		if entering && n.Kind() == ast.KindFencedCodeBlock {
			fenced = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String(), fenced, nil
}
