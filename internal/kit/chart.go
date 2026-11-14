package kit

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"floe/internal/usererr"
)

// The plot's height and the room of its labels, in rem, as part.css and defaults.css set them.
// LIMIT: a chart in a box in a column has less than narrowBox, and its labels may meet; the table holds them.
const (
	plotHeight  = 16.5 - 1 - 2 // the svg's height less its padding
	labelHeight = 1.15
	charNarrow  = 0.8 * 0.62 // an ASCII character in the mono face at --floe-text-small
	charWide    = 0.8
	padInline   = 1.3              // --floe-pad-inline, either side of the box's content
	wideBox     = 34.0             // the content's width where part.css's @container turns
	narrowBox   = 16 - 2*padInline // the content's width in a column at its least, 16rem
	barShare    = 0.9              // of a series' width in a group of bars, the rest parting them
)

type chartTick struct {
	Y    float64
	Text string
	Zero bool
}

type chartLabel struct {
	X    float64
	Text string
	Wide bool
}

type chartSegment struct{ X1, Y1, X2, Y2 float64 }

type chartPoint struct {
	X, Y  float64
	Title string
}

type chartBar struct {
	X, Y, W, H, Mid float64
	Title, Text     string
}

type chartEnd struct {
	Y           float64
	Name, Value string
}

type chartSeries struct {
	N        int
	Name     string
	Segments []chartSegment
	Points   []chartPoint
	Bars     []chartBar
	End      *chartEnd
}

type chartRow struct {
	Name   string
	Values []string
}

// chart is what part.html draws.
// LIMIT: its fields are counted for the part chart's height, text and narrow class alone, so that other CSS comes out
// silently wrong; docs promise neither. Fix the fields and what they assume if own parts come to need to draw charts.
type chart struct {
	Kind, Title, Unit          string
	Sparse, Dense, Names, Thin bool
	Left, Right, RightNarrow   float64
	Ticks                      []chartTick
	Labels                     []chartLabel
	Series                     []chartSeries
	Rows                       []chartRow
}

// chartValues is a series as written: its numbers, as num writes them whole, and as a label writes them.
type chartValues struct {
	name         string
	numbers      []float64
	whole, shown []string
}

type chartInput struct {
	kind, title, unit string
	lowest            any
	x                 []string
	series            []chartValues
}

// LIMIT: a guess of the width, as units in internal/diagram guesses it, left to right only: a wide letter of a
// proportional face (W) takes more than it says and may have too little room. Measure if labels come to be cut.
func charWidth(c rune) float64 {
	if c >= 0x2e80 {
		return charWide
	}
	return charNarrow
}

func labelWidth(s string) float64 {
	w := 0.0
	for _, c := range s {
		w += charWidth(c)
	}
	return w
}

// chartScale is the scale's two ends and step, the ticks at whole steps from its start.
// LIMIT: 5 or so steps of 1, 2 or 5 times a power of 10; values whose difference a float cannot step through get two
// ticks.
func chartScale(lo, hi float64) (from, to, step float64) {
	if hi <= lo {
		hi = min(lo+max(1, math.Abs(lo)), math.MaxFloat64)
		if hi <= lo {
			lo = hi / 2
		}
	}
	raw := hi/5 - lo/5
	step = math.Pow(10, math.Floor(math.Log10(raw)))
	switch f := raw / step; {
	case f > 5:
		step *= 10
	case f > 2:
		step *= 5
	case f > 1:
		step *= 2
	}
	if raw == 0 || step == 0 || math.IsInf(step, 0) || math.IsNaN(step) {
		return lo, hi, hi - lo
	}
	return max(math.Floor(lo/step)*step, -math.MaxFloat64), min(math.Ceil(hi/step)*step, math.MaxFloat64), step
}

// digitsOf counts the digits of a number written, not its sign, grouping and point.
func digitsOf(text string) int {
	n := 0
	for _, c := range text {
		if c >= '0' && c <= '9' {
			n++
		}
	}
	return n
}

