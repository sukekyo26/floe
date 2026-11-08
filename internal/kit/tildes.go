package kit

import (
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// strikethrough is goldmark's Strikethrough of two tildes alone, as cmark-gfm's CMARK_OPT_STRIKETHROUGH_DOUBLE_TILDE
// reads it: the GFM spec takes one too, which strikes through a range (2026-11-01~11-08) or a path (~/work) unasked.
// LIMIT: when goldmark has a setting of two tildes alone, take it for this.
var strikethrough goldmark.Extender = tildesExtension{}

type tildesExtension struct{}

func (tildesExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(tildes{extension.NewStrikethroughParser()}, 500)))
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
