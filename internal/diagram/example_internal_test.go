package diagram

import (
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

// Of an example's mistakes, those of icons are the user's, at the diagram's root; any other is floe's failure.
func TestExampleMistakesSplit(t *testing.T) {
	t.Parallel()

	unknown := usererr.NewIn("", "#/parts/3/icon", "diagram_icon_unknown", "icon", "aws/x")
	mine, err := exampleMistakes("aws", usererr.List{unknown, usererr.New("icons_no_dir")})
	if err != nil || len(mine) != 2 || mine[0].At != "#" || mine[1].At != "#" {
		t.Errorf("exampleMistakes = %v, %v", mine, err)
	}
	layout := usererr.List{unknown, usererr.NewIn("", "#/parts/1", "diagram_overlap", "a", "b")}
	var found usererr.List
	kept, err := exampleMistakes("aws", layout)
	if kept != nil || err == nil || found.Add(err) == nil {
		t.Errorf("a mistake of the example's layout: %v, want floe's failure", err)
	}
}
