// Package yamlpos reads a YAML file as one document and points at where something was written in it: the
// line and column of a node, and the mistakes there.
package yamlpos

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"

	"floe/internal/bom"
	"floe/internal/textpos"
	"floe/internal/usererr"
)

// Pos is a line and column of a YAML file, both from 1.
type Pos struct {
	Line, Column int
}

// Style is how a scalar is written, as the parser read it.
type Style int

const (
	Plain   Style = iota // not quoted, or not a scalar
	Double               // in "…"
	Single               // in '…'
	Literal              // a | text
	Folded               // a > text
)

// styleOf returns how n, a scalar, is written.
func styleOf(n *yaml.Node) Style {
	switch {
	case n.Style&yaml.DoubleQuotedStyle != 0:
		return Double
	case n.Style&yaml.SingleQuotedStyle != 0:
		return Single
	case n.Style&yaml.LiteralStyle != 0:
		return Literal
	case n.Style&yaml.FoldedStyle != 0:
		return Folded
	}
	return Plain
}

// unwritten reports whether n is a value with nothing written: go.yaml.in/yaml/v4 gives it as a plain null, empty.
func unwritten(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Style == 0 && n.Value == "" && n.Tag == "!!null" && n.Anchor == ""
}

// at returns where n is written.
func at(n *yaml.Node) Pos {
	return Pos{Line: n.Line, Column: n.Column}
}

// New returns the mistake worded by the catalog message key with args, at at in file.
func New(file string, at Pos, key string, args ...any) *usererr.Error {
	return usererr.NewAt(file, at.Line, at.Column, key, args...)
}

// library returns the mistake go.yaml.in/yaml/v4 told in err, its words as they are (its Message, without the
// context it adds before them), at its place: its line and column, or, where it gives none (the mistakes of its
// reader, a control character), its byte in src, the bytes it read. A mistake on a line it gives, right after a |
// or > text (afterBlock), is told with how to mend the text's indentation after its words: they tell what the line
// holds, not that the text ended before it. Any other error is returned as it is: floe's failure.
func library(file string, src []byte, err error) error {
	var le *yaml.LoadError
	if !errors.As(err, &le) {
		return fmt.Errorf("yamlpos: read YAML: %w", err)
	}
	line, column := le.Mark.Line, le.Mark.Column
	if line == 0 {
		line, column = textpos.Of(src, min(max(le.Mark.Index, 0), len(src)))
		return usererr.LibraryAt(file, line, column, le.Message)
	}
	if afterBlock(src, line) {
		return usererr.LibraryAtWith(file, line, column, "yaml_after_block", le.Message)
	}
	return usererr.LibraryAt(file, line, column, le.Message)
}

// afterBlock is whether a | or > text ends right before line of src, a line go.yaml.in/yaml/v4 told a mistake on:
// whether what is written last in src before the line, read alone, is such a text. Such a text ends only before a
// line indented less than it, so a line of it written less deep than its first is read as YAML out of the text.
// No indentation is counted here.
// LIMIT: a mistake told on a later line, where src before that line is not read either, passes: a shallower line
// read as the start of what goes on over lines (one at no indentation that is no key, one starting with " or ').
// So does one after a | or > text that is a key (? |).
// LIMIT: the words added tell to indent the text's lines alike or to write its indentation, which does not mend a
// first line starting with a tab (go.yaml.in/yaml/v4's words tell the tab), nor a text whose indentation is
// written and too deep for its lines (a: |4 over three spaces).
func afterBlock(src []byte, line int) bool {
	start := textpos.At(src, line, 1)
	if start < 0 {
		return false
	}
	l, err := yaml.NewLoader(bytes.NewReader(src[:start]))
	if err != nil {
		return false
	}
	var last *yaml.Node
	for {
		var doc yaml.Node
		err := l.Load(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false
		}
		last = &doc
	}
	if last == nil {
		return false
	}
	n := lastNode(last)
	return n.Kind == yaml.ScalarNode && (styleOf(n) == Literal || styleOf(n) == Folded)
}

// lastNode is what is written last under n: n itself when it holds nothing, an empty [] or {} and an alias too.
func lastNode(n *yaml.Node) *yaml.Node {
	if len(n.Content) == 0 {
		return n
	}
	return lastNode(n.Content[len(n.Content)-1])
}

// ErrEmpty is returned by Parse for a file without a document, for the caller to word.
var ErrEmpty = errors.New("no YAML document")

// ParseMap reads src as Parse does, and its values as Decode does, refusing a top that is not keys and values at
// its start: the files floe reads are keys and values. A file without a document reads as {} under a root of nil,
// for the schema to tell what it lacks.
func ParseMap(file string, src []byte) (*Node, map[string]any, error) {
	body, err := Parse(file, src)
	switch {
	case errors.Is(err, ErrEmpty):
		return nil, map[string]any{}, nil
	case err != nil:
		return nil, nil, err
	case body.Kind != Mapping:
		return nil, nil, New(file, Of(body), "yaml_top_not_keys")
	}
	values, _ := Decode(body).(map[string]any) //nolint:errcheck // a mapping, just above
	return body, values, nil
}

