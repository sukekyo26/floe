package diagram

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Pos is a line and column of the spec's YAML, both from 1.
type Pos struct {
	Line, Column int
}

// At returns the position, so that every part embedding Pos tells where it was written.
func (p Pos) At() Pos {
	return p
}

// Error is a mistake in a spec, at a line and column of its YAML.
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

// anchors finds the first anchor, alias or tag. They are refused, so that every value is written
// where its position points.
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

func posOf(n ast.Node) Pos {
	if m, ok := n.(*ast.MappingNode); ok && len(m.Values) > 0 {
		n = m.Values[0].Key
	}
	p := n.GetToken().Position
	return Pos{Line: p.Line, Column: p.Column}
}

// fields reads the keys of one mapping and keeps the first mistake, so a decoder reads every key
// in a row and checks once at the end. The keys a decoder asks for are the keys it knows.
type fields struct {
	file   string
	kind   string // names the mapping in messages: diagram, box, edge…
	at     Pos    // where a missing key is reported
	values []*ast.MappingValueNode
	asked  []string
	err    *Error
}

func newFields(file, kind string, at Pos, n ast.Node) (*fields, error) {
	f := &fields{file: file, kind: kind, at: at, values: nil, asked: nil, err: nil}
	m, ok := n.(*ast.MappingNode)
	if !ok {
		if _, null := n.(*ast.NullNode); !null {
			at = posOf(n)
		}
		return nil, f.errAt(at, "diagram_not_mapping", kind)
	}
	f.values = m.Values
	return f, nil
}

func (f *fields) errAt(at Pos, key string, args ...any) *Error {
	return &Error{File: f.file, Pos: at, Err: usererr.New(key, args...)}
}

func (f *fields) fail(n ast.Node, key string, args ...any) {
	if f.err == nil {
		f.err = f.errAt(posOf(n), key, args...)
	}
}

// get returns the value of key, or nil when the mapping does not have it.
func (f *fields) get(key string) ast.Node {
	f.asked = append(f.asked, key)
	return f.lookup(key)
}

// lookup is get for a key already asked for, to point a later mistake at its value.
func (f *fields) lookup(key string) ast.Node {
	for _, v := range f.values {
		if v.Key.String() == key {
			return v.Value
		}
	}
	return nil
}

func (f *fields) missing(key string) {
	if f.err == nil {
		f.err = f.errAt(f.at, "diagram_missing_key", f.kind, key)
	}
}

// done reports an unknown key before any other mistake: a misspelt key also leaves the key it
// meant missing, and the misspelling is what to fix.
func (f *fields) done() error {
	for _, v := range f.values {
		if !slices.Contains(f.asked, v.Key.String()) {
			return f.errAt(posOf(v.Key), "diagram_unknown_key", f.kind, v.Key.String(), strings.Join(f.asked, ", "))
		}
	}
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fields) number(key string, n ast.Node) float64 {
	var v float64
	switch n := n.(type) {
	case *ast.IntegerNode:
		switch i := n.Value.(type) {
		case int64:
			v = float64(i)
		case uint64:
			v = float64(i)
		}
	case *ast.FloatNode:
		v = n.Value
	default:
		f.fail(n, "diagram_not_number", f.kind, key)
		return 0
	}
	if math.IsInf(v, 0) {
		f.fail(n, "diagram_not_number", f.kind, key)
	}
	return v
}

// num returns the number under key, which must be given.
func (f *fields) num(key string) float64 {
	n := f.get(key)
	if n == nil {
		f.missing(key)
		return 0
	}
	return f.number(key, n)
}

func (f *fields) optNum(key string, def float64) float64 {
	n := f.get(key)
	if n == nil {
		return def
	}
	return f.number(key, n)
}

// optNumPtr returns nil when key is not given, for a number whose every value means something.
func (f *fields) optNumPtr(key string) *float64 {
	n := f.get(key)
	if n == nil {
		return nil
	}
	v := f.number(key, n)
	return &v
}

// text takes any scalar as text, so a label such as 2026 or 1.0 reads as written.
func (f *fields) text(key string, n ast.Node) string {
	switch n := n.(type) {
	case *ast.StringNode:
		return n.Value
	case *ast.LiteralNode:
		return n.Value.Value
	case *ast.IntegerNode, *ast.FloatNode, *ast.BoolNode:
		return n.GetToken().Value
	default:
		f.fail(n, "diagram_not_text", f.kind, key)
		return ""
	}
}

// str returns the text under key, which must be given.
func (f *fields) str(key string) string {
	n := f.get(key)
	if n == nil {
		f.missing(key)
		return ""
	}
	return f.text(key, n)
}

func (f *fields) optStr(key string) string {
	n := f.get(key)
	if n == nil {
		return ""
	}
	return f.text(key, n)
}

func (f *fields) flag(key string) bool {
	n := f.get(key)
	if n == nil {
		return false
	}
	b, ok := n.(*ast.BoolNode)
	if !ok {
		f.fail(n, "diagram_not_bool", f.kind, key)
		return false
	}
	return b.Value
}

// oneOf returns the text under key, def when it is not given, and refuses a value not in allowed.
func (f *fields) oneOf(key, def string, allowed ...string) string {
	n := f.get(key)
	if n == nil {
		return def
	}
	v := f.text(key, n)
	f.allow(n, key, v, allowed)
	return v
}

func (f *fields) allow(n ast.Node, key, v string, allowed []string) {
	if f.err == nil && !slices.Contains(allowed, v) {
		f.fail(n, "diagram_unknown_value", f.kind, key, v, strings.Join(allowed, ", "))
	}
}

// list returns the items under key, or nil when it is not given.
func (f *fields) list(key string) []ast.Node {
	n := f.get(key)
	if n == nil {
		return nil
	}
	s, ok := n.(*ast.SequenceNode)
	if !ok {
		f.fail(n, "diagram_not_list", f.kind, key)
		return nil
	}
	return s.Values
}

// pair reads [x, y] under key, or nil when it is not given.
func (f *fields) pair(key string) *[2]float64 {
	n := f.get(key)
	if n == nil {
		return nil
	}
	s, ok := n.(*ast.SequenceNode)
	if !ok || len(s.Values) != 2 {
		f.fail(n, "diagram_not_pair", f.kind, key)
		return nil
	}
	return &[2]float64{f.number(key, s.Values[0]), f.number(key, s.Values[1])}
}
