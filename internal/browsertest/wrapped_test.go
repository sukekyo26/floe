package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// findWrapping keeps, in window.floeWrap, the rows that wrap: a flex row that wraps, and a grid whose children stand
// in fewer columns when the grid is 1px wide than when it is 100000px wide (auto-fit, auto-fill), whatever the page's width.
const findWrapping = `(() => {
  const cols = (e) => new Set([...e.children].filter((c) => c.getClientRects().length).map((c) => Math.round(c.getBoundingClientRect().left))).size
  const at = (e, width) => {
    const was = e.style.cssText
    for (const [k, v] of [['width', width], ['min-width', '0'], ['max-width', 'none']]) e.style.setProperty(k, v, 'important')
    const n = cols(e)
    e.style.cssText = was
    return n
  }
  window.floeWrap = [...document.querySelectorAll('body *')].filter((e) => {
    const s = getComputedStyle(e)
    return /flex/.test(s.display) ? s.flexWrap !== 'nowrap' : /grid/.test(s.display) && at(e, '1px') < at(e, '100000px')
  })
  return ''
})()`

// measureAlike returns, for each row that wraps, the kinds of its children (the same tag, part and look)
// whose borders' widths and styles or padding differ from one child to another, and those of one tone whose borders'
// colours differ. A child in a state marked by a class (options' pick) is of its siblings' kind, so that a class
// does not set the first child apart: a state is drawn by an outline, a ground or --floe-inset, not by a border or
// padding, as a theme's decoration is
const measureAlike = `(window.floeWrap || []).flatMap((e) => {
  const kinds = new Map(), add = (key, look) => kinds.set(key, (kinds.get(key) || new Set()).add(look))
  for (const c of e.children) {
    if (!c.getClientRects().length || c.matches(':disabled')) continue
    const kind = c.tagName.toLowerCase() +
      ['floePart', 'floeLook'].map((d) => c.dataset[d] ? '[' + c.dataset[d] + ']' : '').join('')
    const s = getComputedStyle(c), sides = ['Top', 'Right', 'Bottom', 'Left']
    add(kind, sides.map((side) => [s['border' + side + 'Width'], s['border' + side + 'Style'], s['padding' + side]].join(' ')).join(', '))
    add(kind + ' of tone ' + (c.dataset.floeTone || 'none'), sides.map((side) => s['border' + side + 'Style'] === 'none' ? '' : s['border' + side + 'Color']).join(', '))
  }
  const part = e.closest('[data-floe-part]')
  const where = (part ? part.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + [...e.classList].map((x) => '.' + x).join('')
  return [...kinds].filter(([, looks]) => looks.size > 1).map(([key, looks]) => where + ' > ' + key + ': ' + [...looks].join(' / '))
}).filter((v, i, a) => a.indexOf(v) === i).join('\n')`

// TestUIWrappedAlike draws every part and floe's templates' examples in every theme at a phone's width and a wide
// screen's, and finds, in every row that wraps (a flex row that wraps, a grid of auto-fit or auto-fill columns),
// the children of one kind framed and padded alike: which child begins a line is the width's, so no CSS marks a
// child by its place in such a row. A disabled child is left out, and a side without a line is not read for its
// colour (a picked tab's).
// LIMIT: a row is found only where the samples draw it with two children or more, and one's own parts and templates
// are not drawn; a row a sample does not reach is seen on the page
func TestUIWrappedAlike(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	pages := map[string]string{}
	everyPart, err := os.ReadFile("../corpus/testdata/every-part/every-part.yaml")
	if err != nil {
		t.Fatal(err)
	}
	shot, err := os.ReadFile("../corpus/testdata/every-part/shots/moved.png")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "shots/moved.png", string(shot))
	pages["every-part"] = s.write(t, "every.yaml", string(everyPart))
	examples, err := filepath.Glob("../corpus/testdata/example/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range examples {
		data, err := os.ReadFile(ex)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(ex), ".yaml")
		pages[name] = s.write(t, "example-"+name+".yaml", string(data))
	}
	defer b.unsize()
	var amiss []string
	for _, theme := range themes {
		for name, file := range pages {
			slug := "alike-" + name + "-" + theme.Name()
			s.floe(t, "publish", file, "--slug", slug, "--theme", theme.Name())
			b.size(480, 900, false)
			b.navigate("/a/" + slug + "/?raw")
			b.eval(findWrapping)
			for _, w := range []int64{480, 1600} {
				b.size(w, 900, false)
				if got := b.eval(measureAlike); got != "" {
					for _, l := range strings.Split(got, "\n") {
						amiss = append(amiss, fmt.Sprintf("%s in %s at %dpx: %s", name, theme.Name(), w, l))
					}
				}
			}
		}
	}
	check(t, "wrapped rows", amiss, "")
}
