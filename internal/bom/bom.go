// Package bom leaves out the UTF-8 byte order mark at the start of a file, as YAML 1.2 and RFC 8259 read
// it: no part of the text; and refuses every other mark, which the libraries read as a character.
package bom

import (
	"bytes"

	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
)

var utf8BOM = []byte("\xef\xbb\xbf")

// Trim returns src without the one UTF-8 byte order mark at its start, if it has one; every other byte is
// left as it is. Refuse refuses the other marks; what else a file may hold (UTF-16) is its reader's.
func Trim(src []byte) []byte {
	return bytes.TrimPrefix(src, utf8BOM)
}

// At reports whether src starts with a byte order mark.
func At(src []byte) bool {
	return bytes.HasPrefix(src, utf8BOM)
}

// Refuse refuses each byte order mark in src, file as Trim left it, at its line and column: YAML and JSON read
// one past the start as a character of a key or a value, a name that is not the one written.
// LIMIT: a mark in quotes or a comment is refused too, which YAML 1.2 takes in quotes; one a text needs is
// written as an escape in double quotes. Take them if a file comes to need marks written as they are.
func Refuse(file string, src []byte) usererr.List {
	var found usererr.List
	for at := 0; ; at += len(utf8BOM) {
		i := bytes.Index(src[at:], utf8BOM)
		if i < 0 {
			return found
		}
		at += i
		line, column := textpos.Of(src, at)
		found = append(found, usererr.NewAt(file, line, column, "text_bom"))
	}
}
