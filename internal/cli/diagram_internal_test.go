package cli

import "testing"

// A mistake at no file goes after the files given, though an empty one was given.
func TestFileAt(t *testing.T) {
	t.Parallel()

	named := []arg{{name: "", abs: "", err: nil}, {name: "a.yaml", abs: "/a.yaml", err: nil}}
	for file, want := range map[string]int{"": 2, "a.yaml": 1, "b.yaml": 2} {
		if got := fileAt(named, file); got != want {
			t.Errorf("fileAt(%q) = %d, want %d", file, got, want)
		}
	}
}
