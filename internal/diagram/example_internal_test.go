package diagram

import (
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

// Of an example's mistakes, those of icons are the user's: an icon's name at the diagram's root, one of the icons'
// files or of the whole output where it is; any other is floe's failure.
func TestExampleMistakesSplit(t *testing.T) {
	t.Parallel()

	unknown := usererr.NewIn("", "#/parts/3/icon", "diagram_icon_unknown", "icon", "aws/x")
	noDir := usererr.NewWhole("icons_no_dir")
	broken := usererr.NewAt("/i/aws/index.json", 1, 1, "icons_index_no_version", "aws")
	mine, err := exampleMistakes("aws", usererr.List{unknown, noDir, broken})
	if err != nil || len(mine) != 3 || mine[0].At() != "#" || mine[1] != noDir || mine[2] != broken {
		t.Errorf("exampleMistakes = %v, %v", mine, err)
	}
	layout := usererr.List{unknown, usererr.NewIn("", "#/parts/1", "diagram_overlap", "a", "b")}
	var found usererr.List
	kept, err := exampleMistakes("aws", layout)
	if kept != nil || err == nil || found.Add(err) == nil {
		t.Errorf("a mistake of the example's layout: %v, want floe's failure", err)
	}
}
