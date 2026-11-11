package pagediff

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var update = flag.Bool("update", false, "write testdata/every-part.golden anew")

func spans(sp []Span) string {
	var b strings.Builder
	for _, s := range sp {
		switch s.Op {
		case opDel:
			b.WriteString("[-" + s.Text + "-]")
		case opIns:
			b.WriteString("{+" + s.Text + "+}")
		default:
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

// readSections is the sections of page, read as Compare reads them.
func readSections(t *testing.T, page []byte) []section {
	t.Helper()

	_, sections, err := readTree(page)
	if err != nil {
		t.Fatal(err)
	}
	return sections
}

// told is the comparison of two pages within most as a person reads it, a block the same left out: each section by its
// state and heading, each block by its kind and its words as they differ, reshaped for a diagram or image whose words
// are the same and whose content differs.
func told(t *testing.T, from, to string, most int) string {
	t.Helper()

	a, b := readSections(t, []byte(from)), readSections(t, []byte(to))
	c := compare(a, b, newWork(most))
	var out strings.Builder
	line := func(kind, words string) { fmt.Fprintf(&out, "  [%s] %s\n", kind, words) }
	for i, s := range c.result.Sections {
		fmt.Fprintf(&out, "%s h%d %s\n", s.State, s.Level, spans(s.Heading))
		st := c.steps[i]
		switch {
		case st.from < 0:
			for _, blk := range b[st.to].blocks {
				line(blk.kind, "{+"+blk.text+"+}")
			}
		case st.to < 0:
			for _, blk := range a[st.from].blocks {
				line(blk.kind, "[-"+blk.text+"-]")
			}
		case s.State == StateChanged:
			for _, p := range st.pairs {
				switch {
				case p.from == nil:
					line(p.to.kind, "{+"+p.to.text+"+}")
				case p.to == nil:
					line(p.from.kind, "[-"+p.from.text+"-]")
				case p.from.key == p.to.key:
				case p.spans == nil:
					line(p.to.kind+" reshaped", p.to.text)
				default:
					line(p.to.kind, spans(p.spans))
				}
			}
		}
	}
	return out.String()
}

func TestCompare(t *testing.T) {
	t.Parallel()

	terminal := func(run, out string) string {
		return `<h2>A</h2><figure data-floe-part="terminal"><figcaption>sh</figcaption><pre><span class="floe-hidden">Command:</span>` +
			`<span class="run">` + run + `</span><span class="out">` + out + `</span><span class="floe-hidden">Command:</span><span class="run">go vet</span></pre></figure>`
	}
	for _, tc := range []struct{ name, from, to, want string }{
		{"a heading's number", "<h2>3. Where</h2><p>body</p>", "<h2>4. Where</h2><p>body</p>", "changed h2 [-3-]{+4+}. Where\n"},
		{
			"a heading quite another", "<h2>Inn</h2><p>body</p>", "<h2>Where to stay</h2><p>body</p>",
			"removed h2 [-Inn-]\n  [p] [-body-]\nadded h2 {+Where to stay+}\n  [p] {+body+}\n",
		},
		{
			"one heading many times", "<h2>A</h2><h3>Why</h3><p>x</p><h2>B</h2><h3>Why</h3><p>y</p>", "<h2>A</h2><h3>Why</h3><p>x</p><h2>B</h2><h3>Why</h3><p>z added</p>",
			"same h2 A\nsame h3 Why\nsame h2 B\nchanged h3 Why\n  [p] [-y-]\n  [p] {+z added+}\n",
		},
		{
			"sections swapped", "<h2>A</h2><p>a</p><h2>B</h2><p>b</p>", "<h2>B</h2><p>b</p><h2>A</h2><p>a</p>",
			"removed h2 [-A-]\n  [p] [-a-]\nsame h2 B\nadded h2 {+A+}\n  [p] {+a+}\n",
		},
		{"a section added", "<h2>A</h2><p>a</p><h2>C</h2>", "<h2>A</h2><p>a</p><h2>B</h2><p>b</p><h2>C</h2>", "same h2 A\nadded h2 {+B+}\n  [p] {+b+}\nsame h2 C\n"},
		{"no heading", "<p>the first line</p><p>the second line</p>", "<p>the first line</p><p>the second line, mended</p>", "changed h0 \n  [p] the second line{+, mended+}\n"},
		{"words made bold", "<h2>A</h2><p>an important word</p>", "<h2>A</h2><p>an <b>important</b> word</p>", "same h2 A\n"},
		{"a mark within a word", "<h2>A</h2><p>大事な話</p>", "<h2>A</h2><p><b>大事</b>な話</p>", "same h2 A\n"},
		{"a link's target", "<h2>A</h2><p><a href=/x>there</a></p>", "<h2>A</h2><p><a href=/y>there</a></p>", "same h2 A\n"},
		{"spaces", "<h2>A</h2><p>a  b</p>", "<h2>A</h2><p>a\n b</p>", "same h2 A\n"},
		{"what is not read", "<h2>A</h2><p>x</p><script>a()</script><style>p{}</style><nav><a href=#a>A</a></nav><button>Copy</button><progress>1 of 2</progress>", "<h2>A</h2><p>x</p><script>b()</script><style>p{color:red}</style><nav><a href=#b>B</a></nav><button>Copied</button><progress>2 of 2</progress>", "same h2 A\n"},
		{"elements apart", "<h2>A</h2><div><span>Copied</span><span>1,840</span></div>", "<h2>A</h2><div><span>Copied</span><span>1,841</span></div>", "changed h2 A\n  [div] Copied 1,84[-0-]{+1+}\n"},
		{"cells", "<h2>A</h2><table><tr><td><span>a</span><span>b</span></td><td>26000</td></tr></table>", "<h2>A</h2><table><tr><td><span>a</span><span>b</span></td><td>38000</td></tr></table>", "changed h2 A\n  [tr] a b | [-26-]{+38+}000\n"},
		{"an image of another src", "<h2>A</h2><img src=data:a alt=Map>", "<h2>A</h2><img src=data:b alt=Map>", "changed h2 A\n  [img reshaped] Map\n"},
		{"a diagram's shape", "<h2>A</h2><svg viewBox='0 0 1 1'><text>ALB</text></svg>", "<h2>A</h2><svg viewBox='0 0 2 2'><text>ALB</text></svg>", "changed h2 A\n  [svg reshaped] ALB\n"},
		{"a diagram's words", "<h2>A</h2><svg><text>ALB</text><text>ECS one</text></svg>", "<h2>A</h2><svg><text>ALB</text><text>ECS two</text></svg>", "changed h2 A\n  [svg] ALB / ECS [-one-]{+two+}\n"},
		{
			"a diagram's number in the page, its ids and what names them",
			`<h2>A</h2><svg id=dg1-e5><marker id=dg1-e5_arrow></marker><use href="#dg1-e5_arrow"/><path marker-end="url(#dg1-e5_arrow)"/><text>ALB</text></svg>`,
			`<h2>A</h2><svg id=dg2-e5><marker id=dg2-e5_arrow></marker><use href="#dg2-e5_arrow"/><path marker-end="url(#dg2-e5_arrow)"/><text>ALB</text></svg>`,
			"same h2 A\n",
		},
		{
			"a diagram added before another", `<h2>A</h2><svg id=dg1><path id=dg1-p /><text>B</text></svg>`, `<h2>A</h2><svg id=dg1><path id=dg1-p /><text>N</text></svg><svg id=dg2><path id=dg2-p /><text>B</text></svg>`,
			"changed h2 A\n  [svg] {+N+}\n",
		},
		{"a colour named as an id", `<h2>A</h2><svg><rect id="red" fill="red"/></svg>`, `<h2>A</h2><svg><rect id="blue" fill="blue"/></svg>`, "changed h2 A\n  [svg reshaped] \n"},
		{"an id named as an attribute", `<h2>A</h2><svg><rect id="x" x="1"/></svg>`, `<h2>A</h2><svg><rect id="y" x="1"/></svg>`, "same h2 A\n"},
		{"an id its value's reference", `<h2>A</h2><svg><g id="a"/><use href="#a" fill="url(#a)" aria-labelledby="a"/></svg>`, `<h2>A</h2><svg><g id="b"/><use href="#b" fill="url(#b)" aria-labelledby="b"/></svg>`, "same h2 A\n"},
		{"a reference to the svg's own id", `<h2>A</h2><svg id="a"><use href="#a"/></svg>`, `<h2>A</h2><svg id="b"><use href="#b"/></svg>`, "same h2 A\n"},
		{"a reference to another", `<h2>A</h2><svg><g id="a"/><g id="b"/><use href="#a"/></svg>`, `<h2>A</h2><svg><g id="a"/><g id="b"/><use href="#b"/></svg>`, "changed h2 A\n  [svg reshaped] \n"},
		{"an image's alt", "<h2>A</h2><img src=a alt='a map of Kyoto'>", "<h2>A</h2><img src=a alt='a map of Osaka'>", "changed h2 A\n  [img] a map of [-Kyoto-]{+Osaka+}\n"},
		{"an image in a cell", "<h2>A</h2><table><tr><td><img src=a alt='a map of Kyoto'></td></tr></table>", "<h2>A</h2><table><tr><td><img src=a alt='a map of Osaka'></td></tr></table>", "changed h2 A\n  [tr] a map of [-Kyoto-]{+Osaka+}\n"},
		{"an image in a heading", "<h2>The map <img src=a alt=one></h2>", "<h2>The map <img src=a alt=two></h2>", "changed h2 The map [-one-]{+two+}\n"},
		{"spaces between lines", "<h2>A</h2><pre><span>a</span>\n<span>b</span></pre>", "<h2>A</h2><pre><span>a</span>\n<span>c</span></pre>", "changed h2 A\n  [pre] [-b-]\n  [pre] {+c+}\n"},
		{"words last in a pre", `<h2>A</h2><pre><span>a</span><span class="floe-hidden">one</span></pre>`, `<h2>A</h2><pre><span>a</span><span class="floe-hidden">two</span></pre>`, "changed h2 A\n  [pre] [-one-]\n  [pre] {+two+}\n"},
		{"small marks words", "<h2>A</h2><p><small>大事</small>な話</p>", "<h2>A</h2><p>大事な話</p>", "same h2 A\n"},
		{"old marks of words", "<h2>A</h2><p><big>大</big><tt>事</tt><font>な</font><strike>話</strike><nobr>し</nobr></p>", "<h2>A</h2><p>大事な話し</p>", "same h2 A\n"},
		{"a heading's level", "<h2>A</h2><p>a</p>", "<h3>A</h3><p>a</p>", "same h3 A\n"},
		{"lines not alike", "<h2>A</h2><p>the hotel by the station</p>", "<h2>A</h2><p>at most a thousand yen</p>", "changed h2 A\n  [p] [-the hotel by the station-]\n  [p] {+at most a thousand yen+}\n"},
		{"of another kind", "<h2>A</h2><p>the same words</p>", "<h2>A</h2><ul><li>the same words</li></ul>", "changed h2 A\n  [p] [-the same words-]\n  [li] {+the same words+}\n"},
		{"words of tags stay words", "<h2>A</h2><p>&lt;img src=x onerror=alert(1)&gt;</p>", "<h2>A</h2><p>&lt;script&gt;alert(2)&lt;/script&gt;</p>", "changed h2 A\n  [p] [-<img src=x onerror=alert(1)>-]\n  [p] {+<script>alert(2)</script>+}\n"},
		{"a line of code", "<h2>A</h2><pre><code>a := 1\nb := 2</code></pre>", "<h2>A</h2><pre><code>a := 1\nb := 3</code></pre>", "changed h2 A\n  [pre] b := [-2-]{+3+}\n"},
		{"indent alone", "<h2>A</h2><pre><code>if x {\n  y()\n}</code></pre>", "<h2>A</h2><pre><code>if x {\n    y()\n}</code></pre>", "changed h2 A\n  [pre] {+  +}  y()\n"},
		{"a blank line added", "<h2>A</h2><pre><code>a\nb</code></pre>", "<h2>A</h2><pre><code>a\n\nb</code></pre>", "changed h2 A\n  [pre] {++}\n"},
		{"terminal", terminal("go test", "ok\nFAIL x"), terminal("go test -race", "ok\nok y"), "changed h2 A\n  [pre] Command: go test{+ -race+}\n  [pre] [-FAIL x-]\n  [pre] {+ok y+}\n"},
		{
			"diff", `<h2>A</h2><pre><code><span class="add">+a</span><span class="del">-b</span></code></pre>`, `<h2>A</h2><pre><code><span class="add">+a</span><span class="del">-c</span></code></pre>`,
			"changed h2 A\n  [pre] -[-b-]{+c+}\n",
		},
		{
			"words only a screen reader reads", `<h2>A</h2><ul><li><span><span class="floe-hidden">Not done:</span>Buy milk</span></li></ul>`, `<h2>A</h2><ul><li><span><span class="floe-hidden">Done:</span>Buy milk</span></li></ul>`,
			"changed h2 A\n  [li] [-Not d-]{+D+}one: Buy milk\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if s := told(t, tc.from, tc.to, pageMost); s != tc.want {
				t.Errorf("got\n%swant\n%s", s, tc.want)
			}
		})
	}
}

// A pre's lines, in its forms: the spaces between elements are none, and blank lines of its words are.
func TestPreLines(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ html, want string }{
		{"<pre><span>a</span>\n<span>b</span></pre>", "a|b"},
		{"<pre><code>a\n\nb\n</code></pre>", "a||b"},
		{`<pre><span class="floe-hidden">run</span><span>a</span>` + "\n" + `<span class="floe-hidden">last</span></pre>`, "run a|last"},
	} {
		sections := readSections(t, []byte(tc.html))
		lines := make([]string, len(sections[0].blocks))
		for i, b := range sections[0].blocks {
			lines[i] = b.text
		}
		if got := strings.Join(lines, "|"); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.html, got, tc.want)
		}
	}
}