// Parse reads src as one YAML document with go.yaml.in/yaml/v4 and returns its body, or ErrEmpty when src holds
// none. A byte order mark at its start is left out, as YAML 1.2 reads it. Before it is read, a file over 1 MiB is
// refused alone, else a byte order mark past the start, a byte that is no UTF-8 and U+0085, U+2028 and U+2029
// (bytesOf). A syntax error, and a nesting past go.yaml.in/yaml/v4's depth, is its own (library). A second document
// is refused at its start.
// Else walks of the body refuse, all together, sorted: anchors, aliases, merge keys and tags, not walking what
// they hold; a key that is not text as YAML 1.2 reads it, and a key written twice in one mapping; a key in { } or
// [ ] without a value; a plain value YAML 1.2 reads as a number JSON has none for (.inf, .nan) or floe cannot hold;
// and a comment that may have cut a value short (comments).
func Parse(file string, src []byte) (*Node, error) {
	doc, err := document(file, src)
	if err != nil {
		return nil, err
	}
	body := doc.Content[0]
	w := &walk{file: file, src: bom.Trim(src), found: nil}
	w.node(body)
	w.comments(doc)
	if err := w.found.Sorted().Err(); err != nil {
		return nil, err //nolint:wrapcheck // the mistakes of src
	}
	return newNode(body), nil
}

