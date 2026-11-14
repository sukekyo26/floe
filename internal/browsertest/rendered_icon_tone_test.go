package browsertest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"testing"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const iconTonePage = `template: report
lang: en
title: Icon tones
sections:
  - heading: Icons
    parts:
      - diagram:
          title: Lucide icons
          parts:
            - icon: { name: a, x: 20, y: 20, icon: lucide/database, tone: bad }
            - icon: { name: b, x: 120, y: 20, icon: lucide/server }
            - frame: { x: 220, y: 10, w: 200, h: 100, icon: lucide/cloud, label: Frame }
            - graph: { x: 20, y: 140, nodes: [ { icon: { name: g, icon: lucide/users, tone: good } } ] }
`

// iconInks tells, as JSON, each icon <use> of the first diagram as [its box in the window, the colour its color is]:
// the colour the mask over currentColor paints its lines.
const iconInks = `(() => {
  const out = [];
  for (const u of document.querySelector('svg.dg').querySelectorAll('use.icon')) {
    const r = u.getBoundingClientRect(), c = getComputedStyle(u).color.match(/\d+/g).map(Number);
    out.push({x: r.x, y: r.y, w: r.width, h: r.height, rgb: c.slice(0, 3), tone: u.dataset.floeTone || ''});
  }
  return JSON.stringify(out);
})()`

type iconInk struct {
	X, Y, W, H float64
	RGB        [3]int
	Tone       string
}

// TestRenderedIconTone draws lucide icons as an icon part, a frame's corner and a graph's node: each paints its lines
// in its <use>'s color, which is the tone's or the text's, on the screen, zoomed in and in print. A mask filled
// otherwise, or a CSS rule that no longer reaches the <use>, draws them black or in one colour.
func TestRenderedIconTone(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.floe(t, "publish", s.write(t, "icon-tone.yaml", iconTonePage), "--slug", "icon-tone")
	s.b.navigate("/a/icon-tone/?raw")
	s.b.until("the diagram", "document.querySelector('svg.dg use.icon')")
	s.b.eval("document.querySelector('svg.dg').scrollIntoView(), true")
	check := func(when string) {
		t.Helper()
		var inks []iconInk
		if err := json.Unmarshal([]byte(s.b.eval(iconInks)), &inks); err != nil {
			t.Fatal(err)
		}
		if len(inks) != 4 {
			t.Fatalf("%s: %d icons, want 4", when, len(inks))
		}
		shot := screenshot(t, s.b)
		colours := map[string][3]int{}
		for _, ink := range inks {
			// Those of no tone (the part b and the frame's corner) are all in the text's colour.
			if c, ok := colours[ink.Tone]; ok && c != ink.RGB {
				t.Errorf("%s: icons of tone %q in %v and %v", when, ink.Tone, c, ink.RGB)
			}
			colours[ink.Tone] = ink.RGB
			if !inked(shot, ink) {
				t.Errorf("%s: the icon of tone %q at %.0f,%.0f paints no line in its color %v", when, ink.Tone, ink.X, ink.Y, ink.RGB)
			}
		}
		if colours["bad"] == colours[""] || colours["good"] == colours[""] {
			t.Errorf("%s: a tone draws in the text's colour: %v", when, colours)
		}
	}
	check("on the screen")
	s.b.eval("document.querySelector('.floe-zoom-tools [data-zoom=\"in\"]').click(), true")
	s.b.still()
	check("zoomed in")
	s.b.media("print")
	defer s.b.media("")
	s.b.still()
	check("in print")
}

func screenshot(t *testing.T, b *browser) image.Image {
	t.Helper()
	shot, err := chromedp.Call(b.ctx, page.CaptureScreenshot, page.CaptureScreenshotParams{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(shot.Data))
	if err != nil {
		t.Fatal(fmt.Errorf("the screenshot: %w", err))
	}
	return img
}

// inked reports whether a pixel inside ink's box is its colour, within what antialiasing leaves of a line 2 units
// wide at 48: some pixel of a line is wholly covered.
func inked(img image.Image, ink iconInk) bool {
	for y := int(ink.Y); y < int(ink.Y+ink.H); y++ {
		for x := int(ink.X); x < int(ink.X+ink.W); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if near(int(r>>8), ink.RGB[0]) && near(int(g>>8), ink.RGB[1]) && near(int(b>>8), ink.RGB[2]) {
				return true
			}
		}
	}
	return false
}

func near(a, b int) bool { return a-b <= 6 && b-a <= 6 }
