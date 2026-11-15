// Package cycle finds the names that use each other, in a graph of names: the computed values of
// computed.yaml, the looks parts take.
package cycle

import "slices"

// Of returns a cycle of each tangle of names, sorted, that use each other (a strongly connected component
// of uses, or one name using itself), in the order of their least names: the first path from the least name
// of the tangle back to it, going through the names of the tangle in their order. A name uses only names.
func Of(names []string, uses map[string][]string) [][]string {
	tangle := tangles(names, uses)
	var found [][]string
	for _, n := range names {
		t := tangle[n]
		if t[0] != n || len(t) == 1 && !slices.Contains(uses[n], n) {
			continue
		}
		found = append(found, pathBack(n, t, uses))
	}
	return found
}

// tangles returns the tangle of each of names, the names that use it and that it uses, through others too,
// sorted: Tarjan's strongly connected components.
func tangles(names []string, uses map[string][]string) map[string][]string {
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	tangle := map[string][]string{}
	var visit func(n string)
	visit = func(n string) {
		index[n], low[n] = len(index), len(index)
		stack = append(stack, n)
		onStack[n] = true
		for _, u := range uses[n] {
			if _, seen := index[u]; !seen {
				if !slices.Contains(names, u) {
					continue
				}
				visit(u)
				low[n] = min(low[n], low[u])
			} else if onStack[u] {
				low[n] = min(low[n], index[u])
			}
		}
		if low[n] == index[n] {
			// n is the first of its tangle seen: the tangle is the names above it on the stack.
			i := slices.Index(stack, n)
			t := slices.Sorted(slices.Values(stack[i:]))
			for _, m := range t {
				tangle[m], onStack[m] = t, false
			}
			stack = stack[:i]
		}
	}
	for _, n := range names {
		if _, seen := index[n]; !seen {
			visit(n)
		}
	}
	return tangle
}

// pathBack returns the first path from start back to it, going depth first through the names of tangle in
// their order: start → … → start.
func pathBack(start string, tangle []string, uses map[string][]string) []string {
	seen := map[string]bool{}
	var path []string
	var walk func(n string) bool
	walk = func(n string) bool {
		path = append(path, n)
		for _, u := range uses[n] {
			switch {
			case u == start:
				path = append(path, u)
				return true
			case !slices.Contains(tangle, u) || seen[u]:
				continue
			}
			seen[u] = true
			if walk(u) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}
	walk(start)
	return path
}
