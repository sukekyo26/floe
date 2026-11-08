package browsertest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// measureRowsLook returns a line for each row (rowsSel) not in the page's line height, and for each box of a part or
// element drawn as rows not padded as that of an element of html's rows on the page, when there is one.
const measureRowsLook = `(() => {
  ` + rowOwnerJS + `
  const ratio = (e) => { const c = getComputedStyle(e); return parseFloat(c.lineHeight) / parseFloat(c.fontSize); };
  const page = ratio(document.body), out = new Set();
  for (const e of document.querySelectorAll(` + rowsSel + `)) {
    if (Math.abs(ratio(e) - page) > 0.005) out.add(owner(e) + ': line height ' + ratio(e).toFixed(2) + ', the page\'s ' + page.toFixed(2));
  }
  const html = document.querySelector('div[data-floe-look="rows"]'), box = (e) => getComputedStyle(e).padding;
  if (html) {
    for (const e of document.querySelectorAll('[data-floe-look~="rows"], :is([data-floe-part="entries"], [data-floe-part="keys"], [data-floe-part="checklist"], [data-floe-part="links"])')) {
      if (box(e) !== box(html)) out.add((e.dataset.floePart || 'look rows') + ': its box padded ' + box(e) + ', html\'s rows ' + box(html));
    }
  }
  return [...out].join('\n');
})()`

// TestRenderedRows draws rowParts in every theme, and design's and postmortem's examples (their pages' line heights are
// not report's): every row is in the page's line height, and the box of every part drawn as rows is padded as an
// element of html's rows is.
func TestRenderedRows(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	file := s.write(t, "rows.yaml", rowPadPage)
	check := func(what string) {
		if got := s.b.eval(measureRowsLook); got != "" {
			t.Errorf("%s:\n  %s", what, strings.ReplaceAll(got, "\n", "\n  "))
		}
	}
	for _, theme := range themes {
		s.floe(t, "publish", file, "--slug", "rows-"+theme, "--theme", theme)
		s.b.navigate("/a/rows-" + theme + "/?raw")
		check("rowParts in " + theme)
	}
	for _, sample := range []string{"design", "postmortem"} {
		data, err := os.ReadFile("../corpus/testdata/example/" + sample + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		s.floe(t, "publish", s.write(t, sample+".yaml", string(data)+rowsSection), "--slug", "rows-"+sample)
		s.b.navigate("/a/rows-" + sample + "/?raw")
		check(sample)
	}
}
