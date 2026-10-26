package css

import "testing"

// oneString takes a url( holding one string alone, or an address, and refuses one holding more: tdewolff
// v2.8.20 makes those a BadURL before oneString sees them, so the tokens are made here.
func TestOneString(t *testing.T) {
	t.Parallel()

	for data, want := range map[string]bool{
		`url("x")`: true, `url( 'x' )`: true, `url(x)`: true, "url(\t\"x\"\n)": true, `url(`: true, `url("")`: true,
		`url("x" y)`: false, `url("x" "y")`: false, `url('x' ')'`: false, `url("x`: true,
	} {
		if got := oneString([]byte(data)); got != want {
			t.Errorf("oneString(%q) = %v, want %v", data, got, want)
		}
	}
}
