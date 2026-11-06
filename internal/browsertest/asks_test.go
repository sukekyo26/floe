package browsertest_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// TestUIAsksHead reads the count of questions at the head of a handoff: of those still to decide, in the warning's colour
// only while there is one, with the link to the section as long as the section stands.
func TestUIAsksHead(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	for _, c := range []struct{ asks, want string }{
		{"asks:\n  - { question: a, decided: x }\n  - { question: b, options: [x, y], decided: y }\n", "0 link"},
		{"asks:\n  - { question: a, decided: x }\n  - { question: b }\n", "1 link hot"},
		{"", "0"},
	} {
		page := "template: handoff\nlang: en\ntitle: Head\ndate: 2030-01-01\n" + c.asks
		s.floe(t, "publish", s.write(t, "head.yaml", page), "--slug", "head", "--force")
		s.b.navigate("/a/head/?raw")
		got := s.b.eval(`(() => {
  const li = document.querySelectorAll('.counts li')[2];
  return li.querySelector('b').textContent + (li.querySelector('a[href="#asks"]') ? ' link' : '') + (li.matches('.hot[data-floe-tone=warn]') ? ' hot' : '');
})()`)
		if got != c.want {
			t.Errorf("asks %q: %q; want %q", c.asks, got, c.want)
		}
	}
}

// TestUIAsksPills measures the pills beside an option's words, in every theme and as wide as a phone: their middles level, but for a pill folded under the words.
func TestUIAsksPills(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	const asks = "template: report\nlang: en\ntitle: Pills\nsections:\n  - heading: Pills\n    parts:\n      - asks:\n" +
		"          - { question: q, options: [a, 'b `c`'], recommend: 'b `c`', decided: 'b `c`' }\n"
	defer s.b.unsize()
	for _, theme := range themes {
		s.floe(t, "publish", s.write(t, "pills.yaml", asks), "--slug", "pills-"+theme, "--theme", theme)
		for _, w := range []int64{1000, 390} {
			s.b.size(w, 800, false)
			s.b.navigate("/a/pills-" + theme + "/?raw")
			got := s.b.eval(`(() => {
  const middle = (e) => { const r = e.getBoundingClientRect(); return r.top + r.height / 2; };
  const pills = [...document.querySelectorAll('[data-floe-part=asks] .choices [data-floe-part=cell]')];
  if (pills.length !== 2) return pills.length + ' pills';
  const beside = pills.filter((p) => p.getBoundingClientRect().top < p.parentElement.firstElementChild.getBoundingClientRect().bottom);
  return beside.map((p) => middle(p) - middle(p.parentElement.firstElementChild)).filter((d) => Math.abs(d) > 0.75).join(' ');
})()`)
			if got != "" {
				t.Errorf("%s at %d px: a pill's middle stands off the words' by %s", theme, w, got)
			}
		}
	}
}
