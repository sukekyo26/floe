package pagediff

import (
	"bytes"
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"floe/internal/usererr"
)

// Words are the words Mark writes on a page, in the language of the one who reads it: over what is gone, and over a
// diagram or image's earlier version.
type Words struct {
	Gone   string
	Before string
}

// Mark is the page to with what differs from the page from marked on it, for the sandbox alone: the words added and
// gone in their places, a block framed only when its words cannot show what differs, what is gone after the part it
// was in and an earlier diagram or image under the later. Each place is numbered floe-diff.<k>, in the order of the
// page, as the Places of Compare. The page's own elements keep their ids and attributes but the classes added, and
// the open and name of a details that holds a place.
func Mark(from, to []byte, words Words) ([]byte, error) {
	_, tree, err := marked(from, to, pageMost, words)
	if err != nil {
		return nil, err
	}
	withStyle(tree)
	var out bytes.Buffer
	if err := html.Render(&out, tree); err != nil {
		return nil, usererr.Library(err) //nolint:wrapcheck // the library's words
	}
	return out.Bytes(), nil
}

// marked compares the pages and marks the later's tree: the one comparison both Compare and Mark are of. The earlier
// is read to be compared, and for what of it is put in the later.
func marked(from, to []byte, most int, words Words) (Result, *html.Node, error) {
	treeA, a, err := readTree(from)
	if err != nil {
		return Result{}, nil, err
	}
	treeB, b, err := readTree(to)
	if err != nil {
		return Result{}, nil, err
	}
	c := compare(a, b, &work{left: most})
	m := newMarker(treeB, words)
	m.mark(treeA, a, b, c)
	places := m.number()
	for i, k := range places {
		if k >= 0 {
			c.result.Sections[i].Place = &k
		}
	}
	c.result.Counts.Places = m.count
	return c.result, treeB, nil
}

// The classes marks are drawn by, and the style that draws them, in colours of no theme.
const (
	classIns    = "floe-diff-ins"
	classDel    = "floe-diff-del"
	classChange = "floe-diff-ch"
	classAdd    = "floe-diff-add"
	classGone   = "floe-diff-gone"
	classSr     = "floe-diff-sr"
	classBefore = "floe-diff-before"
	classAt     = "floe-diff-at"
	markStyle   = `.floe-diff-ins{background:#d6f5df;color:inherit;text-decoration:underline 2px #1a7f37;text-underline-offset:3px}
.floe-diff-del{background:#ffe0e0;color:#a40e26;text-decoration:line-through 2px}
.floe-diff-ch{outline:2px solid #d4a017;outline-offset:4px}
.floe-diff-add{outline:2px solid #1a7f37;outline-offset:4px}
tr.floe-diff-ch,tr.floe-diff-add{outline-offset:-2px}
.floe-diff-gone{margin:6px 0;padding-left:10px;border-left:3px solid #cf222e}
div.floe-diff-gone{color:#a40e26}
div.floe-diff-gone>p{margin:2px 0}
:not(div).floe-diff-gone,div.floe-diff-gone:has(>:not(p,.floe-diff-sr)){opacity:.55}
.floe-diff-sr{position:absolute;width:1px;height:1px;margin:-1px;padding:0;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap;border:0}
.floe-diff-before{margin:12px 0;padding:6px 12px 12px;border:2px solid #d4a017;border-radius:8px;background:#fffbea;color:#24292f}
.floe-diff-before>summary{font:600 13px system-ui,sans-serif;color:#7a5900;cursor:pointer;padding:4px 0}
.floe-diff-at{scroll-margin-top:30vh}`
)

// marker marks the later page's tree, going through the comparison in order.
type marker struct {
	root, body *html.Node
	words      Words
	// last is the later's element gone through last, in box, its section's box; tail what was put in after it last.
	last, lastBox, tail *html.Node
	pending             []*html.Node
	// added is what floe put in; kept, the earlier's elements that hold a block the later keeps; copied, those copied.
	added, kept, copied map[*html.Node]bool
	held                map[holdKey]bool
	labels              map[*html.Node]*html.Node
	groups              []*group
	cur                 *group
	count               int
}

// group is a place: what is marked one after another in a section, of its result's section sec.
type group struct {
	sec   int
	nodes []*html.Node
}

