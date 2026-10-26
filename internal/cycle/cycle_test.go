package cycle_test

import (
	"fmt"
	"testing"

	"github.com/sukekyo26/floe/internal/cycle"
)

// Of tells a cycle of each tangle, from its least name back to it, and none of names that use no cycle.
func TestOf(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		uses map[string][]string
		want string
	}{
		{map[string][]string{"a": {"b"}, "b": nil}, "[]"},
		{map[string][]string{"p": {"p"}}, "[[p p]]"},
		{map[string][]string{"p": {"q", "r"}, "q": {"p"}, "r": {"p"}}, "[[p q p]]"},
		{map[string][]string{"p": {"s"}, "s": {"q", "r"}, "q": {"s"}, "r": {"p"}}, "[[p s r p]]"},
		{map[string][]string{"p": {"q"}, "q": {"p"}, "r": {"s"}, "s": {"r"}}, "[[p q p] [r s r]]"},
		{map[string][]string{"c": {"q"}, "p": {"q"}, "q": {"p", "gone"}}, "[[p q p]]"},
	} {
		var names []string
		for _, n := range []string{"a", "b", "c", "p", "q", "r", "s"} {
			if _, ok := tt.uses[n]; ok {
				names = append(names, n)
			}
		}
		if got := fmt.Sprint(cycle.Of(names, tt.uses)); got != tt.want {
			t.Errorf("Of(%v) = %s, want %s", tt.uses, got, tt.want)
		}
	}
}
