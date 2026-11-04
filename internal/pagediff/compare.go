// Package pagediff compares two versions of a stored page, as drawn, by their words: what a reader reads, under its
// headings. Its
// answer holds words alone, never a page's HTML, for the management page to write as text.
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

// Block is a block of a section, kind its element's name. Reshaped tells a diagram or image whose words are the same
// and whose
// content differs.
type Block struct {
	Kind     string `json:"kind"`
	Reshaped bool   `json:"reshaped,omitempty"`
	Spans    []Span `json:"spans"`
}

// Section is a section told in both versions or either: its state one of the State values. A section the same holds no
// blocks.
type Section struct {
	State   string  `json:"state"`
	Level   int     `json:"level"`
	Heading []Span  `json:"heading"`
	Blocks  []Block `json:"blocks,omitempty"`
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

// Result is what differs between two versions of a page.
type Result struct {
	Counts   Counts    `json:"counts"`
	Sections []Section `json:"sections"`
}

const (
	// LIMIT: past this the answer is coarser, and the same pages may answer differently. Enough for the 17 MB of 20,000
	// paragraphs all changed.
	timeLimit = 3 * time.Second
	alignMost = time.Second
	wordsMost = 100 * time.Millisecond
)

// Compare compares the page from to the page to, as drawn: an error for one the HTML parser gives up on.
func Compare(from, to []byte) (Result, error) {
	return compareIn(from, to, timeLimit)
}

func compareIn(from, to []byte, limit time.Duration) (Result, error) {
	c := clock{end: time.Now().Add(limit)}
	a, err := read(from)
	if err != nil {
		return Result{}, err
	}
	b, err := read(to)
	if err != nil {
		return Result{}, err
	}
	out := Result{Counts: Counts{Changed: 0, Added: 0, Removed: 0, Same: 0}, Sections: []Section{}}
	// Sections always, as they are few and short: one added would otherwise tell all those after it removed and added.
	for _, p := range align(library(alignMost), headings(a), headings(b)) {
		for _, s := range c.sectionPair(a, b, p) {
			out.Sections = append(out.Sections, s)
			out.Counts.count(s.State)
		}
	}
	return out, nil
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
	blocks := make([]Block, len(s.blocks))
	for i, b := range s.blocks {
		blocks[i] = wholeBlock(b, op)
	}
	return Section{State: state, Level: s.level, Heading: []Span{{Op: op, Text: s.heading}}, Blocks: blocks}
}

func wholeBlock(b block, op string) Block {
	return Block{Kind: b.kind, Reshaped: false, Spans: []Span{{Op: op, Text: b.text}}}
}

// pair is the places of what goes together in two lists, -1 for one in the other alone.
type pair struct{ a, b int }

// sectionPair is what a pair of sections comes to: one, or as their headings are not alike, the one removed and the
// other added.
func (c clock) sectionPair(a, b []section, p pair) []Section {
	switch {
	case p.a < 0:
		return []Section{whole(b[p.b], opIns, StateAdded)}
	case p.b < 0:
		return []Section{whole(a[p.a], opDel, StateRemoved)}
	}
	from, to := a[p.a], b[p.b]
	heading := []Span{{Op: opSame, Text: to.heading}}
	if from.heading != to.heading {
		if heading = c.words(from.heading, to.heading); heading == nil {
			return []Section{whole(from, opDel, StateRemoved), whole(to, opIns, StateAdded)}
		}
	}
	keysA, keysB := blockKeys(from), blockKeys(to)
	if from.heading == to.heading && slices.Equal(keysA, keysB) {
		return []Section{{State: StateSame, Level: to.level, Heading: heading, Blocks: nil}}
	}
	blocks := []Block{}
	for _, q := range align(c.library(alignMost), keysA, keysB) {
		blocks = append(blocks, c.blockPair(from.blocks, to.blocks, q)...)
	}
	return []Section{{State: StateChanged, Level: to.level, Heading: heading, Blocks: blocks}}
}

func (c clock) blockPair(a, b []block, q pair) []Block {
	switch {
	case q.a < 0:
		return []Block{wholeBlock(b[q.b], opIns)}
	case q.b < 0:
		return []Block{wholeBlock(a[q.a], opDel)}
	}
	from, to := a[q.a], b[q.b]
	if from.key == to.key {
		return []Block{wholeBlock(to, opSame)}
	}
	if from.kind == to.kind {
		if from.text == to.text {
			return []Block{{Kind: to.kind, Reshaped: true, Spans: []Span{{Op: opSame, Text: to.text}}}}
		}
		if spans := c.words(from.text, to.text); spans != nil {
			return []Block{{Kind: to.kind, Reshaped: false, Spans: spans}}
		}
	}
	return []Block{wholeBlock(from, opDel), wholeBlock(to, opIns)}
}

// clock is the time left for one comparison.
type clock struct{ end time.Time }

// library is the differ with no more than most of the time left, nil once there is none: a DiffTimeout of 0 would mean
// no limit.
func (c clock) library(most time.Duration) *diffmatchpatch.DiffMatchPatch {
	left := time.Until(c.end)
	if left <= 0 {
		return nil
	}
	return library(min(left, most))
}

func library(timeout time.Duration) *diffmatchpatch.DiffMatchPatch {
	d := diffmatchpatch.New()
	d.DiffTimeout = timeout
	return d
}

// words compares two lines by their characters, as Japanese has no spaces to cut at: nil when they are not alike, or no
// time is left.
// LIMIT: alike is the same characters making half of the longer line at least, a rule of thumb: below it the pieces in
// common only
// make two lines hard to read.
func (c clock) words(from, to string) []Span {
	d := c.library(wordsMost)
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
			r, ok := ids[key]
			if !ok {
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
// LIMIT: by place, so that sections or blocks moved read as removed and added, and of those left the first are paired
// whatever they
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
