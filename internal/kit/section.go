package kit

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// WithoutSection is the Markdown md without its section headed heading: from the line of the level 2 heading whose
// words are heading, among the document's own blocks, to the line of the next heading of level 1 or 2, or the end.
// A heading inside a quote or a list, or a line like one in fenced code, is no section's. It is for floe's own docs:
// none headed so is floe's failure.
func WithoutSection(md []byte, heading string) ([]byte, error) {
	doc := goldmark.New().Parser().Parse(text.NewReader(md))
	from, to := -1, len(md)
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		// LIMIT: a heading of no words (a bare ##) tells no place, and does not end the section. floe's docs have none;
		// find its line some other way if one comes to.
		if !ok || h.Level > 2 || h.Lines().Len() == 0 {
			continue
		}
		if from >= 0 {
			to = lineStart(md, h)
			break
		}
		if h.Level == 2 && string(h.Lines().Value(md)) == heading {
			from = lineStart(md, h)
		}
	}
	if from < 0 {
		return nil, fmt.Errorf("kit: no section %q to leave out", heading) //nolint:err113 // floe's own doc
	}
	return append(bytes.Clone(md[:from]), md[to:]...), nil
}

// lineStart is where the line of h's first words starts in md: the line of its # (ATX), or its first line (setext).
func lineStart(md []byte, h *ast.Heading) int {
	return bytes.LastIndexByte(md[:h.Lines().At(0).Start], '\n') + 1
}