// shownNumber is a number as a chart's label writes it: as num does up to 9 digits; past them, rounded to 3 significant
// digits or all of the whole part; past 9 digits so too, in 3 digits and an exponent. The whole number is in the table.
// LIMIT: numbers of 10 digits or more can come out alike (1234567890, 1234999999), read in the table and the title.
func shownNumber(f float64, text string) string {
	if digitsOf(text) <= 9 {
		return text
	}
	if whole := digitsOf(strconv.FormatFloat(math.Trunc(math.Abs(f)), 'f', 0, 64)); whole <= 9 {
		if t, err := decimal("num", f, max(3, whole)); err == nil && digitsOf(t) <= 9 {
			return t
		}
	}
	return strconv.FormatFloat(f, 'g', 3, 64)
}

// shortLabel cuts a name of x to what fits 8rem, the whole name being in the table.
func shortLabel(s string) string {
	w := 0.0
	for i, c := range s {
		if w += charWidth(c); w > 8 {
			return s[:i] + "…"
		}
	}
	return s
}

// placed is err at a place in the part's value, when it is one the user can fix.
func placed(err error, at string) error {
	var e *usererr.Error
	if errors.As(err, &e) {
		return e.PlacedIn(at)
	}
	return err
}

// chartNumbers reads series i's values into found's mistakes, returning a failure of floe's.
func chartNumbers(i int, name string, vs []any, found *usererr.List) (chartValues, error) {
	got := chartValues{name: name, numbers: nil, whole: nil, shown: nil}
	for j, n := range vs {
		f, err := float(fmt.Sprintf("chart: series %d %q", i+1, name), n)
		var text string
		if err == nil {
			text, err = decimal("num", n, 0)
		}
		if failed := found.Add(placed(err, fmt.Sprintf("#/series/%d/values/%d", i, j))); failed != nil {
			return got, failed
		}
		if err == nil {
			got.numbers = append(got.numbers, f)
			got.whole = append(got.whole, text)
			got.shown = append(got.shown, shownNumber(f, text))
		}
	}
	return got, nil
}

// chartValuesOf reads the series, telling each one's mistakes.
func chartValuesOf(list []any, names int) ([]chartValues, error) {
	var found usererr.List
	series := make([]chartValues, 0, len(list))
	for i, one := range list {
		s, ok := one.(map[string]any)
		if !ok {
			return nil, usererr.New("kit_chart_shape", fmt.Sprintf("series %d", i+1))
		}
		vs, ok := s["values"].([]any)
		if !ok {
			return nil, usererr.New("kit_chart_shape", fmt.Sprintf("series %d", i+1))
		}
		name := stringOf(s["name"])
		if len(vs) != names {
			err := usererr.New("kit_chart_length", i+1, name, len(vs), names).
				PlacedIn(fmt.Sprintf("#/series/%d/values", i))
			if failed := found.Add(err); failed != nil {
				return nil, failed
			}
			continue
		}
		got, failed := chartNumbers(i, name, vs, &found)
		if failed != nil {
			return nil, failed
		}
		series = append(series, got)
	}
	return series, found.Err()
}

func chartInputOf(v any) (chartInput, error) {
	var in chartInput
	m, ok := v.(map[string]any)
	if !ok {
		return in, usererr.New("kit_chart_shape", "the value")
	}
	in.kind, in.title, in.unit, in.lowest = stringOf(m["kind"]), stringOf(m["title"]), stringOf(m["unit"]), m["min"]
	if in.kind != "line" && in.kind != "bar" {
		return in, usererr.New("kit_chart_shape", "kind")
	}
	xs, ok := m["x"].([]any)
	if !ok || len(xs) == 0 {
		return in, usererr.New("kit_chart_shape", "x")
	}
	list, ok := m["series"].([]any)
	if !ok || len(list) == 0 {
		return in, usererr.New("kit_chart_shape", "series")
	}
	for _, x := range xs {
		in.x = append(in.x, stringOf(x))
	}
	var err error
	in.series, err = chartValuesOf(list, len(xs))
	return in, err
}