// document returns the one document of src (its body is its Content[0]), src held first to the bytes floe reads
// (bytesOf). A document with nothing written (--- at the end, or two in a row) is no document, as YAML 1.2 reads it
// as null, which floe has no use for: a second is told only when it holds something. A directive (%YAML 1.1) heads
// the document after it.
// LIMIT: a surrogate escaped in double quotes, lone or of a pair ("\udc00", "\ud83d\ude00"), go.yaml.in/yaml/v4
// refuses in its words, which do not say to write the character itself; word it if one comes to matter.
func document(file string, src []byte) (*yaml.Node, error) {
	src, err := bytesOf(file, src)
	if err != nil {
		return nil, err
	}
	l, err := yaml.NewLoader(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("yamlpos: make a YAML loader: %w", err)
	}
	var body *yaml.Node
	for {
		var doc yaml.Node
		err := l.Load(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, library(file, src, err)
		}
		if len(doc.Content) == 0 || unwritten(doc.Content[0]) {
			continue
		}
		if body != nil {
			return nil, New(file, at(&doc), "yaml_documents")
		}
		body = &doc
	}
	if body == nil {
		return nil, ErrEmpty
	}
	return body, nil
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

// maxSize is the size of a YAML file floe reads, as the bytes of the file.
// LIMIT: 1 MiB, past the largest page's data seen (326 KB), and the file is read whole before it is refused:
// the memory goes as the file's size. Refuse it where it is read if reading a very large file comes to matter.
const maxSize = 1 << 20

// bytesOf returns src, file, without the byte order mark at its start, which YAML 1.2 allows: go.yaml.in/yaml/v4,
// textpos and floe's checks all read src so, so that their places agree.
// A file over maxSize is refused alone, at the file, before anything reads it. Else bytesOf refuses, together,
// each byte order mark past the start, and either the first byte that is no UTF-8 or, in UTF-8 alone, each
// U+0085, U+2028 and U+2029. A mark past the start would be read as a character of a key or a value.
// go.yaml.in/yaml/v4 reads UTF-16 with a mark without a mistake, which floe's checks of bytes and places, all of
// UTF-8, would miss: a byte that is no UTF-8 is refused before it reads.
// The three characters are looked for only in UTF-8: in another encoding (Shift_JIS) their bytes come inside
// other characters, and would be told where no such character is.
func bytesOf(file string, src []byte) ([]byte, error) {
	if len(src) > maxSize {
		return nil, New(file, Pos{Line: 0, Column: 0}, "yaml_too_large", len(src), maxSize)
	}
	src = bom.Trim(src)
	read := notUTF8(file, src)
	if read == nil {
		read = lineSeparators(file, src).Err()
	}
	return src, usererr.Collect(bom.Refuse(file, src).Err(), read) //nolint:wrapcheck // the mistakes of src
}

// lineSeparators refuses each U+0085, U+2028 and U+2029 of src, file, valid UTF-8, at its line and column, in
// quotes and comments too: YAML 1.2 reads them as characters, but go.yaml.in/yaml/v4 as line breaks, ending a
// value or a comment there. An escape in double quotes ("\u2028") is no such character.
func lineSeparators(file string, src []byte) usererr.List {
	var ats []int
	for at := 0; ; {
		i := bytes.IndexAny(src[at:], "\u0085\u2028\u2029")
		if i < 0 {
			break
		}
		ats = append(ats, at+i)
		_, size := utf8.DecodeRune(src[at+i:])
		at += i + size
	}
	found := make(usererr.List, 0, len(ats))
	for k, p := range textpos.Each(src, ats) {
		r, _ := utf8.DecodeRune(src[ats[k]:])
		found = append(found, usererr.NewAt(file, p.Line, p.Column, "yaml_line_separator",
			fmt.Sprintf("U+%04X", r), fmt.Sprintf(`\u%04x`, r)))
	}
	return found
}

// walk goes through a body once and notes what floe refuses in it.
type walk struct {
	file  string
	src   []byte // the bytes go.yaml.in/yaml/v4 read, to show a key it gives no text of
	found usererr.List
}

// node walks n.
// LIMIT: a , in [ ] ends a plain text too, [1,500 words] being two items, which is not caught: either half is an item
// as right as any; nor is a plain text in [ ] or { } over lines, which YAML 1.2 reads as one text with a space
// (a: [x, then y] on the next line is "x y"), where a , was left out. Nor is a , in { } before a rest with ": "
// ({ label: 1,500 words: many } reads a key 500 words of the value many). Catch them if a schema comes to take
// such text.
func (w *walk) node(n *yaml.Node) {
	if w.wrapper(n) {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		first := map[string]*yaml.Node{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			w.pair(n.Style&yaml.FlowStyle != 0, n.Content[i], n.Content[i+1], first)
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			w.node(item)
		}
	case yaml.ScalarNode:
		w.number(n)
	case yaml.DocumentNode, yaml.AliasNode, yaml.StreamNode:
	}
}

// wrapper refuses n when it is an anchor, alias or tag, and reports it: what it holds is not walked, to be
// judged once written out. A tag of ! alone go.yaml.in/yaml/v4 gives without TaggedStyle, as the tag itself.
func (w *walk) wrapper(n *yaml.Node) bool {
	if !wrapped(n) {
		return false
	}
	w.found = append(w.found, New(w.file, at(n), "yaml_no_anchors"))
	return true
}

// wrapped is whether n is an anchor, alias or tag.
func wrapped(n *yaml.Node) bool {
	return n.Anchor != "" || n.Kind == yaml.AliasNode || n.Style&yaml.TaggedStyle != 0 || n.Tag == "!"
}

// pair walks a key and its value; first holds the keys of their mapping met before, by name. A merge key is
// refused with its value, which is not walked. A key is a name: one that is not text as YAML 1.2 reads it (0x10,
// ~, true, a list) would be read as another name than the one written, and one written twice keeps only one of
// its values, which go.yaml.in/yaml/v4 does not tell. A key of a { } with no value written ({a, b: 1}, {a: }) is
// refused: a , in text may have ended the text before it.
func (w *walk) pair(flow bool, key, value *yaml.Node, first map[string]*yaml.Node) {
	if key.Tag == "!!merge" {
		w.found = append(w.found, New(w.file, at(key), "yaml_no_anchors"))
		return
	}
	switch {
	case w.wrapper(key):
	case key.Kind != yaml.ScalarNode || styleOf(key) == Plain && read12(key.Value).kind != kindText:
		w.found = append(w.found, New(w.file, at(key), "yaml_key_not_text", w.name(key)))
	default:
		if before, twice := first[key.Value]; twice {
			w.found = append(w.found, New(w.file, at(key), "yaml_key_twice", Head(key.Value), before.Line))
		} else {
			first[key.Value] = key
		}
		if flow && unwritten(value) {
			w.found = append(w.found, New(w.file, at(key), "yaml_flow_key_alone", Head(key.Value)))
		}
	}
	w.node(value)
}

// name returns key as its mistake shows it: its text, or, for a key that has none (a list, keys and values, or
// nothing written), the head of the line it is written on, shown alone and judged by nothing.
func (w *walk) name(key *yaml.Node) string {
	if key.Value != "" {
		return Head(key.Value)
	}
	start := textpos.At(w.src, key.Line, key.Column)
	if start < 0 {
		return ""
	}
	return Head(strings.TrimRight(string(w.src[start:textpos.NextLine(w.src, start)]), "\r\n"))
}

// number refuses n, a scalar, when it is plain and YAML 1.2 reads it as a number floe does not take: a float past
// float64, one that is not finite, or an integer past int64 and uint64.
func (w *walk) number(n *yaml.Node) {
	if styleOf(n) != Plain {
		return
	}
	r := read12(n.Value)
	var key string
	switch {
	case r.outside:
		key = "yaml_number_outside_float"
	case !r.finite():
		key = "yaml_not_finite"
	case r.kind == kindInt && !r.fits():
		key = "yaml_number_too_big"
	default:
		return
	}
	w.found = append(w.found, New(w.file, at(n), key, n.Value))
}

// shownEnd is how many characters of a text its mistake shows: the head of one (Head), the end of one cut short
// (endOf).
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
