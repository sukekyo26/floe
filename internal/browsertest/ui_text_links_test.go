package browsertest_test

import "testing"

// underlined tells, of each link sel finds, its words and whether it is underlined: a link among words has nothing
// else to tell it from them, and Tailwind's preflight takes the browser's own underline off.
func underlined(sel string) string {
	return "[...document.querySelectorAll(" + js(sel) + ")].map((a) => a.textContent + ' ' + getComputedStyle(a).textDecorationLine).join(', ')"
}

// TestUITextLinks: the links among the words of a note, of a list with nothing to show, of a task not there and of the
// printed frame are underlined, and that of a task not there is as dark as the words over it.
func TestUITextLinks(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	defer b.unsize()

	out := make([]string, 0, 6)
	b.navigate("/a/d/?v=1")
	b.until("v1 told read", pageNews("d")+".then((n) => n === 'version')")
	b.until("an old version's words", "document.querySelectorAll('[data-testid=ver-note] a').length === 2")
	out = append(out, b.eval(underlined("[data-testid=ver-note] a")), b.eval(underlined("#print-raw")+".replace(location.origin, '')"))
	b.navigate("/a/d/?v=99")
	b.until("a newer version's words", "document.querySelector('[data-testid=page-note] a')")
	out = append(out, b.eval(underlined("[data-testid=page-note] a")))

	// All three of the task list's are drawn, the one shown hidden no more
	b.navigate("/tasks")
	b.until("no task", "!document.getElementById('tasks-empty').hidden")
	out = append(out, b.eval(underlined("#tasks-none a, #tasks-failed a, #tasks-empty a")))
	b.navigate("/tasks?project=none&task=none")
	b.until("not there", "!document.getElementById('task-missing').hidden")
	out = append(out, b.eval(underlined("#task-missing a")),
		b.eval("getComputedStyle(document.getElementById('task-missing-up')).color === getComputedStyle(document.querySelector('#task-missing strong')).color"))

	check(t, "links", out, "Open the latest version underline, See what differs from the latest underline|/a/d/?raw&v=1 underline|"+
		"See what changed underline|Open another project underline, Back to the page list underline, Back to the page list underline|"+
		"Back to the task list underline|true")
}
