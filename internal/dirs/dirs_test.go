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
		{"/w", "a/../b.yaml", "/w/a/../b.yaml", true},
		{"/w/", "b.yaml", "/w/b.yaml", true},
		{"/w", "/x//y.yaml", "/x//y.yaml", true},
		{"", "a.yaml", "", false},
		{"", "/a.yaml", "/a.yaml", true},
		{"/w", "", "", true},
	} {
		if abs, ok := dirs.Under(tt.cwd, tt.path); abs != tt.abs || ok != tt.ok {
			t.Errorf("Under(%q, %q) = %q, %v, want %q, %v", tt.cwd, tt.path, abs, ok, tt.abs, tt.ok)
		}
	}
}

func TestDirJoin(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]string{"a.yaml": ".", "link/../a.yaml": "link/..", "/a.yaml": "/", "/w/a.yaml": "/w"} {
		if got := dirs.Dir(path); got != want {
			t.Errorf("Dir(%q) = %q, want %q", path, got, want)
		}
	}
	for _, tt := range [][3]string{{".", "a.html", "a.html"}, {"link/..", "a.html", "link/../a.html"}, {"/", "a.html", "/a.html"}} {
		if got := dirs.Join(tt[0], tt[1]); got != tt[2] {
			t.Errorf("Join(%q, %q) = %q, want %q", tt[0], tt[1], got, tt[2])
		}
	}
}
