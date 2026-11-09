package kit

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"

	cjkfriendly "github.com/tats-u/goldmark-cjk-friendly/v2"
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
	return template.HTML(html), nil //nolint:gosec // G203: raw HTML is escaped (newMarkdown)
}

// markdown is GFM, as extension.GFM lists its extensions, but its strikethrough of two tildes alone, and its * and ~~
// beside CJK letters read by markdown-cjk-friendly (newMarkdown).
// LIMIT: GFM's extensions are listed by hand; TestMarkdownIsGFM fails when goldmark adds one to extension.GFM.
var markdown = newMarkdown(
	goldmark.WithExtensions(extension.Linkify, extension.Table, strikethrough, extension.TaskList),
)

// newMarkdown is a goldmark of opts that writes raw HTML as the text written (rawAsText), and reads * and _ beside CJK
// letters by markdown-cjk-friendly: every Markdown of floe is made here, so that no tag written in it reaches the page
// and a Japanese sentence reads its emphasis.
// LIMIT: * and ~~ beside a CJK letter are read by markdown-cjk-friendly, so that a * written as a mark beside a CJK
// letter or a CJK mark (設定は*.yamlと*.json, L*、b*, 案*（…）) can pair up and draw italics where CommonMark draws the
// marks: plain on the page; write \* or code there.
// goldmark-cjk-friendly copies goldmark's ScanDelimiter: TestCJKFriendlyKeepsCommonMark watches that it reads as
// goldmark beside letters of no CJK.
func newMarkdown(opts ...goldmark.Option) goldmark.Markdown {
	raw := goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(rawAsText{}, 0)))
	return goldmark.New(append(opts, raw, goldmark.WithExtensions(cjkfriendly.CJKFriendlyEmphasis))...)
}

// rawAsText writes raw HTML as the text it was written as: an HTML block as a paragraph of its lines.
// LIMIT: an HTML block runs as CommonMark reads it: one of a tag alone or of <div> and the like to a blank line, and
// one of <style>, <script>, <pre>, <textarea>, <!--, <?, <!X or <![CDATA[ to its closing mark, or to the end of the
// text when it has none; the Markdown in it shows as written. Write the tag in backticks, or a blank line after it, if
// writers trip on it.
// LIMIT: an HTML block in an item of a tight list is a paragraph there, spaced apart unlike the other items: plain on
// the page.
// LIMIT: an HTML block is one paragraph, so its line breaks and indents fold into spaces on the page: plain there.
type rawAsText struct{}

func (rawAsText) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindRawHTML, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		raw, ok := n.(*ast.RawHTML)
		if !ok || !entering {
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkSkipChildren, writeText(w, raw.Segments.Value(src))
	})
	r.Register(ast.KindHTMLBlock, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		block, ok := n.(*ast.HTMLBlock)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}
		b := block.Lines().Value(src)
		if block.HasClosure() {
			b = append(b, block.ClosureLine.Value(src)...)
		}
		if _, err := w.WriteString("<p>"); err != nil {
			return ast.WalkStop, err //nolint:wrapcheck // convert says which function failed
		}
		if err := writeText(w, bytes.TrimRight(b, "\r\n")); err != nil {
			return ast.WalkStop, err
		}
		_, err := w.WriteString("</p>\n")
		return ast.WalkContinue, err //nolint:wrapcheck // convert says which function failed
	})
}

// writeText writes b as text, escaped: util.EscapeHTML makes a NUL U+FFFD too.
func writeText(w util.BufWriter, b []byte) error {
	_, err := w.Write(util.EscapeHTML(b))
	return err //nolint:wrapcheck // convert says which function failed
}

// markdownLine reads paragraphs alone, without the paragraph transformers that make a line a table or a link
// reference definition, so that a line like "-", "1." or "# x" stays text.
var markdownLine = newMarkdown(
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(util.Prioritized(parser.NewParagraphParser(), 1000)),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
	)),
	goldmark.WithExtensions(strikethrough, extension.Linkify),
)

// markdownLabel reads as markdownLine does and writes a link, one written or one Linkify finds, as its words alone, and
// raw HTML as the text written, as markdownLine does.
var markdownLabel = newMarkdown(
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(util.Prioritized(parser.NewParagraphParser(), 1000)),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
	)),
	goldmark.WithExtensions(strikethrough, extension.Linkify),
	goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(wordsOfLinks{}, 0))),
)

// wordsOfLinks renders a link as what it holds and an autolink as its text.
type wordsOfLinks struct{}

func (wordsOfLinks) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindLink, func(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
		return ast.WalkContinue, nil
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

// markdownHTML writes Markdown text as HTML, a table in the table part's look. Raw HTML in it is drawn as the text
// written: a data item takes HTML only when its schema says format: html.
func markdownHTML(text string) (template.HTML, error) {
	html, err := convert(markdown, "markdown", text)
	if err != nil {
		return "", err
	}
	// With raw HTML escaped, every <table> in the output is one goldmark drew.
	html = strings.ReplaceAll(html, "<table>", `<div data-floe-part="`+markdownPart+`"><table>`)
	html = strings.ReplaceAll(html, "</table>", "</table></div>")
	return template.HTML(strings.TrimSpace(html)), nil //nolint:gosec // G203: raw HTML is escaped (newMarkdown)
}

// MarkdownFlow writes Markdown text as markdownHTML does, in a text's box (data-floe-text): a flow of blocks base.css
// spaces in one place, where page.css and a theme's base.css stop. Every Markdown a part holds is drawn so: what its
// part.html draws by markdown, and an item of format markdown in its value; a template's own Markdown is in a box of
// its own (Markdown).
func MarkdownFlow(text string) (template.HTML, error) {
	return flow(text, "data-floe-text")
}

// flow writes Markdown text as markdownHTML does, in floe's box marked by the attribute mark. Empty Markdown, which
// draws nothing, has no box.
func flow(text, mark string) (template.HTML, error) {
	html, err := markdownHTML(text)
	if html != "" {
		html = template.HTML("<div "+mark+">") + html + "</div>" //nolint:gosec // G203: raw HTML is escaped (newMarkdown)
	}
	return html, err
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
