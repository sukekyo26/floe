package css

import (
	"strconv"
	"strings"
	"testing"
)

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

// TestTokens gives each token the depth of the blocks around it, an opener and its closer that of what is around them,
// a closer of another kind none of its own: the ) in [ ] and the } in f( ) are tokens of what holds them.
func TestTokens(t *testing.T) {
	t.Parallel()

	toks := Tokens([]byte(".a{b:f(1[2)]}"))
	got := make([]string, 0, len(toks))
	for _, tok := range toks {
		got = append(got, string(tok.Data)+strconv.Itoa(tok.Depth))
	}
	if want := ".0 a0 {0 b1 :1 f(1 12 [2 23 )3 ]2 }2"; strings.Join(got, " ") != want {
		t.Errorf("Tokens: %s, want %s", strings.Join(got, " "), want)
	}
}
