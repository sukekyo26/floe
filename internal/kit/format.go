package kit

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// formatNum, num in page.html, writes a number with its digits grouped by three, as it is or rounded
// to a count of significant digits written before it (num 3 writes 0.0064812 as 0.00648):
// {{.Calc.total | num}} or {{.Calc.total | num 3}}.
// LIMIT: grouped with , and a decimal point . as English and Japanese write them. Take the separators
// from the page's language when a catalog comes for one that writes them otherwise.
func formatNum(args ...any) (string, error) {
	digits := 0
	switch len(args) {
	case 1:
	case 2:
		d, ok := args[0].(int)
		if !ok || d < 1 {
			return "", usererr.New("kit_num_digits")
		}
		digits = d
	default:
		return "", usererr.New("kit_num_digits")
	}
	return decimal("num", args[len(args)-1], digits)
}

// formatBytes, bytes in page.html, writes a count of bytes in the binary unit that keeps it under
// 1024, to 3 significant digits but every digit before the point: 1536 is 1.5 KiB, 1023 is 1,023 B,
// and 1047552 is 1,023 KiB.
func formatBytes(v any) (string, error) {
	f, err := float("bytes", v)
	if err != nil {
		return "", err
	}
	unit := 0
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	for math.Abs(f) >= 1024 && unit < len(units)-1 {
		f /= 1024
		unit++
	}
	// A count under 1024 is written exactly, as v.
	if unit > 0 {
		f = roundFloat(f, max(3, len(strconv.FormatFloat(math.Trunc(math.Abs(f)), 'f', 0, 64))))
		// Rounded, 1048575 is 1024 KiB: 1 MiB.
		if math.Abs(f) >= 1024 && unit < len(units)-1 {
			f /= 1024
			unit++
		}
		v = f
	}
	text, err := decimal("bytes", v, 0)
	if err != nil {
		return "", err
	}
	return text + " " + units[unit], nil
}

// decimal writes v grouped, rounded to digits significant digits unless digits is 0. A whole number
// is written exactly, beyond the 2^53 a float holds, and rounded as one.
func decimal(fn string, v any, digits int) (string, error) {
	var text string
	if i, ok := whole(v); ok {
		if digits > 0 {
			i = roundWhole(i, digits)
		}
		text = i.String()
	} else {
		f, err := float(fn, v)
		if err != nil {
			return "", err
		}
		if digits > 0 {
			f = roundFloat(f, digits)
		}
		if f == 0 {
			f = 0 // not -0
		}
		text = strconv.FormatFloat(f, 'f', -1, 64)
	}
	sign := ""
	if t, ok := strings.CutPrefix(text, "-"); ok {
		sign, text = "-", t
	}
	integer, fraction, dot := strings.Cut(text, ".")
	var b strings.Builder
	b.WriteString(sign)
	for i, r := range integer {
		if i > 0 && (len(integer)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if dot {
		b.WriteString("." + fraction)
	}
	return b.String(), nil
}

// whole returns v as an integer, when it is a whole number.
func whole(v any) (*big.Int, bool) {
	switch n := v.(type) {
	case int:
		return big.NewInt(int64(n)), true
	case int64:
		return big.NewInt(n), true
	case uint64:
		return new(big.Int).SetUint64(n), true
	}
	return nil, false
}

// float returns v as a float, or the mistake of the function fn taking it.
func float(fn string, v any) (float64, error) {
	if i, ok := whole(v); ok {
		f, _ := new(big.Float).SetInt(i).Float64()
		return f, nil
	}
	if f, ok := v.(float64); ok {
		return f, nil
	}
	// An item of the schema not written comes as nil, as null does.
	if v == nil {
		return 0, usererr.New("kit_number_missing", fn)
	}
	return 0, usererr.New("kit_not_number", fn, fmt.Sprintf("%v (%T)", v, v))
}

// roundWhole rounds i to digits significant digits, half away from zero.
func roundWhole(i *big.Int, digits int) *big.Int {
	abs := new(big.Int).Abs(i)
	n := len(abs.String())
	if digits >= n {
		return i
	}
	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n-digits)), nil)
	q, r := new(big.Int).QuoRem(abs, unit, new(big.Int))
	if r.Lsh(r, 1).Cmp(unit) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	q.Mul(q, unit)
	if i.Sign() < 0 {
		q.Neg(q)
	}
	return q
}

// roundFloat rounds f to digits significant digits: the shortest text of it, without the noise of the
// float's binary digits.
func roundFloat(f float64, digits int) float64 {
	if r, err := strconv.ParseFloat(strconv.FormatFloat(f, 'g', digits, 64), 64); err == nil {
		return r
	}
	return f
}
