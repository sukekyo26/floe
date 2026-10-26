package schema

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/goccy/go-yaml/ast"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Validate checks the values of d against c, the keys floe reads itself (doc.Shared) left out: each leaf of the
// library's ValidationError in its words, at the line and column of its InstanceLocation in d.Root. The
// mistakes are returned Sorted, so that their order does not follow the library's map order.
func (c *Compiled) Validate(d *doc.Data) error {
	values := maps.Clone(d.Values)
	for _, key := range doc.Shared {
		delete(values, key)
	}
	err := c.Root.Validate(values)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return fmt.Errorf("schema: validate %s: %w", d.File, err)
	}
	var found usererr.List
	leaves(ve, func(e *jsonschema.ValidationError) {
		if names, ok := e.ErrorKind.(*kind.AdditionalProperties); ok {
			// The library lists them in the order of a map.
			slices.Sort(names.Properties)
		}
		at := placeOf(d, e)
		found = append(found, usererr.LibraryAt(d.File, at.Line, at.Column, e.ErrorKind.LocalizedString(english)))
	})
	return found.Sorted().Err()
}

// placeOf returns where the mistake e is written in d: the value at its InstanceLocation, or for a property
// name, as placeOfName finds it; the top of the file without a node.
func placeOf(d *doc.Data, e *jsonschema.ValidationError) yamlpos.Pos {
	if d.Root == nil {
		return yamlpos.Pos{Line: 1, Column: 1}
	}
	if pn, ok := e.ErrorKind.(*kind.PropertyNames); ok {
		return placeOfName(d.Root, len(e.InstanceLocation), pn.Property)
	}
	return yamlpos.OfValue(yamlpos.Find(d.Root, e.InstanceLocation))
}

// placeOfName returns where a property name the schema refuses is: the key name of the one object at depth
// in root that has it, or the top of the file when there are none or more. The keys every data file shares are not
// values the schema checks.
// LIMIT: jsonschema v6.0.3 shares the slice of InstanceLocation with later checks, which change what it holds
// (upstream #276, #269): only its length is right. Go back to the library's place once it is fixed; the
// mistakes told at the top of the file then move, and the golden files change.
// LIMIT: an object the same propertyNames applies to twice (allOf of one $defs twice) is told twice, as
// usererr does not yet tell a mistake once.
func placeOfName(root *ast.MappingNode, depth int, name string) yamlpos.Pos {
	var found []*ast.MappingValueNode
	var walk func(n ast.Node, depth int, top bool)
	walk = func(n ast.Node, depth int, top bool) {
		switch n := n.(type) {
		case *ast.MappingNode:
			for _, v := range n.Values {
				key := yamlpos.KeyOf(v).GetToken().Value
				switch {
				case top && slices.Contains(doc.Shared, key):
				case depth == 0 && key == name:
					found = append(found, v)
				case depth > 0:
					walk(v.Value, depth-1, false)
				}
			}
		case *ast.SequenceNode:
			if depth > 0 {
				for _, v := range n.Values {
					walk(v, depth-1, false)
				}
			}
		}
	}
	walk(root, depth, true)
	if len(found) == 1 {
		return yamlpos.Of(yamlpos.KeyOf(found[0]))
	}
	return yamlpos.Pos{Line: 1, Column: 1}
}
