package kit

import (
	cjkfriendly "github.com/tats-u/goldmark-cjk-friendly/v2"
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// strikethrough is markdown-cjk-friendly's strikethrough (goldmark's, with CJK letters read) of two tildes alone, as
// cmark-gfm's CMARK_OPT_STRIKETHROUGH_DOUBLE_TILDE reads it: the GFM spec takes one too, which strikes through a range (2026-11-01~11-08) or a path (~/work) unasked.
// LIMIT: when goldmark has a setting of two tildes alone, take it for this.
// LIMIT: two tildes beside an escaped one (\~~~a~~) stay text, as in goldmark, which reads the escaped one as a tilde.
var strikethrough goldmark.Extender = tildesExtension{}

type tildesExtension struct{}

func (tildesExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(tildes{cjkfriendly.NewCJKFriendlyStrikethroughParser()}, 500)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500)))
}

// tildes hands goldmark's strikethrough a run of two tildes alone; any other run stays text.
type tildes struct{ parser.InlineParser }

func (t tildes) Parse(parent gast.Node, block text.Reader, pc parser.Context) gast.Node {
	line, _ := block.PeekLine()
	n := 0
	for n < len(line) && line[n] == '~' {
		n++
	}
	if n != 2 {
		return nil
	}
	return t.InlineParser.Parse(parent, block, pc)
}
