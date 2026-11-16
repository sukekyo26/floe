package dirs_test

import (
	"errors"
	"testing"

	"floe/internal/dirs"
)

// underCase is a path given and where Under puts it, or why not; the cases of each system are in its own file.
type underCase struct {
	name, cwd, path, abs string
	err                  error
}

// dirCase is a path and its directory as Dir gives it; joinCase a directory, a name and what Join makes of them.
type (
	dirCase  struct{ path, want string }
	joinCase struct{ dir, name, want string }
)

func TestUnder(t *testing.T) {
	t.Parallel()

	for _, tt := range append(underCases, underCase{"empty", underCases[0].cwd, "", "", nil}) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if abs, err := dirs.Under(tt.cwd, tt.path); abs != tt.abs || !errors.Is(err, tt.err) || (err == nil) != (tt.err == nil) {
				t.Errorf("Under(%q, %q) = %q, %v, want %q, %v", tt.cwd, tt.path, abs, err, tt.abs, tt.err)
			}
		})
	}
}

func TestDirJoin(t *testing.T) {
	t.Parallel()

	for _, tt := range append(dirCases, dirCase{"a.yaml", "."}) {
		t.Run("Dir "+tt.path, func(t *testing.T) {
			t.Parallel()
			if got := dirs.Dir(tt.path); got != tt.want {
				t.Errorf("Dir(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
	for _, tt := range append(joinCases, joinCase{".", "a.html", "a.html"}) {
		t.Run("Join "+tt.dir, func(t *testing.T) {
			t.Parallel()
			if got := dirs.Join(tt.dir, tt.name); got != tt.want {
				t.Errorf("Join(%q, %q) = %q, want %q", tt.dir, tt.name, got, tt.want)
			}
		})
	}
}
