package tmpl_test

import (
	"strings"
	"testing"
)

// tech-spec draws decided under the options weighed, and in a section of its own, in the contents too, on a page that
// weighs none: decided is not lost with decisions left out.
func TestTechSpecDecidedAlone(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	base := "lang: en\ntemplate: tech-spec\ntitle: t\nbackground: b\ngoals: [g]\nsolution: s\ndecided: ['Kept as **dead**']\n"
	alone := drawPage(t, k, base, none)
	if !strings.Contains(alone, `<h2 id="decided">`) || !strings.Contains(alone, `href="#decided"`) ||
		!strings.Contains(alone, "<strong>dead</strong>") {
		t.Errorf("decided without decisions is not drawn in a section of its own, in the contents:\n%s", alone)
	}
	weighed := drawPage(t, k, base+"decisions:\n  - { question: q, options: [{ title: a, pick: Taken }] }\n", none)
	if strings.Contains(weighed, `<h2 id="decided">`) || !strings.Contains(weighed, "<strong>dead</strong>") {
		t.Errorf("decided with decisions is not drawn under them:\n%s", weighed)
	}
}
