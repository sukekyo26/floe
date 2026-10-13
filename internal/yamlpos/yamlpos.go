// Package yamlpos points at where something was written in a YAML file: the line and column of a
// node, and mistakes reported there, so that a diagram spec and a data file word theirs alike.
package yamlpos

import (
	"errors"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Pos is a line and column of a YAML file, both from 1.
type Pos struct {
	Line, Column int
}

// At returns the position, so that every value embedding Pos tells where it was written.
func (p Pos) At() Pos {
	return p
}

// Of returns where n, which must not be nil, was written: a mapping at its first key, so that it
// points at text rather than the indentation before it.
func Of(n ast.Node) Pos {
	if m, ok := n.(*ast.MappingNode); ok && len(m.Values) > 0 {
		n = m.Values[0].Key
	}
	p := n.GetToken().Position
	return Pos{Line: p.Line, Column: p.Column}
}

// ErrEmpty is returned by Parse for a file without a document, for the caller to word.
var ErrEmpty = errors.New("no YAML document")

// Parse reads src as one YAML document and returns its body, or ErrEmpty when src holds none. A
// syntax error is reported at its token, and a second document at its start, worded by the key
// documents.
func Parse(file string, src []byte, documents string) (ast.Node, error) {
	doc, err := parser.ParseBytes(src, 0)
	if err != nil {
		var se *yaml.SyntaxError
		if errors.As(err, &se) && se.Token != nil {
			at := Pos{Line: se.Token.Position.Line, Column: se.Token.Position.Column}
			return nil, New(file, at, "yaml_syntax", se.Message)
		}
		return nil, New(file, Pos{Line: 1, Column: 1}, "yaml_syntax", err.Error())
	}
	var body ast.Node
	var head *Pos // a directive (%YAML) heads the document after it
	for _, d := range doc.Docs {
		switch b := d.Body.(type) {
		case *ast.DirectiveNode:
			// At the --- before it, when one ended the document before.
			at := startOf(d, Of(b))
			head = &at
			continue
		case nil:
			// An empty document: --- at the end, or one goccy/go-yaml left empty after a second ---,
			// dropping what followed.
			if d.Start != nil {
				if at, ok := dropped(src, d.Start.Position); ok {
					return nil, New(file, at, documents)
				}
			}
			continue
		}
		if body != nil {
			at := startOf(d, Of(d.Body))
			if head != nil {
				at = *head
			}
			return nil, New(file, at, documents)
		}
		body, head = d.Body, nil
	}
	if body == nil {
		return nil, ErrEmpty
	}
	return body, nil
}

// startOf returns where d starts: at its ---, or at at without one.
func startOf(d *ast.DocumentNode, at Pos) Pos {
	if d.Start != nil {
		return Pos{Line: d.Start.Position.Line, Column: d.Start.Position.Column}
	}
	return at
}

// dropped finds text after the --- at start, which goccy/go-yaml reads as an empty document and
// loses: a second --- with text after it. ok is false when only comments and ... follow.
func dropped(src []byte, start *token.Position) (Pos, bool) {
	for _, t := range lexer.Tokenize(string(src)) {
		p := t.Position
		if p.Line < start.Line || p.Line == start.Line && p.Column <= start.Column {
			continue
		}
		if t.Type != token.CommentType && t.Type != token.DocumentEndType {
			return Pos{Line: p.Line, Column: p.Column}, true
		}
	}
	return Pos{Line: 0, Column: 0}, false
}

// Error is a mistake in a YAML file, at a line and column of it.
type Error struct {
	File string
	Pos  Pos
	Err  *usererr.Error
}

// New returns the mistake worded by the catalog message key with args, at at in file.
func New(file string, at Pos, key string, args ...any) *Error {
	return &Error{File: file, Pos: at, Err: usererr.New(key, args...)}
}

// Error words the mistake in English.
func (e *Error) Error() string {
	return e.Localize(i18n.EnglishCatalog())
}

// Localize words the mistake in the language of c, after the file, line and column.
func (e *Error) Localize(c *i18n.Catalog) string {
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Pos.Line, e.Pos.Column, e.Err.Localize(c))
}

// Problems are mistakes reported all at once, each at its own line and column.
type Problems []*Error

// Error words the mistakes in English, one per line.
func (p Problems) Error() string {
	return p.Localize(i18n.EnglishCatalog())
}

// Localize words the mistakes in the language of c, one per line.
func (p Problems) Localize(c *i18n.Catalog) string {
	lines := make([]string, len(p))
	for i, e := range p {
		lines[i] = e.Localize(c)
	}
	return strings.Join(lines, "\n")
}

// FirstAnchor returns the first anchor, alias, merge key (<<) or tag under n, or nil. Callers refuse them, so that
// every value is written where its position points.
func FirstAnchor(n ast.Node) ast.Node {
	a := &anchors{found: nil}
	ast.Walk(a, n)
	return a.found
}

type anchors struct {
	found ast.Node
}

func (a *anchors) Visit(n ast.Node) ast.Visitor {
	if a.found != nil {
		return nil
	}
	switch n.(type) {
	case *ast.AnchorNode, *ast.AliasNode, *ast.TagNode, *ast.MergeKeyNode:
		a.found = n
		return nil
	default:
		return a
	}
}
