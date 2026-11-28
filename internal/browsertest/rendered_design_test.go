package browsertest_test

import "testing"

// TestRenderedDesign draws design's samples of colour and type, which put the values written into a style
// attribute: each tile is the colour written and each sample the size and weight written, so the page's CSP lets the
// attribute through and html/template keeps a value of the shapes the schema lets in (not ZgotmplZ).
func TestRenderedDesign(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.write(t, "screen.svg", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 36 64" width="36" height="64"/>`)
	data := s.write(t, "design.yaml", `template: design
lang: en
title: Samples
goal: 'The samples'
screens:
  - { name: 'A screen', phone: screen.svg }
colors:
  - { name: 'Long', value: '#1d6670' }
  - { name: 'Short', value: '#abc' }
  - { name: 'Clear', value: '#1d667080' }
type:
  - { name: 'Title', size: '24px', weight: 700 }
  - { name: 'Small', size: '12.5px' }
`)
	s.floe(t, "publish", data, "--slug", "design-samples")
	b.navigate("/a/design-samples/?raw")
	got := b.eval(`(() => [...document.querySelectorAll('.tile')].map((e) => getComputedStyle(e).backgroundColor)
  .concat([...document.querySelectorAll('.sample')].map((e) => getComputedStyle(e).fontSize + ' ' + getComputedStyle(e).fontWeight))
  .join(', '))()`)
	if want := "rgb(29, 102, 112), rgb(170, 187, 204), rgba(29, 102, 112, 0.5), 24px 700, 12.5px 400"; got != want {
		t.Errorf("design's samples: %s, want %s, the values written", got, want)
	}
}
