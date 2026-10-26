// Package diagram draws a diagram spec, a canvas and the parts placed on it, as inline SVG. A spec is checked
// against its schema (the schema.json of floe's part diagram, which its caller compiles) first, wherever it
// comes from; floe then resolves its names, lays it out
// once for every language its text is written in, and checks the layout. Every mistake is at a JSON pointer in
// the spec, for the caller to place.
package diagram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// URL is where the schema of a spec is held: that of floe's part diagram.
const URL = "floe:///parts/diagram.json"

// Text is text in each language by tag, as i18n.Tag reads it; text written plain is English.
type Text map[string]string

// UnmarshalJSON reads text plain, or by language, its tags as i18n.Tag reads them. Two tags of one language
// (JA and ja) are the spec's mistake, at its root, as the text's place is not known here.
// LIMIT: the mistake stops the diagram's other checks and names no text by place; rework it if it comes up often.
func (t *Text) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*t = Text{"en": s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err //nolint:wrapcheck // floe's failure: the schema checked the shape
	}
	tags := slices.Sorted(maps.Keys(m))
	*t = Text{}
	for i, tag := range tags {
		if first := i18n.Again(tags, i); first != "" {
			return usererr.NewIn("", "#", "lang_twice", first, tag) //nolint:wrapcheck // the spec's mistake
		}
		(*t)[string(i18n.Tag(tag))] = m[tag]
	}
	return nil
}

// in is t in lang, as i18n falls back: lang, then its parents, then English.
func (t Text) in(lang i18n.Lang) string {
	for _, l := range i18n.Fallbacks(lang) {
		if s, ok := t[string(l)]; ok {
			return s
		}
	}
	return t["en"]
}

// all is t in each of its languages, in the order of their tags.
func (t Text) all() []string {
	out := make([]string, 0, len(t))
	for _, k := range slices.Sorted(maps.Keys(t)) {
		out = append(out, t[k])
	}
	return out
}

type spec struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Title  Text    `json:"title"`
	Parts  []part  `json:"parts"`
}

// part is one of its kinds, the one that is not nil.
type part struct {
	Frame  *frame  `json:"frame"`
	Box    *box    `json:"box"`
	Table  *table  `json:"table"`
	Icon   *icon   `json:"icon"`
	Edge   *edge   `json:"edge"`
	Dot    *dot    `json:"dot"`
	Bar    *bar    `json:"bar"`
	Axis   *axis   `json:"axis"`
	Circle *circle `json:"circle"`
	Label  *label  `json:"label"`
	Line   *line   `json:"line"`
	Raw    *raw    `json:"raw"`
}

type frame struct {
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	Label  Text    `json:"label"`
	Accent bool    `json:"accent"`
	Icon   string  `json:"icon"`
	Square bool    `json:"square"`
}

type box struct {
	Name   string  `json:"name"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Origin string  `json:"origin"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	Rows   []row   `json:"rows"`
	Tone   string  `json:"tone"`
	Shape  string  `json:"shape"`
}

// row is a line of text in a box, and its style; "" for the default of its place.
type row struct {
	Text  Text
	Style string
}

func (r *row) UnmarshalJSON(b []byte) error {
	var pair []json.RawMessage
	if json.Unmarshal(b, &pair) == nil {
		if len(pair) != 2 {
			return fmt.Errorf("a row of %d values", len(pair)) //nolint:err113 // floe's failure: the schema checked it
		}
		if err := json.Unmarshal(pair[1], &r.Style); err != nil {
			return err //nolint:wrapcheck // as above
		}
		b = pair[0]
	}
	return json.Unmarshal(b, &r.Text) //nolint:wrapcheck // as above
}

type table struct {
	Name   string     `json:"name"`
	X      float64    `json:"x"`
	Y      float64    `json:"y"`
	Origin string     `json:"origin"`
	W      float64    `json:"w"`
	Title  Text       `json:"title"`
	Rows   []tableRow `json:"rows"`
	Tone   string     `json:"tone"`
}

// tableRow is a name and a type, or a separator; Name names it as an edge's end.
type tableRow struct {
	Name  string
	Cells []Text
	Sep   bool
}

