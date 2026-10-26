package yamlpos

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"

	"github.com/sukekyo26/floe/internal/usererr"
)

// kind is the type a plain scalar is read as.
type kind int

const (
	kindText kind = iota
	kindInt
	kindFloat
)

// reading is a plain scalar as read: its kind, and its value for a number.
type reading struct {
	kind kind
	i    *big.Int
	f    float64
}

// core is the Core Schema of YAML 1.2.2, 10.3.2 Tag Resolution: the regular expressions of its int and float
// rows, in its order, as the specification writes them without the spaces it adds to be read, each matching a
// whole value. It is a rule the specification sets, not a guess at how a parser reads; its null and bool rows
// are left out, as goccy/go-yaml reads them alike.
var core = []struct {
	read func(text string) reading
	re   *regexp.Regexp
}{
	{base(10, ""), regexp.MustCompile(`^(?:[-+]?[0-9]+)$`)},
	{base(8, "0o"), regexp.MustCompile(`^(?:0o[0-7]+)$`)},
	{base(16, "0x"), regexp.MustCompile(`^(?:0x[0-9a-fA-F]+)$`)},
	{number, regexp.MustCompile(`^(?:[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?)$`)},
	{infinity, regexp.MustCompile(`^(?:[-+]?(?:\.inf|\.Inf|\.INF))$`)},
	{notANumber, regexp.MustCompile(`^(?:\.nan|\.NaN|\.NAN)$`)},
}

func base(b int, prefix string) func(string) reading {
	return func(text string) reading {
		i, _ := new(big.Int).SetString(strings.TrimPrefix(text, prefix), b)
		return reading{kind: kindInt, i: i, f: 0}
	}
}

// number reads a float of the Number row; one past float64 is ±Inf.
func number(text string) reading {
	f, _ := strconv.ParseFloat(text, 64) //nolint:errcheck // the row's form: ErrRange alone, with ±Inf or 0
	return reading{kind: kindFloat, i: nil, f: f}
}

func infinity(text string) reading {
	sign := 1
	if strings.HasPrefix(text, "-") {
		sign = -1
	}
	return reading{kind: kindFloat, i: nil, f: math.Inf(sign)}
}

func notANumber(string) reading {
	return reading{kind: kindFloat, i: nil, f: math.NaN()}
}

// read12 returns how YAML 1.2 reads text, a plain scalar.
func read12(text string) reading {
	for _, row := range core {
		if row.re.MatchString(text) {
			return row.read(text)
		}
	}
	return reading{kind: kindText, i: nil, f: 0}
}

// startsNumber reports whether a number can start text: a digit, +, - or ., which every int and float row
// of core begins with.
func startsNumber(text string) bool {
	return text != "" && strings.ContainsRune("0123456789+-.", rune(text[0]))
}

// plainNumber returns how YAML 1.2 reads n when it is a plain scalar a number can start, false else.
func plainNumber(n ast.Node) (reading, bool) {
	tk := n.GetToken()
	if tk.Type == token.SingleQuoteType || tk.Type == token.DoubleQuoteType || !startsNumber(tk.Value) {
		return reading{kind: kindText, i: nil, f: 0}, false
	}
	return read12(tk.Value), true
}

// goccy returns how goccy/go-yaml read n.
func goccy(n ast.Node) reading {
	switch n := n.(type) {
	case *ast.IntegerNode:
		switch v := n.Value.(type) {
		case int64:
			return reading{kind: kindInt, i: big.NewInt(v), f: 0}
		case uint64:
			return reading{kind: kindInt, i: new(big.Int).SetUint64(v), f: 0}
		}
	case *ast.FloatNode:
		return reading{kind: kindFloat, i: nil, f: n.Value}
	case *ast.InfinityNode:
		return reading{kind: kindFloat, i: nil, f: n.Value}
	case *ast.NanNode:
		return reading{kind: kindFloat, i: nil, f: math.NaN()}
	}
	return reading{kind: kindText, i: nil, f: 0}
}

// fits reports whether r, an integer, is one floe reads: an int64 or a uint64.
func (r reading) fits() bool {
	return r.i.IsInt64() || r.i.Sign() >= 0 && r.i.IsUint64()
}

func (r reading) finite() bool {
	return r.kind != kindFloat || !math.IsInf(r.f, 0) && !math.IsNaN(r.f)
}

func (r reading) same(o reading) bool {
	switch {
	case r.kind != o.kind:
		return false
	case r.kind == kindInt:
		return r.i.Cmp(o.i) == 0
	case r.kind == kindFloat:
		return r.f == o.f
	}
	return true
}

// written returns r, a number, as YAML 1.2 and goccy/go-yaml both read it: an integer in decimal, a float with
// a "." (goccy/go-yaml reads a number with one "." as a float, and 1e+21 as text).
func (r reading) written() string {
	if r.kind == kindInt {
		return r.i.String()
	}
	s := strconv.FormatFloat(r.f, 'g', -1, 64)
	if strings.Contains(s, ".") {
		return s
	}
	if i := strings.IndexByte(s, 'e'); i >= 0 {
		return s[:i] + ".0" + s[i:]
	}
	return s + ".0"
}

// shown returns r, a number, as the mistakes show what YAML 1.2 reads.
func (r reading) shown() string {
	if r.kind == kindInt {
		return r.i.String()
	}
	return strconv.FormatFloat(r.f, 'g', -1, 64)
}

// numberMistake returns the mistake of n, a plain scalar a number can start, that YAML 1.2 reads as want and
// goccy/go-yaml as got, nil when floe takes it: one YAML 1.2 reads as not finite, as an integer floe cannot
// hold, or that the two read otherwise, worded by how they differ, with a way of writing it both read alike.
func numberMistake(file string, n ast.Node, want, got reading) *usererr.Error {
	written, at := n.GetToken().Value, Of(n)
	switch {
	case !want.finite():
		return New(file, at, "yaml_not_finite", written)
	case want.kind == kindInt && !want.fits():
		return New(file, at, "yaml_number_too_big", written)
	case want.same(got):
		return nil
	case want.kind == kindText:
		return New(file, at, "yaml_number_not_number", written, got.written())
	case got.kind == kindText:
		return New(file, at, "yaml_number_not_text", written, want.shown(), want.written())
	}
	// Both are numbers: goccy/go-yaml read a 0 at the start as octal, the one way the two differ so.
	return New(file, at, "yaml_number_octal", written, got.shown(), want.written(), octal(got))
}

// octal returns r, an integer, as YAML 1.2 writes it in octal, or in decimal below 0, which it has no octal for.
func octal(r reading) string {
	if r.i.Sign() < 0 {
		return r.i.String()
	}
	return "0o" + r.i.Text(8)
}

// number12 reports whether YAML 1.2 reads n, a plain scalar, as a number.
func number12(n ast.Node) bool {
	r, ok := plainNumber(n)
	return ok && r.kind != kindText
}
