// Package pagediff compares two versions of a stored page, as drawn, by their words: what a reader reads, under its
// headings. Compare answers the words alone, for the management page to write as text; Mark answers the later page
// with the places that differ marked, for the sandbox alone. Both come of one comparison, of the pages alone.
package pagediff

import (
	"iter"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"floe/internal/page"
)

// Span is words that are the same in both versions, gone from the later or added to it.
type Span struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

const (
	opSame = "same"
	opDel  = "del"
	opIns  = "ins"
)

// Section is a section told in both versions or either: its state one of the State values, its heading's words as
// they differ. Place is the number of its first place on the marked page (Mark), none for a section with no place
// there.
type Section struct {
	State   string `json:"state"`
	Level   int    `json:"level"`
	Heading []Span `json:"heading"`
	Place   *int   `json:"place,omitempty"`
}

// The states of a section.
const (
	StateSame    = "same"
	StateChanged = "changed"
	StateAdded   = "added"
	StateRemoved = "removed"
)

// Counts counts the sections by state.
type Counts struct {
	Changed int `json:"changed"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Same    int `json:"same"`
}

// Result is what differs between two versions of a page, and of each place of the marked page, by its number, the
// section it is in.
type Result struct {
	Counts   Counts    `json:"counts"`
	Sections []Section `json:"sections"`
	Places   []int     `json:"places"`
}

// The bounds of the differ's work, as the product of the lengths it compares (keys or runes): the answer is of the
// pages alone, never of the time it took. never keeps the differ's own time limit out of reach, as a limit of 0 would
// stop its half-match and cut lines otherwise.
// LIMIT: past these the answer is coarser: blocks go by place, lines are replaced whole. Enough for the 17 MB of
// 20,000 paragraphs all changed within the bound TestCompareBounded sets.
const (
	never    = 100 * 365 * 24 * time.Hour
	rowsMost = 4_000_000
	callMost = 50_000_000
	pageMost = 400_000_000
)

// compared is the sections of two pages compared, in order: of each, what goes into the result and what is marked.
type compared struct {
	result Result
	steps  []step
}

// step is a section of the result, of the sections a[from] and b[to] (-1 for none), its blocks paired in order.
type step struct {
	from, to int
	pairs    []blockPair
}

// blockPair is a block of a section and the one it is paired with, nil for one in a version alone; spans the words
// that differ, for a pair of the same kind whose words are alike.
type blockPair struct {
	from, to *block
	spans    []Span
	// cells are the cells of a pair of rows paired, of a table whose cells span no more than one row and column.
	cells []cellPair
}

// cellPair is a cell of a row and the one it is paired with, nil for one in a version alone; spans the words that
// differ, for cells whose words are alike.
type cellPair struct {
	from, to *cell
	spans    []Span
}

func compare(a, b []section, w *work) compared {
	c := compared{steps: nil, result: Result{
		Counts: Counts{Changed: 0, Added: 0, Removed: 0, Same: 0}, Sections: []Section{}, Places: []int{},
	}}
	// Sections always, as they are few and short: one added would otherwise tell all those after it removed and added.
	for _, p := range align(w.always(len(a), len(b)), headings(a), headings(b)) {
		w.sectionPair(&c, a, b, p)
	}
	return c
}

func (c *compared) add(s Section, st step) {
	c.result.Sections = append(c.result.Sections, s)
	c.result.Counts.count(s.State)
	c.steps = append(c.steps, st)
}

func (n *Counts) count(state string) {
	switch state {
	case StateChanged:
		n.Changed++
	case StateAdded:
		n.Added++
	case StateRemoved:
		n.Removed++
	default:
		n.Same++
	}
}

func headings(sections []section) []string {
	keys := make([]string, len(sections))
	for i, s := range sections {
		keys[i] = s.heading
	}
	return keys
}

func blockKeys(s section) []string {
	keys := make([]string, len(s.blocks))
	for i, b := range s.blocks {
		keys[i] = b.key
	}
	return keys
}

func whole(s section, op, state string) Section {
	return Section{State: state, Level: s.level, Heading: []Span{{Op: op, Text: s.heading}}, Place: nil}
}

// pair is the places of what goes together in two lists, -1 for one in the other alone.
type pair struct{ a, b int }

// sectionPair adds what a pair of sections comes to: one, or as their headings are not alike, the one removed and the
// other added.
func (w *work) sectionPair(c *compared, a, b []section, p pair) {
	switch {
	case p.a < 0:
		c.add(whole(b[p.b], opIns, StateAdded), step{from: -1, to: p.b, pairs: nil})
		return
	case p.b < 0:
		c.add(whole(a[p.a], opDel, StateRemoved), step{from: p.a, to: -1, pairs: nil})
		return
	}
	from, to := a[p.a], b[p.b]
	heading := []Span{{Op: opSame, Text: to.heading}}
	if from.heading != to.heading {
		if heading = w.words(from.heading, to.heading); heading == nil {
			c.add(whole(from, opDel, StateRemoved), step{from: p.a, to: -1, pairs: nil})
			c.add(whole(to, opIns, StateAdded), step{from: -1, to: p.b, pairs: nil})
			return
		}
	}
	keysA, keysB := blockKeys(from), blockKeys(to)
	if from.heading == to.heading && slices.Equal(keysA, keysB) {
		same := make([]blockPair, len(to.blocks))
		for i := range to.blocks {
			same[i] = blockPair{from: &a[p.a].blocks[i], to: &b[p.b].blocks[i], spans: nil, cells: nil}
		}
		c.add(Section{State: StateSame, Level: to.level, Heading: heading, Place: nil}, step{from: p.a, to: p.b, pairs: same})
		return
	}
	var pairs []blockPair
	blocksA, blocksB := a[p.a].blocks, b[p.b].blocks
	d := w.library(len(keysA), len(keysB))
	aligned := align(d, keysA, keysB)
	// The rows again by their cells, none past the blocks' alignment.
	if d != nil {
		aligned = w.rows(blocksA, blocksB, aligned)
	}
	for _, q := range aligned {
		pairs = append(pairs, w.blockPair(blocksA, blocksB, q)...)
	}
	c.add(Section{State: StateChanged, Level: to.level, Heading: heading, Place: nil},
		step{from: p.a, to: p.b, pairs: pairs})
}

func (w *work) blockPair(a, b []block, q pair) []blockPair {
	switch {
	case q.a < 0:
		return []blockPair{{from: nil, to: &b[q.b], spans: nil, cells: nil}}
	case q.b < 0:
		return []blockPair{{from: &a[q.a], to: nil, spans: nil, cells: nil}}
	}
	from, to := &a[q.a], &b[q.b]
	if from.key == to.key {
		return []blockPair{{from: from, to: to, spans: nil, cells: nil}}
	}
	if from.kind == to.kind {
		if from.text == to.text {
			return []blockPair{{from: from, to: to, spans: nil, cells: nil}}
		}
		if spans := w.words(from.text, to.text); spans != nil {
			var cells []cellPair
			if w.cellsCompared(from, to) {
				cells = w.cellPairs(from.cells, to.cells)
			}
			return []blockPair{{from: from, to: to, spans: spans, cells: cells}}
		}
	}
	return []blockPair{{from: from, to: nil, spans: nil, cells: nil}, {from: nil, to: to, spans: nil, cells: nil}}
}

// rows pairs again the rows of a table whose keys differ, between those the same: by the cells they share, as a
// column gone or added changes every row's words, and by place one row gone would pair those after it off by one. A
// run of blocks not all rows stays as it is.
func (w *work) rows(a, b []block, pairs []pair) []pair {
	var out []pair
	for i := 0; i < len(pairs); {
		j := i
		for j < len(pairs) && (pairs[j].a < 0 || pairs[j].b < 0 || a[pairs[j].a].key != b[pairs[j].b].key) {
			j++
		}
		out = append(out, w.rowRun(a, b, pairs[i:j])...)
		if j < len(pairs) {
			out = append(out, pairs[j])
		}
		i = j + 1
	}
	return out
}

// rowRun pairs the rows of a run by the longest sequence of rows that share half their cells or more, in order: its
// work, a row of one compared with each of the other, taken from what is left.
func (w *work) rowRun(a, b []block, run []pair) []pair {
	xs, ys, ok := rowsOf(a, b, run)
	cost := len(xs) * len(ys)
	// LIMIT: the longest sequence by a table of n×m, as the differ compares runes, not rows by their likeness; past
	// rowsMost of it, the rows go by place as they came.
	if !ok || cost == 0 || cost > rowsMost || cost > w.left {
		return run
	}
	w.left -= cost
	// The rows paired by their cells, and between them those left paired by place, as align leaves a run.
	var out []pair
	i, j := 0, 0
	for _, q := range sameRows(cellCounts(a, xs), cellCounts(b, ys)) {
		out = byPlace(out, xs[i:q.a], ys[j:q.b])
		out = append(out, pair{a: xs[q.a], b: ys[q.b]})
		i, j = q.a+1, q.b+1
	}
	return byPlace(out, xs[i:], ys[j:])
}

// rowsOf are the blocks of a and of b in run: false when one of them is not a row.
func rowsOf(a, b []block, run []pair) (xs, ys []int, ok bool) {
	for _, p := range run {
		if p.a >= 0 {
			if a[p.a].kind != "tr" {
				return nil, nil, false
			}
			xs = append(xs, p.a)
		}
		if p.b >= 0 {
			if b[p.b].kind != "tr" {
				return nil, nil, false
			}
			ys = append(ys, p.b)
		}
	}
	return xs, ys, true
}

// sameRows is the longest sequence of the rows xs and ys, their cells counted, that share cells, as indexes into them.
func sameRows(xs, ys []cellCount) []pair {
	n, m := len(xs), len(ys)
	best := make([][]int32, n+1)
	shared := make([][]bool, n)
	for i := range best {
		best[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		shared[i] = make([]bool, m)
		for j := m - 1; j >= 0; j-- {
			best[i][j] = max(best[i+1][j], best[i][j+1])
			if shared[i][j] = shareCells(xs[i], ys[j]); shared[i][j] {
				best[i][j] = max(best[i][j], best[i+1][j+1]+1)
			}
		}
	}
	var matched []pair
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case shared[i][j] && best[i][j] == best[i+1][j+1]+1:
			matched = append(matched, pair{a: i, b: j})
			i++
			j++
		case best[i+1][j] >= best[i][j+1]:
			i++
		default:
			j++
		}
	}
	return matched
}

// byPlace pairs the places xs of one list and ys of the other one by one from the first, those of the longer left
// alone, and adds the pairs to out.
func byPlace(out []pair, xs, ys []int) []pair {
	for k := range max(len(xs), len(ys)) {
		p := pair{a: -1, b: -1}
		if k < len(xs) {
			p.a = xs[k]
		}
		if k < len(ys) {
			p.b = ys[k]
		}
		out = append(out, p)
	}
	return out
}

// upTo is the places from up to to.
func upTo(from, to int) []int {
	out := make([]int, 0, max(to-from, 0))
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}

// cellCount is a row's cells counted by their words, and how many it has.
type cellCount struct {
	words map[string]int
	n     int
}

// cellCounts counts the cells of the rows of bs at the places at.
func cellCounts(bs []block, at []int) []cellCount {
	out := make([]cellCount, len(at))
	for i, k := range at {
		words := make(map[string]int, len(bs[k].cells))
		for _, c := range bs[k].cells {
			words[c.text]++
		}
		out[i] = cellCount{words: words, n: len(bs[k].cells)}
	}
	return out
}

// shareCells tells whether two rows share half the cells of the longer at least, by their words.
func shareCells(x, y cellCount) bool {
	shared := 0
	for text, k := range x.words {
		shared += min(k, y.words[text])
	}
	return shared*2 >= max(x.n, y.n) && shared > 0
}

// cellPairs pairs the cells of two rows by their words, as blocks are: those alike compared by their words, the others
// each in one row alone.
func (w *work) cellPairs(a, b []cell) []cellPair {
	keys := func(cells []cell) []string {
		out := make([]string, len(cells))
		for i, c := range cells {
			out[i] = c.text
		}
		return out
	}
	var out []cellPair
	for _, q := range align(w.library(len(a), len(b)), keys(a), keys(b)) {
		switch {
		case q.a < 0:
			out = append(out, cellPair{from: nil, to: &b[q.b], spans: nil})
		case q.b < 0:
			out = append(out, cellPair{from: &a[q.a], to: nil, spans: nil})
		case a[q.a].text == b[q.b].text:
			out = append(out, cellPair{from: &a[q.a], to: &b[q.b], spans: nil})
		default:
			if spans := w.words(a[q.a].text, b[q.b].text); spans != nil {
				out = append(out, cellPair{from: &a[q.a], to: &b[q.b], spans: spans})
			} else {
				out = append(out, cellPair{from: &a[q.a], to: nil, spans: nil}, cellPair{from: nil, to: &b[q.b], spans: nil})
			}
		}
	}
	return out
}

// tables tells of each table whether a cell of it spans more than one row or column: a cell put in or left out there
// moves the columns of other rows.
type tables map[*html.Node]bool

// spanned tells it of the table of the row tr, true for a row of no table.
func (t tables) spanned(tr *html.Node) bool {
	table := tableOf(tr)
	if table == nil {
		return true
	}
	if got, ok := t[table]; ok {
		return got
	}
	got := false
	for g := range rowGroups(table) {
		for r := g.FirstChild; r != nil; r = r.NextSibling {
			if r.Type == html.ElementNode && r.DataAtom == atom.Tr && rowSpans(r) {
				got = true
			}
		}
	}
	t[table] = got
	return got
}

// rowSpans tells whether a cell of the row tr spans more than one row or column.
func rowSpans(tr *html.Node) bool {
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		for _, key := range []string{"colspan", "rowspan"} {
			if v, ok := page.Attr(c, key); ok && strings.TrimFunc(v, page.IsSpace) != "1" {
				return true
			}
		}
	}
	return false
}

// rowGroups are the groups of rows (thead, tbody, tfoot) of table itself, not of a table in its cells: the HTML parser
// puts every row in one.
func rowGroups(table *html.Node) iter.Seq[*html.Node] {
	return func(yield func(*html.Node) bool) {
		for c := table.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.DataAtom == atom.Thead || c.DataAtom == atom.Tbody || c.DataAtom == atom.Tfoot) &&
				!yield(c) {
				return
			}
		}
	}
}

// cellsCompared tells whether two rows paired are compared cell by cell: rows of tables whose cells span one row and
// column.
func (t tables) cellsCompared(from, to *block) bool {
	return from.kind == "tr" && to.kind == "tr" && !t.spanned(from.node) && !t.spanned(to.node)
}

// tableOf is the table n is in, nil for none.
func tableOf(n *html.Node) *html.Node {
	for ; n != nil; n = n.Parent {
		if n.Type == html.ElementNode && n.DataAtom == atom.Table && n.Namespace == "" {
			return n
		}
	}
	return nil
}

// work is what is left of the differ's work for one comparison, and which tables have cells that span.
type work struct {
	left int
	tables
}

func newWork(most int) *work {
	return &work{left: most, tables: tables{}}
}

// library is the differ to compare lists of lengths n and m, its work taken from what is left: nil when it is more
// than one call may take, or than is left.
func (w *work) library(n, m int) *diffmatchpatch.DiffMatchPatch {
	cost := n * m
	if cost > callMost || cost > w.left {
		return nil
	}
	w.left -= cost
	return library()
}

// always is the differ for the sections, which are compared whatever is left, within one call's bound.
func (w *work) always(n, m int) *diffmatchpatch.DiffMatchPatch {
	cost := n * m
	if cost > callMost {
		return nil
	}
	w.left = max(w.left-cost, 0)
	return library()
}

func library() *diffmatchpatch.DiffMatchPatch {
	d := diffmatchpatch.New()
	d.DiffTimeout = never
	return d
}

// words compares two lines by their characters, as Japanese has no spaces to cut at: nil when they are not alike, or
// past the work left.
// LIMIT: alike is the same characters making half of the longer line at least, a rule of thumb: below it the pieces in
// common only
// make two lines hard to read.
func (w *work) words(from, to string) []Span {
	d := w.library(utf8.RuneCountInString(from), utf8.RuneCountInString(to))
	if d == nil {
		return nil
	}
	diffs := d.DiffCleanupSemantic(d.DiffMain(from, to, false))
	same := 0
	spans := make([]Span, len(diffs))
	for i, x := range diffs {
		spans[i] = Span{Op: opOf(x.Type), Text: x.Text}
		if x.Type == diffmatchpatch.DiffEqual {
			same += utf8.RuneCountInString(x.Text)
		}
	}
	if same*2 < max(utf8.RuneCountInString(from), utf8.RuneCountInString(to)) {
		return nil
	}
	return spans
}

func opOf(t diffmatchpatch.Operation) string {
	switch t {
	case diffmatchpatch.DiffDelete:
		return opDel
	case diffmatchpatch.DiffInsert:
		return opIns
	default:
		return opSame
	}
}

// surrogates are the numbers from which no rune can be made.
const (
	surrogates = 0xD800
	surrogateN = 0x800
	mostRunes  = 0x10FFFF - surrogateN
)

// numbered is each list's keys as runes, the same key the same rune, for the differ to compare: false for more keys
// than there are
// runes. The differ's own DiffLinesToRunes cuts at newlines, which a key may hold, and panics past that many.
func numbered(a, b []string) (runesA, runesB []rune, ok bool) {
	ids := map[string]rune{}
	number := func(keys []string) ([]rune, bool) {
		runes := make([]rune, len(keys))
		for i, key := range keys {
			r, known := ids[key]
			if !known {
				if len(ids) >= mostRunes {
					return nil, false
				}
				r = rune(len(ids) + 1) //nolint:gosec // bounded by mostRunes above
				if r >= surrogates {
					r += surrogateN
				}
				ids[key] = r
			}
			runes[i] = r
		}
		return runes, true
	}
	ra, ok := number(a)
	if !ok {
		return nil, nil, false
	}
	rb, ok := number(b)
	return ra, rb, ok
}

// align pairs the keys of two lists: those the same, in order, and between them what is left of each, one by one from
// the first. d nil,
// or for too many keys, it goes by place alone.
// LIMIT: by place: sections or blocks moved read as removed and added, a section added before others of its heading
// pairs them off by one, so that those not added read as changed, and of those left the first are paired whatever they
// hold (words tells when they are not alike).
func align(d *diffmatchpatch.DiffMatchPatch, a, b []string) []pair {
	var diffs []diffmatchpatch.Diff
	if ra, rb, ok := numbered(a, b); d != nil && ok {
		diffs = d.DiffMainRunes(ra, rb, false)
	}
	out := make([]pair, 0, max(len(a), len(b)))
	i, j := 0, 0
	left := func(toI, toJ int) {
		out = byPlace(out, upTo(i, toI), upTo(j, toJ))
		i, j = toI, toJ
	}
	atI, atJ := 0, 0
	for _, x := range diffs {
		n := utf8.RuneCountInString(x.Text)
		switch x.Type {
		case diffmatchpatch.DiffDelete:
			atI += n
		case diffmatchpatch.DiffInsert:
			atJ += n
		default:
			left(atI, atJ)
			for range n {
				out = append(out, pair{a: i, b: j})
				i++
				j++
			}
			atI, atJ = i, j
		}
	}
	left(len(a), len(b))
	return out
}
