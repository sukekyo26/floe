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
	"github.com/yuin/goldmark/renderer"
	gmtext "github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// funcs are the functions page.html draws with, and part.html too but those partFuncs sets over; word, which each
// words.yaml gives, is added where they are read.
var funcs = template.FuncMap{
	"bytes":    formatBytes,
	"contents": contentsScript,
	"num":      formatNum,
	"markdown": Markdown,
	// markdownLine writes a line of Markdown, such as a table cell, as HTML without a paragraph around it.
	"markdownLine": func(text string) (template.HTML, error) { return line(markdownLine, "markdownLine", text) },
	// markdownLabel writes a line of Markdown as markdownLine does, its links as their words: a label that goes in a link.
	"markdownLabel": func(text string) (template.HTML, error) { return line(markdownLabel, "markdownLabel", text) },
}

// line writes text, a line of Markdown, by md, without a paragraph around it. The text is made one paragraph of
// unbroken lines: a blank line would split it, an indented line would open no block, as md reads no indented code,
// and two spaces at the end of a line would break it.
func line(md goldmark.Markdown, name, text string) (template.HTML, error) {
	var lines []string
	for l := range strings.Lines(text) {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	html, err := convert(md, name, strings.Join(lines, "\n"))
	if err != nil {
		return "", err
	}
	html = strings.TrimSuffix(strings.TrimPrefix(html, "<p>"), "</p>\n")
	return template.HTML(html), nil //nolint:gosec // G203: goldmark leaves raw HTML out
}

// markdown is GFM, as extension.GFM lists its extensions, but its strikethrough of two tildes alone.
// LIMIT: when extension.GFM takes another extension, add it here too: read gfm.go when goldmark is upgraded.
var markdown = goldmark.New(
	goldmark.WithExtensions(extension.Linkify, extension.Table, strikethrough, extension.TaskList),
)

// markdownLine reads paragraphs alone, without the paragraph transformers that make a line a table or a link
// reference definition, so that a line like "-", "1." or "# x" stays text.
var markdownLine = goldmark.New(
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(util.Prioritized(parser.NewParagraphParser(), 1000)),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
	)),
	goldmark.WithExtensions(strikethrough, extension.Linkify),
)

// markdownLabel reads as markdownLine does and writes a link, one written or one Linkify finds, as its words alone, and
// raw HTML as nothing, not even the comment markdownLine leaves: a label of a tag alone is empty.
var markdownLabel = goldmark.New(
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(util.Prioritized(parser.NewParagraphParser(), 1000)),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
	)),
	goldmark.WithExtensions(strikethrough, extension.Linkify),
	goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(wordsOfLinks{}, 0))),
)

// wordsOfLinks renders a link as what it holds, an autolink as its text and raw HTML as nothing.
type wordsOfLinks struct{}

func (wordsOfLinks) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindLink, func(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
		return ast.WalkContinue, nil
	})
	r.Register(ast.KindRawHTML, func(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
		return ast.WalkSkipChildren, nil
	})
	r.Register(ast.KindAutoLink, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if link, ok := n.(*ast.AutoLink); ok && entering {
			if _, err := w.Write(util.EscapeHTML(link.Label(src))); err != nil {
				return ast.WalkStop, fmt.Errorf("%w: markdownLabel: %w", ErrFunc, err)
			}
		}
		return ast.WalkContinue, nil
	})
}

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
	// No space at its ends: a text of raw HTML alone leaves its box a comment alone, which :empty takes (base.css).
	return template.HTML(strings.TrimSpace(html)), nil //nolint:gosec // G203: goldmark leaves raw HTML out
}

// MarkdownFlow writes Markdown text as markdownHTML does, in a text's box (data-floe-text): a flow of blocks base.css
// spaces in one place, where page.css and a theme's base.css stop. Every Markdown a part holds is drawn so: what its
// part.html draws by markdown, and an item of format markdown in its value; a template's own Markdown is in a box of
// its own (Markdown).
func MarkdownFlow(text string) (template.HTML, error) {
	return flow(text, "data-floe-text")
}

// flow writes Markdown text as markdownHTML does, in floe's box marked by the attribute mark. Markdown that draws no
// block, empty or raw HTML alone (which markdown leaves out), is drawn as it is, with no box.
func flow(text, mark string) (template.HTML, error) {
	html, err := markdownHTML(text)
	if drawsBlock(text) {
		html = template.HTML("<div "+mark+">") + html + "</div>" //nolint:gosec // G203: markdownHTML's own output
	}
	return html, err
}

// drawsBlock reports whether goldmark reads in text a block other than raw HTML, which markdown leaves out.
func drawsBlock(text string) bool {
	doc := markdown.Parser().Parse(gmtext.NewReader([]byte(text)))
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if n.Kind() != ast.KindHTMLBlock {
			return true
		}
	}
	return false
}

// Markdown writes a template's own Markdown, an item of format markdown or what page.html's markdown draws, as
// markdownHTML does, in the box of a template's Markdown (data-floe-markdown): spaced and its headings sized as a
// text's by base.css, while page.css and a theme's base.css reach in it, as they do not in a text's.
func Markdown(text string) (template.HTML, error) {
	return flow(text, "data-floe-markdown")
}

// TextPart is the part of Markdown that stands among other parts (any.json, stack, an item of entries, checks or
// options), where a code part can stand beside it: the one whose fences are warned of.
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
