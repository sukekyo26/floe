package diagram_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/icons"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// check parses a spec of a 300x200 canvas (or w x h when given in head) with parts and checks it.
func check(t *testing.T, head, parts string) error {
	t.Helper()
	if head == "" {
		head = "width: 300\nheight: 200\n"
	}
	return diagram.Check(parse(t, "s.yaml", "id: d\ntitle: t\n"+head+"parts:\n"+parts), standIn)
}

func TestCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, head, parts string
		want              []string
	}{
		{
			name: "clean", head: "width: 300\nheight: 120\n",
			parts: `  - frame: { x: 2, y: 2, w: 296, h: 116 }
  - frame: { x: 10, y: 10, w: 280, h: 100, label: f }
  - box: { x: 30, y: 44, w: 100, h: 52, rows: [a, b] }
  - box: { x: 170, y: 44, w: 100, h: 52, rows: [c] }
  - edge: { d: "M130,70 H170", label: x, at: [150, 56] }
`,
			want: nil,
		},
		{
			// the five kinds of mistake of run.sh, all reported at once
			name: "every kind at once", head: "width: 300\nheight: 120\n",
			parts: `  - frame: { x: 10, y: 10, w: 150, h: 100, label: f }
  - box: { x: 20, y: 40, w: 60, h: 40, rows: [とても長い名前の箱] }
  - box: { x: 70, y: 40, w: 60, h: 40, rows: [b] }
  - box: { x: 140, y: 40, w: 60, h: 40, rows: [c] }
  - box: { x: 220, y: 40, w: 40, h: 30, rows: [d] }
  - edge: { d: "M240,0 V120" }
  - box: { x: 270, y: 90, w: 40, h: 30, rows: [e] }
`,
			want: []string{
				`s.yaml:7:5: box "とても長い名前の箱": "とても長い名前の箱" needs about 89px more width`,
				`s.yaml:12:5: box "e" sticks out of the 300x120 canvas`,
				`s.yaml:7:5: box "とても長い名前の箱" overlaps box "b" (line 8)`,
				`s.yaml:9:5: box "c" straddles the edge of frame "f" (line 6)`,
				`s.yaml:11:5: edge "M240,0 V120" runs through box "d" (line 10)`,
			},
		},
		{
			name: "text, canvas and edges through boxes",
			parts: `  - box: { x: 10, y: 10, w: 60, h: 20, rows: [a, b], shape: hex }
  - table: { x: 100, y: 10, w: 80, title: users, rows: [[identifier, string], ---, [id]] }
  - dot: { cx: 295, cy: 50 }
  - circle: { cx: 150, cy: 150, r: 60 }
  - bar: { x1: 100, x2: 180, y: 120 }
  - bar: { x1: 100, x2: 180, y: 150, label: phase }
  - axis: { x1: 10, x2: 290, y: 170, ticks: [[10, 10/06], [290, 10/07]] }
  - icon: { x: 200, y: 40, name: aws/aws-lambda, label: Lambda }
  - label: { text: strong, x: 200, y: 195, anchor: end, strong: true }
  - edge: { d: "M10,50.5 H295", label: own, at: [40, 50.5] }
`,
			want: []string{
				`s.yaml:6:5: box "a": "a" needs about 16px more width`,
				`s.yaml:6:5: box "a": "b" needs about 16px more width`,
				`s.yaml:6:5: box "a": its text needs a height of 46px or more; the box has 20px`,
				`s.yaml:7:5: table "users": "identifier  string" needs about 78px more width`,
				`s.yaml:8:5: dot at 295,50 sticks out of the 300x200 canvas`,
				`s.yaml:12:5: label "10/06" sticks out of the 300x200 canvas`,
				`s.yaml:12:5: label "10/07" sticks out of the 300x200 canvas`,
				`s.yaml:14:5: label "strong" sticks out of the 300x200 canvas`,
				`s.yaml:9:5: circle at 150,150 sticks out of the 300x200 canvas`,
				`s.yaml:15:5: edge "M10,50.5 H295" runs through table "users" (line 7)`,
				`s.yaml:15:5: edge "M10,50.5 H295" runs through icon "aws/aws-lambda" (line 13)`,
			},
		},
		{
			name:  "bar named by its span",
			parts: "  - bar: { x1: 0.5, x2: 400, y: 50 }\n",
			want:  []string{`s.yaml:6:5: bar "0.5-400" sticks out of the 300x200 canvas`},
		},
		{
			name: "crossing edges",
			parts: `  - edge: { d: "M10,100 H290", end: none }
  - edge: { d: "M150,10 V190", end: none }
`,
			want: []string{`s.yaml:6:5: edge "M10,100 H290" crosses edge "M150,10 V190" (line 7) at 150,100; move one of them, or add crossing: true to one if it cannot be avoided`},
		},
		{
			name: "a crossing one edge allows",
			parts: `  - edge: { d: "M10,100 H290", end: none }
  - edge: { d: "M150,10 V190", end: none, crossing: true }
`,
			want: nil,
		},
		{
			name: "branches, joins and shared runs",
			parts: `  - edge: { d: "M10,100 H150 V190", end: none }
  - edge: { d: "M150,100 H290", end: none }
  - edge: { d: "M10,100 H150 V10", end: none }
`,
			want: nil,
		},
		{
			name: "edge through a label",
			parts: `  - label: { text: note, x: 150, y: 80 }
  - edge: { d: "M150,60 V100", end: none }
`,
			want: []string{`s.yaml:7:5: edge "M150,60 V100" runs through label "note" (line 6)`},
		},
		{
			name: "edge through a frame heading",
			parts: `  - frame: { x: 20, y: 120, w: 200, h: 60, label: Heading }
  - edge: { d: "M50,125 V150", end: none }
`,
			want: []string{`s.yaml:7:5: edge "M50,125 V150" runs through the heading of frame "Heading" (line 6)`},
		},
		{
			// the heading of a frame with an icon starts right of the icon, and edges may not cross the icon
			name: "heading beside a frame icon",
			parts: `  - frame: { x: 20, y: 120, w: 200, h: 60, label: Heading, icon: aws/group/region }
  - edge: { d: "M56,125 V150", end: none }
  - edge: { d: "M30,125 V150", end: none }
`,
			want: []string{`s.yaml:8:5: edge "M30,125 V150" runs through the icon of frame "Heading" (line 6)`},
		},
		{
			// the corner icon is a part: a box may not cover it
			name: "box over a frame icon",
			parts: `  - frame: { x: 20, y: 120, w: 200, h: 60, label: Heading, icon: aws/group/region }
  - box: { x: 40, y: 140, w: 60, h: 30, rows: [b] }
`,
			want: []string{`s.yaml:6:5: the icon of frame "Heading" overlaps box "b" (line 7)`},
		},
		{
			name: "icons not there",
			parts: `  - icon: { x: 10, y: 10, name: lambda }
  - icon: { x: 70, y: 10, name: aws/lambda }
  - icon: { x: 130, y: 10, name: aws/amazon-ec2 }
  - frame: { x: 10, y: 100, w: 200, h: 60, label: Cloud, icon: aws/cloud }
  - icon: { x: 190, y: 10, name: aws/ }
`,
			want: []string{
				`s.yaml:6:5: icon "lambda": "lambda" needs its icon set in front (aws), like aws/amazon-ec2 (search with: floe icons search <word>)`,
				`s.yaml:7:5: icon "aws/lambda": no icon "aws/lambda"; did you mean aws/aws-lambda? (search with: floe icons search <word>)`,
				`s.yaml:8:5: icon "aws/amazon-ec2": no icon "aws/amazon-ec2" (search with: floe icons search <word>)`,
				`s.yaml:9:5: frame "Cloud": no icon "aws/cloud"; did you mean aws/group/aws-cloud-logo? (search with: floe icons search <word>)`,
				`s.yaml:10:5: icon "aws/": no icon "aws/" (search with: floe icons search <word>)`,
			},
		},
		{
			// sampling every 2px, as diagram.mjs did, missed the 2px inside of the narrowest bar
			name: "edges through thin or clipped parts",
			parts: `  - bar: { x1: 100, x2: 100, y: 50 }
  - edge: { d: "M1,50 H201", end: none }
  - box: { x: 150, y: 100, w: 40, h: 40, rows: [b] }
  - edge: { d: "M140,104 L156,88", end: none }
  - edge: { d: "M150,150 L186,96", end: none }
`,
			want: []string{
				`s.yaml:7:5: edge "M1,50 H201" runs through bar "100-100" (line 6)`,
				`s.yaml:10:5: edge "M150,150 L186,96" runs through box "b" (line 8)`,
			},
		},
		{
			name: "edges along an edge or touching a corner",
			parts: `  - box: { x: 100, y: 100, w: 40, h: 40, rows: [b] }
  - edge: { d: "M90,139 H150", end: none }
  - edge: { d: "M129,91 L149,111", end: none }
  - edge: { d: "M120,120 V120", end: none }
`,
			want: []string{`s.yaml:9:5: edge "M120,120 V120" runs through box "b" (line 6)`},
		},
		{
			name:  "the same mistake once",
			parts: "  - axis: { x1: 10, x2: 290, y: 100, ticks: [[400, x], [500, x]] }\n",
			want:  []string{`s.yaml:6:5: label "x" sticks out of the 300x200 canvas`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := check(t, tt.head, tt.parts)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("Check failed:\n%v", err)
				}
				return
			}
			if got, want := errString(err), strings.Join(tt.want, "\n"); got != want {
				t.Errorf("Check\n got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestCheckLocalize(t *testing.T) {
	t.Parallel()

	err := check(t, "", "  - frame: { x: 10, y: 10, w: 100, h: 100, label: VPC }\n  - box: { x: 90, y: 40, w: 60, h: 40, rows: [API] }\n")
	var l i18n.Localizer
	if !errors.As(err, &l) {
		t.Fatalf("error %v is not localized", err)
	}
	if got, want := l.Localize(i18n.New("ja")), "s.yaml:7:5: box「API」がframe「VPC」（6 行目）の境界をまたいでいます"; got != want {
		t.Errorf("Localize(ja) = %q, want %q", got, want)
	}
}

// Icons not fetched stop the check with how to fetch them, rather than as a mistake of the spec.
func TestCheckIconsNotFetched(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s := parse(t, "s.yaml", "id: d\ntitle: t\nwidth: 100\nheight: 100\nparts:\n  - icon: { x: 10, y: 10, name: aws/aws-lambda }\n")
	err := diagram.Check(s, icons.New(dir))
	var p yamlpos.Problems
	want := "AWS icons are not in " + filepath.Join(dir, "aws") + `; run "floe icons fetch aws" (downloads about 14MB from aws.amazon.com) and try again`
	if errors.As(err, &p) || errString(err) != want {
		t.Errorf("Check = %v, want %s", err, want)
	}
}
