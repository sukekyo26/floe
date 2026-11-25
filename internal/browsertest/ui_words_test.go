package browsertest_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/servetest"
	"floe/internal/store"
)

// wordsSite is a site in lang with page a on task #1 of project proj, and the three screens to open: the list, the task's page and
// the page whole.
func wordsSite(t *testing.T, lang i18n.Lang) (*site, []string) {
	t.Helper()

	srv := servetest.Start(t, nil)
	s := &site{Site: srv, b: newBrowser(t, srv.Base, lang)}
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	project := "proj"
	if _, err := store.New(s.Env["FLOE_DIR"]).Edit("a", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "a", "--task", "#1", "--title", "one", "--state", "doing")
	return s, []string{"/", "/tasks?project=proj&task=%231", "/a/a/"}
}

// openScreen opens path and waits for the page's script to have run.
func openScreen(b *browser, path string) {
	b.t.Helper()

	b.navigate(path)
	b.until("the page's script", "document.documentElement.hasAttribute('data-loads-begun') && document.readyState === 'complete'")
}

// A dialog's close button says Cancel where a submit button saves what is typed, which closing throws away, and Close where nothing is;
// a question's says No. The dialogs are read on each screen, as it has them, in each language.
// The settings' words are those of the part it opens on, its list, which saves nothing: Close, from the start (S422-P8).
// LIMIT: read as the screens hold them closed; a screen that puts a dialog in only as it opens would need it opened.
func TestUIDialogCloseWords(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	// A page whole: the modals every screen has alone
	frame := []string{"edit-dialog", "find", "keys", "move-dialog", "set-dialog", "settings", "tag-dialog"}
	// The React page list: the modals every screen has, its shelf's (filter-dialog) and its link to add (add-dialog)
	list := []string{
		"add-dialog", "edit-dialog", "filter-dialog", "find", "keys", "move-dialog", "set-dialog", "settings", "tag-dialog",
	}
	// A task's page: no shelf (filter-dialog) and no link to add (add-dialog); the add modal of the task's edit
	taskPage := []string{
		"add-category-dialog", "add-page-dialog", "edit-add-dialog", "edit-dialog", "edit-task-dialog", "find", "keys",
		"kin-dialog", "move-dialog", "set-dialog", "settings", "tag-dialog",
	}
	inEveryLanguage(t, func(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
		t.Helper()

		s, screens := wordsSite(t, lang)
		for _, path := range screens {
			openScreen(s.b, path)
			want := taskPage
			switch {
			case path == "/":
				want = list
			case strings.HasPrefix(path, "/a/"):
				want = frame
			}
			var got [][]json.RawMessage
			if err := json.Unmarshal([]byte(s.b.eval(`JSON.stringify([...document.querySelectorAll('dialog')].map((d) => [d.id,
				[...d.querySelectorAll('button')].some((b) => b.getAttribute('type') === 'submit'),
				[...d.querySelectorAll('button[data-close]')].map((b) => b.textContent.trim())]))`)), &got); err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 0, len(got))
			for _, g := range got {
				var id string
				var submit bool
				var closers []string
				if json.Unmarshal(g[0], &id) != nil || json.Unmarshal(g[1], &submit) != nil || json.Unmarshal(g[2], &closers) != nil {
					t.Fatalf("%s: a dialog read as %s", path, g)
				}
				ids = append(ids, id)
				word := msg("ui_close")
				if submit {
					word = msg("ui_cancel")
				}
				if id == "move-dialog" {
					word = msg("ui_no")
				}
				if id == "settings" {
					word = msg("ui_close")
				}
				if !slices.Equal(closers, []string{word}) {
					t.Errorf("%s, %s: close buttons %q, want one %q", path, id, closers, word)
				}
			}
			slices.Sort(ids)
			if !slices.Equal(ids, want) {
				t.Errorf("%s: dialogs %q, want %q: tell a new one's close word here", path, ids, want)
			}
		}
	})
}

// The help tells of each of the two screens in a part of its own, under a tab of its name: a sentence on the screen, then its words, the
// project among them (the last on the page list, where it narrows the pages, the first on the task list, which is of one project), each
// with its explanation. A word does not move to the other screen's part, and the project is explained as the screen uses it. Read as the
// help opens on the list and on a task's page, in each language.
func TestUIHelpSections(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, func(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
		t.Helper()

		s, screens := wordsSite(t, lang)
		for _, path := range screens[:2] {
			openScreen(s.b, path)
			s.b.eval("document.getElementById('keys-open').click()")
			s.b.until("the help", "document.getElementById('keys').open")
			var got struct {
				Said, Projects, Tabs []string
				Empty                int
			}
			if err := json.Unmarshal([]byte(s.b.eval(`(() => {
				const said = [], projects = [];
				let empty = 0;
				for (const e of document.querySelectorAll('#help-about p, #help-about dt, #help-about dd')) {
					const words = e.textContent.trim();
					if (e.tagName === 'P') said.push('¶ ' + words);
					else if (e.tagName === 'DT') said.push(words);
					else if (!words) empty++;
					else if (said.at(-1) === `+js(msg("ui_project"))+`) projects.push(words);
				}
				const tabs = [...document.querySelectorAll('#keys [data-help=about] span')].map((s) => s.textContent.trim());
				return JSON.stringify({ said, projects, tabs, empty });
			})()`)), &got); err != nil {
				t.Fatal(err)
			}
			want := []string{
				"¶ " + msg("ui_help_pages_text"), msg("ui_view_new"), msg("ui_view_favorites"), msg("ui_state_archived"), msg("ui_project"),
				"¶ " + msg("ui_help_tasks_text"), msg("ui_project"), msg("ui_help_task"), msg("ui_help_state"), msg("ui_task_category"), msg("ui_pins"),
			}
			if !slices.Equal(got.Said, want) {
				t.Errorf("%s: the help says %q, want %q", path, got.Said, want)
			}
			if got.Empty > 0 {
				t.Errorf("%s: %d words of the help have no explanation", path, got.Empty)
			}
			if want := []string{msg("ui_help_project_pages_text"), msg("ui_help_project_tasks_text")}; !slices.Equal(got.Projects, want) {
				t.Errorf("%s: the project is explained as %q, want %q", path, got.Projects, want)
			}
			if want := []string{msg("ui_help_about_pages"), msg("ui_help_about_tasks")}; !slices.Equal(got.Tabs, want) {
				t.Errorf("%s: the help's tab is named %q, want %q", path, got.Tabs, want)
			}
		}
	})
}
