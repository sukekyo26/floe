// Package yamlpos reads a YAML file as one document and points at where something was written in it: the
// line and column of a node, and the mistakes there.
package yamlpos

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

	"github.com/sukekyo26/floe/internal/bom"
	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Pos is a line and column of a YAML file, both from 1.
type Pos struct {
	Line, Column int
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

// KeyOf returns the key of v, the text after ? for an explicit key.
func KeyOf(v *ast.MappingValueNode) ast.Node {
	if k, ok := v.Key.(*ast.MappingKeyNode); ok {
		return k.Value
	}
	return v.Key
}

// KeyName returns the name of the key of v, as KeyOf reads it.
func KeyName(v *ast.MappingValueNode) string {
	return KeyOf(v).GetToken().Value
}

// Quote is how a scalar is quoted, as the parser read it.
type Quote int

const (
	Plain Quote = iota // not quoted, or not a scalar
	Double
	Single
)

// QuoteOf returns how n is quoted.
func QuoteOf(n ast.Node) Quote {
	switch n.GetToken().Type {
	case token.DoubleQuoteType:
		return Double
	case token.SingleQuoteType:
		return Single
	default:
		return Plain
	}
}

// OfValue returns where the value of key is written, ~ and null too, or where key is when the value is left
// out: an empty value has no text to point at.
func OfValue(key, value ast.Node) Pos {
	if n, null := value.(*ast.NullNode); null && n.GetToken().Type == token.ImplicitNullType && key != nil {
		return Of(key)
	}
	return Of(value)
}

// Find follows at, keys of mappings and indexes of sequences, from root. It returns the key node of the last
// step (the - of an entry of a block sequence; nil at root or in a flow sequence) and its value node. It
// stops at the last node it can reach.
func Find(root ast.Node, at []string) (key, value ast.Node) {
	value = root
	for _, step := range at {
		switch n := value.(type) {
		case *ast.MappingNode:
			i := slices.IndexFunc(n.Values, func(v *ast.MappingValueNode) bool { return KeyName(v) == step })
			if i < 0 {
				return key, value
			}
			key, value = KeyOf(n.Values[i]), n.Values[i].Value
		case *ast.SequenceNode:
			i, err := strconv.Atoi(step)
			if err != nil || i < 0 || i >= len(n.Values) {
				return key, value
			}
			key, value = nil, n.Values[i]
			// goccy/go-yaml gives a flow sequence entries too, whose token is the , before each: nil for the first.
			if !n.IsFlowStyle && len(n.Entries) == len(n.Values) {
				key = n.Entries[i] // its -, where an entry left empty is
			}
		default:
			return key, value
		}
	}
	return key, value
}

// New returns the mistake worded by the catalog message key with args, at at in file.
func New(file string, at Pos, key string, args ...any) *usererr.Error {
	return usererr.NewAt(file, at.Line, at.Column, key, args...)
}

// library returns the mistake goccy/go-yaml told in err, in its words, at its token in file (without a
// place when it has none). Any other error is returned as it is: floe's failure.
func library(file string, err error) error {
	var ye yaml.Error
	if !errors.As(err, &ye) {
		return err
	}
	if tk := ye.GetToken(); tk != nil {
		return usererr.LibraryAt(file, tk.Position.Line, tk.Position.Column, ye.GetMessage())
	}
	return usererr.LibraryAt(file, 0, 0, ye.GetMessage())
}

// ErrEmpty is returned by Parse for a file without a document, for the caller to word.
var ErrEmpty = errors.New("no YAML document")

// Decode reads the values of n, a node Parse returned, into v. Its mistake is goccy/go-yaml's (library).
func Decode(file string, n ast.Node, v any) error {
	if err := yaml.NodeToValue(n, v); err != nil {
		return library(file, err)
	}
	return nil
}

// Text returns n as text when it is a text scalar: a string, quoted or not, or a | or > block, read as YAML reads
// it, a block keeping the line breaks its end keeps (| one, |+ all, |- none). A number, true, false or null is not
// text: YAML would read 1.10 as 1.1. err is floe's failure: a text scalar of a node Parse returned decodes.
func Text(n ast.Node) (text string, ok bool, err error) {
	switch n.(type) {
	case *ast.StringNode, *ast.LiteralNode:
	default:
		return "", false, nil
	}
	if err := yaml.NodeToValue(n, &text); err != nil {
		return "", false, fmt.Errorf("yamlpos: decode a text: %w", err)
	}
	return text, true, nil
}

// Parse reads src as one YAML document and returns its body, or ErrEmpty when src holds none. A byte order
// mark at its start is left out, as YAML 1.2 reads it. A syntax error is goccy/go-yaml's (library). A
// second document, or text a second --- drops, is refused at its start, and nothing in it is walked.
// Else one walk of the body refuses, all together, sorted: anchors, aliases, merge keys and tags, not
// walking what they hold; a key that is not text, as goccy/go-yaml or YAML 1.2 reads it; a value or a text
// key a comment may have cut short; a key in { } without a value, but one refused already; and a plain value
// that YAML 1.2 reads otherwise than goccy/go-yaml (017, 1e3), or as a number JSON has none for (.inf, .nan)
// or floe cannot hold.
func Parse(file string, src []byte) (ast.Node, error) {
	body, err := document(file, src)
	if err != nil {
		return nil, err
	}
	w := &walk{file: file, found: nil, failed: nil}
	w.node(body)
	if w.failed != nil {
		return nil, w.failed
	}
	return body, w.found.Sorted().Err()
}

// document returns the body of the one document of src. A document of a directive (%YAML) alone is no
// document: it heads the one after it. src is read as UTF-8, and a byte order mark past its start is refused
// with it, before it is parsed: goccy/go-yaml reads the one and the other as characters of a key or a value
// (U+FFFD for a byte that is no UTF-8), and fails on them where it does not.
// LIMIT: a control character (NUL) and UTF-16 without a mark are UTF-8 and left to goccy/go-yaml, which reads a
// NUL in a value; refuse what YAML 1.2 does not print if one comes to matter.
// LIMIT: a lone surrogate escaped in double quotes ("\udc00") goccy/go-yaml reads as U+FFFD, telling only a high
// one with nothing after it; refuse them if goccy/go-yaml comes to tell them, or one comes to matter.
func document(file string, src []byte) (ast.Node, error) {
	// YAML 1.2 allows a byte order mark at the start, which goccy/go-yaml reads as part of the first key.
	// src is left without it for both the parser and dropped, so that their places agree.
	src = bom.Trim(src)
	if err := usererr.Collect(bom.Refuse(file, src).Err(), notUTF8(file, src)); err != nil {
		return nil, err //nolint:wrapcheck // the mistakes of src
	}
	// Read once, for blockTabs, the parser and dropped: the parser changes its nodes, not the tokens' types
	// and places, which dropped reads after it.
	tks := lexer.Tokenize(string(src))
	begins := starts(src, tks)
	if err := blockTabs(file, src, tks); err != nil {
		return nil, err
	}
	doc, err := parser.Parse(tks, 0)
	if err != nil {
		// LIMIT: goccy/go-yaml's own mistake keeps its place, which drifts as starts tells: its words hold places
		// it counts (already defined at [3:1]), which the place would be counted otherwise than.
		return nil, library(file, err)
	}
	// After the parser, which reads goccy/go-yaml's columns to tell the indentation.
	if begins != nil {
		place(src, tks, begins)
	}
	var body ast.Node
	var head *Pos // a directive heads the document after it
	for _, d := range doc.Docs {
		switch b := d.Body.(type) {
		case *ast.DirectiveNode:
			at := startOf(d, Of(b))
			head = &at
			continue
		case nil:
			// An empty document: --- at the end, or one goccy/go-yaml left empty after a second ---,
			// dropping what followed.
			if d.Start != nil {
				if at, ok := dropped(tks, d.Start.Position); ok {
					return nil, New(file, at, "yaml_documents")
				}
			}
			continue
		}
		if body != nil {
			at := startOf(d, Of(d.Body))
			if head != nil {
				at = *head
			}
			return nil, New(file, at, "yaml_documents")
		}
		body, head = d.Body, nil
	}
	if body == nil {
		return nil, ErrEmpty
	}
	return body, nil
}

// blockTabs refuses each | or > text of tks without an indentation indicator whose first line not of spaces
// alone begins with spaces and a tab, a tab alone too: YAML 1.2 indents with spaces alone, the tab being text,
// but goccy/go-yaml counts it into the indentation, reading the lines after it a space short and dropping
// those it then reads as comments, or failing at one of them. A text it reads right (of one line) is refused
// alike, as the mending (|2) is one. A tab with no space before it goccy/go-yaml refuses itself.
// It runs before the parser, whose mistake past such a text is the misreading's, at another line.
// LIMIT: past a text refused, goccy/go-yaml may read a | in what YAML 1.2 reads as its text as a header, which
// is refused too; read it otherwise if that comes to mislead.
// LIMIT: a header goccy/go-yaml places off (after a tab before it, a quoted text over CRLF lines) is not judged.
// LIMIT: At counts each header from src's start; go on from the one before if a file of many comes to be slow.
func blockTabs(file string, src []byte, tks token.Tokens) error {
	var found usererr.List
	for _, t := range tks {
		if t.Type != token.LiteralType && t.Type != token.FoldedType || strings.ContainsAny(t.Value, "123456789") {
			continue
		}
		// Where goccy/go-yaml's line and column put the header: its Offset counts runes, and drifts more.
		at := textpos.At(src, t.Position.Line, t.Position.Column)
		if at < 0 || !bytes.HasPrefix(src[at:], []byte(t.Value)) {
			continue
		}
		for line := textpos.NextLine(src, at); line < len(src); line = textpos.NextLine(src, line) {
			spaces := len(src[line:]) - len(bytes.TrimLeft(src[line:], " "))
			end := line + spaces
			if end == len(src) || src[end] == '\n' || src[end] == '\r' {
				continue
			}
			if spaces > 0 && src[end] == '\t' {
				l, c := textpos.Of(src, end)
				found = append(found, usererr.NewAt(file, l, c, "yaml_block_tab"))
			}
			break
		}
	}
	return found.Sorted().Err()
}

// starts returns the byte of src each of tks begins at, or nil when the text of one is not there: goccy/go-yaml's
// own lines and columns drift (a tab it does not count, the spaces it drops after a plain value, a CRLF in a
// comment or in quotes counted twice). A token's Origin is the spaces and line breaks before it and the text it
// read; nothing else is between tokens but the spaces and tabs before a line break that goccy/go-yaml leaves out
// of a plain value's Origin, which are passed only where the text goes on otherwise. A token without text (an
// empty block) begins where the next one does, as goccy/go-yaml places it too.
// LIMIT: goccy/go-yaml keeps no hex digits of \x, \u and \U in double quotes in Origin: a file of one keeps
// goccy/go-yaml's places.
func starts(src []byte, tks token.Tokens) []int {
	begins := make([]int, len(tks))
	c := 0
	for i, t := range tks {
		for c < len(src) && strings.IndexByte(" \t\r\n", src[c]) >= 0 {
			c++
		}
		begins[i] = c
		text := strings.TrimLeft(t.Origin, " \t\r\n")
		for j := 0; j < len(text); {
			if c < len(src) && src[c] == text[j] {
				c, j = c+1, j+1
				continue
			}
			end := len(src) - len(bytes.TrimLeft(src[c:], " \t"))
			if end == c || end < len(src) && src[end] != '\n' && src[end] != '\r' {
				return nil
			}
			c = end
		}
	}
	return begins
}

// place writes over the line and column of each of tks at the byte of src it begins at.
// LIMIT: a token the parser makes, copying a place (a null left out, the value of a tag), keeps goccy/go-yaml's
// place; OfValue tells a null left out at its key, and floe refuses tags.
func place(src []byte, tks token.Tokens, begins []int) {
	for i, p := range textpos.Each(src, begins) {
		tks[i].Position.Line, tks[i].Position.Column = p.Line, p.Column
	}
}

// startOf returns where d starts: at its ---, or at at without one.
func startOf(d *ast.DocumentNode, at Pos) Pos {
	if d.Start != nil {
		return Pos{Line: d.Start.Position.Line, Column: d.Start.Position.Column}
	}
	return at
}

// notUTF8 refuses src, file, at its first byte that is no UTF-8: once, as the one way to mend it is to save the
// file as UTF-8 (a file of Shift_JIS has hundreds). A U+FFFD written in UTF-8 is a character.
func notUTF8(file string, src []byte) error {
	if utf8.Valid(src) {
		return nil
	}
	for at := 0; at < len(src); {
		r, size := utf8.DecodeRune(src[at:])
		if r == utf8.RuneError && size == 1 {
			line, column := textpos.Of(src, at)
			return usererr.NewAt(file, line, column, "yaml_not_utf8")
		}
		at += size
	}
	return nil
}

// dropped finds text after the --- at start, which goccy/go-yaml reads as an empty document and
// loses: a second --- with text after it. ok is false when only comments and ... follow.
func dropped(tks token.Tokens, start *token.Position) (Pos, bool) {
	for _, t := range tks {
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

// walk goes through a body once and notes what floe refuses in it.
type walk struct {
	file   string
	found  usererr.List
	failed error // floe's failure, which stops the walk's mistakes from being told
}

// wrapper refuses n when it is an anchor, alias or tag, and reports it: what it holds is not walked, to be
// judged once written out.
func (w *walk) wrapper(n ast.Node) bool {
	switch n.(type) {
	case *ast.AnchorNode, *ast.AliasNode, *ast.TagNode:
		w.found = append(w.found, New(w.file, Of(n), "yaml_no_anchors"))
		return true
	}
	return false
}

func (w *walk) node(n ast.Node) {
	if w.wrapper(n) {
		return
	}
	switch n := n.(type) {
	case *ast.MappingNode:
		for _, v := range n.Values {
			w.pair(v, n.IsFlowStyle)
		}
	case *ast.SequenceNode:
		for _, v := range n.Values {
			w.node(v)
		}
	case *ast.IntegerNode, *ast.FloatNode, *ast.InfinityNode, *ast.NanNode, *ast.StringNode:
		// A number refused may have been cut short too: quoting it as told would drop the comment unseen.
		refused := w.number(n)
		if _, isText := n.(*ast.StringNode); isText || refused {
			w.cut(n)
		}
	}
}

// number refuses n, a plain scalar a number can start, and reports it, when YAML 1.2 reads it otherwise than
// goccy/go-yaml, or as a number floe does not take.
func (w *walk) number(n ast.Node) bool {
	want, ok := plainNumber(n)
	if !ok {
		return false
	}
	err := numberMistake(w.file, n, want, goccy(n))
	if failed := w.found.Add(err); failed != nil {
		w.failed = cmp.Or(w.failed, failed)
	}
	return err != nil
}

// pair walks a key and its value. A merge key is refused with its value, which is not walked. A key is a
// name: one that is not text (0x10, ~, 1.0), or that YAML 1.2 reads as a number (1e3, 08), would be read as
// another name than the one written. It is judged as a key alone, and for a comment that cut it short; a key
// of a { } written without a ":" after it, which YAML gives an empty value ({ a, b: 1 }), unless refused
// already, cut short too.
func (w *walk) pair(v *ast.MappingValueNode, flow bool) {
	if _, merge := v.Key.(*ast.MergeKeyNode); merge {
		w.found = append(w.found, New(w.file, Of(v.Key), "yaml_no_anchors"))
		return
	}
	key := KeyOf(v)
	text, isText := key.(*ast.StringNode)
	switch {
	case w.wrapper(key):
	case !isText || number12(text):
		w.found = append(w.found, New(w.file, Of(key), "yaml_key_not_text", key.GetToken().Value))
	case w.cut(text):
	default:
		if flow && v.Start.Type != token.MappingValueType {
			w.found = append(w.found, New(w.file, Of(key), "yaml_flow_key_alone", Head(key.GetToken().Value)))
		}
	}
	w.node(v.Value)
}

// cut refuses n, a text, and reports it, when a comment on its last line may have cut it short. YAML reads
// " #" in a plain value as the start of a comment, and the value as what is before it, which no later check
// sees is short. The comment may be the author's, which floe cannot tell from text cut short, so either is
// refused. A comment after a quoted value cuts nothing. The caller gives it text, and a number it refused,
// not a number, true or ~ it takes, which the text cut short would not read as: an item that takes text
// refuses such a number by its type, and one that takes a number gets one.
// LIMIT: text cut to a number both read alike (title: 2024 #1) passes; catch it if a schema takes such text.
// LIMIT: a , in [ ] ends a plain text too, [1,500 words] being two items, which is not caught: either half
// is an item as right as any. Nor is a , in { } before a rest with ": " in it ({ label: 1,500 words: many }
// reads a key 500 words of the value many). Catch them if a schema comes to take such text.
// LIMIT: a line of a plain value over lines that begins with # is a comment, and the value ends before it
// (title: Plan, then "  #1 draft"), which is not caught: the comment is not after the value's text.
func (w *walk) cut(n ast.Node) bool {
	tk := n.GetToken()
	if QuoteOf(n) != Plain || tk.Next == nil || tk.Next.Type != token.CommentType ||
		tk.Next.Position.Line != lastLine(tk) {
		return false
	}
	c := tk.Next.Position
	w.found = append(w.found, usererr.NewAt(w.file, c.Line, c.Column, "yaml_comment_cut", endOf(tk.Value)))
	return true
}

// lastLine returns the line the text of tk ends on, which a plain value written over lines does after
// the line it begins on.
// LIMIT: a CR alone ending a line is not counted, so a comment cutting such a value is not caught; count the
// lines as textpos does if one comes to matter.
func lastLine(tk *token.Token) int {
	return tk.Position.Line + strings.Count(strings.TrimSpace(tk.Origin), "\n")
}

// shownEnd is how many characters of a value cut short its mistake shows: its end, where the cut is.
const shownEnd = 40

// Head returns the head of text, such as the rest of text a , may have ended, to show in a mistake on one line:
// a line break as a space, and only the first shownEnd characters before "…" of a longer one.
func Head(text string) string {
	r := []rune(strings.ReplaceAll(text, "\n", " "))
	if len(r) <= shownEnd {
		return string(r)
	}
	return string(r[:shownEnd]) + "…"
}

// endOf returns the end of value, a value cut short, to show in its mistake on one line: a line break,
// which a blank line in a value over lines makes, as a space, and only the last shownEnd characters
// after "…" of a longer one.
func endOf(value string) string {
	r := []rune(strings.ReplaceAll(value, "\n", " "))
	if len(r) <= shownEnd {
		return string(r)
	}
	return "…" + string(r[len(r)-shownEnd:])
}
