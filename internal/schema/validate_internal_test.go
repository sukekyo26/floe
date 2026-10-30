package schema

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// A property name refused in a Data without a node is placed at the file alone, without walking a root of nil.
func TestPlaceOfNoNode(t *testing.T) {
	t.Parallel()

	e := &jsonschema.ValidationError{ErrorKind: &kind.PropertyNames{Property: "Bad"}, InstanceLocation: []string{}}
	if got := placeOf(&doc.Data{File: "d.yaml"}, e); got != (yamlpos.Pos{}) {
		t.Errorf("placeOf = %v, want the file alone", got)
	}
}
