package yamlpos

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// kind is the type a plain scalar is read as.
type kind int

const (
	kindText kind = iota
	kindNull
	kindTrue
	kindFalse
	kindInt
	kindFloat
)

// reading is a plain scalar as read: its kind, and its value for a number.
type reading struct {
	kind    kind
	i       *big.Int
	f       float64
	outside bool // a float of the Number row past float64: ±Inf, or 0 for one that is not 0
}

// core is the Core Schema of YAML 1.2.2, 10.3.2 Tag Resolution: the regular expressions of its rows, in its order,
// as the specification writes them without the spaces it adds to be read, each matching a whole value; its null row
// and its row of an empty value are one here, and its bool row is two, true and false, which read the same texts.
// It is a rule the specification sets, not a guess at how a parser reads.
var core = []struct {
	read func(text string) reading
	re   *regexp.Regexp
}{
	{reads(kindNull), regexp.MustCompile(`^(?:null|Null|NULL|~|)$`)},
	{reads(kindTrue), regexp.MustCompile(`^(?:true|True|TRUE)$`)},
	{reads(kindFalse), regexp.MustCompile(`^(?:false|False|FALSE)$`)},
	{base(10, ""), regexp.MustCompile(`^(?:[-+]?[0-9]+)$`)},
	{base(8, "0o"), regexp.MustCompile(`^(?:0o[0-7]+)$`)},
	{base(16, "0x"), regexp.MustCompile(`^(?:0x[0-9a-fA-F]+)$`)},
	{number, regexp.MustCompile(`^(?:[-+]?(?:\.[0-9]+|[0-9]+(?:\.[0-9]*)?)(?:[eE][-+]?[0-9]+)?)$`)},
	{infinity, regexp.MustCompile(`^(?:[-+]?(?:\.inf|\.Inf|\.INF))$`)},
	{notANumber, regexp.MustCompile(`^(?:\.nan|\.NaN|\.NAN)$`)},
}

func reads(k kind) func(string) reading {
	return func(string) reading { return reading{kind: k, i: nil, f: 0, outside: false} }
}

func base(b int, prefix string) func(string) reading {
	return func(text string) reading {
		i, _ := new(big.Int).SetString(strings.TrimPrefix(text, prefix), b)
		return reading{kind: kindInt, i: i, f: 0, outside: false}
	}
}

// number reads a float of the Number row, outside float64 when it becomes ±Inf, or 0 while a digit of its
// significand is not.
func number(text string) reading {
	f, _ := strconv.ParseFloat(text, 64) //nolint:errcheck // the row's form parses; its range is judged below
	significand, _, _ := strings.Cut(strings.ToLower(text), "e")
	outside := math.IsInf(f, 0) || f == 0 && strings.ContainsAny(significand, "123456789")
	return reading{kind: kindFloat, i: nil, f: f, outside: outside}
}

func infinity(text string) reading {
	sign := 1
	if strings.HasPrefix(text, "-") {
		sign = -1
	}
	return reading{kind: kindFloat, i: nil, f: math.Inf(sign), outside: false}
}

func notANumber(string) reading {
	return reading{kind: kindFloat, i: nil, f: math.NaN(), outside: false}
}

// read12 returns how YAML 1.2 reads text, a plain scalar.
func read12(text string) reading {
	for _, row := range core {
		if row.re.MatchString(text) {
			return row.read(text)
		}
	}
	return reading{kind: kindText, i: nil, f: 0, outside: false}
}

// fits reports whether r, an integer, is one floe reads: an int64 or a uint64.
func (r reading) fits() bool {
	return r.i.IsInt64() || r.i.IsUint64()
}

func (r reading) finite() bool {
	return r.kind != kindFloat || !math.IsInf(r.f, 0) && !math.IsNaN(r.f)
}
