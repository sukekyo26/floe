package testfs

import "testing"

// UnixOnly and Pending skip on Windows, and on Unix let the test run on.
func TestMarks(t *testing.T) {
	t.Parallel()

	for name, mark := range map[string]func(testing.TB, string){"UnixOnly": UnixOnly, "Pending": Pending} {
		ran := false
		t.Run(name, func(t *testing.T) {
			mark(t, "a test of the mark")
			ran = true
		})
		if ran == isWindows {
			t.Errorf("%s: ran on to its end = %v, on Windows = %v", name, ran, isWindows)
		}
	}
}
