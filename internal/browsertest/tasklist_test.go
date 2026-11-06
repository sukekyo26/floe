package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// taskWork is tasks.json of TestUITaskList: each state's order by its times, tasks of no page, one project of no page, and
// the project of none. #6 is done with archived pages alone, #3 waiting with them, and #8 done with one page left.
const taskWork = `{
  "shop": {
    "#1": {"title": "One", "state": "waiting", "step": "design", "at": "2026-01-02T00:00:00Z", "url": "https://example.com/a/b/c"},
    "#2": {"title": "Two", "state": "waiting", "at": "2026-01-01T00:00:00Z"},
    "#4": {"state": "doing", "at": "2026-01-01T00:00:00Z"},
    "#5": {"state": "doing", "at": "2026-01-02T00:00:00Z"},
    "#6": {"state": "done", "at": "2026-01-01T00:00:00Z"},
    "#7": {"state": "done", "at": "2026-01-02T00:00:00Z"},
    "#8": {"state": "done", "at": "2026-01-03T00:00:00Z"}
  },
  "blog": {"#3": {"state": "waiting", "at": "2026-01-03T00:00:00Z"}},
  "solo": {"#9": {"state": "waiting", "at": "2026-01-04T00:00:00Z"}},
  "/": {"#10": {"state": "waiting", "at": "2026-01-05T00:00:00Z"}}
}
`

// taskRows says the rows of the list, each its title and the words under it.
const taskRows = `[...document.querySelectorAll('#list > li')].map((li) => li.querySelector('.title').textContent + ' [' +
  [...li.querySelector('.sub').children].map((e) => e.textContent).join(', ') + ']').join('; ')`

