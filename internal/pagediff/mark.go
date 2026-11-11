package pagediff

import (
	"bytes"
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"floe/internal/page"
	"floe/internal/usererr"
)

// Words are the words Diff writes on a page, in the language of the one who reads it: over what is gone, and over a
// diagram or image's earlier version.
type Words struct {
	Gone   string
	Before string
}

// Diff compares the page from to the page to, as drawn, and answers both of one comparison: what differs, in words
// (Result, for the contents and counts of the management page), and the page to marked, for the sandbox alone: an
// error for one the HTML parser gives up on. The page marked has the words added and gone in their places, a block
// framed only when its words cannot show what differs, what is gone after the part it was in (in a table, where it
// was) and an earlier diagram or image under the later. Each place is numbered floe-diff.<k>, in the order of the
// page, as the Places of the Result. The page's own elements keep their ids and attributes but the classes added, and
// the open and name of a details that holds a place.
func Diff(from, to []byte, words Words) (Result, []byte, error) {
	result, tree, err := marked(from, to, pageMost, words)
	if err != nil {
		return Result{}, nil, err
	}
	withStyle(tree)
	var out bytes.Buffer
	if err := html.Render(&out, tree); err != nil {
		return Result{}, nil, usererr.Library(err) //nolint:wrapcheck // the library's words
	}
	return result, out.Bytes(), nil
}

// marked compares the pages and marks the later's tree: the one comparison both of Diff's answers are of. The earlier
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
	w := newWork(most)
	c := compare(a, b, w)
	m := newMarker(treeB, words)
	m.tables = w.tables
	m.mark(treeA, a, b, c)
	if m.copies > 0 {
		carryStyles(treeA, treeB)
	}
	places, sections := m.number()
	c.result.Places = sections
	for i, k := range places {
		if k >= 0 {
			c.result.Sections[i].Place = &k
		}
	}
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
	classOut    = "floe-diff-out"
	classBefore = "floe-diff-before"
	classAt     = "floe-diff-at"
	markStyle   = `.floe-diff-ins{background:rgba(26,127,55,.18);color:inherit;
  text-decoration:underline 2px #1a7f37;text-underline-offset:3px}
.floe-diff-del{background:rgba(207,34,46,.18);color:inherit;text-decoration:line-through 2px #cf222e}
.floe-diff-ch{outline:2px solid #d4a017;outline-offset:4px}
.floe-diff-add{outline:2px solid #1a7f37;outline-offset:4px}
tr.floe-diff-ch,tr.floe-diff-add{outline-offset:-2px}
.floe-diff-out{outline:2px dashed #cf222e;outline-offset:4px}
.floe-diff-sr{position:absolute;width:1px;height:1px;margin:-1px;padding:0;overflow:hidden;
  clip:rect(0 0 0 0);white-space:nowrap;border:0}
.floe-diff-before{margin:12px 0;padding:6px 12px 12px;border:2px solid #d4a017;border-radius:8px;
  background:#fffbea;color:#24292f}
.floe-diff-before>summary{font:600 13px system-ui,sans-serif;color:#7a5900;cursor:pointer;padding:4px 0}
.floe-diff-at{scroll-margin-top:30vh}`
)

// marker marks the later page's tree, going through the comparison in order.
type marker struct {
	root, body *html.Node
	words      Words
	// last is the later's element gone through last, in box, its section's box; tail what was put in after it last.
	last, lastBox, tail *html.Node
	// pending waits to go in after last; depth is the depth of the part the first of it was in the earlier, 0 for none.
	pending []*html.Node
	depth   int
	// added is what floe put in; kept, the earlier's elements that hold a block the later keeps; copied, those copied.
	added, kept, copied map[*html.Node]bool
	held                map[holdKey]bool
	ordinals            map[*html.Node]map[*html.Node]int
	edits               []edit
	gones               []*goneCopy
	tables
	// rows tells of each table of the earlier whose rows are paired the later's it is; lastRow, the later's row of
	// each table gone through last; waiting, rows gone of a table before any of its rows was gone through.
	rows    map[*html.Node]*html.Node
	lastRow map[*html.Node]*html.Node
	waiting map[*html.Node][]waitingRow
	branch  *goneCopy
	copies  int
	labels  map[*html.Node]*html.Node
	groups  []*group
	cur     *group
}

// group is a place: what is marked one after another in a section, of its result's section sec.
type group struct {
	sec   int
	nodes []*html.Node
	has   map[*html.Node]bool
}