type holdKey struct {
	parent *html.Node
	tag    string
}

func newMarker(root *html.Node, words Words) *marker {
	return &marker{
		root: root, body: bodyOf(root), words: words, last: nil, lastBox: nil, tail: nil, pending: nil,
		added: map[*html.Node]bool{}, kept: map[*html.Node]bool{}, copied: map[*html.Node]bool{}, held: map[holdKey]bool{}, labels: map[*html.Node]*html.Node{},
		groups: nil, cur: nil, count: 0,
	}
}

// boxes are the boxes of the sections: what a section's heading is in (an hgroup's), or for the one before the first
// heading the next's, or body.
func boxes(sections []section, body *html.Node) []*html.Node {
	out := make([]*html.Node, len(sections))
	next := body
	for i := len(sections) - 1; i >= 0; i-- {
		if h := sections[i].head; h != nil && h.Parent != nil {
			next = h.Parent
			if next.DataAtom == atom.Hgroup && next.Parent != nil {
				next = next.Parent
			}
		}
		out[i] = next
	}
	return out
}

func bodyOf(root *html.Node) *html.Node {
	for n := range root.Descendants() {
		if n.Type == html.ElementNode && n.DataAtom == atom.Body && n.Namespace == "" {
			return n
		}
	}
	return root
}

// mark marks the later page, walking the comparison's sections and blocks in order.
func (m *marker) mark(treeA *html.Node, a, b []section, c compared) {
	boxA, boxB := boxes(a, bodyOf(treeA)), boxes(b, m.body)
	for _, st := range c.steps {
		for _, p := range st.pairs {
			if p.from != nil && p.to != nil {
				m.keep(p.from.node)
			}
		}
		if st.from >= 0 && st.to >= 0 {
			m.keep(a[st.from].head)
		}
	}
	for i, st := range c.steps {
		m.cur = nil
		switch {
		case st.to < 0:
			from := a[st.from]
			if from.head != nil {
				m.gone(i, &block{kind: "", text: from.heading, key: "", node: from.head, from: nil}, boxA[st.from])
			}
			for j := range from.blocks {
				m.gone(i, &from.blocks[j], boxA[st.from])
			}
		case st.from < 0:
			to := b[st.to]
			if to.head != nil {
				m.visit(to.head, boxB[st.to])
				m.show(i, to.head, "", to.from, added(to.heading), classAdd)
			}
			for j := range to.blocks {
				blk := &to.blocks[j]
				m.visit(blk.node, boxB[st.to])
				m.show(i, blk.node, blk.kind, blk.from, added(blk.text), classAdd)
			}
		default:
			m.changed(i, a[st.from], b[st.to], boxA[st.from], boxB[st.to], st.pairs, c.result.Sections[i].Heading)
		}
	}
	m.flush(nil, nil)
}

func (m *marker) changed(sec int, from, to section, boxA, boxB *html.Node, pairs []blockPair, heading []Span) {
	if to.head != nil {
		m.visit(to.head, boxB)
		if from.heading != to.heading {
			m.show(sec, to.head, "", to.from, heading, classChange)
		} else {
			m.cur = nil
		}
	}
	for _, p := range pairs {
		switch {
		case p.to == nil:
			m.gone(sec, p.from, boxA)
		case p.from == nil:
			m.visit(p.to.node, boxB)
			m.show(sec, p.to.node, p.to.kind, p.to.from, added(p.to.text), classAdd)
		case p.from.key == p.to.key:
			m.visit(p.to.node, boxB)
			m.cur = nil
		case figure(p.to.kind):
			m.visit(p.to.node, boxB)
			m.frame(sec, partOf(p.to.node), classChange)
			m.before(sec, p.from.node)
		default:
			m.visit(p.to.node, boxB)
			m.show(sec, p.to.node, p.to.kind, p.to.from, p.spans, classChange)
		}
	}
}

// show marks the words of a block that differ: framed alone when they cannot be, as of a diagram or image, or when a
// mark cannot be held where it goes.
func (m *marker) show(sec int, n *html.Node, kind string, from []origin, spans []Span, class string) {
	switch {
	case figure(kind):
		m.frame(sec, partOf(n), class)
	case !m.markWords(from, spans):
		m.frame(sec, n, class)
	default:
		m.place(sec, n)
	}
}