// One added before others of its heading is told as the LIMIT of align says.
func TestSameHeadings(t *testing.T) {
	t.Parallel()

	const from, to = "<h2>Why</h2><p>first reason</p>", "<h2>Why</h2><p>a new reason</p><h2>Why</h2><p>first reason</p>"
	got, _, err := Diff([]byte(from), []byte(to), testWords)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counts.Changed != 1 || got.Counts.Added != 1 {
		t.Errorf("got %+v\n%s", got.Counts, told(t, from, to, pageMost))
	}
}

func TestCounts(t *testing.T) {
	t.Parallel()

	const from, to = "<h2>A</h2><p>a</p><h2>B</h2><p>b b</p><h2>C</h2>", "<h2>A</h2><p>a</p><h2>B</h2><p>b c</p><h2>D</h2><h2>Other heading</h2>"
	got, _, err := Diff([]byte(from), []byte(to), testWords)
	if err != nil {
		t.Fatal(err)
	}
	// Of each place, the section it is in, by the sections' order.
	if want := (Counts{Changed: 1, Added: 2, Removed: 1, Same: 1}); got.Counts != want || fmt.Sprint(got.Places) != "[1 2 3 4]" {
		t.Errorf("got %+v %v, want %+v\n%s", got.Counts, got.Places, want, told(t, from, to, pageMost))
	}
}