// holdKey is a question to the parser: whether a parent, of the start tags of it and its ancestors, holds tag.
type holdKey struct {
	starts string
	tag    string
}

func newMarker(root *html.Node, words Words) *marker {
	return &marker{
		root: root, body: bodyOf(root), words: words, last: nil, lastBox: nil, tail: nil, pending: nil, depth: 0,
		added: map[*html.Node]bool{}, kept: map[*html.Node]bool{}, copied: map[*html.Node]bool{},
		held: map[holdKey]bool{}, edits: nil, ordinals: map[*html.Node]map[*html.Node]int{}, gones: nil, branch: nil,
		copies: 0, tables: tables{}, rows: map[*html.Node]*html.Node{}, lastRow: map[*html.Node]*html.Node{},
		waiting: map[*html.Node][]waitingRow{}, labels: map[*html.Node]*html.Node{}, groups: nil, cur: nil,
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

// mark marks the later page, walking the comparison's sections and blocks in order.
func (m *marker) mark(treeA *html.Node, a, b []section, c compared) {
	boxA, boxB := boxes(a, bodyOf(treeA)), boxes(b, m.body)
	m.keepPaired(a, c.steps)
	for i, st := range c.steps {
		m.cur = nil
		switch {
		case st.to < 0:
			m.goneSection(i, a[st.from], boxA[st.from])
		case st.from < 0:
			m.addedSection(i, b[st.to], boxB[st.to])
		default:
			m.changed(i, a[st.from], b[st.to], boxA[st.from], boxB[st.to], st.pairs, c.result.Sections[i].Heading)
		}
	}
	m.flush(nil, nil)
	m.writeWords()
	m.trim()
}

// keepPaired keeps the earlier page's blocks and headings that are paired, as they are not gone, and the tables whose
// rows' cells are compared.
func (m *marker) keepPaired(a []section, steps []step) {
	for _, st := range steps {
		for _, p := range st.pairs {
			if p.from != nil && p.to != nil {
				m.keep(p.from.node)
				if m.cellsCompared(p.from, p.to) {
					m.rows[tableOf(p.from.node)] = tableOf(p.to.node)
				}
			}
		}
		if st.from >= 0 && st.to >= 0 {
			m.keep(a[st.from].head)
		}
	}
}

func (m *marker) goneSection(sec int, from section, box *html.Node) {
	if from.head != nil {
		m.gone(sec, &block{kind: "", text: from.heading, key: "", node: from.head, from: nil, cells: nil}, box)
	}
	for j := range from.blocks {
		m.gone(sec, &from.blocks[j], box)
	}
}

func (m *marker) addedSection(sec int, to section, box *html.Node) {
	if to.head != nil {
		m.visit(to.head, box)
		m.show(sec, to.head, "", to.from, added(to.heading), classAdd)
	}
	for j := range to.blocks {
		blk := &to.blocks[j]
		m.visit(blk.node, box)
		m.show(sec, blk.node, blk.kind, blk.from, added(blk.text), classAdd)
	}
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
			m.rowAt(p.to)
			m.show(sec, p.to.node, p.to.kind, p.to.from, added(p.to.text), classAdd)
		case p.from.key == p.to.key:
			m.visit(p.to.node, boxB)
			m.rowAt(p.to)
			m.cur = nil
		case p.cells != nil:
			m.visit(p.to.node, boxB)
			m.rowAt(p.to)
			m.showCells(sec, p.to.node, p.cells)
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
	m.last, m.lastBox, m.tail, m.branch = n, box, nil, nil
}

func (m *marker) place(sec int, n *html.Node) {
	if m.cur == nil || m.cur.sec != sec {
		m.cur = &group{sec: sec, nodes: nil, has: map[*html.Node]bool{}}
		m.groups = append(m.groups, m.cur)
	}
	if !m.cur.has[n] {
		m.cur.has[n] = true
		m.cur.nodes = append(m.cur.nodes, n)
	}
}

func (m *marker) frame(sec int, n *html.Node, class string) {
	addClass(n, class)
	m.place(sec, n)
}

// gone puts what is gone of the earlier page in after what was gone through last, as it was: the element under its
// section's box that holds it (the part) copied, whole when the later keeps nothing of it, else the branches to what
// is gone alone, copied as they are found. What is gone of one part one after another goes in one copy.
func (m *marker) gone(sec int, b *block, box *html.Node) {
	if b.kind == "tr" {
		if later := m.rows[tableOf(b.node)]; later != nil {
			if last := m.lastRow[later]; last == nil {
				m.waiting[later] = append(m.waiting[later], waitingRow{sec: sec, row: b, box: box})
			} else if !m.putRow(sec, b.node, last, true) {
				m.copyGone(sec, b, box)
			}
			return
		}
	}
	m.copyGone(sec, b, box)
}

// copyGone puts what is gone in as its part copied, as gone tells.
func (m *marker) copyGone(sec int, b *block, box *html.Node) {
	under := childOf(b.node, m.boxOf(b.node, box))
	if under == nil {
		under = b.node
	}
	if c := m.branch; c != nil && c.under == under && len(m.pending) > 0 && m.pending[len(m.pending)-1] == c.root {
		c.add(b.node)
		m.place(sec, c.root)
		return
	}
	// Words of the part itself, between its blocks, are not of a branch: the part goes whole.
	whole := !m.kept[under] || b.node == under
	if whole && m.copied[under] {
		return
	}
	m.copied[under] = true
	if len(m.pending) == 0 {
		m.depth = depth(under)
	}
	c := &goneCopy{
		under: under, root: nil, of: nil, back: map[*html.Node]*html.Node{}, gone: nil, whole: whole,
		places: map[*html.Node]map[*html.Node]int{},
	}
	if whole {
		c.root, c.of = copyTree(under)
	} else {
		c.root = shallow(under)
		c.of = map[*html.Node]*html.Node{under: c.root}
	}
	c.add(b.node)
	m.gones = append(m.gones, c)
	m.branch = c
	m.pending = append(m.pending, c.root)
	m.place(sec, c.root)
}

// boxOf is the box n, gone, is copied under: box, else its nearest ancestor that holds n (a block of the section out
// of its heading's box), and past one of which nothing is kept, its parent, so that a box gone whole goes as it was.
func (m *marker) boxOf(n, box *html.Node) *html.Node {
	for box.Parent != nil && box.DataAtom != atom.Body && (childOf(n, box) == nil || !m.kept[box]) {
		box = box.Parent
	}
	return box
}

// depth is the number of n's ancestors.
func depth(n *html.Node) int {
	d := 0
	for a := n.Parent; a != nil; a = a.Parent {
		d++
	}
	return d
}

// waitingRow is a row gone of a table, at sec in its section's box, to put in before the first row of it gone through.
type waitingRow struct {
	sec int
	row *block
	box *html.Node
}

// rowAt goes through a row of the later page: the rows gone of its table before it go in before it, and those gone
// after it go after it.
func (m *marker) rowAt(b *block) {
	if b.kind != "tr" {
		return
	}
	later := tableOf(b.node)
	for _, w := range m.waiting[later] {
		if !m.putRow(w.sec, w.row.node, b.node, false) {
			m.copyGone(w.sec, w.row, w.box)
		}
	}
	delete(m.waiting, later)
	m.lastRow[later] = b.node
}

// putRow puts row, gone, in the later's table where it was: after the row at (before it, for after false), in the
// group of rows of row's kind (thead, tbody, tfoot); one of a kind the later's table has not, last in its thead or
// first in its first tbody. Its words go struck through, as all words gone. False when there is no such group, or the
// HTML parser holds no tr there: then it goes in as other parts gone, after the table.
// rowGroup is where a row of a group of rows kind goes in table, where the row it was by is in another group: the
// table's first of kind, at its start (a head row at its end), or else at the start of its first tbody.
func rowGroup(table *html.Node, kind string) (parent, before *html.Node) {
	for g := range rowGroups(table) {
		if g.Data == kind {
			if kind == "thead" {
				return g, nil
			}
			return g, g.FirstChild
		}
	}
	for g := range rowGroups(table) {
		if g.DataAtom == atom.Tbody {
			return g, g.FirstChild
		}
	}
	return nil, nil
}

func (m *marker) putRow(sec int, row, at *html.Node, after bool) bool {
	kind := row.Parent.Data
	parent, before := at.Parent, at
	if after {
		before = at.NextSibling
		for before != nil && m.added[before] {
			before = before.NextSibling
		}
	}
	if parent.Data != kind {
		parent, before = rowGroup(tableOf(at), kind)
	}
	if parent == nil || !m.holds(parent, "tr") {
		return false
	}
	c := m.copyOld(row)
	parent.InsertBefore(c, before)
	m.added[c] = true
	m.showGone(c, nil)
	m.place(sec, c)
	if after {
		m.lastRow[tableOf(at)] = c
	}
	return true
}

// showCells marks the cells of a row paired: those alike by their words, those added by theirs, those gone put in
// where they were, their words struck through. The row is framed when a mark cannot be held.
func (m *marker) showCells(sec int, row *html.Node, cells []cellPair) {
	ok := true
	for i, c := range cells {
		switch {
		case c.from != nil && c.to != nil && c.spans != nil:
			ok = m.markWords(c.to.from, c.spans) && ok
		case c.from == nil && strings.TrimFunc(c.to.text, page.IsSpace) != "":
			ok = m.markWords(c.to.from, added(c.to.text)) && ok
		case c.to == nil:
			var before *html.Node
			for _, next := range cells[i+1:] {
				if next.to != nil {
					before = next.to.node
					break
				}
			}
			if !m.holds(row, c.from.node.Data) {
				ok = false
				continue
			}
			put := m.copyOld(c.from.node)
			row.InsertBefore(put, before)
			m.added[put] = true
			m.showGone(put, nil)
		}
	}
	if !ok {
		addClass(row, classChange)
	}
	m.place(sec, row)
}

// strike wraps the words in n, put in the page, in a del, where one is held: words gone, shown as words gone are.
func (m *marker) strike(n *html.Node, apart map[*html.Node]bool) {
	var words []*html.Node
	for d := range n.Descendants() {
		if d.Type == html.TextNode && strings.TrimFunc(d.Data, page.IsSpace) != "" && !within(d, apart) {
			words = append(words, d)
		}
	}
	for _, t := range words {
		if !m.holds(t.Parent, "del") {
			continue
		}
		del := element(atom.Del, classDel)
		t.Parent.InsertBefore(del, t)
		t.Parent.RemoveChild(t)
		del.AppendChild(t)
	}
}

// within tells whether n is in one of the nodes of set.
func within(n *html.Node, set map[*html.Node]bool) bool {
	for ; n != nil && len(set) > 0; n = n.Parent {
		if set[n] {
			return true
		}
	}
	return false
}

// goneCopy is the copy of a part of the earlier page that holds what is gone: under, copied as root, of telling the
// copy of each node copied, back the node each copy is of; gone, the blocks of it gone; whole when the later keeps
// nothing of it.
type goneCopy struct {
	under, root *html.Node
	of, back    map[*html.Node]*html.Node
	gone        []*html.Node
	whole       bool
	// places are the places of the children of each parent of the earlier page a child of which is copied.
	places map[*html.Node]map[*html.Node]int
}

// add copies old, gone, into the copy, with the branch down to it, and the title of the tab of floe's tabs it is in.
func (c *goneCopy) add(old *html.Node) {
	c.gone = append(c.gone, old)
	c.branch(old)
	if label := tabOf(old); label != nil {
		c.branch(label)
	}
}

// branch copies old into the copy, all in it, and the elements on the way down to it from the part alone, each in its
// place among those copied by the earlier page's order. Nothing for one copied already, or a copy whole.
func (c *goneCopy) branch(old *html.Node) {
	if c.whole || c.of[old] != nil {
		return
	}
	var path []*html.Node
	for a := old.Parent; a != c.under; a = a.Parent {
		if a == nil {
			return
		}
		path = append(path, a)
	}
	slices.Reverse(path)
	for _, a := range path {
		if c.of[a] == nil {
			n := shallow(a)
			c.of[a], c.back[n] = n, a
			c.insert(n, a)
		}
	}
	n, of := copyTree(old)
	for k, v := range of {
		c.of[k], c.back[v] = v, k
	}
	c.insert(n, old)
}

// insert puts n, the copy of old, under the copy of old's parent, before the first copied there that comes after old.
func (c *goneCopy) insert(n, old *html.Node) {
	parent := c.of[old.Parent]
	place := c.places[old.Parent]
	if place == nil {
		place = map[*html.Node]int{}
		i := 0
		for k := old.Parent.FirstChild; k != nil; k = k.NextSibling {
			place[k] = i
			i++
		}
		c.places[old.Parent] = place
	}
	var before *html.Node
	for k := parent.FirstChild; k != nil; k = k.NextSibling {
		if o, ok := c.back[k]; ok && place[o] > place[old] {
			before = k
			break
		}
	}
	parent.InsertBefore(n, before)
}

// trim shows each copy of what is gone as marks are shown: a details on its way opened, the tab of floe's tabs it is
// in picked, as the tabs show the panel after the title picked alone. It is set apart from the page (apart); one that
// holds a diagram or image is framed, as the line through its words does not show on them. The word for a screen
// reader goes first.
// LIMIT: the rows gone of a table whose cells span rows or columns are copied after it, with no head, and a block of
// words between others in one element copies the whole element: a head or the words apart would be a rule more.
func (m *marker) trim() {
	for _, c := range m.gones {
		for _, old := range c.gone {
			if n := c.of[old]; n != nil {
				openDetails(n)
			}
			if label := tabOf(old); label != nil && c.of[label] != nil {
				pick(c.of[label])
			}
		}
		m.numberItems(c)
		m.apart(c.root)
		// A tab's title copied only to show the tab is not gone.
		titles := map[*html.Node]bool{}
		for _, old := range c.gone {
			if label := tabOf(old); label != nil && c.of[label] != nil && !slices.Contains(c.gone, label) {
				titles[c.of[label]] = true
			}
		}
		m.showGone(c.root, titles)
	}
}

// showGone shows n, what is gone put in, as all that is gone is shown: its words struck through in a del where one is
// held (but those in apart, copied with it to show it), framed when it holds a diagram or image, which a line through
// words does not show on, and the word for a screen reader first.
func (m *marker) showGone(n *html.Node, apart map[*html.Node]bool) {
	addClass(n, classGone)
	m.strike(n, apart)
	if holdsFigure(n) {
		addClass(n, classOut)
	}
	m.put(n, m.said())
}

// numberItems gives each item copied of an ol the number it had in the earlier page, as its value, so that an item gone
// reads as it was numbered, whatever was left out before it; a reversed ol copied counts by those values alone.
func (m *marker) numberItems(c *goneCopy) {
	for old, n := range c.of {
		if n.Type != html.ElementNode || n.Namespace != "" {
			continue
		}
		switch n.DataAtom {
		case atom.Ol:
			page.RemoveAttr(n, "reversed")
		case atom.Li:
			if owner := listOwner(old); owner != nil && owner.DataAtom == atom.Ol {
				if m.ordinals[owner] == nil {
					m.ordinals[owner] = ordinals(owner)
				}
				page.SetAttr(n, "value", strconv.Itoa(m.ordinals[owner][old]))
			}
		default:
		}
	}
}

// listOwner is the list owner of an li, by the steps of HTML's "list owner" (the li element): its closest ol, ul or
// menu ancestor, else its parent.
// LIMIT: an li not rendered has none in HTML; one here is still numbered.
func listOwner(li *html.Node) *html.Node {
	for a := li.Parent; a != nil; a = a.Parent {
		if a.Type == html.ElementNode && a.Namespace == "" &&
			(a.DataAtom == atom.Ol || a.DataAtom == atom.Ul || a.DataAtom == atom.Menu) {
			return a
		}
	}
	return li.Parent
}

// ordinals are the ordinal values of the owned list items of owner, an ol, by the steps of HTML's "ordinal value" (the
// li element) and "starting value" (the ol element): numbering from the ol's starting value (its start parsed, else
// the number of its owned items when reversed, else 1); each item in tree order takes its value parsed when it has
// one, then numbering goes down when the ol is reversed, else up.
func ordinals(owner *html.Node) map[*html.Node]int {
	var items []*html.Node
	for d := range owner.Descendants() {
		if d.Type == html.ElementNode && d.Namespace == "" && d.DataAtom == atom.Li && listOwner(d) == owner {
			items = append(items, d)
		}
	}
	_, reversed := page.Attr(owner, "reversed")
	numbering := 1
	if reversed {
		numbering = len(items)
	}
	if start, ok := page.Attr(owner, "start"); ok {
		if n, ok := htmlInteger(start); ok {
			numbering = n
		}
	}
	out := make(map[*html.Node]int, len(items))
	for _, item := range items {
		if v, ok := page.Attr(item, "value"); ok {
			if n, ok := htmlInteger(v); ok {
				numbering = n
			}
		}
		out[item] = numbering
		if reversed {
			numbering--
		} else {
			numbering++
		}
	}
	return out
}

// htmlInteger reads s by the steps of HTML's "rules for parsing integers": ASCII whitespace skipped, a - or + sign,
// then the digits up to the first that is not one, negated for -. False for no digits (the rules' error), or a number
// past an int.
func htmlInteger(s string) (int, bool) {
	s = strings.TrimLeftFunc(s, page.IsSpace)
	sign := 1
	if strings.HasPrefix(s, "-") {
		sign, s = -1, s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0, false
	}
	return sign * n, true
}

// tabTitles are the titles of the tabs of floe's tabs n is in, the nearest first.
func tabTitles(n *html.Node) []*html.Node {
	var titles []*html.Node
	for c := n; c != nil && c.Parent != nil; c = c.Parent {
		v, _ := page.Attr(c.Parent, "data-floe-part")
		if c.Parent.DataAtom != atom.Form || v != "tabs" || c.DataAtom == atom.Label {
			continue
		}
		for l := c.PrevSibling; l != nil; l = l.PrevSibling {
			if l.Type == html.ElementNode && l.DataAtom == atom.Label {
				titles = append(titles, l)
				break
			}
		}
	}
	return titles
}

// tabOf is the title of the nearest tab of floe's tabs n is in, nil for none.
func tabOf(n *html.Node) *html.Node {
	if titles := tabTitles(n); len(titles) > 0 {
		return titles[0]
	}
	return nil
}

// pick picks the tab whose title is label: its radio checked, the others of its tabs (the titles beside it) not.
func pick(label *html.Node) {
	for l := label.Parent.FirstChild; l != nil; l = l.NextSibling {
		if r := tabRadio(l); r != nil {
			page.RemoveAttr(r, "checked")
		}
	}
	if r := tabRadio(label); r != nil {
		page.SetAttr(r, "checked", "")
	}
}

// tabRadio is the radio of a tab's title l of floe's tabs, its child; nil for none.
func tabRadio(l *html.Node) *html.Node {
	if l.Type != html.ElementNode || l.DataAtom != atom.Label {
		return nil
	}
	for c := l.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == atom.Input {
			return c
		}
	}
	return nil
}

// holdsFigure tells whether n holds a diagram or image.
func holdsFigure(n *html.Node) bool {
	for d := range withSelf(n) {
		if d.Type == html.ElementNode && (d.DataAtom == atom.Img || (d.Data == "svg" && d.Namespace == "svg")) {
			return true
		}
	}
	return false
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
	if len(m.pending) == 0 {
		m.depth = 0
	}
	m.pending = append(m.pending, d)
	m.place(sec, d)
	m.flush(nil, nil)
}

// partOf is the nearest element a part draws that holds n, n itself for none: a diagram's frame clips what is
// drawn around the diagram.
func partOf(n *html.Node) *html.Node {
	for p := n; p != nil; p = p.Parent {
		if _, ok := page.Attr(p, "data-floe-part"); ok && p.Type == html.ElementNode {
			return p
		}
	}
	return n
}

// flush puts in what is pending: after what was put in last, else after the element that holds what was gone through
// last as deep as the part pending was in the earlier (or under its box), else before the one that holds next, else at
// the end of body.
// LIMIT: the depth is the earlier page's: a page whose later version wraps its parts in an element more puts what is
// gone one level in. Floe's templates keep their frame; pair the boxes if a page comes to change it.
func (m *marker) flush(next, nextBox *html.Node) {
	if len(m.pending) == 0 {
		return
	}
	var parent, before *html.Node
	switch {
	case m.tail != nil:
		parent, before = m.tail.Parent, m.tail.NextSibling
	case m.last != nil:
		under := childOf(m.last, m.lastBox)
		if a := ancestorAt(m.last, m.depth); m.depth > 0 && a != nil {
			under = a
		}
		if under != nil {
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

// ancestorAt is n or its ancestor of depth d, nil when n is shallower.
func ancestorAt(n *html.Node, d int) *html.Node {
	for k := depth(n); k > d; k-- {
		n = n.Parent
	}
	if n == nil || depth(n) != d {
		return nil
	}
	return n
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
	c, _ := copyTree(n)
	m.apart(c)
	return c
}

// apart sets a copy of the earlier page apart from the page and the other copies: its scripts left out, its ids and
// names, and all that names them, given a head of its own, so that two copies of one tabs make no group.
func (m *marker) apart(c *html.Node) {
	var scripts []*html.Node
	for d := range c.Descendants() {
		if d.Type == html.ElementNode && d.Data == "script" {
			scripts = append(scripts, d)
		}
	}
	for _, s := range scripts {
		s.Parent.RemoveChild(s)
	}
	prefix := "floe-diff.old." + strconv.Itoa(m.copies) + "."
	m.copies++
	renumber(c, func(_ int, id string) string { return prefix + id })
}

// copyTree is n and all in it copied, and the copy of each of its nodes.
func copyTree(n *html.Node) (*html.Node, map[*html.Node]*html.Node) {
	of := map[*html.Node]*html.Node{}
	var copyOf func(n *html.Node) *html.Node
	copyOf = func(n *html.Node) *html.Node {
		c := shallow(n)
		of[n] = c
		for k := n.FirstChild; k != nil; k = k.NextSibling {
			c.AppendChild(copyOf(k))
		}
		return c
	}
	return copyOf(n), of
}

// shallow is n alone copied, its attributes with it.
func shallow(n *html.Node) *html.Node {
	c := newNode(n.Type, n.DataAtom, n.Data)
	c.Namespace, c.Attr = n.Namespace, slices.Clone(n.Attr)
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
			edits = inserted(edits, from[pos:pos+n])
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
	m.edits = append(m.edits, edits...)
	return true
}

// inserted is edits with the runes of from added: the runes of one text node one after another are one ins.
func inserted(edits []edit, from []origin) []edit {
	for _, o := range from {
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
	return edits
}

// writeWords puts in the marks of words that markWords found, all at once: a text node may hold the words of more
// than one block (the lines of a pre of one text), and is cut where they are only once all are known.
func (m *marker) writeWords() {
	// From the end of each text node, so that the places before stay where they are; at one place the wrap first,
	// which the del then goes before.
	slices.SortStableFunc(m.edits, func(x, y edit) int {
		return cmp.Or(cmp.Compare(y.at, x.at), cmp.Compare(len(x.del), len(y.del)))
	})
	for _, e := range m.edits {
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
	m.edits = nil
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
// parent. Those it does not: in a table but not a cell, in textarea or title, in svg or MathML.
func (m *marker) holds(parent *html.Node, tag string) bool {
	chain, starts := startTags(parent)
	if got, ok := m.held[holdKey{starts: starts, tag: tag}]; ok {
		return got
	}
	// The marks of words are asked of one parent together, each closed before the next.
	tags := []string{tag}
	if tag == "ins" || tag == "del" || tag == "span" {
		tags = []string{"ins", "del", "span"}
	}
	m.probe(chain, starts, tags)
	return m.held[holdKey{starts: starts, tag: tag}]
}

// startTags are parent and its ancestors, the outermost first, and their start tags.
func startTags(parent *html.Node) ([]*html.Node, string) {
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
	return chain, b.String()
}

// probe asks the parser which of tags the last of chain holds, of their start tags starts, and keeps the answers.
func (m *marker) probe(chain []*html.Node, starts string, tags []string) {
	b := starts
	for _, t := range tags {
		b += "<" + t + ` data-floe-probe="` + t + `"></` + t + ">"
		m.held[holdKey{starts: starts, tag: t}] = false
	}
	root, err := html.Parse(strings.NewReader(b))
	if err != nil {
		return
	}
	for n := range root.Descendants() {
		if t, probe := page.Attr(n, "data-floe-probe"); probe && n.Type == html.ElementNode {
			m.held[holdKey{starts: starts, tag: t}] = n.Data == t && n.Namespace == "" && sameChain(n.Parent, chain)
		}
	}
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
// section's first, -1 for none, and the section of each number. A place with no spot to hold a span has none. The
// places in a tab not picked are one, at the tab's title; a details that holds a place is opened, and its name taken
// off so that all of them stay so.
func (m *marker) number() (places, sections []int) {
	order := map[*html.Node]int{}
	i := 0
	for n := range m.root.Descendants() {
		order[n] = i
		i++
	}
	first := map[*group]int{}
	for _, g := range m.groups {
		first[g] = len(order)
		for _, n := range g.nodes {
			first[g] = min(first[g], order[n])
		}
	}
	slices.SortStableFunc(m.groups, func(x, y *group) int { return cmp.Compare(first[x], first[y]) })
	numbers := map[*html.Node]int{}
	sections = []int{}
	firstOf := map[int]int{}
	for _, g := range m.groups {
		nodes := slices.Clone(g.nodes)
		slices.SortStableFunc(nodes, func(x, y *html.Node) int { return cmp.Compare(order[x], order[y]) })
		if len(nodes) == 0 {
			continue
		}
		m.reveal(nodes)
		s := m.anchor(nodes)
		if s == nil {
			continue
		}
		k, ok := numbers[s]
		if !ok {
			k = len(sections)
			numbers[s] = k
			sections = append(sections, g.sec)
			page.SetAttr(s, "id", "floe-diff."+strconv.Itoa(k))
		}
		if at, ok := firstOf[g.sec]; !ok || k < at {
			firstOf[g.sec] = k
		}
	}
	places = make([]int, 0)
	for sec, k := range firstOf {
		for len(places) <= sec {
			places = append(places, -1)
		}
		places[sec] = k
	}
	return places, sections
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
	if !m.put(n, at) {
		return nil
	}
	return at
}

// put puts span in n where it is held: before n's first words held where a span can be (not the words for a screen
// reader), else just before n, a box floe put in. False for nowhere.
func (m *marker) put(n, span *html.Node) bool {
	for d := range withSelf(n) {
		if d.Type == html.TextNode && strings.TrimFunc(d.Data, page.IsSpace) != "" && d.Parent != nil &&
			!page.HasClass(d.Parent, classSr) && m.holds(d.Parent, "span") {
			d.Parent.InsertBefore(span, d)
			return true
		}
	}
	// Not inside a box floe put in, which may be a part copied whose CSS reads its children: just before it.
	if m.added[n] && n.Parent != nil && m.holds(n.Parent, "span") {
		n.Parent.InsertBefore(span, n)
		return true
	}
	return false
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
	for _, l := range tabTitles(n) {
		if !picked(l) {
			label = l
		}
	}
	return label
}

func picked(label *html.Node) bool {
	for d := range label.Descendants() {
		if _, ok := page.Attr(d, "checked"); ok && d.DataAtom == atom.Input {
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
		page.RemoveAttr(p, "name")
		page.SetAttr(p, "open", "")
	}
}

func element(a atom.Atom, class string) *html.Node {
	n := newNode(html.ElementNode, a, a.String())
	if class != "" {
		n.Attr = []html.Attribute{{Namespace: "", Key: "class", Val: class}}
	}
	return n
}

func text(s string) *html.Node {
	return newNode(html.TextNode, 0, s)
}

func newNode(typ html.NodeType, a atom.Atom, data string) *html.Node {
	return &html.Node{
		Parent: nil, FirstChild: nil, LastChild: nil, PrevSibling: nil, NextSibling: nil,
		Type: typ, DataAtom: a, Data: data, Namespace: "", Attr: nil,
	}
}

func addClass(n *html.Node, class string) {
	if page.HasClass(n, class) {
		return
	}
	v, _ := page.Attr(n, "class")
	if v = strings.TrimRightFunc(v, page.IsSpace); v != "" {
		v += " "
	}
	page.SetAttr(n, "class", v+class)
}

// carryStyles puts the styles of the earlier page's head the later's has not at the end of the later's head: the CSS
// of a part only the earlier draws, which what is copied of it needs.
// LIMIT: of two versions in different themes, the earlier's rules apply to the later page too, where they match.
func carryStyles(earlier, later *html.Node) {
	from, to := page.First(earlier, atom.Head), page.First(later, atom.Head)
	if from == nil || to == nil {
		return
	}
	has := map[string]bool{}
	for d := range to.Descendants() {
		if d.Type == html.ElementNode && d.DataAtom == atom.Style {
			has[styleText(d)] = true
		}
	}
	for d := range from.Descendants() {
		if d.Type == html.ElementNode && d.DataAtom == atom.Style && d.Namespace == "" && !has[styleText(d)] {
			c, _ := copyTree(d)
			to.AppendChild(c)
			has[styleText(d)] = true
		}
	}
}

func styleText(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(c.Data)
	}
	return b.String()
}

// withStyle puts the style of the marks at the end of the page's head.
func withStyle(root *html.Node) {
	if head := page.First(root, atom.Head); head != nil {
		s := element(atom.Style, "")
		s.AppendChild(text(markStyle))
		head.AppendChild(s)
	}
}

// bodyOf is the body of the page root, the parser always makes one: root itself for none.
func bodyOf(root *html.Node) *html.Node {
	if body := page.First(root, atom.Body); body != nil {
		return body
	}
	return root
}
