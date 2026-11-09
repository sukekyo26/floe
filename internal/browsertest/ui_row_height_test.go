package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// TestUIRowHeight holds a page's row to one rule on the page list and a task's page: of one height on both, its buttons 26px, the
// items of its menu taller and wrapping within themselves, and the ring of a button reached by the keys clear of the title above it and
// inside the row's foot.
func TestUIRowHeight(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	s.floe(t, "publish", s.page(t, "plan.yaml", "Plan", "<p>x</p>"), "--slug", "plan")
	if _, err := st.Edit("plan", store.Edit{Project: ptrTo("demo")}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "plan", "--task", "S1", "--step", "design")

	// The row of plan: its height; its buttons' heights; the items of its menu, open, each with its height and whether its words overflow
	// it; and the room the ring of its archive button, reached, leaves to the title above and to the row's foot below (3px: --ring past
	// its 1px offset)
	const row = "document.querySelector('#list li[data-slug=plan]')"
	const measure = `(() => { const li = ` + row + `, r = (e) => e.getBoundingClientRect();
		const buttons = [...li.querySelectorAll(':scope > .actions > .icon-btn, :scope > .actions > .more > summary')].map((e) => r(e).height);
		const more = li.querySelector('.more'); more.open = true;
		const items = [...li.querySelectorAll('.menu > *')].filter((e) => e.getClientRects().length);
		const tall = items.every((e) => r(e).height >= 36), within = items.every((e) => e.scrollHeight <= e.clientHeight);
		more.open = false;
		const btn = li.querySelector('.actions > .archive'); btn.focus();
		const ring = btn.matches(':focus-visible') && r(btn).top - 3 >= r(li.querySelector('.title')).bottom && r(btn).bottom + 3 <= r(li).bottom - 1;
		return [r(li).height.toFixed(1), buttons.join(','), items.length > 0 && tall, within, ring].join(' '); })()`
	var out []string
	for _, width := range []int64{1440, 390} {
		b.size(width, 900, width < 600)
		got := map[string]string{}
		for _, p := range [][2]string{{"list", "/?project=demo"}, {"task", "/tasks?project=demo&task=S1"}} {
			b.navigate(p[1])
			b.until("the row", row)
			b.still()
			got[p[0]] = b.eval(measure)
		}
		b.unsize()
		height, rest, _ := strings.Cut(got["list"], " ")
		taskHeight, _, _ := strings.Cut(got["task"], " ")
		out = append(out, "same height: "+strconv.FormatBool(height == taskHeight), rest, strings.SplitN(got["task"], " ", 2)[1])
	}
	check(t, "row height", out, strings.Join([]string{
		"same height: true", "26,26,26 true true true", "26,26,26 true true true",
		"same height: true", "26,26,26 true true true", "26,26,26 true true true",
	}, "|"))
}
