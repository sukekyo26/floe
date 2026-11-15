package yamlpos

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"

	"floe/internal/bom"
	"floe/internal/textpos"
)

// Kind is what a Node holds.
type Kind int

const (
	Scalar   Kind = iota // text, a number, true or false, or null
	Mapping              // keys and values
	Sequence             // items
)

// Node is a value of a YAML file Parse read, and where it is written.
type Node struct {
	Kind  Kind
	Pairs []Pair  // of a Mapping, as written
	Items []*Node // of a Sequence
	// Value is a Scalar as the parser read it: text without its quotes, a number or true as written (1.10), "" for
	// nothing written.
	Value string
	Style Style
	Null  bool // null, ~ or nothing written

	at        Pos // where Of places it
	start     Pos // its first character: the { or [ of a flow mapping or sequence
	flow      bool
	unwritten bool // a null with nothing written
	text      bool // a text scalar
	node      ast.Node
}

// Pair is a key and its value.
type Pair struct {
	Key, Value *Node
}

// newNode returns n, a body Parse walked without a mistake, as a Node; err is floe's failure.
func newNode(n ast.Node) (*Node, error) {
	tk := n.GetToken()
	out := &Node{
		Kind: Scalar, Pairs: nil, Items: nil, Value: "", Style: Plain, Null: false,
		at: writtenAt(n), start: Pos{Line: tk.Position.Line, Column: tk.Position.Column}, flow: false, unwritten: false,
		text: false, node: n,
	}
	var err error
	switch n := n.(type) {
	case *ast.MappingNode:
		out.Kind, out.flow = Mapping, n.IsFlowStyle
		out.Pairs, err = newPairs(n.Values)
	case *ast.SequenceNode:
		out.Kind, out.flow = Sequence, n.IsFlowStyle
		out.Items, err = newItems(n.Values)
	default:
		err = out.scalar(n)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func newPairs(values []*ast.MappingValueNode) ([]Pair, error) {
	pairs := make([]Pair, 0, len(values))
	for _, v := range values {
		key, err := newNode(keyOf(v))
		if err != nil {
			return nil, err
		}
		value, err := newNode(v.Value)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, Pair{Key: key, Value: value})
	}
	return pairs, nil
}

func newItems(values []ast.Node) ([]*Node, error) {
	items := make([]*Node, 0, len(values))
	for _, v := range values {
		item, err := newNode(v)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// scalar reads n, a scalar, into out.
func (out *Node) scalar(n ast.Node) error {
	tk := n.GetToken()
	switch n := n.(type) {
	case *ast.StringNode:
		out.Value, out.Style, out.text = n.Value, quoteOf(n), true
	case *ast.LiteralNode:
		out.Value, out.Style, out.text = n.Value.Value, Literal, true
		if strings.HasPrefix(n.Start.Value, ">") {
			out.Style = Folded
		}
	case *ast.NullNode:
		out.Null, out.unwritten = true, tk.Type == token.ImplicitNullType
		if !out.unwritten {
			out.Value = tk.Value // goccy/go-yaml's token of nothing written holds null
		}
	case *ast.IntegerNode, *ast.FloatNode, *ast.BoolNode, *ast.InfinityNode, *ast.NanNode:
		out.Value = tk.Value
	default:
		return fmt.Errorf("yamlpos: a value of the type %T", n) //nolint:err113 // floe's own
	}
	return nil
}

// Of returns where n, which must not be nil, was written: a mapping at its first key, so that it points at text
// rather than the indentation before it.
func Of(n *Node) Pos {
	return n.at
}

// Text returns n as text when it is a text scalar: a string, quoted or not, or a | or > block, read as YAML reads
// it, a block keeping the line breaks its end keeps (| one, |+ all, |- none). A number, true, false or null is not
// text: YAML would read 1.10 as 1.1.
func Text(n *Node) (string, bool) {
	if !n.text {
		return "", false
	}
	return n.Value, true
}

// OfValue returns where the value of key is written, ~ and null too: a scalar at itself; a mapping or a sequence,
// or a value left out, at key, the line it starts on (a mapping at its first key would read as that key's, and an
// empty value has no text to point at). With no key, an item of a sequence is at itself: a mapping or a sequence in
// { } or [ ] at its { or [, any other at where it starts (a mapping at its first key), as YAML 1.2 parsers do not all
// tell where its - is.
// LIMIT: a mistake about one key placed by the path to it (additionalProperties naming one) lands where its value
// is: at the key for a mapping or a sequence (zz: with y: 1 below it, 3:1), at the value for a scalar (zz: 1, 3:5).
// A data file's mistake could go to the key itself, as schema.Validate has the file's nodes, but a diagram spec's
// comes as a JSON pointer (schema.ValidateAt), which cannot name a key apart from its value: both are placed alike.
// LIMIT: a pair in [ ] written without { } (a: [b: 1]) is a mapping goccy/go-yaml reads as not in flow style, with no
// { to point at: it is placed at its first key, which is where it starts too.
func OfValue(key, value *Node) Pos {
	switch {
	case key != nil && (value.Kind != Scalar || value.unwritten):
		return key.at
	case value.Kind != Scalar && value.flow:
		return value.start
	}
	return value.at
}

// Name returns the name of the key of p.
func (p Pair) Name() string {
	return p.Key.Value
}

// find follows at, keys of mappings and indexes of sequences, from root. It returns the key node of the last
// step (nil at root or for an item of a sequence) and its value node. It stops at the last node it can reach.
func find(root *Node, at []string) (key, value *Node) {
	value = root
	for _, step := range at {
		switch value.Kind {
		case Mapping:
			i := slices.IndexFunc(value.Pairs, func(p Pair) bool { return p.Name() == step })
			if i < 0 {
				return key, value
			}
			key, value = value.Pairs[i].Key, value.Pairs[i].Value
		case Sequence:
			i, err := strconv.Atoi(step)
			if err != nil || i < 0 || i >= len(value.Items) {
				return key, value
			}
			key, value = nil, value.Items[i]
		default:
			return key, value
		}
	}
	return key, value
}

// Locate finds the value at the path at under root, and returns where it is written, as OfValue places it, and its
// node; at the last value it reaches when the path goes on past it. When it reaches none under root (at is empty,
// its first key is not written, or root is nil, a file of nothing), it returns no line and no node: the mistake is
// placed at the file alone, as at the first key it would read as that key's.
func Locate(root *Node, at []string) (Pos, *Node) {
	if root != nil {
		if key, value := find(root, at); value != root {
			return OfValue(key, value), value
		}
	}
	return Pos{Line: 0, Column: 0}, nil
}

// Written returns where the characters of n, a scalar of src, the YAML file Parse read, start when they are
// written there as YAML reads them, for a place inside the value: a plain or quoted value on one line, at its first
// character after the quote; a | text, at the first character of its first line, past the indentation; false for
// any other. The characters of src are only compared with the value read, which tells where they are: nothing is
// judged by reading src.
// LIMIT: a value whose characters are not written as read is not placed inside: an escape in double quotes, two
// single quotes, a value over lines, a > text, a | text whose indentation is text too, a plain value goccy/go-yaml
// reads without its tabs, and any value of a file whose places goccy/go-yaml counts otherwise (starts). Map the
// characters read to those written if a place inside such a value comes to matter.
func Written(src []byte, n *Node) (Pos, bool) {
	none := Pos{Line: 0, Column: 0}
	src = bom.Trim(src)
	at := textpos.At(src, n.at.Line, n.at.Column)
	if n.Kind != Scalar || at < 0 {
		return none, false
	}
	rest := src[at:]
	switch n.Style {
	case Plain:
		if bytes.HasPrefix(rest, []byte(n.Value)) {
			return n.at, true
		}
	case Single:
		// One ' read is written as two: 'a''' (read a') would match 'a''.
		if !strings.Contains(n.Value, "'") && bytes.HasPrefix(rest, []byte("'"+n.Value+"'")) {
			return Pos{Line: n.at.Line, Column: n.at.Column + 1}, true
		}
	case Double:
		// A value written with an escape does not match: at the first escape, src has a \ where the value read has
		// another character, or, for \\, one more \, which the next character of the value or the " then meets.
		if bytes.HasPrefix(rest, []byte(`"`+n.Value+`"`)) {
			return Pos{Line: n.at.Line, Column: n.at.Column + 1}, true
		}
	case Literal:
		return blockWritten(src, n.at.Line, rest, n.Value)
	case Folded:
	}
	return none, false
}

// blockWritten returns where value, a | text whose header (rest on) is on the line header of src, starts: on the line
// after the header, past the spaces before its first line that holds text, which must be that line of value, as the
// same line of the text.
func blockWritten(src []byte, header int, rest []byte, value string) (Pos, bool) {
	none := Pos{Line: 0, Column: 0}
	lines := strings.Split(value, "\n")
	first := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) != "" })
	if first < 0 {
		return none, false
	}
	at := textpos.NextLine(src, len(src)-len(rest))
	for at < len(src) {
		end := textpos.NextLine(src, at)
		line := strings.TrimRight(string(src[at:end]), "\r\n")
		if strings.TrimSpace(line) == "" {
			at = end
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if line[indent:] != lines[first] {
			return none, false
		}
		return Pos{Line: header + 1, Column: indent + 1}, true
	}
	return none, false
}
