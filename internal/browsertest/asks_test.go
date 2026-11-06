package browsertest_test

import "testing"

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