// taskPages publishes the pages of taskWork, each by its project, task, step and whether archived.
func taskPages(t *testing.T, s *site) {
	t.Helper()

	st := store.New(s.Env["FLOE_DIR"])
	for _, p := range []struct {
		slug, project, task, step string
		archived, pin             bool
	}{
		{"p1", "shop", "#1", "design", false, false},
		{"p2", "shop", "#1", "", false, false},
		{"p3", "blog", "#3", "", true, false},
		{"p6", "shop", "#6", "", true, false},
		{"p8", "shop", "#8", "", false, false},
		{"p8a", "shop", "#8", "", true, false},
		{"p10", "", "#10", "", false, false},
		{"p11", "shop", "#11", "", false, false},
		{"pinned", "shop", "", "", false, true},
		{"gonepin", "shop", "", "", true, true},
	} {
		s.floe(t, "publish", s.page(t, p.slug+".yaml", p.slug, ""), "--slug", p.slug)
		if _, err := st.Edit(p.slug, store.Edit{
			Project: &p.project, Archived: &p.archived,
			Work: &store.WorkEdit{Task: &p.task, Step: &p.step, Pin: &p.pin},
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// The task list: nothing of it where nothing is on a task; j and k move the focus alone, reading nothing; each state in
// its own order; what is done and archived whole goes to the archive, nothing else; projects as the page list picks them.
func TestUITaskList(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	file := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
	writeWork := func(text string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil { //nolint:gosec // G703: a path the test names
			t.Fatal(err)
		}
	}
	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	const screens = "document.getElementById('screens').hidden"
	const rowsDrawn = "document.querySelector('#list > li')"

	// Of no task: the page list as it was, and on /tasks the words how, with no way between the two.
	b.navigate("/")
	b.until("the list read", "document.getElementById('empty').hidden === false")
	say("no work, list: switch hidden", screens)
	b.navigate("/tasks")
	b.until("the guide", "!document.getElementById('tasks-empty').hidden")
	say("no work, tasks: switch hidden", screens)
	say("no work, tasks: shelf shown", "document.getElementById('shelf').offsetParent !== null")

	// A task's row alone is a way there: a task waiting must not be out of reach.
	s.floe(t, "work", "--task", "#99", "--project", "solo")
	b.navigate("/")
	b.until("the switch", "!"+screens)
	say("a row alone, list: current", "document.getElementById('to-list').getAttribute('aria-current')")
	s.floe(t, "work", "--task", "#99", "--project", "solo", "--forget")

	taskPages(t, s)
	writeWork(taskWork)

	// Waiting, the longest in it first, of every project, the project named on each; of no project, none.
	b.navigate("/tasks")
	b.until("the rows", rowsDrawn)
	say("waiting", taskRows)
	say("views", "[...document.querySelectorAll('.views button[data-view]')].map((v) => v.dataset.view + ' ' + v.querySelector('.n').textContent).join(', ')")
	say("head", "document.getElementById('count').textContent")
	say("pins", "[...document.querySelectorAll('.pin')].map((p) => p.textContent + ' ' + p.getAttribute('href')).join(', ')")
	say("hrefs", "[...document.querySelectorAll('#list > li .title')].map((a) => a.getAttribute('href')).join(' ')")
	say("link", "(() => { const a = document.querySelector('.xlink'); return [a.textContent, a.href, a.target, a.rel, a.title].join(' '); })()")
	say("current", "document.getElementById('to-tasks').getAttribute('aria-current')")
	say("search shown", "document.getElementById('search').hidden + ' ' + (document.getElementById('find-open').offsetParent !== null)")

	// j and k: the focus alone, nothing beside the list and nothing read.
	news := b.eval("document.getElementById('news-n').textContent")
	b.pressKey("j")
	say("j", "document.activeElement.textContent")
	b.pressKey("j")
	say("j j", "document.activeElement.textContent")
	b.pressKey("k")
	say("j j k", "document.activeElement.textContent")
	say("nothing picked", "document.querySelector('#list li.picked') === null && document.getElementById('read-frame').hidden")
	if got := b.eval("document.getElementById('news-n').textContent"); got != news {
		t.Errorf("unread went from %s to %s", news, got)
	}

	// The other states in their order, the URL and what the tab keeps of them, and the way back to each screen.
	for _, view := range []string{"doing", "done", "archived"} {
		b.eval("document.querySelector('.views [data-view=" + view + "]').click()")
		say(view, taskRows)
		say(view+" head", "document.getElementById('count').textContent")
		say(view+" url", "location.search + ' ' + sessionStorage.getItem('artifacts.tasks')")
	}
	say("the list's own key untouched", "sessionStorage.getItem('artifacts.list')")
	say("switch hrefs", "document.getElementById('to-list').getAttribute('href') + ' ' + document.getElementById('to-tasks').getAttribute('href')")
	b.eval("document.querySelector('.views [data-view=waiting]').click()")
	say("the first view is left out", "location.search")

	// The projects. One of no page shows here and not on the page list, which keeps its colours.
	say("chips", "[...document.querySelectorAll('#chips .chip')].map((c) => c.textContent + ' ' + c.style.getPropertyValue('--c')).join(', ')")
	for _, query := range []string{"?project=shop", "?project=%2F", "?project=old&view=doing", "?project=shop&view=archived"} {
		b.navigate("/tasks" + query)
		b.until("the rows of "+query, rowsDrawn+" || !document.getElementById('tasks-none').hidden")
		say(query, taskRows)
		say(query+" none", "document.getElementById('tasks-none').hidden ? '' : document.querySelector('#tasks-none strong').textContent + ' / ' + !document.getElementById('tasks-all').hidden")
	}
	b.eval("document.getElementById('tasks-all').click()")
	say("all projects again", "location.search + ' ' + document.querySelectorAll('#list > li').length")
	b.navigate("/?project=shop")
	b.until("the page list", rowsDrawn)
	say("page list chips", "[...document.querySelectorAll('#chips .chip')].map((c) => c.textContent + ' ' + c.style.getPropertyValue('--c')).join(', ')")
	say("page list, its query kept", "sessionStorage.getItem('artifacts.list') + ' ' + document.getElementById('to-list').getAttribute('href')")

	// On the task list the bar's unread leads to the page list, narrowing nothing here.
	b.navigate("/tasks")
	b.until("the bar's unread", "!document.getElementById('news').hidden")
	b.eval("document.getElementById('news').click()")
	b.until("the page list of the new", "location.pathname === '/' && location.search === '?view=new'")

	// No task is out of reach, nor a row of nothing but pins made up: what cannot be read says so on the task list alone,
	// and goes when mended.
	writeWork("{ broken")
	b.navigate("/")
	b.until("the page list", rowsDrawn)
	b.until("a look at the tasks", "document.getElementById('list').children.length > 0 && !document.getElementById('screens').hidden")
	say("broken, list: error hidden", "document.getElementById('error').hidden")
	b.navigate("/tasks")
	b.until("the error", "!document.getElementById('error').hidden")
	say("broken, tasks: failed shown", "!document.getElementById('tasks-failed').hidden")
	say("broken, tasks", "[...document.querySelectorAll('.views button[data-view]')].map((v) => v.dataset.view).join(',') + ' pins ' + document.querySelectorAll('.pin').length + ' rows ' + document.querySelectorAll('#list > li').length")
	say("broken, tasks: says why", "document.getElementById('error').textContent.includes('mend it by hand')")
	writeWork(taskWork)
	b.until("mended", "document.getElementById('error').hidden && "+rowsDrawn)
	say("mended: failed hidden", "document.getElementById('tasks-failed').hidden")

	if got, want := strings.Join(out, "\n"), taskListWant; got != want {
		t.Errorf("the task list:\n%s\nwant:\n%s", got, want)
	}

	// The three widths: the shelf beside the rows, over them, and over them with nothing past the screen's edge.
	for _, c := range []struct {
		width  int64
		narrow bool
	}{{1440, false}, {900, true}, {400, true}, {360, true}} {
		b.size(c.width, 800, false)
		b.navigate("/tasks")
		b.until("the rows", rowsDrawn)
		shelf, col := b.rect("document.getElementById('shelf')"), b.rect("document.getElementById('listcol')")
		beside := col.X >= shelf.X+shelf.Width-1
		filter := b.eval("document.getElementById('narrow-filter').offsetParent !== null") == "true"
		// The bar's own too, whose last thing goes off the screen when the logo's word stays
		over := b.eval("document.documentElement.scrollWidth > innerWidth || document.getElementById('server').getBoundingClientRect().right > innerWidth") == "true"
		reader := b.eval("document.getElementById('reader').offsetParent !== null") == "true"
		if beside == c.narrow || filter != c.narrow || over || reader {
			t.Errorf("width %d: beside %t, filter %t, past the edge %t, reader %t", c.width, beside, filter, over, reader)
		}
		if got := fmt.Sprint(int64(col.X + col.Width)); got != fmt.Sprint(c.width) {
			t.Errorf("width %d: the rows end at %s", c.width, got)
		}
	}
	b.unsize()
}

// taskListWant is what TestUITaskList reads of the list, in the order it reads it.
const taskListWant = `no work, list: switch hidden: true
no work, tasks: switch hidden: true
no work, tasks: shelf shown: false
a row alone, list: current: page
waiting: #2Two [0 page(s), shop]; #1One [design, 2 page(s), 2 unread, shop, b/c]; #3 [0 page(s), 1 archived, blog]; #9 [0 page(s), solo]; #10 [1 page(s), 1 unread]
views: waiting 5, doing 3, done 2, archived 1
head: 5 · longest in Waiting first
pins: pinned /a/pinned/
hrefs: /tasks?project=shop&task=%232 /tasks?project=shop&task=%231 /tasks?project=blog&task=%233 /tasks?project=solo&task=%239 /tasks?project=%2F&task=%2310
link: b/c https://example.com/a/b/c _blank noopener https://example.com/a/b/c
current: page
search shown: true true
j: #2Two
j j: #1One
j j k: #2Two
nothing picked: true
doing: #11 [1 page(s), 1 unread, shop]; #5 [0 page(s), shop]; #4 [0 page(s), shop]
doing head: 3 · latest change first
doing url: ?view=doing ?view=doing
done: #8 [1 page(s), 1 archived, 1 unread, shop]; #7 [0 page(s), shop]
done head: 2 · latest change first
done url: ?view=done ?view=done
archived: #6 [0 page(s), 1 archived, shop]
archived head: 1 · latest change first
archived url: ?view=archived ?view=archived
the list's own key untouched: 
switch hrefs: / /tasks?view=archived
the first view is left out: 
chips: shop2 var(--pc0), blog1 var(--pc1), solo1 var(--pc2), Not set1 var(--faint)
?project=shop: #2Two [0 page(s)]; #1One [design, 2 page(s), 2 unread, b/c]
?project=shop none: 
?project=%2F: #10 [1 page(s), 1 unread]
?project=%2F none: 
?project=old&view=doing: 
?project=old&view=doing none: No tasks in old / true
?project=shop&view=archived: #6 [0 page(s), 1 archived]
?project=shop&view=archived none: 
all projects again: ?view=archived 1
page list chips: shop5 var(--pc0), Not set1 var(--faint)
page list, its query kept: ?project=shop /?project=shop
broken, list: error hidden: true
broken, tasks: failed shown: true
broken, tasks: waiting,doing,done,archived pins 1 rows 0
broken, tasks: says why: true
mended: failed hidden: true`
