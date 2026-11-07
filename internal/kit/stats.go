package kit

import (
	"math/big"
	"regexp"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// stat is what part.html draws of an item of stats. Value is as written, as a bar's.
// LIMIT: a change has no unit, the value's being read just before it, and how a screen reader reads its - is left
// to it.
type stat struct {
	Value                      any
	Label, Unit, Note, Tone    string
	From, Change, Way, WayTone string
}

// statNumber is the form the schema of the part stats gives from and value. TestStatNumbers holds the two alike.
var statNumber = regexp.MustCompile(`^-?[0-9]+([.][0-9]+)?$`)

// scaled is a number of statNumber's form as a whole number, its point moved decimals to the right.
func scaled(s string, decimals int) *big.Int {
	whole, fraction, _ := strings.Cut(s, ".")
	i, _ := new(big.Int).SetString(whole+fraction+strings.Repeat("0", decimals-len(fraction)), 10)
	return i
}

func decimalsIn(s string) int {
	_, fraction, _ := strings.Cut(s, ".")
	return len(fraction)
}

// statChange is value less from, two numbers of statNumber's form, and its sign, written to the decimals of the longer
// of the two: exact, as a float would write 1.8000000000000000444 for 3.1 to 1.3, and in whole numbers, which have no
// bound, as a fraction cannot read a million decimals.
// LIMIT: no bound on the digits, and the time grows as their square (1.2 s at a million decimals, 13.2 s at four
// million): a page is its writer's own, under the API's 32 MiB. Bound the digits if pages come to hold such numbers.
func statChange(from, value string) (string, int) {
	decimals := max(decimalsIn(from), decimalsIn(value))
	diff := new(big.Int).Sub(scaled(value, decimals), scaled(from, decimals))
	sign := diff.Sign()
	digits := diff.Abs(diff).Text(10)
	if decimals > 0 {
		if len(digits) <= decimals {
			digits = strings.Repeat("0", decimals-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-decimals] + "." + digits[len(digits)-decimals:]
	}
	switch sign {
	case -1:
		return "-" + digits, sign
	case 1:
		return "+" + digits, sign
	}
	return digits, sign
}

// statOf reads the item at i into found's mistakes, returning a failure of floe's. Only what is counted is looked at;
// the schema says the rest, as with chart.
func statOf(i int, item any, found *usererr.List) (stat, error) {
	m, ok := item.(map[string]any)
	if !ok {
		return stat{}, found.Add(usererr.New("kit_stat_item", i+1))
	}
	// LIMIT: a part of one's own whose schema lets label, unit, note or tone be other than text draws them empty, plain
	// on the page, and better without from is not read: what they may be is its schema's to say.
	s := stat{
		Value: m["value"], Label: stringOf(m["label"]), Unit: stringOf(m["unit"]), Note: stringOf(m["note"]),
		Tone: stringOf(m["tone"]), From: stringOf(m["from"]), Change: "", Way: "", WayTone: "",
	}
	if m["from"] == nil {
		return s, nil
	}
	value, better := stringOf(m["value"]), stringOf(m["better"])
	counted := true
	for _, c := range []struct {
		key  string
		good bool
	}{
		{"from", statNumber.MatchString(s.From)},
		{"value", statNumber.MatchString(value)},
		{"better", better == "lower" || better == "higher"},
	} {
		if c.good {
			continue
		}
		counted = false
		if failed := found.Add(usererr.New("kit_stat_change", i+1, s.Label, c.key)); failed != nil {
			return stat{}, failed
		}
	}
	if !counted {
		return s, nil
	}
	change, sign := statChange(s.From, value)
	switch {
	case sign == 0:
		s.Way, s.WayTone = "same", "muted"
		return s, nil
	case (sign < 0) == (better == "lower"):
		s.Way, s.WayTone = "better", "good"
	default:
		s.Way, s.WayTone = "worse", "bad"
	}
	s.Change = change
	return s, nil
}

// LIMIT: mistakes are told at the part's value, naming the item by its number and label, as with bars: stats knows not
// where in the part's value the list is. Place them at the item if that misleads.
func statsOf(items []any) ([]stat, error) {
	stats := make([]stat, 0, len(items))
	var found usererr.List
	for i, item := range items {
		s, failed := statOf(i, item, &found)
		if failed != nil {
			return nil, failed
		}
		stats = append(stats, s)
	}
	return stats, found.Err()
}
