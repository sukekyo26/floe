package dirs_test

import (
	"testing"

	"floe/internal/dirs"
	"floe/internal/testfs"
)

// The cases written with / for the separator and /w for an absolute path are Unix's: Windows takes neither.
const (
	underPending = "S315-P2: Under takes / for no separator and /w for no absolute path"
	dirPending   = "S315-P2: Dir and Join take / for no separator"
)

func TestUnder(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, cwd, path, abs string
		ok                   bool
		pending              string
	}{
		{"relative", "/w", "a/../b.yaml", "/w/a/../b.yaml", true, underPending},
		{"cwd ends in /", "/w/", "b.yaml", "/w/b.yaml", true, underPending},
		{"absolute", "/w", "/x//y.yaml", "/x//y.yaml", true, underPending},
		{"relative, no cwd", "", "a.yaml", "", false, ""},
		{"absolute, no cwd", "", "/a.yaml", "/a.yaml", true, underPending},
		{"empty", "/w", "", "", true, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				testfs.Pending(t, tt.pending)
			}
			if abs, ok := dirs.Under(tt.cwd, tt.path); abs != tt.abs || ok != tt.ok {
				t.Errorf("Under(%q, %q) = %q, %v, want %q, %v", tt.cwd, tt.path, abs, ok, tt.abs, tt.ok)
			}
		})
	}
}

func TestDirJoin(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ path, want, pending string }{
		{"a.yaml", ".", ""},
		{"link/../a.yaml", "link/..", dirPending},
		{"/a.yaml", "/", dirPending},
		{"/w/a.yaml", "/w", dirPending},
	} {
		t.Run("Dir "+tt.path, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				testfs.Pending(t, tt.pending)
			}
			if got := dirs.Dir(tt.path); got != tt.want {
				t.Errorf("Dir(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
	for _, tt := range []struct{ dir, name, want, pending string }{
		{".", "a.html", "a.html", ""},
		{"link/..", "a.html", "link/../a.html", dirPending},
		{"/", "a.html", "/a.html", dirPending},
	} {
		t.Run("Join "+tt.dir, func(t *testing.T) {
			t.Parallel()
			if tt.pending != "" {
				testfs.Pending(t, tt.pending)
			}
			if got := dirs.Join(tt.dir, tt.name); got != tt.want {
				t.Errorf("Join(%q, %q) = %q, want %q", tt.dir, tt.name, got, tt.want)
			}
		})
	}
}
