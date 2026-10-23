package diagram

import (
	"math"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// fields reads the keys of one mapping and keeps every mistake, one to a value, so a decoder reads every
// key in a row and checks once at the end. The keys a decoder asks for are the keys it knows.
type fields struct {
	file     string
	kind     string      // names the mapping in messages: diagram, box, edge…
	at       yamlpos.Pos // where a missing key is reported
	values   []*ast.MappingValueNode
	asked    []string
	mistakes usererr.List
	missed   usererr.List // of keys not given, told unless one given is unknown
}

func newFields(file, kind string, at yamlpos.Pos, n ast.Node) (*fields, error) {
	f := &fields{file: file, kind: kind, at: at, values: nil, asked: nil, mistakes: nil, missed: nil}
	m, ok := n.(*ast.MappingNode)
	if !ok {
		if _, null := n.(*ast.NullNode); !null {
			at = yamlpos.Of(n)
		}
		return nil, f.errAt(at, "diagram_not_mapping", kind)
	}
	f.values = m.Values
	return f, nil
}

func (f *fields) errAt(at yamlpos.Pos, key string, args ...any) *usererr.Error {
	return yamlpos.New(f.file, at, key, args...)
}

// fail tells the mistake key of the value n, unless it has one: what is checked of a value it does not hold
// would only say again what is wrong with it.
func (f *fields) fail(n ast.Node, key string, args ...any) {
	f.add(f.errAt(yamlpos.Of(n), key, args...))
}

func (f *fields) add(e *usererr.Error) {
	if !f.failed(yamlpos.Pos{Line: e.Line, Column: e.Column}) {
		f.mistakes = append(f.mistakes, e)
	}
}

func (f *fields) failed(at yamlpos.Pos) bool {
	return slices.ContainsFunc(f.mistakes, func(e *usererr.Error) bool {
		return e.Line == at.Line && e.Column == at.Column
	})
}

// ok reports whether key is given and its value read without a mistake, for a check that reads it.
func (f *fields) ok(key string) bool {
	n := f.lookup(key)
	return n != nil && !f.failed(yamlpos.Of(n))
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
	f.absent(f.at, "diagram_missing_key", f.kind, key)
}

// absent tells the mistake key, at at, of a key not given, which done tells only if no key given is unknown.
func (f *fields) absent(at yamlpos.Pos, key string, args ...any) {
	f.missed = append(f.missed, f.errAt(at, key, args...))
}

// done returns every mistake: each unknown key, the mistakes of the values, then the keys not given, unless
// a key is unknown: a misspelt key also leaves the key it meant missing, and the misspelling is what to fix.
func (f *fields) done() error {
	var unknown usererr.List
	for _, v := range f.values {
		if !slices.Contains(f.asked, v.Key.String()) {
			unknown = append(unknown,
				f.errAt(yamlpos.Of(v.Key), "diagram_unknown_key", f.kind, v.Key.String(), strings.Join(f.asked, ", ")))
		}
	}
	if len(unknown) > 0 {
		return slices.Concat(unknown, f.mistakes).Err()
	}
	return slices.Concat(f.mistakes, f.missed).Err()
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

// size returns the length under key, which must be given and above 0.
func (f *fields) size(key string) float64 {
	v := f.num(key)
	f.positive(key, v)
	return v
}

func (f *fields) optSize(key string, def float64) float64 {
	v := f.optNum(key, def)
	f.positive(key, v)
	return v
}

func (f *fields) positive(key string, v float64) {
	if n := f.lookup(key); n != nil && v <= 0 {
		f.fail(n, "diagram_not_positive", f.kind, key)
	}
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
		if yamlpos.ColonText(n) {
			f.add(yamlpos.NewHinted(f.file, yamlpos.Of(n), "yaml_hint_colon", "diagram_not_text", f.kind, key))
		}
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
	if !slices.Contains(allowed, v) {
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
