package yamlpos

import (
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
)

// Place returns err with each mistake of no file at a JSON pointer (usererr.Error.At) placed in file instead: at
// the line and column of the value the pointer leads to from prefix under root, its pointer dropped. A mistake
// with a file of its own, or with no pointer (of the whole output), is kept as it is; a failure is returned as
// it is.
func Place(err error, file string, root ast.Node, prefix []string) error {
	var found usererr.List
	if failed := found.Add(err); failed != nil {
		return failed
	}
	for i, e := range found {
		if e.File != "" || e.At == "" {
			continue
		}
		at := OfValue(Find(root, slices.Concat(prefix, steps(e.At))))
		placed := *e
		placed.File, placed.Line, placed.Column, placed.At = file, at.Line, at.Column, ""
		found[i] = &placed
	}
	return found.Err()
}

// steps are the keys and indexes of the JSON pointer at ("#/a/0"), unescaped as RFC 6901 says: ~1 then ~0.
func steps(at string) []string {
	rest, ok := strings.CutPrefix(at, "#/")
	if !ok {
		return nil
	}
	parts := strings.Split(rest, "/")
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}
	return parts
}