// chartPlot places what is drawn, in percent of the plot, and its ticks: at whole steps from the start, written to
// digits decimals. The values are divided before they are compared, as the difference of two floats at their ends is
// past a float.
type chartPlot struct {
	bar              bool
	n, digits        int
	from, to, by, at float64
}

// chartPlotOf is the scale of the values. A min above a value is not drawn from: the scale would lose both it and 0.
// A min written takes the decimals it is written in, as the ticks go on from it.
// LIMIT: compared as floats, so that a difference past 2^53 reads as none.
func chartPlotOf(in chartInput) (chartPlot, error) {
	lowest, highest := math.Inf(1), math.Inf(-1)
	for _, s := range in.series {
		for _, f := range s.numbers {
			lowest, highest = min(lowest, f), max(highest, f)
		}
	}
	lo, hi, hasMin := min(0, lowest), max(0, highest), false
	if in.lowest != nil && in.kind == "line" {
		written, err := float("chart: min", in.lowest)
		if err != nil {
			return chartPlot{}, placed(err, "#/min") //nolint:exhaustruct // no plot
		}
		if written <= lowest {
			lo, hi, hasMin = written, highest, true
		}
	}
	from, to, step := chartScale(lo, hi)
	digits := max(0, int(-math.Floor(math.Log10(step))))
	if hasMin {
		from = lo
		to = min(from+math.Ceil(hi/step-from/step)*step, math.MaxFloat64)
		if to <= from {
			to = min(from+step, math.MaxFloat64)
		}
		if to <= from {
			from = to - step
		}
		digits = max(digits, decimalsOf(from))
	}
	return chartPlot{
		bar: in.kind == "bar", n: len(in.x), digits: digits,
		from: from, to: to, by: max(math.Abs(from), math.Abs(to)), at: step,
	}, nil
}

