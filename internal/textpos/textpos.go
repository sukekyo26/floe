// Package textpos turns a byte of a file into its line and column, as an editor counts them.
package textpos

import "unicode/utf8"

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