func figure(kind string) bool {
	return kind == "svg" || kind == "img"
}

func added(text string) []Span {
	return []Span{{Op: opIns, Text: text}}
}

func (m *marker) keep(n *html.Node) {
	for ; n != nil && !m.kept[n]; n = n.Parent {
		m.kept[n] = true
	}
}

// visit goes on to n of the later page, in its section's box: what is gone before it goes in first.
func (m *marker) visit(n, box *html.Node) {
	m.flush(n, box)
	m.last, m.lastBox, m.tail = n, box, nil
}

func (m *marker) place(sec int, n *html.Node) {
	if m.cur == nil || m.cur.sec != sec {
		m.cur = &group{sec: sec, nodes: nil}
		m.groups = append(m.groups, m.cur)
	}
	if !slices.Contains(m.cur.nodes, n) {
		m.cur.nodes = append(m.cur.nodes, n)
	}
}

func (m *marker) frame(sec int, n *html.Node, class string) {
	addClass(n, class)
	m.place(sec, n)
}

// gone puts what is gone of the earlier page in after what was gone through last: the element under its section's box
// that holds it copied, when that holds nothing the later keeps, else its words in a box of words gone.
func (m *marker) gone(sec int, b *block, box *html.Node) {
	under := childOf(b.node, box)
	whole := under != nil && !m.kept[under]
	if whole && m.copied[under] {
		return
	}
	if whole {
		m.copied[under] = true
		c := m.copyOld(under)
		addClass(c, classGone)
		c.InsertBefore(m.said(), c.FirstChild)
		m.pending = append(m.pending, c)
		m.place(sec, c)
		return
	}
	var words *html.Node
	if n := len(m.pending); n > 0 && m.pending[n-1].DataAtom == atom.Div && hasClass(m.pending[n-1], classGone) && m.pending[n-1].FirstChild != nil && hasClass(m.pending[n-1].FirstChild, classSr) {
		words = m.pending[n-1]
	} else {
		words = element(atom.Div, classGone)
		words.AppendChild(m.said())
		m.pending = append(m.pending, words)
	}
	if b.text != "" {
		p := element(atom.P, "")
		del := element(atom.Del, classDel)
		del.AppendChild(text(b.text))
		p.AppendChild(del)
		words.AppendChild(p)
	}
	m.place(sec, words)
}

// said is the word that tells what is gone, for a screen reader alone.
func (m *marker) said() *html.Node {
	sr := element(atom.Span, classSr)
	sr.AppendChild(text(m.words.Gone + " "))
	return sr
}

// before puts the earlier version of a diagram or image that changed under the later: its part (the nearest element
// a part draws) copied in a details, after what was gone through last.
func (m *marker) before(sec int, n *html.Node) {
	part := partOf(n)
	d := element(atom.Details, classBefore)
	summary := element(atom.Summary, "")
	summary.AppendChild(text(m.words.Before))
	d.AppendChild(summary)
	d.AppendChild(m.copyOld(part))
	m.pending = append(m.pending, d)
	m.place(sec, d)
	m.flush(nil, nil)
}

// partOf is the nearest element a part draws that holds n, n itself for none: a diagram's frame clips what is
// drawn around the diagram.
func partOf(n *html.Node) *html.Node {
	for p := n; p != nil; p = p.Parent {
		if _, ok := attrOf(p, "data-floe-part"); ok && p.Type == html.ElementNode {
			return p
		}
	}
	return n
}

// flush puts in what is pending: after what was put in last, else after the element under its box that holds what was
// gone through last, else before the one that holds next, else at the end of body.
func (m *marker) flush(next, nextBox *html.Node) {
	if len(m.pending) == 0 {
		return
	}
	var parent, before *html.Node
	switch {
	case m.tail != nil:
		parent, before = m.tail.Parent, m.tail.NextSibling
	case m.last != nil:
		if under := childOf(m.last, m.lastBox); under != nil {
			for under.NextSibling != nil && m.added[under.NextSibling] {
				under = under.NextSibling
			}
			parent, before = under.Parent, under.NextSibling
		} else {
			parent, before = m.lastBox, nil
		}
	case next != nil:
		if under := childOf(next, nextBox); under != nil {
			parent, before = under.Parent, under
		} else {
			parent, before = nextBox, nextBox.FirstChild
		}
	default:
		parent, before = m.body, nil
	}
	for _, n := range m.pending {
		parent.InsertBefore(n, before)
		m.added[n] = true
	}
	m.tail = m.pending[len(m.pending)-1]
	m.pending = nil
}

