package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// The × on a pin of the task list takes the pin off where it is, by a real press, and leaves the focus on the next pin's ×, the last's, and
// the help once none is left; the page stays on its task. At every width a long title gives way and the × stays whole, inside its pin.
func TestUIPinOff(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	long := strings.Repeat("A long title of a pinned page ", 6)
	for _, page := range [][2]string{{"p1", "T p1"}, {"p2", long}, {"p3", "T p3"}} {
		slug, title := page[0], page[1]
		s.floe(t, "publish", s.page(t, slug+".yaml", title, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "work", "p1", "--task", "#1")
	s.floe(t, "work", "p1", "p2", "p3", "--pin")
	defer b.unsize()
	b.size(1440, 900, false)
	b.navigate("/tasks?project=proj")
	b.until("the pins", "document.querySelectorAll('.pin-off').length === 3")

	const chip = "document.querySelector('.pin-chip[data-slug=p2]')"
	for _, width := range []int64{1440, 1100, 900, 800, 360} {
		b.size(width, 900, width == 360)
		b.until("the width", fmt.Sprintf("innerWidth === %d", width))
		whole := b.eval(`(() => {
			const chip = ` + chip + `.getBoundingClientRect();
			const off = ` + chip + `.querySelector('.pin-off').getBoundingClientRect();
			const words = ` + chip + `.querySelector('.pin-title');
			return [off.width === 22, off.height === 22, off.right <= chip.right, off.left >= words.getBoundingClientRect().right, off.top >= chip.top, off.bottom <= chip.bottom,
				words.scrollWidth > words.clientWidth, document.documentElement.scrollWidth <= innerWidth].join(' ');
		})()`)
		if whole != "true true true true true true true true" {
			t.Errorf("width %d: the pin is out of shape: %s", width, whole)
		}
	}
	b.size(1440, 900, false)
	b.until("the width", "innerWidth === 1440")

	var out []string
	press := func(slug string) {
		t.Helper()
		r := b.rect("document.querySelector('.pin-chip[data-slug=" + slug + "] .pin-off')")
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
		b.until("the pin off", "!document.querySelector('.pin-chip[data-slug="+slug+"]') && document.activeElement !== document.body")
		out = append(out, b.eval(`[location.pathname, [...document.querySelectorAll('.pin-chip')].map((c) => c.dataset.slug).join(), document.activeElement.getAttribute('aria-label'), [...document.querySelectorAll('.toast')].at(-1)?.textContent ?? ''].join(' | ')`))
	}
	press("p1")
	press("p3")
	press("p2")
	out = append(out, "the page's task: "+b.eval("items.find((a) => a.slug === 'p1').work.task"))
	if got, want := strings.Join(out, "\n"), `/tasks | p3,p2 | Take the pin off `+strings.TrimSpace(long)+` | Took the pin off “T p1”
/tasks | p2 | Take the pin off `+strings.TrimSpace(long)+` | Took the pin off “T p3”
/tasks |  | About pinned pages | Took the pin off “`+strings.TrimSpace(long)+`”
the page's task: #1`; got != want {
		t.Errorf("taking the pins off:\n%s\nwant:\n%s", got, want)
	}
}
