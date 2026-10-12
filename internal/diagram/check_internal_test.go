package diagram

import (
	"math"
	"testing"
)

func TestFormatNum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		v    float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"},
		{120, "120"},
		{-0.5, "-0.5"},
		{0.30000000000000004, "0.30000000000000004"},
		{1e-6, "0.000001"},
		{1.5e-7, "1.5e-7"},
		{1e21, "1e+21"},
		{123456789012345680000, "123456789012345680000"},
	}
	for _, tt := range tests {
		if got := formatNum(tt.v); got != tt.want {
			t.Errorf("formatNum(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}