// childOf is the element that holds n (n itself too) whose parent is box: nil when box holds n not.
func childOf(n, box *html.Node) *html.Node {
	for ; n != nil; n = n.Parent {
		if n.Parent == box {
			return n
		}
	}
	return nil
}

// copyOld is n of the earlier page copied, its scripts left out, its ids and the names of its inputs, and all that
// names them, apart from the later page's.
func (m *marker) copyOld(n *html.Node) *html.Node {
	c := deepCopy(n)
	var scripts []*html.Node
	for d := range c.Descendants() {
		if d.Type == html.ElementNode && d.Data == "script" {
			scripts = append(scripts, d)
		}
	}
	for _, s := range scripts {
		s.Parent.RemoveChild(s)
	}
	renumber(c, func(_ int, id string) string { return "floe-diff.old." + id })
	return c
}

// edit is a change to a text node: the bytes [at, end) wrapped in an ins, or del's words put in at at.
type edit struct {
	n       *html.Node
	at, end int
	del     string
}

// markWords marks the words of spans in the later page, whose words from tells: those added wrapped in an ins, those
// gone put in a del where they were. False when there is nothing of them to mark, or one cannot be marked, as words
// from no text node or a mark not held where it goes, which leaves the words as they were: all or none.
func (m *marker) markWords(from []origin, spans []Span) bool {
	var edits []edit
	pos := 0
	for _, s := range spans {
		n := utf8.RuneCountInString(s.Text)
		switch s.Op {
		case opSame:
			pos += n
		case opIns:
			for i := pos; i < pos+n; i++ {
				o := from[i]
				if o.n == nil {
					continue
				}
				size := runeSize(o)
				if k := len(edits) - 1; k >= 0 && edits[k].del == "" && edits[k].n == o.n && edits[k].end == o.at {
					edits[k].end += size
				} else {
					edits = append(edits, edit{n: o.n, at: o.at, end: o.at + size, del: ""})
				}
			}
			pos += n
		case opDel:
			at, ok := point(from, pos)
			if !ok {
				return false
			}
			at.del = s.Text
			edits = append(edits, at)
		}
	}
	if len(edits) == 0 {
		return false
	}
	for _, e := range edits {
		tag := "ins"
		if e.del != "" {
			tag = "del"
		}
		if !m.holds(e.n.Parent, tag) {
			return false
		}
	}
	// From the end of each text node, so that the places before stay where they are; at one place the wrap first,
	// which the del then goes before.
	slices.SortStableFunc(edits, func(x, y edit) int {
		return cmp.Or(cmp.Compare(y.at, x.at), cmp.Compare(len(x.del), len(y.del)))
	})
	for _, e := range edits {
		rest := e.n.Data[e.end:]
		if e.del != "" {
			rest = e.n.Data[e.at:]
		}
		mid := e.n.Data[e.at:e.end]
		e.n.Data = e.n.Data[:e.at]
		after := e.n.NextSibling
		var put *html.Node
		if e.del != "" {
			put = element(atom.Del, classDel)
			put.AppendChild(text(e.del))
		} else {
			put = element(atom.Ins, classIns)
			put.AppendChild(text(mid))
		}
		e.n.Parent.InsertBefore(put, after)
		if rest != "" {
			e.n.Parent.InsertBefore(text(rest), after)
		}
	}
	return true
}

// point is where words gone at pos go: before the next rune of a text node, else after the one before.
func point(from []origin, pos int) (edit, bool) {
	for i := pos; i < len(from); i++ {
		if o := from[i]; o.n != nil {
			return edit{n: o.n, at: o.at, end: o.at, del: ""}, true
		}
	}
	for i := min(pos, len(from)) - 1; i >= 0; i-- {
		if o := from[i]; o.n != nil {
			end := o.at + runeSize(o)
			return edit{n: o.n, at: end, end: end, del: ""}, true
		}
	}
	return edit{n: nil, at: 0, end: 0, del: ""}, false
}

func runeSize(o origin) int {
	_, size := utf8.DecodeRuneInString(o.n.Data[o.at:])
	return size
}

