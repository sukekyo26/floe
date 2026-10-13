// Package yamlpos points at where something was written in a YAML file: the line and column of a
// node, and mistakes reported there, so that a diagram spec and a data file word theirs alike.
package yamlpos

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml/ast"

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

// Of returns where n was written: a mapping at its first key, so that it points at text rather than
// the indentation before it.
func Of(n ast.Node) Pos {
	if m, ok := n.(*ast.MappingNode); ok && len(m.Values) > 0 {
		n = m.Values[0].Key
	}
	p := n.GetToken().Position
	return Pos{Line: p.Line, Column: p.Column}
}

// Error is a mistake in a YAML file, at a line and column of it.
type Error struct {
	File string
	Pos  Pos
	Err  *usererr.Error
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
