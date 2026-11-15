package browsertest_test

import (
	"strings"
	"testing"
)

// outsideOwn is the head of a page of one's own: its palette in a layer of its own, its color-scheme of a meta, and
// .probe drawn by the variables it reads with a fallback.
const outsideOwn = `<meta name="color-scheme" content="light dark"><style>
@layer host { :root { --bg: rgb(10, 20, 30); --accent: rgb(200, 0, 100); } }
.probe { color: var(--accent, rgb(1, 1, 1)); background: var(--bg); width: var(--text-width, 33px);
  border-top: var(--rule, 3px dotted rgb(2, 2, 2)); box-shadow: var(--shadow, 0 0 0 1px rgb(3, 3, 3)); }
</style>`

// outsideApart opens a page of outsideOwn holding .probe and fragment after it, and returns {seen, amiss}: what the
// page draws of .probe (outside) against without the fragment, amiss at .probe.
func outsideApart(t *testing.T, s *site, page, fragment string) measured {
	t.Helper()

	openOwn(t, s, "alone.html", outsideOwn, `<p class="probe">p</p>`)
	alone := s.b.eval(outside)
	openOwn(t, s, "around.html", outsideOwn, `<p class="probe">p</p>`+fragment)
	s.b.until("the toolbar", "document.querySelector('.floe-zoom-tools')")
	m := measured{seen: 1}
	if got := s.b.eval(outside); got != alone {
		m.amiss = append(m.amiss, amiss{Page: page, At: ".probe", Text: got + ", without the fragment " + alone})
	}
	return m
}

const lookFragmentOutsideFix = "let a diagram's CSS reach its own root alone, under the diagram's root, not the page around it"

// lookFragmentOutside pastes the fragment of floe diagram render of every diagram (fragments) into a page of one's own
// (outsideApart): the page outside the diagrams keeps what it draws.
func lookFragmentOutside(t *testing.T, s *site) measured {
	t.Helper()

	fragment, _ := fragments(t, s)
	return outsideApart(t, s, "a page of one's own", fragment)
}

// brokenFragmentOutside pastes the fragment of a diagram whose style ends in a rule for the page's .probe outside the
// diagram's root, as specific as to win over the page's.
func brokenFragmentOutside(t *testing.T, s *site) (string, measured) {
	t.Helper()

	fragment := s.floe(t, "diagram", "render", s.write(t, "tones.yaml", tonesSpec))
	if !strings.Contains(fragment, "</style>") {
		t.Fatal("the fragment holds no style to break")
	}
	return ".probe", outsideApart(t, s, "broken", strings.Replace(fragment, "</style>", "p.probe { color: rgb(9, 9, 9); }</style>", 1))
}
