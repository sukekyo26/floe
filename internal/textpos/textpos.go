// Package textpos turns a place in a file into its line and column, as an editor counts them: a byte of it, or
// where text/template's words say it failed.
package textpos

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Of returns the line and the column of the byte at of src, both from 1: a LF, a CRLF and a CR each end a
// line, and the column counts the runes before it in its line, a byte that is no UTF-8 as one.
// LIMIT: a tab is one column, and U+2028, U+2029 and FF end no line.
func Of(src []byte, at int) (line, column int) {
	line, start := 1, 0
	for i := range at {
		if src[i] == '\n' || src[i] == '\r' && (i+1 >= len(src) || src[i+1] != '\n') {
			line, start = line+1, i+1
		}
	}
	return line, utf8.RuneCount(src[start:at]) + 1
}

// Ending returns the line and the column of the character that ends at the byte offset of src: the place
// encoding/json names (a SyntaxError's or an UnmarshalTypeError's Offset), which may fall inside a character,
// read as that character's start. An offset of 0 is the start of src.
func Ending(src []byte, offset int) (line, column int) {
	_, size := utf8.DecodeLastRune(src[:offset])
	return Of(src, offset-size)
}

// OfTemplate returns where the head of msg, text/template's words of src read as name, says it failed, and msg
// without the head: "template: name:line:column: " of an execution, at line and column as Of counts them, and
// "template: name:line: " of a parse, at the line TemplateLine says and no column (0). The head's column is in
// bytes from 0, a byte of src (its end is none). line is 0 when the head says no place in src; without such a
// head (another name, no numbers, a line of 0, a column below 0), line is 0 and rest msg.
// LIMIT: text/template gives where it failed in its words alone; take it from the error once it gives it apart.
func OfTemplate(src []byte, msg, name string) (line, column int, rest string) {
	after, ok := strings.CutPrefix(msg, "template: "+name+":")
	if !ok {
		return 0, 0, msg
	}
	lineText, after, ok := strings.Cut(after, ":")
	at, err := strconv.Atoi(lineText)
	if !ok || err != nil || at < 1 {
		return 0, 0, msg
	}
	// A parse's words follow ": " at once; an execution's, its column.
	if words, parse := strings.CutPrefix(after, " "); parse {
		return TemplateLine(src, at), 0, words
	}
	columnText, words, ok := strings.Cut(after, ": ")
	bytesIn, err := strconv.Atoi(columnText)
	if !ok || err != nil || bytesIn < 0 {
		return 0, 0, msg
	}
	if start := lineStart(src, at); start >= 0 && start+bytesIn < len(src) {
		line, column = Of(src, start+bytesIn)
	}
	return line, column, words
}

// TemplateLine returns the line, as Of counts it, of the start of line of src, a line as text/template counts
// them (\n alone); 0 when src has no such line. The end of src is a place in it, after its last line break,
// when src is not empty (text/template's unexpected EOF).
// LIMIT: a CR alone ending a line inside line is not seen: the line its start is on is told, as text/template
// gives no column to tell more.
func TemplateLine(src []byte, line int) int {
	start := lineStart(src, line)
	if line < 1 || start < 0 || len(src) == 0 {
		return 0
	}
	at, _ := Of(src, start)
	return at
}

// lineStart returns the byte of src that starts line, counting \n alone; -1 for none.
func lineStart(src []byte, line int) int {
	start := 0
	for range line - 1 {
		i := bytes.IndexByte(src[start:], '\n')
		if i < 0 {
			return -1
		}
		start += i + 1
	}
	return start
}
