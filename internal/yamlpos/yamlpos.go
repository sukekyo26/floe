// Package yamlpos points at where something was written in a YAML file: the line and column of a
// node, and mistakes reported there, so that a diagram spec and a data file word theirs alike.
package yamlpos

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

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

// Find follows at, keys of mappings and indexes of sequences, from root. It returns the key node of
// the last step (nil at root or for an index), its value node, and the path written as
// sections[0].parts[1]. It stops at the last node it can reach.
func Find(root ast.Node, at []string) (key, value ast.Node, path string) {
	value = root
	var b strings.Builder
	for _, step := range at {
		switch n := value.(type) {
		case *ast.MappingNode:
			i := slices.IndexFunc(n.Values, func(v *ast.MappingValueNode) bool { return v.Key.GetToken().Value == step })
			if i < 0 {
				return key, value, b.String()
			}
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(step)
			key, value = n.Values[i].Key, n.Values[i].Value
		case *ast.SequenceNode:
			i, err := strconv.Atoi(step)
			if err != nil || i < 0 || i >= len(n.Values) {
				return key, value, b.String()
			}
			fmt.Fprintf(&b, "[%d]", i)
			key, value = nil, n.Values[i]
		default:
			return key, value, b.String()
		}
	}
	return key, value, b.String()
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
	// A directive (%YAML) heads the document after it, and --- at the end starts none.
	var docs []*ast.DocumentNode
	for _, d := range doc.Docs {
		if _, directive := d.Body.(*ast.DirectiveNode); d.Body != nil && !directive {
			docs = append(docs, d)
		}
	}
	switch {
	case len(docs) == 0:
		return nil, ErrEmpty
	case len(docs) > 1:
		// At the --- that starts the second document, or at its body after a ... that ended the first.
		at := Of(docs[1].Body)
		if s := docs[1].Start; s != nil {
			at = Pos{Line: s.Position.Line, Column: s.Position.Column}
		}
		return nil, New(file, at, documents)
	}
	return docs[0].Body, nil
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

// FirstAnchor returns the first anchor, alias or tag under n, or nil. Callers refuse them, so that
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
	case *ast.AnchorNode, *ast.AliasNode, *ast.TagNode:
		a.found = n
		return nil
	default:
		return a
	}
}
