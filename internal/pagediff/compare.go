// Package pagediff compares two versions of a stored page, as drawn, by their words: what a reader reads, under its
// headings. Compare answers the words alone, for the management page to write as text; Mark answers the later page
// with the places that differ marked, for the sandbox alone. Both come of one comparison, of the pages alone.
package pagediff

import (
	"slices"
	"time"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"
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

// Counts counts the sections by state, and the places of the marked page.
type Counts struct {
	Changed int `json:"changed"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Same    int `json:"same"`
	Places  int `json:"places"`
}

// Result is what differs between two versions of a page.
type Result struct {
	Counts   Counts    `json:"counts"`
	Sections []Section `json:"sections"`
}

// The bounds of the differ's work, as the product of the lengths it compares (keys or runes): the answer is of the
// pages alone, never of the time it took. never keeps the differ's own time limit out of reach, as a limit of 0 would
// stop its half-match and cut lines otherwise.
// LIMIT: past these the answer is coarser: blocks go by place, lines are replaced whole. Enough for the 17 MB of
// 20,000 paragraphs all changed within the bound TestCompareBounded sets.
const (
	never    = 100 * 365 * 24 * time.Hour
	callMost = 50_000_000
	pageMost = 400_000_000
)

// Compare compares the page from to the page to, as drawn: an error for one the HTML parser gives up on.
func Compare(from, to []byte) (Result, error) {
	return compareIn(from, to, pageMost)
}

func compareIn(from, to []byte, most int) (Result, error) {
	out, _, err := marked(from, to, most, Words{Gone: "", Before: ""})
	return out, err
}

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
}

func compare(a, b []section, w *work) compared {
	c := compared{result: Result{Counts: Counts{Changed: 0, Added: 0, Removed: 0, Same: 0, Places: 0}, Sections: []Section{}}, steps: nil}
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
			same[i] = blockPair{from: &a[p.a].blocks[i], to: &b[p.b].blocks[i], spans: nil}
		}
		c.add(Section{State: StateSame, Level: to.level, Heading: heading, Place: nil}, step{from: p.a, to: p.b, pairs: same})
		return
	}
	var pairs []blockPair
	for _, q := range align(w.library(len(keysA), len(keysB)), keysA, keysB) {
		pairs = append(pairs, w.blockPair(a[p.a].blocks, b[p.b].blocks, q)...)
	}
	c.add(Section{State: StateChanged, Level: to.level, Heading: heading, Place: nil}, step{from: p.a, to: p.b, pairs: pairs})
}

func (w *work) blockPair(a, b []block, q pair) []blockPair {
	switch {
	case q.a < 0:
		return []blockPair{{from: nil, to: &b[q.b], spans: nil}}
	case q.b < 0:
		return []blockPair{{from: &a[q.a], to: nil, spans: nil}}
	}
	from, to := &a[q.a], &b[q.b]
	if from.key == to.key {
		return []blockPair{{from: from, to: to, spans: nil}}
	}
	if from.kind == to.kind {
		if from.text == to.text {
			return []blockPair{{from: from, to: to, spans: nil}}
		}
		if spans := w.words(from.text, to.text); spans != nil {
			return []blockPair{{from: from, to: to, spans: spans}}
		}
	}
	return []blockPair{{from: from, to: nil, spans: nil}, {from: nil, to: to, spans: nil}}
}

// work is what is left of the differ's work for one comparison.
type work struct{ left int }

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
		for i < toI || j < toJ {
			p := pair{a: -1, b: -1}
			if i < toI {
				p.a = i
				i++
			}
			if j < toJ {
				p.b = j
				j++
			}
			out = append(out, p)
		}
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
