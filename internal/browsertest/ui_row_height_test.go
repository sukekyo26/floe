package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"floe/internal/store"
)

// TestUIRowHeight holds a page's row to one rule on the page list and a task's page: of one height on both, its buttons 26px, the
// items of its menu taller and wrapping within themselves, and the ring of a button reached by the keys clear of the title above it and
// inside the row's foot.
func TestUIRowHeight(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	s.floe(t, "publish", s.page(t, "plan.yaml", "Plan", "<p>x</p>"), "--slug", "plan")
	if _, err := st.Edit("plan", store.Edit{Project: new("demo")}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "plan", "--task", "S1", "--category", "design")

	// The row of plan: its height; its buttons' heights; the items of its menu, open and narrowed so that some wrap, each at least 36px
	// and none with its words overflowing it; and the room the ring of its archive button, reached, leaves to the title above and to the
	// row's foot below (its outline's width past its offset)
	const row = "document.querySelector('#list li[data-slug=plan]')"
	const measure = `(() => { const li = ` + row + `, r = (e) => e.getBoundingClientRect();
		const buttons = [...li.querySelectorAll(':scope > [data-testid=actions] > button, :scope > [data-testid=actions] > [data-testid=more] > summary')].map((e) => r(e).height);
		const more = li.querySelector('[data-testid=more]');
		// Narrowed, so that its items' words wrap to two lines
		const menu = more.querySelector('[data-testid=menu]'); menu.style.minWidth = '0'; menu.style.width = '8rem';
		const items = [...li.querySelectorAll('[data-testid=menu] :is(button, a)')].filter((e) => e.getClientRects().length);
		const tall = items.every((e) => r(e).height >= 36), within = items.every((e) => e.scrollHeight <= e.clientHeight);
		const wrapped = items.some((e) => r(e).height > 40);
		menu.style.minWidth = ''; menu.style.width = ''; more.querySelector('summary').click();
		const btn = li.querySelector('[data-testid=actions] > [data-testid=archive]'); btn.focus();
		const st = getComputedStyle(btn), out = parseFloat(st.outlineWidth) + parseFloat(st.outlineOffset);
		const ring = btn.matches(':focus-visible') && r(btn).top - out >= r(li.querySelector('[data-testid=title]')).bottom && r(btn).bottom + out <= r(li).bottom - 1;
		return [r(li).height.toFixed(1), buttons.join(','), items.length > 0 && tall, wrapped, within, ring].join(' '); })()`
	out := make([]string, 0, 6)
	for _, width := range []int64{1440, 390} {
		b.size(width, 900, width < 600)
		got := map[string]string{}
		for _, p := range [][2]string{{"list", "/?project=demo"}, {"task", "/tasks?project=demo&task=S1"}} {
			b.navigate(p[1])
			b.until("the row", row)
			b.still()
			// The menu opened first: React draws it open after the press
			menuItemShown(b, "#list li[data-slug=plan]", "edit", "")
			got[p[0]] = b.eval(measure)
		}
		b.unsize()
		height, rest, _ := strings.Cut(got["list"], " ")
		taskHeight, _, _ := strings.Cut(got["task"], " ")
		out = append(out, "same height: "+strconv.FormatBool(height == taskHeight), rest, strings.SplitN(got["task"], " ", 2)[1])
	}
	check(t, "row height", out, strings.Join([]string{
		"same height: true", "26,26,26 true true true true", "26,26,26 true true true true",
		"same height: true", "26,26,26 true true true true", "26,26,26 true true true true",
	}, "|"))
}
