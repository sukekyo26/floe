package schema

import (
	"maps"
	"slices"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ShapeFieldsMissing returns those of names shapeFields does not sort.
func ShapeFieldsMissing(names []string) []string {
	var missing []string
	for _, n := range names {
		if _, ok := shapeFields[n]; !ok {
			missing = append(missing, n)
		}
	}
	return missing
}

// FalseSchemas walks every schema c reaches, in the files it holds, and returns the Location of each boolean schema
// false there and the files it reached.
func (c *Compiled) FalseSchemas() (falses, files []string) {
	reached := map[string]bool{}
	enter := func(base string) bool {
		_, held := c.res[base]
		return held || base == c.url
	}
	c.every(c.Root, enter, func(n *jsonschema.Schema) {
		base, _ := split(n.Location)
		reached[base] = true
		if n.Bool != nil && !*n.Bool {
			falses = append(falses, n.Location)
		}
	}, func(*jsonschema.Schema, *jsonschema.Schema) {})
	return falses, slices.Sorted(maps.Keys(reached))
}