func decimalsOf(f float64) int {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

func (p chartPlot) y(f float64) float64 {
	return round2((p.to/p.by - f/p.by) / (p.to/p.by - p.from/p.by) * 100)
}

func (p chartPlot) x(i int) float64 {
	switch {
	case p.bar:
		return round2((float64(i) + 0.5) / float64(p.n) * 100)
	case p.n == 1:
		return 50
	}
	return round2(float64(i) / float64(p.n-1) * 100)
}

// tickValues is the values of the ticks and their words as num writes them, long when one has more digits than a
// label takes.
func tickValues(p chartPlot) (at []float64, words []string, long bool, err error) {
	// By count, not t += step: a step below the values' precision would never end.
	for i, count := 0, int(math.Round(p.to/p.at-p.from/p.at)); i <= count; i++ {
		t := min(p.from+float64(i)*p.at, p.to)
		if len(at) > 0 && t == at[len(at)-1] {
			continue
		}
		v := roundTo(t, p.digits)
		text, err := decimal("num", v, 0)
		if err != nil {
			return nil, nil, false, err
		}
		at, words = append(at, v), append(words, text)
		long = long || digitsOf(text) > 9
		if t >= p.to {
			break
		}
	}
	return at, words, long, nil
}

// wordsTell says that no two words are alike and that each reads as its line's value, by less than the scale can show.
func wordsTell(at []float64, words []string, step float64) bool {
	seen := map[string]bool{}
	for i, word := range words {
		f, err := strconv.ParseFloat(strings.ReplaceAll(word, ",", ""), 64)
		if err != nil || seen[word] || math.Abs(f-at[i]) > step/100 {
			return false
		}
		seen[word] = true
	}
	return true
}

// chartTicks is the ticks and the room of their words, each of which says the value of its line. A line for 0 stands
// on any scale across it, with no word if no tick falls on 0.
// LIMIT: past 9 digits, in as few digits as tell the ticks apart and read as their lines, to 5; beyond those no words,
// as a tick read wrong is worse than the lines alone, the numbers being in the table. Draw from a base if such charts
// come to be needed.
func chartTicks(p chartPlot) ([]chartTick, float64, error) {
	at, words, long, err := tickValues(p)
	if err != nil {
		return nil, 0, err
	}
	told := !long && wordsTell(at, words, p.at)
	for prec := 3; prec <= 5 && !told; prec++ {
		for i, t := range at {
			words[i] = strconv.FormatFloat(t, 'g', prec, 64)
		}
		told = wordsTell(at, words, p.at)
	}
	ticks := make([]chartTick, 0, len(at)+1)
	room := 0.0
	for i, t := range at {
		text := words[i]
		if !told {
			text = ""
		}
		ticks = append(ticks, chartTick{Y: p.y(t), Text: text, Zero: t == 0})
		room = max(room, labelWidth(text))
	}
	if p.from < 0 && p.to > 0 && !slices.Contains(at, 0) {
		ticks = append(ticks, chartTick{Y: p.y(0), Text: "", Zero: true})
	}
	return ticks, room, nil
}

func (in chartInput) hoverTitle(i, j int) string {
	title := in.x[j] + " " + in.series[i].name + " " + in.series[i].whole[j]
	if in.unit != "" {
		title += " " + in.unit
	}
	return title
}

// drawBars puts the bars and their numbers in c.
// LIMIT: bars under 1px come out as a run of lines, the numbers being in the table.
func (c *chart) drawBars(in chartInput, p chartPlot) {
	n := float64(p.n)
	group := min(100/n*0.5, 24)
	width := group / float64(len(in.series))
	// A bar under 4px at the wide box has no room for a frame: drawn with one, it would be frame alone. In a narrow box
	// part.css takes every bar's frame off.
	wide := wideBox - c.Left - c.Right
	c.Thin = width*barShare*wide*16/100 < 4
	drawn := false
	if len(in.series) == 1 {
		room := 0.0
		for _, t := range in.series[0].shown {
			room = max(room, labelWidth(t))
		}
		c.Sparse = n*(room+0.3) > narrowBox-c.Left-c.Right
		drawn = n*(room+0.3) <= wide
	}
	for i, values := range in.series {
		s := chartSeries{N: i + 1, Name: values.name, Segments: nil, Points: nil, Bars: nil, End: nil}
		for j, f := range values.numbers {
			x := p.x(j) - group/2 + float64(i)*width
			y0, y1 := p.y(max(f, 0)), p.y(min(f, 0))
			text := ""
			if drawn {
				text = values.shown[j]
			}
			s.Bars = append(s.Bars, chartBar{
				X: round2(x + width*(1-barShare)/2), Y: y0, W: round2(width * barShare), H: round2(y1 - y0),
				Mid: round2(x + width/2), Title: in.hoverTitle(i, j), Text: text,
			})
		}
		c.Series = append(c.Series, s)
	}
}

// drawLines puts the lines and the labels at their ends in c, edge being the room half of a name of x takes.
func (c *chart) drawLines(in chartInput, p chartPlot, edge float64) {
	room, narrow := edge, edge
	ends := make([]*chartEnd, 0, len(in.series))
	for i, values := range in.series {
		s := chartSeries{N: i + 1, Name: values.name, Segments: nil, Points: nil, Bars: nil, End: nil}
		for j, f := range values.numbers {
			s.Points = append(s.Points, chartPoint{X: p.x(j), Y: p.y(f), Title: in.hoverTitle(i, j)})
			if j > 0 {
				before := values.numbers[j-1]
				s.Segments = append(s.Segments, chartSegment{X1: p.x(j - 1), Y1: p.y(before), X2: p.x(j), Y2: p.y(f)})
			}
		}
		last := len(values.numbers) - 1
		s.End = &chartEnd{Y: p.y(values.numbers[last]), Name: values.name, Value: values.shown[last]}
		ends = append(ends, s.End)
		room = max(room, 0.7+labelWidth(values.name)+charNarrow+labelWidth(s.End.Value))
		narrow = max(narrow, 0.7+labelWidth(s.End.Value))
		c.Series = append(c.Series, s)
	}
	spread(ends, labelHeight/plotHeight*100)
	// Names too long stay in the legend alone, not to take over half of the plot's room.
	c.Names = room <= (wideBox-c.Left)/2
	if !c.Names {
		room = narrow
	}
	c.Right, c.RightNarrow = round2(room+0.2), round2(narrow+0.2)
}

// drawLabels puts the names of x in c, each width's by the room it leaves, so that they do not meet however long the
// numbers beside them.
// LIMIT: with no room for two, as for long names in a column, one name stands alone, the others being in the table;
// two would meet.
func (c *chart) drawLabels(in chartInput, p chartPlot, longest float64) {
	stride := func(room float64) int { return max(1, int(math.Ceil(float64(p.n)*(longest+0.8)/max(room, 1)))) }
	wide, narrow := stride(wideBox-c.Left-c.Right), stride(narrowBox-c.Left-c.RightNarrow)
	// The narrow box shows some of those the wide one does: a stride of its own would keep only where both fall.
	narrow = (narrow + wide - 1) / wide * wide
	for i, name := range in.x {
		if i%wide == 0 {
			c.Labels = append(c.Labels, chartLabel{X: p.x(i), Text: shortLabel(name), Wide: i%narrow != 0})
		}
	}
}

// chartOf reads a chart's whole value and counts where each thing goes, part.html writing the SVG.
// LIMIT: given a piece of a part's value, the places of its mistakes are off, as they count from the part's value.
// LIMIT: a fifth series is not refused for an own part, whose schema says what it takes: it has no colour.
// LIMIT: no bound on the points, a page only growing (ten thousand points of one series, 2 MB). Thin them if pages
// come to be slow.
func chartOf(v any) (*chart, error) {
	in, err := chartInputOf(v)
	if err != nil {
		return nil, err
	}
	p, err := chartPlotOf(in)
	if err != nil {
		return nil, err
	}
	ticks, room, err := chartTicks(p)
	if err != nil {
		return nil, err
	}
	longest := 0.0
	for _, name := range in.x {
		longest = max(longest, labelWidth(shortLabel(name)))
	}
	edge := 0.0
	if !p.bar {
		edge = longest/2 + 0.2
	}
	rows := make([]chartRow, 0, p.n)
	for j, name := range in.x {
		row := chartRow{Name: name, Values: nil}
		for _, s := range in.series {
			row.Values = append(row.Values, s.whole[j])
		}
		rows = append(rows, row)
	}
	c := &chart{
		Kind: in.kind, Title: in.title, Unit: in.unit,
		Sparse: false, Dense: p.n > 12, Names: false, Thin: false,
		Left: round2(max(room+0.7, 2, edge)), Right: round2(edge + 0.2), RightNarrow: round2(edge + 0.2),
		Ticks: ticks, Labels: nil, Series: nil, Rows: rows,
	}
	if p.bar {
		c.drawBars(in, p)
	} else {
		c.drawLines(in, p, edge)
	}
	c.drawLabels(in, p, longest)
	return c, nil
}

// spread moves the labels at the ends of the lines apart by gap, in order, keeping them as near as it can.
func spread(ends []*chartEnd, gap float64) {
	order := slices.Clone(ends)
	slices.SortStableFunc(order, func(a, b *chartEnd) int { return cmp.Compare(a.Y, b.Y) })
	for i := 1; i < len(order); i++ {
		order[i].Y = max(order[i].Y, order[i-1].Y+gap)
	}
	if last := len(order) - 1; order[last].Y > 100 {
		order[last].Y = 100
		for i := last - 1; i >= 0; i-- {
			order[i].Y = min(order[i].Y, order[i+1].Y-gap)
		}
	}
	for _, e := range order {
		e.Y = round2(e.Y)
	}
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func roundTo(f float64, digits int) float64 {
	v, err := strconv.ParseFloat(strconv.FormatFloat(f, 'f', digits, 64), 64)
	if err != nil {
		return f
	}
	return v
}
