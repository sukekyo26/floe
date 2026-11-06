package diagram

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
