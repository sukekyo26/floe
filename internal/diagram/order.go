package diagram

import (
	"cmp"
	"slices"
)

// maxTrips is how many round trips of sweeps order takes at most.
// LIMIT: 12 trips of median sweeps, no more, keeping the fewest crossings seen; a graph whose order keeps changing
// may keep crossings another order would not. Sweep more, or by another key, if one comes to.
const maxTrips = 12

// order orders each rank to cross fewer links: as built (the boxes as written, then the points by their edge's
// order) when no link crosses; else by round trips of sweeps, down then up, each rank by the median of where its
// nodes' links reach in the rank before. It stops when no link crosses, a trip changes nothing, or at maxTrips, and
// keeps the order of fewest crossings seen, the first of them on a tie. It returns the trips taken.
func (gr *group) order() int {
	least := gr.crossings()
	if least == 0 {
		return 0
	}
	best := cloneRanks(gr.ranks)
	trip := 0
	for trip < maxTrips {
		trip++
		before := cloneRanks(gr.ranks)
		for r := 1; r < len(gr.ranks); r++ {
			gr.sweep(r, true)
		}
		for r := len(gr.ranks) - 2; r >= 0; r-- {
			gr.sweep(r, false)
		}
		n := gr.crossings()
		if n < least {
			least, best = n, cloneRanks(gr.ranks)
		}
		if n == 0 || slices.EqualFunc(before, gr.ranks, slices.Equal) {
			break
		}
	}
	gr.ranks = best
	return trip
}

func cloneRanks(ranks [][]int) [][]int {
	out := make([][]int, len(ranks))
	for i, r := range ranks {
		out[i] = slices.Clone(r)
	}
	return out
}

// positions is the index of each node in its rank.
func (gr *group) positions() []int {
	pos := make([]int, len(gr.nodes))
	for _, r := range gr.ranks {
		for i, n := range r {
			pos[n] = i
		}
	}
	return pos
}

// sweep orders rank r by the median position of its nodes' links in the rank above (down) or below: a node with
// no link there keeps its place, and the others fill the places left, stably, so that ties keep their order.
func (gr *group) sweep(r int, down bool) {
	pos := gr.positions()
	keys := make(map[int]float64, len(gr.ranks[r]))
	var moving []int
	for _, n := range gr.ranks[r] {
		var at []int
		if down {
			for _, l := range gr.nodes[n].ins {
				at = append(at, pos[gr.links[l].from])
			}
		} else {
			for _, l := range gr.nodes[n].outs {
				at = append(at, pos[gr.links[l].to])
			}
		}
		if len(at) > 0 {
			keys[n] = median(at)
			moving = append(moving, n)
		}
	}
	slices.SortStableFunc(moving, func(a, b int) int { return cmp.Compare(keys[a], keys[b]) })
	k := 0
	for i, n := range gr.ranks[r] {
		if _, ok := keys[n]; ok {
			gr.ranks[r][i] = moving[k]
			k++
		}
	}
}

// median is the middle of xs, or the mean of the middle two.
func median[T int | float64](xs []T) float64 {
	slices.Sort(xs)
	m := len(xs)
	if m%2 == 1 {
		return float64(xs[m/2])
	}
	return float64(xs[m/2-1]+xs[m/2]) / 2
}

// crossings counts the pairs of links between two ranks whose ends are in opposite orders, rank by rank: the
// links sorted by their upper end, the pairs out of order at the lower end (with a Fenwick tree, O(E log V)).
func (gr *group) crossings() int {
	pos := gr.positions()
	total := 0
	for r := 0; r+1 < len(gr.ranks); r++ {
		var pairs [][2]int
		for _, n := range gr.ranks[r] {
			for _, l := range gr.nodes[n].outs {
				pairs = append(pairs, [2]int{pos[n], pos[gr.links[l].to]})
			}
		}
		slices.SortFunc(pairs, func(a, b [2]int) int { return cmp.Or(a[0]-b[0], a[1]-b[1]) })
		size := len(gr.ranks[r+1])
		tree := make([]int, size+1)
		seen := 0
		for _, p := range pairs {
			// The links seen so far that reach further right below cross this one.
			atMost := 0
			for i := p[1] + 1; i > 0; i -= i & -i {
				atMost += tree[i]
			}
			total += seen - atMost
			for i := p[1] + 1; i <= size; i += i & -i {
				tree[i]++
			}
			seen++
		}
	}
	return total
}
