package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// taskWork is tasks.json of TestUITaskList: each state's order by its times, tasks of no page, one project of no page, and
// the project of none. #6 is archived, #3 waiting with archived pages alone, and #8 done with one page left.
const taskWork = `{
  "shop": {
    "#1": {"title": "One", "state": "waiting", "step": "design", "at": "2026-01-02T00:00:00Z", "url": "https://example.com/a/b/c"},
    "#2": {"title": "Two", "state": "waiting", "at": "2026-01-01T00:00:00Z"},
    "#4": {"state": "doing", "at": "2026-01-01T00:00:00Z"},
    "#5": {"state": "doing", "at": "2026-01-02T00:00:00Z"},
    "#6": {"state": "archived", "at": "2026-01-01T00:00:00Z"},
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

// taskFilterName says the button of the narrow screen: its name, that at least 2em of it shows, its count and whether it is lit.
const taskFilterName = `(() => { const name = document.getElementById('narrow-filter-name');
  return [name.textContent, name.getBoundingClientRect().width >= 2 * parseFloat(getComputedStyle(name).fontSize) - 0.5, document.getElementById('narrow-filter-n').textContent, document.getElementById('narrow-filter').classList.contains('narrowed')].join(' '); })()`

// mustInt reads a number the page said.
func mustInt(t *testing.T, s string) int {
	t.Helper()

	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

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
		{"pinonly", "pin", "", "", false, true},
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
// its own order; what is archived goes to the archive, nothing else; projects as the page list picks them.
func TestUITaskList(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	file := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
	writeTasks := func(text string) {
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
	const rowsDrawn = "document.querySelector('#list > li')"

	// Of no task: the way between the two is there all the same, and on /tasks the words how.
	b.navigate("/")
	b.until("the list read", "document.getElementById('empty').hidden === false")
	b.navigate("/tasks")
	b.until("the guide", "!document.getElementById('tasks-empty').hidden")
	say("no work, tasks: shelf shown", "document.getElementById('shelf').offsetParent !== null")

	taskPages(t, s)
	writeTasks(taskWork)

	// Opened with no project: the one where a task has waited longest, written in the URL with no entry of its own, and kept.
	b.navigate("/")
	b.until("the page list", rowsDrawn)
	before := b.eval("history.length")
	b.navigate("/tasks")
	b.until("the rows", rowsDrawn)
	say("opened", "location.search + ' ' + localStorage.getItem('artifacts.taskProject')")
	if got := b.eval("history.length"); got != fmt.Sprint(mustInt(t, before)+1) {
		t.Errorf("history went from %s to %s", before, got)
	}
	say("waiting", taskRows)
	say("views", "[...document.querySelectorAll('.views button[data-view]')].map((v) => v.dataset.view + ' ' + v.querySelector('.n').textContent).join(', ')")
	say("head", "document.getElementById('count').textContent")
	say("pins", "[...document.querySelectorAll('.pin')].map((p) => p.textContent + ' ' + p.getAttribute('href')).join(', ')")
	say("hrefs", "[...document.querySelectorAll('#list > li .title')].map((a) => a.getAttribute('href')).join(' ')")
	say("a row's project, in its title alone", "document.querySelector('#list > li .title').title.replaceAll('\\n', ' / ')")
	say("link", "(() => { const a = document.querySelector('.xlink'); return [a.textContent, a.href, a.target, a.rel, a.title].join(' '); })()")
	say("current", "document.getElementById('to-tasks').getAttribute('aria-current')")
	say("search shown", "document.getElementById('search').hidden + ' ' + (document.getElementById('find-open').offsetParent !== null)")
	say("chips", "[...document.querySelectorAll('#chips .chip')].map((c) => c.textContent + ' ' + c.getAttribute('aria-pressed') + ' ' + c.style.getPropertyValue('--c')).join(', ')")

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
		say(view+" chips", "[...document.querySelectorAll('#chips .n')].map((n) => n.textContent).join(' ')")
	}
	say("the list's own key untouched", "sessionStorage.getItem('artifacts.list')")
	say("switch hrefs", "document.getElementById('to-list').getAttribute('href') + ' ' + document.getElementById('to-tasks').getAttribute('href')")
	b.eval("document.querySelector('.views [data-view=waiting]').click()")
	say("the first view is left out", "location.search")

	// A project pressed: to it, the view kept; again, nothing; and it is what opens next.
	b.eval("document.querySelector('.chip[data-project=blog]').click()")
	say("to blog", "location.search + ' ' + localStorage.getItem('artifacts.taskProject')")
	b.eval("document.querySelector('.chip[data-project=blog]').click()")
	say("blog again", "location.search + ' ' + document.querySelector('.chip[data-project=blog]').getAttribute('aria-pressed')")
	b.navigate("/tasks?view=doing")
	b.until("kept", "location.search.includes('project=')")
	say("the one kept, the view kept", "location.search")

	// Each kind of project, and one gone, whose URL changes nothing kept; the way to another.
	for _, query := range []string{"?project=shop", "?project=%2F", "?project=pin", "?project=shop&view=archived", "?project=old&view=doing"} {
		b.navigate("/tasks" + query)
		b.until("the rows of "+query, rowsDrawn+" || !document.getElementById('tasks-none').hidden")
		say(query, taskRows)
		say(query+" none", "document.getElementById('tasks-none').hidden ? '' : document.querySelector('#tasks-none strong').textContent + ' / ' + !document.getElementById('tasks-other').hidden")
		say(query+" pins", "[...document.querySelectorAll('.pin')].map((p) => p.textContent).join(', ') + ' | ' + document.querySelector('.pins-none')?.textContent")
		say(query+" kept", "localStorage.getItem('artifacts.taskProject')")
	}
	say("gone, on the shelf", "document.querySelector('.chip[aria-pressed=true]').textContent")
	b.moving("localStorage.setItem('artifacts.taskProject', 'blog'); document.getElementById('tasks-other').click()")
	b.until("the project kept", "location.pathname === '/tasks' && location.search === '?project=blog'")

	// What another window chose is not taken back at the next look.
	b.navigate("/tasks?project=shop")
	b.until("the rows", rowsDrawn)
	b.eval("localStorage.setItem('artifacts.taskProject', 'blog'); drawn = ''; load()")
	b.until("the next look", "document.querySelector('#list > li')")
	b.still()
	say("another window's", "localStorage.getItem('artifacts.taskProject')")
	b.eval("document.querySelector('.chip[data-project=shop]').click()")
	say("pressed, it is kept", "localStorage.getItem('artifacts.taskProject')")

	// The shelf folded: the head says the project.
	b.eval("localStorage.setItem('list.columns', JSON.stringify({ open: false }))")
	b.navigate("/tasks?project=shop")
	b.until("the rows", rowsDrawn)
	say("folded", "document.getElementById('shelf-what').hidden + ' ' + document.getElementById('shelf-what').textContent")
	b.eval("localStorage.removeItem('list.columns')")

	// One kept that is gone is passed over; with no task waiting, the shelf's first.
	b.eval("localStorage.setItem('artifacts.taskProject', 'gone')")
	b.navigate("/tasks")
	b.until("a project", "location.search.includes('project=')")
	say("the kept gone", "location.search")
	b.eval("localStorage.removeItem('artifacts.taskProject')")
	writeTasks(strings.ReplaceAll(taskWork, `"waiting"`, `"doing"`))
	b.navigate("/tasks")
	b.until("a project", "location.search.includes('project=')")
	say("none waiting", "location.search")
	// With the longest wait in another project than the shelf's first, that one.
	b.eval("localStorage.removeItem('artifacts.taskProject')")
	writeTasks(strings.ReplaceAll(taskWork, "2026-01-03T00:00:00Z\"}},", "2025-01-03T00:00:00Z\"}},"))
	b.navigate("/tasks")
	b.until("a project", "location.search.includes('project=')")
	say("waited longest", "location.search")
	writeTasks(taskWork)

	// The way up of a task's page opened by its URL: its project's list, at the task's view, whatever list was last seen.
	const up = "document.getElementById('task-up').getAttribute('href')"
	for _, c := range []struct{ list, task string }{
		{"?view=doing&project=shop", "?project=shop&task=%231"},
		{"?project=blog", "?project=shop&task=%231"},
		{"?project=blog", "?project=shop&task=%235"},
		{"?project=blog", "?project=shop&task=%236"},
		{"?project=blog", "?project=%2F&task=%2310"},
	} {
		b.navigate("/tasks" + c.list)
		b.until("the list kept", "sessionStorage.getItem('artifacts.tasks') === "+js(c.list))
		b.navigate("/tasks" + c.task)
		b.until("the task's page", "!document.getElementById('task-head').hidden")
		say("up "+c.list+" "+c.task, up)
	}

	// The page list as it was: a project pressed again is let go, and it has its own.
	b.navigate("/?project=shop")
	b.until("the page list", rowsDrawn)
	say("page list chips", "[...document.querySelectorAll('#chips .chip')].map((c) => c.textContent + ' ' + c.style.getPropertyValue('--c')).join(', ')")
	say("page list, its query kept", "sessionStorage.getItem('artifacts.list') + ' ' + document.getElementById('to-list').getAttribute('href')")
	b.eval("document.querySelector('.chip[data-project=shop]').click()")
	say("page list, let go", "location.search")

	// On the task list the bar's unread leads to the page list, narrowing nothing here.
	b.navigate("/tasks")
	b.until("the bar's unread", "!document.getElementById('news').hidden")
	b.moving("document.getElementById('news').click()")
	b.until("the page list of the new", "location.pathname === '/' && location.search === '?view=new'")

	// The ? beside the pins: opened under its button, within the window; Esc and the button close it.
	b.navigate("/tasks?project=shop")
	b.until("the pins", "document.querySelector('.pin')")
	const tip = "document.getElementById('pins-tip').matches(':popover-open')"
	help := b.rect("document.getElementById('pins-help')")
	b.click(help.X+help.Width/2, help.Y+help.Height/2)
	b.until("the words open", tip)
	say("help", "document.getElementById('pins-tip').textContent")
	box := b.rect("document.getElementById('pins-tip')")
	if box.Y < help.Y+help.Height || box.X < 16 || box.X+box.Width > 1440 {
		t.Errorf("the words at %+v, the button at %+v", box, help)
	}
	b.pressKey("\x1b")
	b.until("Esc to close", "!"+tip)
	b.click(help.X+help.Width/2, help.Y+help.Height/2)
	b.until("the words open", tip)
	b.click(help.X+help.Width/2, help.Y+help.Height/2)
	b.until("the button to close", "!"+tip)
	// Narrow, within the window too, 16px from its edge.
	for _, width := range []int64{400, 360} {
		b.size(width, 800, false)
		narrow := b.rect("document.getElementById('pins-help')")
		b.click(narrow.X+narrow.Width/2, narrow.Y+narrow.Height/2)
		b.until("the words open", tip)
		box = b.rect("document.getElementById('pins-tip')")
		if box.X < 15.5 || box.X+box.Width > float64(width)-15.5 {
			t.Errorf("width %d: the words at %+v", width, box)
		}
		b.pressKey("\x1b")
		b.until("Esc to close", "!"+tip)
	}
	b.size(1440, 800, false)

	// What cannot be read says so on the task list alone, and goes when mended. The project in use stays, not called gone, and the one
	// kept is not written over; with neither there is none, and no pins.
	writeTasks("{ broken")
	b.navigate("/")
	b.until("the page list", rowsDrawn)
	say("broken, list: error hidden", "document.getElementById('error').hidden")
	b.eval("localStorage.setItem('artifacts.taskProject', 'blog')")
	b.navigate("/tasks?project=shop")
	b.until("the error", "!document.getElementById('error').hidden")
	say("broken, tasks: failed shown", "!document.getElementById('tasks-failed').hidden + ' ' + document.getElementById('tasks-none').hidden")
	say("broken, tasks", "location.search + ' ' + localStorage.getItem('artifacts.taskProject') + ' ' + [...document.querySelectorAll('.views button[data-view]')].map((v) => v.dataset.view).join(',') + ' pins ' + document.querySelectorAll('.pin').length + ' rows ' + document.querySelectorAll('#list > li').length")
	say("broken, tasks: says why", "document.getElementById('error').textContent.includes('mend it by hand')")
	b.navigate("/tasks")
	b.until("the error", "!document.getElementById('error').hidden")
	say("broken, the one kept", "location.search + ' ' + localStorage.getItem('artifacts.taskProject')")
	b.eval("localStorage.removeItem('artifacts.taskProject')")
	b.navigate("/tasks")
	b.until("the error", "!document.getElementById('error').hidden")
	say("broken, none kept", "location.search + ' ' + document.getElementById('pins').hidden")
	writeTasks(taskWork)
	b.until("mended", "document.getElementById('error').hidden && "+rowsDrawn)
	say("mended", "location.search + ' ' + document.getElementById('tasks-failed').hidden")
	// The kept one written nowhere while unreadable, so that the project mended is the one the tasks choose.
	writeTasks("{ broken")
	b.eval("localStorage.setItem('artifacts.taskProject', 'gone')")
	b.navigate("/tasks")
	b.until("the error", "!document.getElementById('error').hidden")
	say("broken, gone kept", "location.search + ' ' + sessionStorage.getItem('artifacts.tasks') + ' ' + localStorage.getItem('artifacts.taskProject')")
	writeTasks(taskWork)
	b.until("mended", "document.getElementById('error').hidden && "+rowsDrawn)
	say("mended, chosen anew", "location.search + ' ' + localStorage.getItem('artifacts.taskProject')")

	// Pressed while unreadable, the project borrowed is the one chosen.
	writeTasks("{ broken")
	b.eval("localStorage.setItem('artifacts.taskProject', 'blog')")
	b.navigate("/tasks")
	b.until("the error", "!document.getElementById('error').hidden && document.querySelector('.chip[data-project=blog]')")
	b.eval("document.querySelector('.chip[data-project=blog]').click()")
	writeTasks(taskWork)
	b.until("mended", "document.getElementById('error').hidden && "+rowsDrawn)
	say("pressed while unreadable", "location.search")

	// A pin archived is no project, and no words that the list is empty against it.
	writeTasks("{}")
	for _, slug := range []string{"p1", "p2", "p3", "p6", "p8", "p8a", "p10", "p11", "pinned", "pinonly"} {
		s.floe(t, "rm", slug)
	}
	b.navigate("/")
	b.until("the list read", "!document.getElementById('all-archived').hidden")
	b.navigate("/tasks")
	b.until("the guide", "!document.getElementById('tasks-empty').hidden")
	s.floe(t, "publish", s.page(t, "p1.yaml", "p1", ""), "--slug", "p1")
	s.floe(t, "work", "p1", "--task", "#1", "--state", "waiting")
	writeTasks(taskWork)

	if got, want := strings.Join(out, "\n"), taskListWant; got != want {
		t.Errorf("the task list:\n%s\nwant:\n%s", got, want)
	}

	// The three widths: the shelf beside the rows, over them, and over them with nothing past the screen's edge.
	for _, c := range []struct {
		width  int64
		narrow bool
	}{{1440, false}, {900, true}, {400, true}, {360, true}} {
		b.size(c.width, 800, false)
		b.navigate("/tasks?project=shop")
		b.until("the rows", rowsDrawn)
		shelf, col := b.rect("document.getElementById('shelf')"), b.rect("document.getElementById('listcol')")
		beside := col.X >= shelf.X+shelf.Width-1
		filter := b.eval("document.getElementById('narrow-filter').offsetParent !== null") == "true"
		// The bar's own too, whose last thing goes off the screen when the logo's word stays
		over := b.eval("document.documentElement.scrollWidth > innerWidth || document.getElementById('server').getBoundingClientRect().right > innerWidth") == "true"
		reader := b.eval("document.getElementById('reader').offsetParent !== null") == "true"
		// Narrow, the button says the project, at every width, not lit as narrowed
		if c.narrow {
			if got := b.eval("document.getElementById('narrow-filter').getBoundingClientRect().right <= innerWidth"); got != "true" {
				t.Errorf("width %d: the button is off the screen", c.width)
			}
			name := b.eval(taskFilterName)
			if name != "shop true  false" {
				t.Errorf("width %d: the button: %s", c.width, name)
			}
		}
		if beside == c.narrow || filter != c.narrow || over || reader {
			t.Errorf("width %d: beside %t, filter %t, past the edge %t, reader %t", c.width, beside, filter, over, reader)
		}
		if got := fmt.Sprint(int64(col.X + col.Width)); got != fmt.Sprint(c.width) {
			t.Errorf("width %d: the rows end at %s", c.width, got)
		}
	}
	b.unsize()

	// In every language, on the narrowest: the name, at least 2em of it.
	for _, lang := range i18n.Langs() {
		other := newBrowser(t, s.Base, lang)
		other.size(360, 800, false)
		other.navigate("/tasks?project=shop")
		other.until("the rows", rowsDrawn)
		if got := other.eval(taskFilterName); got != "shop true  false" {
			t.Errorf("%s at 360: the button: %s", lang, got)
		}
	}
}

// taskListWant is what TestUITaskList reads of the list, in the order it reads it.
const taskListWant = `no work, tasks: shelf shown: false
opened: ?project=shop shop
waiting: #2Two [0 page(s)]; #1One [design, 2 page(s), 2 unread, b/c]
views: todo 1, doing 2, waiting 2, done 2, archived 1
head: 2 · longest in Waiting first
pins: pinned /a/pinned/
hrefs: /tasks?project=shop&task=%232 /tasks?project=shop&task=%231
a row's project, in its title alone: #2 / Two / shop
link: b/c https://example.com/a/b/c _blank noopener https://example.com/a/b/c
current: page
search shown: true true
chips: shop2 true var(--pc0), blog1 false var(--pc1), pin0 false var(--pc2), solo1 false var(--pc3), Not set1 false var(--faint)
j: #2Two
j j: #1One
j j k: #2Two
nothing picked: true
doing: #5 [0 page(s)]; #4 [0 page(s)]
doing head: 2 · latest change first
doing url: ?view=doing&project=shop ?view=doing&project=shop
doing chips: 2 0 0 0 0
done: #8 [1 page(s), 1 archived, 1 unread]; #7 [0 page(s)]
done head: 2 · latest change first
done url: ?view=done&project=shop ?view=done&project=shop
done chips: 2 0 0 0 0
archived: #6 [0 page(s), 1 archived]
archived head: 1 · latest change first
archived url: ?view=archived&project=shop ?view=archived&project=shop
archived chips: 1 0 0 0 0
the list's own key untouched: 
switch hrefs: / /tasks?view=archived&project=shop
the first view is left out: ?project=shop
to blog: ?project=blog blog
blog again: ?project=blog true
the one kept, the view kept: ?view=doing&project=blog
?project=shop: #2Two [0 page(s)]; #1One [design, 2 page(s), 2 unread, b/c]
?project=shop none: 
?project=shop pins: pinned | undefined
?project=shop kept: shop
?project=%2F: #10 [1 page(s), 1 unread]
?project=%2F none: 
?project=%2F pins:  | None
?project=%2F kept: /
?project=pin: 
?project=pin none: No tasks: Waiting / false
?project=pin pins: pinonly | undefined
?project=pin kept: pin
?project=shop&view=archived: #6 [0 page(s), 1 archived]
?project=shop&view=archived none: 
?project=shop&view=archived pins: pinned | undefined
?project=shop&view=archived kept: shop
?project=old&view=doing: 
?project=old&view=doing none: No tasks in old / true
?project=old&view=doing pins:  | None
?project=old&view=doing kept: shop
gone, on the shelf: old0
another window's: blog
pressed, it is kept: shop
folded: false shop
the kept gone: ?project=shop
none waiting: ?project=shop
waited longest: ?project=blog
up ?view=doing&project=shop ?project=shop&task=%231: /tasks?project=shop
up ?project=blog ?project=shop&task=%231: /tasks?project=shop
up ?project=blog ?project=shop&task=%235: /tasks?project=shop&view=doing
up ?project=blog ?project=shop&task=%236: /tasks?project=shop&view=archived
up ?project=blog ?project=%2F&task=%2310: /tasks?project=%2F
page list chips: shop5 var(--pc0), pin1 var(--pc2), Not set1 var(--faint)
page list, its query kept: ?project=shop /?project=shop
page list, let go: 
help: Pinned pagesKeep pages you open again and again, such as an overview or a runbook, here. Only this project's pages are listed. To add one, choose "Pin to the task list" in a page's "…" menu; the × on a pin takes it off.
broken, list: error hidden: true
broken, tasks: failed shown: true true
broken, tasks: ?project=shop blog todo,doing,waiting,done,archived pins 1 rows 0
broken, tasks: says why: true
broken, the one kept:  blog
broken, none kept:  true
mended: ?project=shop true
broken, gone kept:  ?project=shop gone
mended, chosen anew: ?project=shop shop
pressed while unreadable: ?project=blog`