func (r *tableRow) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		if s == "---" {
			r.Sep = true
			return nil
		}
		r.Cells = []Text{{"en": s}}
		return nil
	}
	var named struct {
		Name  string `json:"name"`
		Cells []Text `json:"cells"`
	}
	if bytes.HasPrefix(bytes.TrimSpace(b), []byte("{")) {
		if err := strictly(b, &named); err != nil {
			return err
		}
		r.Name, r.Cells = named.Name, named.Cells
		return nil
	}
	return json.Unmarshal(b, &r.Cells) //nolint:wrapcheck // floe's failure: the schema checked the shape
}

type icon struct {
	Name  string  `json:"name"`
	Icon  string  `json:"icon"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label Text    `json:"label"`
	Size  float64 `json:"size"`
}

// end is an edge's end: the name of a part or a named row, or a point.
type end struct {
	Name  string
	Point *[2]float64
}

func (e *end) UnmarshalJSON(b []byte) error {
	if json.Unmarshal(b, &e.Name) == nil {
		return nil
	}
	e.Point = new([2]float64)
	return json.Unmarshal(b, e.Point) //nolint:wrapcheck // floe's failure: the schema checked the shape
}

type edge struct {
	From     end          `json:"from"`
	To       end          `json:"to"`
	FromSide string       `json:"fromSide"`
	ToSide   string       `json:"toSide"`
	Via      [][2]float64 `json:"via"`
	Straight bool         `json:"straight"`
	Label    Text         `json:"label"`
	At       *[2]float64  `json:"at"`
	Dashed   bool         `json:"dashed"`
	Bold     bool         `json:"bold"`
	Tone     string       `json:"tone"`
	Start    *string      `json:"start"`
	End      *string      `json:"end"`
	Crossing bool         `json:"crossing"`
}

type dot struct {
	Name   string  `json:"name"`
	CX     float64 `json:"cx"`
	CY     float64 `json:"cy"`
	R      float64 `json:"r"`
	Tone   string  `json:"tone"`
	Hollow bool    `json:"hollow"`
}

type bar struct {
	Name  string  `json:"name"`
	X1    float64 `json:"x1"`
	X2    float64 `json:"x2"`
	Y     float64 `json:"y"`
	Tone  string  `json:"tone"`
	Label Text    `json:"label"`
}

type axis struct {
	X1    float64  `json:"x1"`
	X2    float64  `json:"x2"`
	Y     float64  `json:"y"`
	Ticks []tick   `json:"ticks"`
	Grid  *float64 `json:"grid"`
}

// tick is a mark on an axis, [x, text].
type tick struct {
	X    float64
	Text Text
}

func (t *tick) UnmarshalJSON(b []byte) error {
	var pair [2]json.RawMessage
	if err := json.Unmarshal(b, &pair); err != nil {
		return err //nolint:wrapcheck // floe's failure: the schema checked the shape
	}
	if err := json.Unmarshal(pair[0], &t.X); err != nil {
		return err //nolint:wrapcheck // as above
	}
	return json.Unmarshal(pair[1], &t.Text) //nolint:wrapcheck // as above
}

type circle struct {
	Name string  `json:"name"`
	CX   float64 `json:"cx"`
	CY   float64 `json:"cy"`
	R    float64 `json:"r"`
	Tone string  `json:"tone"`
}

type label struct {
	Text   Text     `json:"text"`
	X      *float64 `json:"x"`
	Y      *float64 `json:"y"`
	Anchor string   `json:"anchor"`
	Plain  bool     `json:"plain"`
	Strong bool     `json:"strong"`
	In     []string `json:"in"`
}

type line struct {
	X1   float64 `json:"x1"`
	Y1   float64 `json:"y1"`
	X2   float64 `json:"x2"`
	Y2   float64 `json:"y2"`
	Kind string  `json:"kind"`
}

type raw struct {
	SVG string `json:"svg"`
}

// read reads v, a spec its schema has checked, as a spec: written as JSON once and read back, a key the
// structs do not know refused, so that the schema and the structs cannot drift apart unseen.
func read(v any) (*spec, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("diagram: write the spec: %w", err)
	}
	var s spec
	if err := strictly(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func strictly(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var e *usererr.Error
	if err := dec.Decode(v); errors.As(err, &e) {
		return e
	} else if err != nil {
		return fmt.Errorf("diagram: read a spec its schema took: %w", err)
	}
	return nil
}
