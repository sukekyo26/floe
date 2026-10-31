package diagram

// rank makes a group of the boxes members of g (their indexes in nodes, as written) and its edges (their indexes
// in edges): an edge back up a cycle turned round, each box at the rank of its longest path from a box no edge
// enters, and an edge passing ranks between its ends broken into links by a point at each.
func rank(g *graph, members, edges []int, byName map[string]int) *group {
	gr := &group{
		nodes: nil, links: nil, ranks: nil, chains: map[int][]int{}, reversed: map[int]bool{}, tracks: nil, top: nil, bottom: nil,
	}
	local := map[int]int{}
	for j, i := range members {
		local[i] = j
		b := g.Nodes[i].Box
		gr.nodes = append(gr.nodes, gnode{box: b, of: i, rank: 0, w: 0, h: 0, x: 0, y: 0, ins: nil, outs: nil, written: j})
	}
	n := len(members)
	type arc struct{ from, to, edge int }
	arcs := make([]arc, 0, len(edges))
	outs := make([][]int, n) // the arcs out of each box, as written
	for _, k := range edges {
		outs[local[byName[g.Edges[k].From]]] = append(outs[local[byName[g.Edges[k].From]]], len(arcs))
		arcs = append(arcs, arc{local[byName[g.Edges[k].From]], local[byName[g.Edges[k].To]], k})
	}
	// Depth first from each box as written, on a stack of its own (no recursion): an arc to a box on the way is
	// turned round.
	const (
		unseen = iota
		onWay
		done
	)
	state := make([]int, n)
	type frame struct{ node, next int }
	for s := range n {
		if state[s] != unseen {
			continue
		}
		stack := []frame{{s, 0}}
		state[s] = onWay
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next == len(outs[top.node]) {
				state[top.node] = done
				stack = stack[:len(stack)-1]
				continue
			}
			a := &arcs[outs[top.node][top.next]]
			top.next++
			switch state[a.to] {
			case onWay:
				gr.reversed[a.edge] = true
				a.from, a.to = a.to, a.from
			case unseen:
				state[a.to] = onWay
				stack = append(stack, frame{a.to, 0})
			}
		}
	}
	// The longest path from a box no arc enters (Kahn's way), taking boxes as written.
	into := make([]int, n)
	down := make([][]int, n)
	for i, a := range arcs {
		into[a.to]++
		down[a.from] = append(down[a.from], i)
	}
	var queue []int
	for i := range n {
		if into[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, i := range down[u] {
			v := arcs[i].to
			gr.nodes[v].rank = max(gr.nodes[v].rank, gr.nodes[u].rank+1)
			if into[v]--; into[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	// A point at each rank an arc passes, and a link for each rank it goes down.
	for _, a := range arcs {
		prev := a.from
		for r := gr.nodes[a.from].rank + 1; r <= gr.nodes[a.to].rank; r++ {
			next := a.to
			if r < gr.nodes[a.to].rank {
				next = len(gr.nodes)
				gr.nodes = append(gr.nodes, gnode{box: nil, of: a.edge, rank: r, w: pointW, h: 0, x: 0, y: 0, ins: nil,
					outs: nil, written: len(gr.nodes)})
			}
			gr.link(prev, next, a.edge)
			prev = next
		}
	}
	for i, nd := range gr.nodes {
		for len(gr.ranks) <= nd.rank {
			gr.ranks = append(gr.ranks, nil)
		}
		gr.ranks[nd.rank] = append(gr.ranks[nd.rank], i)
	}
	return gr
}

func (gr *group) link(from, to, edge int) {
	gr.nodes[from].outs = append(gr.nodes[from].outs, len(gr.links))
	gr.nodes[to].ins = append(gr.nodes[to].ins, len(gr.links))
	gr.chains[edge] = append(gr.chains[edge], len(gr.links))
	gr.links = append(gr.links, glink{from: from, to: to, edge: edge, exit: 0, entry: 0, straight: false, track: 0})
}
