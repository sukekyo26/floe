package version_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/version"
)

// `go install` builds skip the justfile's ldflags, so the literal must match VERSION.
func TestVersionMatchesVersionFile(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimSpace(string(data)); version.Version != want {
		t.Errorf("version.Version = %q, VERSION = %q; bump both together", version.Version, want)
	}
}
