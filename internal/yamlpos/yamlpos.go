// Package yamlpos points at where something was written in a YAML file: the line and column of a
// node, and the mistakes there, so that a diagram spec and a data file place theirs alike.
package yamlpos

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

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
// the last step (the - of an entry of a block sequence; nil at root or in a flow sequence), its value
// node, and the path written as sections[0].parts[1]. It stops at the last node it can reach.
func Find(root ast.Node, at []string) (key, value ast.Node, path string) {
	value = root
	var b strings.Builder
	for _, step := range at {
		switch n := value.(type) {
		case *ast.MappingNode:
			i := slices.IndexFunc(n.Values, func(v *ast.MappingValueNode) bool { return KeyOf(v).GetToken().Value == step })
			if i < 0 {
				return key, value, b.String()
			}
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(step)
			key, value = KeyOf(n.Values[i]), n.Values[i].Value
		case *ast.SequenceNode:
			i, err := strconv.Atoi(step)
			if err != nil || i < 0 || i >= len(n.Values) {
				return key, value, b.String()
			}
			fmt.Fprintf(&b, "[%d]", i)
			key, value = nil, n.Values[i]
			// goccy/go-yaml gives a flow sequence entries too, without a - to point at.
			if !n.IsFlowStyle && len(n.Entries) == len(n.Values) {
				key = n.Entries[i] // its -, where an entry left empty is
			}
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
		return nil, Syntax(file, err)
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

// KeyOf returns the key of v, the text after ? for an explicit key.
func KeyOf(v *ast.MappingValueNode) ast.Node {
	if k, ok := v.Key.(*ast.MappingKeyNode); ok {
		return k.Value
	}
	return v.Key
}

// OfValue returns where the value of key is written, or where key is when the value is left out:
// an empty value has no text to point at.
func OfValue(key, value ast.Node) Pos {
	if _, null := value.(*ast.NullNode); null && key != nil {
		return Of(key)
	}
	return Of(value)
}

// Syntax places an error of goccy/go-yaml at its token, or at the start of file without one. When the
// tokens around it show text that YAML read as something else, a hint says to quote it, at where that
// began when it is elsewhere.
func Syntax(file string, err error) *usererr.Error {
	var se *yaml.SyntaxError
	if !errors.As(err, &se) || se.Token == nil {
		return New(file, Pos{Line: 1, Column: 1}, "yaml_syntax", err.Error())
	}
	hint, cause := syntaxHint(se.Token)
	var detail any = se.Message
	if hint != "" {
		detail = HintFirst(hint, se.Message)
	}
	return New(file, Pos{Line: cause.Position.Line, Column: cause.Position.Column}, "yaml_syntax", detail)
}

// NewHinted returns the mistake worded by the catalog message key with args, at at in file, with the
// hint worded by the key hint after it.
func NewHinted(file string, at Pos, hint, key string, args ...any) *usererr.Error {
	return New(file, at, "yaml_hinted", usererr.New(key, args...), usererr.New(hint))
}

// HintFirst words the hint by the catalog message key hint, with what a library said of the mistake,
// detail, after it: the hint names the cause, which the library's words may hide.
func HintFirst(hint, detail string) *usererr.Error {
	return usererr.New("yaml_hint_first", usererr.New(hint), detail)
}

// syntaxHint returns the key of the hint for a syntax error at tk, or "" for none, and the token where
// what the hint names began (tk when it began there):
//   - text with ": " in it: a second key on the line of one (a: b: c), or a value beginning with a
//     reserved ` or @ after a ": " that ends no key, at that ":";
//   - a comment that " #" began in text, at the comment: one that took the } or ] closing its { } or
//     [ ], or one after which the text goes on, on the next line;
//   - a value beginning with ` or @.
func syntaxHint(tk *token.Token) (string, *token.Token) {
	value := strings.TrimSpace(tk.Value)
	reserved := tk.Type == token.InvalidType && (strings.HasPrefix(value, "`") || strings.HasPrefix(value, "@"))
	switch {
	case onLine(tk.Prev, tk, token.MappingValueType) && onLine(tk.Next, tk, token.MappingValueType):
		return "yaml_hint_colon", tk
	case reserved && colonInText(tk.Prev):
		return "yaml_hint_colon", tk.Prev
	case textGoesOn(tk):
		return "yaml_hint_comment", tk.Prev
	}
	if c := closerInComment(tk); c != nil {
		return "yaml_hint_comment", c
	}
	if reserved {
		return "yaml_hint_reserved", tk
	}
	return "", tk
}

// colonInText reports whether colon is a ":" that ends no key: one after another ":" on its line
// (a: b: c), or one in a [ ] after text in which a ` opens inline code, `: ` being in it.
func colonInText(colon *token.Token) bool {
	if !onLine(colon, colon, token.MappingValueType) || !onLine(colon.Prev, colon, token.StringType) {
		return false
	}
	before := colon.Prev.Prev
	switch {
	case onLine(before, colon, token.MappingValueType):
		return true
	case onLine(before, colon, token.SequenceStartType), onLine(before, colon, token.CollectEntryType):
		return strings.Count(colon.Prev.Value, "`")%2 == 1
	}
	return false
}

// textGoesOn reports whether tk, the token of a syntax error, begins the line after a comment that
// follows text on its line, further in than that line and not as a key: text written over lines that
// " #" cut.
// LIMIT: text that goes on in a line with ": " in it reads as a key indented too far, and gets no
// hint. Look past the first ": " if that hides a cut.
func textGoesOn(tk *token.Token) bool {
	c := tk.Prev
	if c == nil || c.Type != token.CommentType || tk.Position.Line != c.Position.Line+1 ||
		c.Prev == nil || c.Prev.Type != token.StringType || lastLine(c.Prev) != c.Position.Line ||
		onLine(tk.Next, tk, token.MappingValueType) {
		return false
	}
	first := c.Prev // the first token on the line the text begins on
	for first.Prev != nil && first.Prev.Position.Line == c.Prev.Position.Line {
		first = first.Prev
	}
	return tk.Position.Column > first.Position.Column
}

// lastLine returns the line the text of tk ends on, which a plain value written over lines does after
// the line it begins on.
func lastLine(tk *token.Token) int {
	return tk.Position.Line + strings.Count(strings.TrimSpace(tk.Origin), "\n")
}

// onLine reports whether t is a token of the type typ on the line of on.
func onLine(t, on *token.Token, typ token.Type) bool {
	return t != nil && t.Type == typ && t.Position.Line == on.Position.Line
}

// closerInComment returns the first comment, in the file of tk, that holds the } or ] of the { } or [ ]
// it begins in, which is left open: " #" took the rest of the line, its closer too. nil for none.
func closerInComment(tk *token.Token) *token.Token {
	for tk.Prev != nil {
		tk = tk.Prev
	}
	type collection struct {
		closer string
		cut    *token.Token // the first comment in it that holds its closer
	}
	var open []collection
	for ; tk != nil; tk = tk.Next {
		switch tk.Type {
		case token.MappingStartType:
			open = append(open, collection{closer: "}", cut: nil})
		case token.SequenceStartType:
			open = append(open, collection{closer: "]", cut: nil})
		case token.MappingEndType, token.SequenceEndType:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		case token.CommentType:
			if n := len(open); n > 0 && open[n-1].cut == nil && strings.Contains(tk.Value, open[n-1].closer) {
				open[n-1].cut = tk
			}
		default:
		}
	}
	for _, c := range open {
		if c.cut != nil {
			return c.cut
		}
	}
	return nil
}

// ColonText reports whether n is how YAML reads a line of plain text with ": " in it: one key, not in
// quotes, right after the - or the [ or , it is an entry of, and a value of text on its line.
// LIMIT: a mapping of one such key and value meant as one is taken for text too. Ask for more of the
// line (a second ": ", a space in the key) if the hint misleads.
func ColonText(n ast.Node) bool {
	var pair *ast.MappingValueNode
	switch m := n.(type) {
	case *ast.MappingNode:
		if len(m.Values) != 1 || m.IsFlowStyle {
			return false
		}
		pair = m.Values[0]
	case *ast.MappingValueNode:
		pair = m
	default:
		return false
	}
	switch pair.Value.(type) {
	case *ast.NullNode, *ast.LiteralNode:
		return false
	}
	k := KeyOf(pair).GetToken()
	if _, text := pair.Value.(ast.ScalarNode); !text || k == nil || k.Type == token.SingleQuoteType ||
		k.Type == token.DoubleQuoteType {
		return false
	}
	entry := onLine(k.Prev, k, token.SequenceEntryType) || onLine(k.Prev, k, token.SequenceStartType) ||
		onLine(k.Prev, k, token.CollectEntryType)
	return entry && pair.Value.GetToken().Position.Line == k.Position.Line
}

// CommentAfter returns the text of the comment after the token of n on its last line, without its #,
// or "" for none: what YAML makes of " #" and the rest of a plain value, written over lines or not. A
// value in quotes ends at its quote, before any comment.
func CommentAfter(n ast.Node) string {
	tk := n.GetToken()
	if tk == nil || tk.Type == token.SingleQuoteType || tk.Type == token.DoubleQuoteType ||
		tk.Next == nil || tk.Next.Type != token.CommentType {
		return ""
	}
	if tk.Next.Position.Line != lastLine(tk) {
		return ""
	}
	return tk.Next.Value
}

// New returns the mistake worded by the catalog message key with args, at at in file.
func New(file string, at Pos, key string, args ...any) *usererr.Error {
	return usererr.NewAt(file, at.Line, at.Column, key, args...)
}

// Place returns e at at in file, in place of any place it had.
func Place(file string, at Pos, e *usererr.Error) *usererr.Error {
	return e.Placed(file, at.Line, at.Column, "")
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
