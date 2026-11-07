package kit

import (
	"strings"
	"testing"
)

// A change is exact, to the decimals of the longer of the two, and better alone says which way is good.
func TestStatsChange(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("1", 2001)
	for _, tt := range []struct{ from, value, better, change, way string }{
		{"3.1", "1.3", "lower", "-1.8", "better"},
		{"9", "4.5", "lower", "-4.5", "better"},
		{"3.10", "1.3", "lower", "-1.80", "better"},
		{"1.50", "1.5", "lower", "", "same"},
		{"-3.50", "-1", "higher", "+2.50", "better"},
		{"7", "007", "lower", "", "same"},
		{"0", "-0", "lower", "", "same"},
		{"100", "120", "higher", "+20", "better"},
		{"100", "80", "higher", "-20", "worse"},
		{"390", "420", "lower", "+30", "worse"},
		{"0.1", "0.2", "lower", "+0.1", "worse"},
		{
			"0.000000000000000001", "123456789012345678901234567890", "lower",
			"+123456789012345678901234567889.999999999999999999", "worse",
		},
		{"2", "1." + long, "lower", "-0." + strings.Repeat("8", 2000) + "9", "better"},
	} {
		got, err := statsOf([]any{map[string]any{"label": "a", "value": tt.value, "from": tt.from, "better": tt.better}})
		if err != nil {
			t.Errorf("%.20s to %.20s: %v", tt.from, tt.value, err)
			continue
		}
		if got[0].Change != tt.change || got[0].Way != tt.way {
			t.Errorf("%.20s to %.20s: %.40q %s, want %.40q %s", tt.from, tt.value, got[0].Change, got[0].Way, tt.change, tt.way)
		}
	}
}
