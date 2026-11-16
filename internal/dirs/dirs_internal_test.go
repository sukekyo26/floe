package dirs

import "testing"

// Shown takes off the prefix of a verbatim path in the two forms people write, and leaves any other as it is; with
// no prefix (Unix) it leaves every path as it is. The cases run on every system.
func TestShown(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ path, want string }{
		{`\\?\C:\Users\u\a.html`, `C:\Users\u\a.html`},
		{`\\?\UNC\srv\share\x.html`, `\\srv\share\x.html`},
		{`\\?\Volume{0b1c}\x.html`, `\\?\Volume{0b1c}\x.html`},
		{`\\?\C:`, `\\?\C:`},
		{`C:\Users\u\a.html`, `C:\Users\u\a.html`},
		{"/w/a.yaml", "/w/a.yaml"},
		{"a.yaml", "a.yaml"},
	} {
		if got := shownWith(tt.path, `\\?\`); got != tt.want {
			t.Errorf("shown %q = %q, want %q", tt.path, got, tt.want)
		}
		if got := shownWith(tt.path, ""); got != tt.path {
			t.Errorf("shown with no prefix %q = %q, want it as it is", tt.path, got)
		}
	}
}
