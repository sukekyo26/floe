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

// Refuse refuses each byte order mark in src, file as Trim left it, at its line and column. YAML and JSON read
// one past the start as a character of a key or a value, so that a name or a text is not the one it shows;
// YAML 1.2 takes it in quotes and RFC 8259 in any string, and they are refused there too, as a mark no one sees
// makes a key another name all the same (decided for floe's files, in YAML and in a schema's JSON alike).
// LIMIT: a mark written in quotes, a JSON string or a comment is refused; one a text needs is written as an
// escape in double quotes. Take them if a file comes to need marks written as they are.
func Refuse(file string, src []byte) usererr.List {
	var ats []int
	for at := 0; ; at += len(utf8BOM) {
		i := bytes.Index(src[at:], utf8BOM)
		if i < 0 {
			break
		}
		at += i
		ats = append(ats, at)
	}
	found := make(usererr.List, 0, len(ats))
	for _, p := range textpos.Each(src, ats) {
		found = append(found, usererr.NewAt(file, p.Line, p.Column, "text_bom"))
	}
	return found
}
