package kit

import (
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// strikethrough is goldmark's Strikethrough of two tildes alone, as cmark-gfm's CMARK_OPT_STRIKETHROUGH_DOUBLE_TILDE
// reads it: the GFM spec takes one too, which strikes through a range (2026-11-01~11-08) or a path (~/work) unasked.
// LIMIT: tildes.Parse is goldmark's strikethroughParser.Parse with its length checked for 2, not up to 2. When goldmark
// has a setting of two tildes alone, drop this copy for that setting.
var strikethrough goldmark.Extender = tildesExtension{}

type tildesExtension struct{}

func (tildesExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(tildes{}, 500)))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500)))
}

// tildes reads a strikethrough's tildes, and is the processor that pairs them.
type tildes struct{}

func (tildes) IsDelimiter(b byte) bool { return b == '~' }

func (tildes) CanOpenCloser(opener, closer *parser.Delimiter) bool { return opener.Char == closer.Char }

func (tildes) OnMatch(int) gast.Node { return ast.NewStrikethrough() }

func (tildes) Trigger() []byte { return []byte{'~'} }

func (tildes) CloseBlock(gast.Node, parser.Context) {}

func (t tildes) Parse(_ gast.Node, block text.Reader, pc parser.Context) gast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 1, t)
	if node == nil || node.OriginalLength != 2 || before == '~' {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}