// A page the parser gives up on is an error, not a page of nothing.
func TestCompareDeep(t *testing.T) {
	t.Parallel()

	if _, _, err := Diff([]byte(strings.Repeat("<div>", 600)), []byte("<p>x</p>"), testWords); err == nil || !strings.Contains(err.Error(), "512") {
		t.Errorf("got %v", err)
	}
}

// With no time left it still answers: the sections go by their headings, one added leaving those after it as they are, those the same
// stay so, and in the others blocks go by place, those not the same replaced whole.
func TestCompareOutOfTime(t *testing.T) {
	t.Parallel()

	const want = "added h2 {+N+}\nchanged h2 A\n  [p] [-x and y-]\n  [p] {+x and w+}\nsame h2 B\n"
	if s := told(t, "<h2>A</h2><p>x and y</p><p>z</p><h2>B</h2><p>the same</p>", "<h2>N</h2><h2>A</h2><p>x and w</p><p>z</p><h2>B</h2><p>the same</p>", 0); s != want {
		t.Errorf("got\n%swant\n%s", s, want)
	}
	// A heading to compare by its words, no time left for it: removed and added, whatever the rest.
	if s, want := told(t, "<h2>3. A</h2>", "<h2>4. A</h2>", 0), "removed h2 [-3. A-]\nadded h2 {+4. A+}\n"; s != want {
		t.Errorf("got\n%swant\n%s", s, want)
	}
}

