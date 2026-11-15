package schema

import (
	"reflect"
	"slices"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// holds reports whether a value of t can hold a *jsonschema.Schema.
func holds(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == reflect.TypeFor[*jsonschema.Schema]() {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Interface:
		return true
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return holds(t.Elem(), seen)
	case reflect.Struct:
		for i := range t.NumField() {
			if t.Field(i).IsExported() && holds(t.Field(i).Type, seen) {
				return true
			}
		}
	default:
	}
	return false
}

// every goes into each exported field of a Schema that can hold one: the list here is what jsonschema
// v6.0.3 has, so that a field a new version adds is looked at (and this list made anew) before it is missed.
func TestEvery(t *testing.T) {
	t.Parallel()

	var got []string
	st := reflect.TypeFor[jsonschema.Schema]()
	for i := range st.NumField() {
		f := st.Field(i)
		if f.IsExported() && holds(f.Type, map[reflect.Type]bool{st: true}) {
			got = append(got, f.Name)
		}
	}
	want := []string{
		"Ref", "RecursiveRef", "DynamicRef", "Enum", "Const", "Not", "AllOf", "AnyOf", "OneOf", "If", "Then", "Else",
		"Properties", "PatternProperties", "AdditionalProperties", "PropertyNames", "Dependencies",
		"DependentSchemas", "UnevaluatedProperties", "Items", "AdditionalItems", "PrefixItems", "Items2020",
		"Contains", "UnevaluatedItems", "Pattern", "ContentSchema", "Default", "Examples", "Extensions",
	}
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))) {
		t.Errorf("the fields of Schema that hold one = %v", got)
	}
}
