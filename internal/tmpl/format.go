package tmpl

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var (
	errNotNumber = errors.New("is not a number")
	errDigits    = errors.New("write num or num with a count of significant digits from 1, like num 3")
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
			return "", fmt.Errorf("num: %w", errDigits)
		}
		digits = d
	default:
		return "", fmt.Errorf("num: %w", errDigits)
	}
	text, err := decimal(args[len(args)-1], digits)
	if err != nil {
		return "", fmt.Errorf("num: %w", err)
	}
	return text, nil
}

// formatBytes, bytes in page.html, writes a count of bytes in the binary unit that keeps it under
// 1024, to 3 significant digits: 1536 is 1.5 KiB.
func formatBytes(v any) (string, error) {
	f, err := float(v)
	if err != nil {
		return "", fmt.Errorf("bytes: %w", err)
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	unit := 0
	for math.Abs(f) >= 1024 && unit < len(units)-1 {
		f /= 1024
		unit++
	}
	text, err := decimal(f, 3)
	if err != nil {
		return "", fmt.Errorf("bytes: %w", err)
	}
	return text + " " + units[unit], nil
}

// decimal writes v grouped, rounded to digits significant digits unless digits is 0. A whole number
// is written exactly, beyond the 2^53 a float holds.
func decimal(v any, digits int) (string, error) {
	var text string
	switch n := v.(type) {
	case int:
		text = strconv.Itoa(n)
	case int64:
		text = strconv.FormatInt(n, 10)
	case uint64:
		text = strconv.FormatUint(n, 10)
	}
	if text == "" || digits > 0 {
		f, err := float(v)
		if err != nil {
			return "", err
		}
		if digits > 0 {
			// The shortest text of f rounded, without the noise of the float's binary digits.
			if r, err := strconv.ParseFloat(strconv.FormatFloat(f, 'g', digits, 64), 64); err == nil {
				f = r
			}
		}
		text = strconv.FormatFloat(f, 'f', -1, 64)
	}
	sign := ""
	if t, ok := strings.CutPrefix(text, "-"); ok {
		sign, text = "-", t
	}
	whole, fraction, dot := strings.Cut(text, ".")
	var b strings.Builder
	b.WriteString(sign)
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if dot {
		b.WriteString("." + fraction)
	}
	return b.String(), nil
}

func float(v any) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case float64:
		return n, nil
	}
	return 0, fmt.Errorf("%v (%T) %w", v, v, errNotNumber)
}
