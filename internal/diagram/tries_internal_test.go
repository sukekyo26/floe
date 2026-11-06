package diagram

import (
	"runtime/debug"
	"slices"
	"testing"
)

// tries is how many of n random cases to run: a fifth under the race detector, whose job here is to see a write to a
// package variable, and all of them in the runs without it.
// LIMIT: this package has no go statement and its parallel tests share only package variables they read, so a fifth
// loses little; a write on a rare path alone would not be seen. Run them all under -race if goroutines come here.
func tries(n int) int {
	if underRace {
		return (n + 4) / 5
	}
	return n
}

// underRace says what the build says, as nothing else fails when it does not: a wrong false costs time, a wrong true
// leaves the graphs unrun everywhere.
func TestUnderRace(t *testing.T) {
	t.Parallel()

	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("no build info")
	}
	raced := slices.Contains(info.Settings, debug.BuildSetting{Key: "-race", Value: "true"})
	if underRace != raced {
		t.Errorf("underRace is %v, the build's -race %v", underRace, raced)
	}
}
