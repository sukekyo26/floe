// Package bom leaves out the UTF-8 byte order mark at the start of a file, as YAML 1.2 and RFC 8259 read
// it: no part of the text.
package bom

import "bytes"

var utf8BOM = []byte("\xef\xbb\xbf")

// Trim returns src without the one UTF-8 byte order mark at its start, if it has one; every other byte is
// left as it is. Each caller tells what it leaves (a second mark, UTF-16) in its own limit comment.
func Trim(src []byte) []byte {
	return bytes.TrimPrefix(src, utf8BOM)
}
