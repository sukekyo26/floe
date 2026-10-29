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
		html, err := convert(markdownLine, "markdownLine", strings.Join(lines, "\n"))
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
	html, err := convert(markdown, "markdown", text)
	if err != nil {
		return "", err
	}
	// With raw HTML left out, every <table> in the output is one goldmark drew.
	html = strings.ReplaceAll(html, "<table>", `<div data-floe-part="`+markdownPart+`"><table>`)
	html = strings.ReplaceAll(html, "</table>", "</table></div>")
	return template.HTML(html), nil //nolint:gosec // G203: goldmark leaves raw HTML out
}

// Markdown writes text, an item of format markdown, as HTML, as markdown in page.html does.
func Markdown(text string) (template.HTML, error) {
	return markdownHTML(text)
}

// TextPart is the part of Markdown that stands among other parts (any.json, stack), where a code part can stand
// beside it: the one whose fences are warned of.
// LIMIT: text is warned of wherever a schema names it, with no code part beside too; mark its places if that misleads.
const TextPart = "text"

// Fenced returns the line in text, Markdown, of its first fenced code block as markdown reads it, from 1; 0 for none.
// LIMIT: text's Markdown is read twice, to draw it and to find fences; return both from one read if that weighs.
func Fenced(text string) int {
	src := []byte(text)
	line := 0
	doc := markdown.Parser().Parse(gmtext.NewReader(src))
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // fn fails nothing
		if entering && n.Kind() == ast.KindFencedCodeBlock {
			line = bytes.Count(src[:max(n.Pos(), 0)], []byte("\n")) + 1
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return line
}

// ErrFunc marks floe's own failure in a function of page.html or part.html: text/template wraps what a function
// returns as it wraps a template's mistake (index out of range), and drawing tells this one as floe's.
var ErrFunc = errors.New("a function of floe failed")

// convert writes text as HTML with md, the function of page.html called name.
// LIMIT: Markdown images show only as data URIs (png, gif, jpeg, webp); refuse them if agents keep writing them.
func convert(md goldmark.Markdown, name, text string) (string, error) {
	var b bytes.Buffer
	if err := md.Convert([]byte(text), &b); err != nil {
		return "", fmt.Errorf("%w: %s: %w", ErrFunc, name, err)
	}
	return b.String(), nil
}
