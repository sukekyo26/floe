package dirs_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/dirs"
)

func TestUnder(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		cwd, path, abs string
		ok             bool
	}{
		{"/w", "a/../b.yaml", "/w/b.yaml", true},
		{"/w", "/x//y.yaml", "/x/y.yaml", true},
		{"", "a.yaml", "", false},
		{"", "/a.yaml", "/a.yaml", true},
		{"/w", "", "", true},
	} {
		if abs, ok := dirs.Under(tt.cwd, tt.path); abs != tt.abs || ok != tt.ok {
			t.Errorf("Under(%q, %q) = %q, %v, want %q, %v", tt.cwd, tt.path, abs, ok, tt.abs, tt.ok)
		}
	}
}
