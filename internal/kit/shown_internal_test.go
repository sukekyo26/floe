package kit

import (
	"path/filepath"
	"testing"
)

// A path of one's own (absolute, on the disk) is shown in the system's separators alone; a name of floe's embedded
// files (relative) with /, on every system.
func TestUnderShown(t *testing.T) {
	t.Parallel()

	own := filepath.Join(t.TempDir(), "parts")
	for _, tt := range []struct{ dir, want string }{
		{own, filepath.Join(own, "mine", "part.css")},
		{"parts", "parts/mine/part.css"},
	} {
		if got := under(tt.dir, "mine", "part.css"); got != tt.want {
			t.Errorf("under(%q) = %q, want %q", tt.dir, got, tt.want)
		}
		if got := (Dir{FS: nil, Name: tt.dir, read: nil}).path("mine/part.css"); got != tt.want {
			t.Errorf("path of %q = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

// A part.html's mistake is placed at its line whether its file is one's own (\ on Windows) or floe's (/): the
// template is named by the file's base either way, as text/template's head says it.
func TestParsePagesPlaced(t *testing.T) {
	t.Parallel()

	for _, file := range []string{filepath.Join(t.TempDir(), "mine", "part.html"), "parts/table/part.html"} {
		_, err := parsePages(file, []byte("<p>\n{{if}}\n</p>\n"), nil, nil)
		if e, ok := err.(interface{ Line() int }); !ok || e.Line() != 2 {
			t.Errorf("parsePages(%q) = %v, want a mistake at line 2", file, err)
		}
	}
}