// All the changes of a big page end near the time limit.
func TestCompareBounded(t *testing.T) { //nolint:paralleltest // it measures time, alone
	var from, to strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&from, "<p>%s</p>", strings.Repeat("あ"+strconv.Itoa(i), 40))
		fmt.Fprintf(&to, "<p>%s</p>", strings.Repeat("い"+strconv.Itoa(i), 40))
	}
	start := time.Now()
	if _, _, err := marked([]byte(from.String()), []byte(to.String()), pageMost, testWords); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 20*time.Second {
		t.Errorf("took %v", took)
	}
}

// Keys may hold newlines, and be more than there are runes.
func TestAlignKeys(t *testing.T) {
	t.Parallel()

	got := align(library(), []string{"a\nb", "c", "d\n"}, []string{"a\nb", "new", "d\n"})
	if s, want := fmt.Sprint(got), "[{0 0} {1 1} {2 2}]"; s != want {
		t.Errorf("got %s, want %s", s, want)
	}
	// Past the surrogates, each number a rune.
	keys := make([]string, surrogates+10)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
	}
	runes, _, _ := numbered(keys, nil)
	for _, r := range runes {
		if !utf8.ValidRune(r) {
			t.Fatalf("%#x is no rune", r)
		}
	}
	const n = mostRunes + 1
	many := make([]string, n)
	for i := range many {
		many[i] = strconv.Itoa(i)
	}
	if _, _, ok := numbered(many, nil); ok {
		t.Error("more keys than runes were numbered")
	}
	all := align(library(), many, []string{"0"})
	if len(all) != n || all[0] != (pair{a: 0, b: 0}) || all[1] != (pair{a: 1, b: -1}) {
		t.Errorf("past the bound: %d %v", len(all), all[:2])
	}
}

