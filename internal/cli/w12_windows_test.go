package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/cli"
)

// What Windows does where floe reads a path under a file as not there (W12, files.NotDir's LIMIT): the answers of
// now, each the twin of a test that is Unix only, so that mending one of them is noticed here.
func TestW12OnWindows(t *testing.T) {
	t.Parallel()

	t.Run("an icons setting under a file", func(t *testing.T) {
		t.Parallel()
		under := filepath.Join(fileIn(t), "i")
		code, _, stderr := runIn("", cli.HomeEnv(t.TempDir(), map[string]string{"FLOE_ICONS_DIR": under}), "icons", "search", "aws", "x")
		if code != 1 || !strings.HasPrefix(stderr, "floe: AWS icons are not in "+filepath.Join(under, "aws")+";") {
			t.Errorf("floe icons search under a file = %d, %q, want 1 and icons_not_fetched", code, stderr)
		}
	})
}