// holds tells whether parent holds an element tag put in as its child, as the HTML parser reads the page again: it
// reads parent's elements down from the root with tag in the last, and the tag must come back an HTML element in
// parent. Those it does not: in a table but not a cell, in select, textarea or title, in svg or MathML.
func (m *marker) holds(parent *html.Node, tag string) bool {
	key := holdKey{parent: parent, tag: tag}
	if got, ok := m.held[key]; ok {
		return got
	}
	var chain []*html.Node
	for n := parent; n != nil && n.Type == html.ElementNode; n = n.Parent {
		chain = append(chain, n)
	}
	slices.Reverse(chain)
	var b strings.Builder
	for _, n := range chain {
		b.WriteString("<" + n.Data)
		for _, a := range n.Attr {
			if a.Namespace == "" {
				b.WriteString(" " + a.Key + `="` + html.EscapeString(a.Val) + `"`)
			}
		}
		b.WriteString(">")
	}
	b.WriteString("<" + tag + " data-floe-probe></" + tag + ">")
	got := false
	if root, err := html.Parse(strings.NewReader(b.String())); err == nil {
		for n := range root.Descendants() {
			if _, probe := attrOf(n, "data-floe-probe"); probe && n.Type == html.ElementNode {
				got = n.Data == tag && n.Namespace == "" && sameChain(n.Parent, chain)
				break
			}
		}
	}
	m.held[key] = got
	return got
}

func sameChain(n *html.Node, chain []*html.Node) bool {
	for i := len(chain) - 1; i >= 0; i-- {
		if n == nil || n.Type != html.ElementNode || n.Data != chain[i].Data || n.Namespace != chain[i].Namespace {
			return false
		}
		n = n.Parent
	}
	return n != nil && n.Type == html.DocumentNode
}

// number puts a mark to go to in each place, in the order of the page, and numbers them so: the number of each
// section's first, -1 for none. A place with no spot to hold a span has none. The places in a tab not picked are one,
// at the tab's title; a details that holds a place is opened, and its name taken off so that all of them stay so.
func (m *marker) number() []int {
	order := map[*html.Node]int{}
	i := 0
	for n := range m.root.Descendants() {
		order[n] = i
		i++
	}
	first := func(g *group) int {
		least := len(order)
		for _, n := range g.nodes {
			least = min(least, order[n])
		}
		return least
	}
	slices.SortStableFunc(m.groups, func(x, y *group) int { return cmp.Compare(first(x), first(y)) })
	at := map[*group]*html.Node{}
	for _, g := range m.groups {
		nodes := slices.Clone(g.nodes)
		slices.SortStableFunc(nodes, func(x, y *html.Node) int { return cmp.Compare(order[x], order[y]) })
		if len(nodes) == 0 {
			continue
		}
		m.reveal(nodes)
		at[g] = m.anchor(nodes)
	}
	numbers := map[*html.Node]int{}
	for n := range m.root.Descendants() {
		if n.Type == html.ElementNode && hasClass(n, classAt) {
			n.Attr = append(n.Attr, html.Attribute{Namespace: "", Key: "id", Val: "floe-diff." + strconv.Itoa(m.count)})
			numbers[n] = m.count
			m.count++
		}
	}
	places := map[int]int{}
	for _, g := range m.groups {
		if s := at[g]; s != nil {
			if k, ok := places[g.sec]; !ok || numbers[s] < k {
				places[g.sec] = numbers[s]
			}
		}
	}
	out := make([]int, 0)
	for sec, k := range places {
		for len(out) <= sec {
			out = append(out, -1)
		}
		out[sec] = k
	}
	return out
}

// reveal shows what is marked, on either page: a details that holds a mark is opened (its name taken off, so that all
// of them stay so), and the title of a tab not picked that holds one is framed, as what it holds does not show.
func (m *marker) reveal(nodes []*html.Node) {
	for _, n := range nodes {
		// What floe put in opens nothing of its own: an earlier diagram's details stays shut.
		d := n
		if m.added[n] {
			d = n.Parent
		}
		openDetails(d)
		if label := hiddenTab(n); label != nil {
			addClass(label, classChange)
		}
	}
}

