package tmpl

import (
	"strings"
	"testing"

	"github.com/expr-lang/expr"
)

// A division by zero and a power that gives ±Inf fail, however the operator is written, and the rest
// of the arithmetic is expr's.
func TestArithmetic(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, err string }{
		{"item / 0 > 1", "1 / 0 is not a finite number"},
		{"(item - 1) / 0 == 0", "0 / 0 is not a finite number"},
		{"0 ** -item > 1", "0 ** -1 is not a finite number"},
		{"0 ^ -item > 1", "0 ** -1 is not a finite number"},
		{"item / 2 == 0.5 && 2 ** 3 == 8 && 2 ^ 3 == 8 && ceil(3 / 2) == 2", ""},
	} {
		p, err := compile(tt.src, scope{Item: nil, Doc: nil}, expr.AsBool())
		if err != nil {
			t.Fatal(err)
		}
		out, err := expr.Run(p, scope{Item: 1, Doc: nil})
		switch {
		case tt.err == "" && (err != nil || out != true):
			t.Errorf("%s = %v, %v", tt.src, out, err)
		case tt.err != "" && (err == nil || !strings.HasPrefix(brief(err), tt.err)):
			t.Errorf("%s = %v, %v; want %s", tt.src, out, err, tt.err)
		}
	}
}