// Every part of every-part reads as a reader reads it: no words run together, none told twice.
func TestReadEveryPart(t *testing.T) {
	t.Parallel()

	page, err := os.ReadFile(filepath.Join("..", "corpus", "testdata", "every-part", "golden", "every-part.html"))
	if err != nil {
		t.Fatal(err)
	}
	sections := readSections(t, page)
	var b strings.Builder
	for _, s := range sections {
		fmt.Fprintf(&b, "## h%d %s\n", s.level, s.heading)
		for _, block := range s.blocks {
			fmt.Fprintf(&b, "  [%s] %s\n", block.kind, block.text)
		}
	}
	const fix = "run `just golden` and read the change in git diff"
	golden := filepath.Join("testdata", "every-part.golden")
	if *update {
		if err = os.WriteFile(golden, []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v; %s", err, fix)
	}
	if b.String() != string(want) {
		t.Errorf("%s differs from what is read now; %s", golden, fix)
	}
}

// TestSameAnswer: the answer is of the pages alone. The differ is never cut by time, the same pages answer the same,
// and every call to it is counted in the work, the cells of rows too.
func TestSameAnswer(t *testing.T) {
	t.Parallel()

	if d := library(); d.DiffTimeout != never {
		t.Errorf("the differ's time limit is %v", d.DiffTimeout)
	}
	from := "<h2>T</h2><table><tr><td>alpha</td><td>x</td><td>one</td></tr><tr><td>beta</td><td>y</td><td>two</td></tr></table>"
	to := "<h2>T</h2><table><tr><td>alpha</td><td>one</td></tr><tr><td>beta</td><td>two</td></tr></table>"
	first, _, err := Diff([]byte(from), []byte(to), testWords)
	if err != nil {
		t.Fatal(err)
	}
	again, _, _ := Diff([]byte(from), []byte(to), testWords) //nolint:errcheck // the same as above
	x, _ := json.Marshal(first)                              //nolint:errcheck // a Result always encodes
	y, _ := json.Marshal(again)                              //nolint:errcheck // as above
	if string(x) != string(y) {
		t.Errorf("the same pages answered %s and %s", x, y)
	}
	// Rows gone of two tables of the earlier, both paired with one of the later, go in by the order of the pages.
	two := "<h2>T</h2><table><tr><td>gone one</td><td>a</td></tr><tr><td>kept</td><td>b</td></tr></table>" +
		"<table><tr><td>gone two</td><td>c</td></tr><tr><td>also kept</td><td>d</td></tr></table>"
	one := "<h2>T</h2><table><tr><td>kept</td><td>b</td></tr><tr><td>also kept</td><td>d</td></tr></table>"
	_, marks, err := Diff([]byte(two), []byte(one), testWords)
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		if _, again, _ := Diff([]byte(two), []byte(one), testWords); string(again) != string(marks) { //nolint:errcheck // as above
			t.Fatalf("the same pages marked otherwise:\n%s\n%s", marks, again)
		}
	}
	a := readSections(t, []byte(from))
	b := readSections(t, []byte(to))
	w := newWork(10_000)
	compare(a, b, w)
	// The sections 1×1, the blocks 2×2 (the rows paired again by their cells within it), each row's words, each row's
	// cells 3×2.
	want := 1 + 2*2 + 15*11 + 14*10 + 3*2 + 3*2
	if used := 10_000 - w.left; used != want {
		t.Errorf("the work counted is %d, want %d", used, want)
	}
}