// anchor is the mark to go to of a place of nodes, put in on either page: in a tab not picked, at the tab's title,
// one for all its places; else before the first words of the nodes a span can go before, or just before the part of
// one that holds none, as a diagram; nil for none.
func (m *marker) anchor(nodes []*html.Node) *html.Node {
	if label := hiddenTab(nodes[0]); label != nil {
		if m.labels[label] == nil {
			m.labels[label] = m.spot(label)
		}
		return m.labels[label]
	}
	for _, n := range nodes {
		if s := m.spot(n); s != nil {
			return s
		}
	}
	for _, n := range nodes {
		if part := partOf(n); !m.added[n] && part.Parent != nil && m.holds(part.Parent, "span") {
			s := element(atom.Span, classAt)
			part.Parent.InsertBefore(s, part)
			return s
		}
	}
	return nil
}

// spot puts the mark to go to of a place in n: before its first words held where a span can be, or first in a box
// floe put in. nil for none.
func (m *marker) spot(n *html.Node) *html.Node {
	at := element(atom.Span, classAt)
	if m.added[n] {
		n.InsertBefore(at, n.FirstChild)
		return at
	}
	for d := range withSelf(n) {
		if d.Type == html.TextNode && strings.TrimFunc(d.Data, unicode.IsSpace) != "" && d.Parent != nil && m.holds(d.Parent, "span") {
			d.Parent.InsertBefore(at, d)
			return at
		}
	}
	return nil
}

func withSelf(n *html.Node) func(func(*html.Node) bool) {
	return func(yield func(*html.Node) bool) {
		if !yield(n) {
			return
		}
		for d := range n.Descendants() {
			if !yield(d) {
				return
			}
		}
	}
}

// hiddenTab is the title of the outermost tab of floe's tabs n is in that is not picked: nil for none.
func hiddenTab(n *html.Node) *html.Node {
	var label *html.Node
	for c := n; c != nil && c.Parent != nil; c = c.Parent {
		p := c.Parent
		if v, _ := attrOf(p, "data-floe-part"); p.DataAtom != atom.Form || v != "tabs" || c.DataAtom == atom.Label {
			continue
		}
		for l := c.PrevSibling; l != nil; l = l.PrevSibling {
			if l.Type == html.ElementNode && l.DataAtom == atom.Label {
				if !picked(l) {
					label = l
				}
				break
			}
		}
	}
	return label
}

func picked(label *html.Node) bool {
	for d := range label.Descendants() {
		if _, ok := attrOf(d, "checked"); ok && d.DataAtom == atom.Input {
			return true
		}
	}
	return false
}

func openDetails(n *html.Node) {
	for p := n; p != nil; p = p.Parent {
		if p.Type != html.ElementNode || p.DataAtom != atom.Details || p.Namespace != "" {
			continue
		}
		p.Attr = slices.DeleteFunc(p.Attr, func(a html.Attribute) bool { return a.Namespace == "" && (a.Key == "name" || a.Key == "open") })
		p.Attr = append(p.Attr, html.Attribute{Namespace: "", Key: "open", Val: ""})
	}
}

func element(a atom.Atom, class string) *html.Node {
	n := &html.Node{Type: html.ElementNode, DataAtom: a, Data: a.String()}
	if class != "" {
		n.Attr = []html.Attribute{{Namespace: "", Key: "class", Val: class}}
	}
	return n
}

func text(s string) *html.Node {
	return &html.Node{Type: html.TextNode, Data: s}
}

func attrOf(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func hasClass(n *html.Node, class string) bool {
	v, _ := attrOf(n, "class")
	return slices.Contains(strings.Fields(v), class)
}

func addClass(n *html.Node, class string) {
	for i, a := range n.Attr {
		if a.Namespace == "" && a.Key == "class" {
			if !slices.Contains(strings.Fields(a.Val), class) {
				n.Attr[i].Val = strings.TrimSpace(a.Val + " " + class)
			}
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Namespace: "", Key: "class", Val: class})
}

// withStyle puts the style of the marks at the end of the page's head.
func withStyle(root *html.Node) {
	for n := range root.Descendants() {
		if n.Type == html.ElementNode && n.DataAtom == atom.Head && n.Namespace == "" {
			s := element(atom.Style, "")
			s.AppendChild(text(markStyle))
			n.AppendChild(s)
			return
		}
	}
}
