package store_test

import (
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/store"
	"floe/internal/usererr"
)

// Each place that refuses a write of a task of no project says so in the words of the language it is told in, never
// tasks.json's "/": one case a place, as two places make store_parent_none. English is told first, as Error() does,
// and Japanese after it from the same mistake.
func TestTaskRefusalsOfNoProject(t *testing.T) {
	t.Parallel()

	no := false
	for _, c := range []struct {
		name  string
		write func(t *testing.T, s *store.Store, root string) error
		en    string
		ja    string
	}{
		{"a parent of no row, as the CLI names it", func(t *testing.T, s *store.Store, _ string) error {
			t.Helper()
			mustEdit(t, s, "#1", store.TaskEdit{Title: new("t"), New: new(true)})
			_, err := editTask(s, "/", "#1", store.TaskEdit{Parent: new("#42")})
			return err
		}, "no task #42 in no project can be a parent", "プロジェクト未設定に、親にできるタスク #42 がありません"},
		{"a parent of no row, as the screen read it", func(t *testing.T, s *store.Store, _ string) error {
			t.Helper()
			mustEdit(t, s, "#1", store.TaskEdit{Title: new("t"), New: new(true)})
			_, err := editTask(s, "/", "#1", store.TaskEdit{New: &no, Rev: new(1), Parent: new("#42"), ParentNew: &no})
			return err
		}, "no task #42 in no project can be a parent", "プロジェクト未設定に、親にできるタスク #42 がありません"},
		{"forgetting a task with a page", func(t *testing.T, s *store.Store, root string) error {
			t.Helper()
			mustEdit(t, s, "#1", store.TaskEdit{Title: new("t"), New: new(true)})
			pageOn(t, s, root, "a", "", false)
			return s.ForgetTask("/", "#1")
		}, "task #1 of no project still has 1 page(s), such as a", "プロジェクト未設定のタスク #1 には、まだ 1 枚のページがあります"},
		{"forgetting a task with a child", func(t *testing.T, s *store.Store, _ string) error {
			t.Helper()
			mustEdit(t, s, "#1", store.TaskEdit{Title: new("t"), New: new(true)})
			mustEdit(t, s, "#2", store.TaskEdit{Parent: new("#1"), New: new(true)})
			return s.ForgetTask("/", "#1")
		}, "task #1 of no project still has 1 child task(s), such as #2", "プロジェクト未設定のタスク #1 には、まだ 1 個の子のタスクがあります"},
		{"a category with a page left out of the order", func(t *testing.T, s *store.Store, root string) error {
			t.Helper()
			mustEdit(t, s, "#1", store.TaskEdit{Categories: &[]string{"c"}, New: new(true)})
			pageOn(t, s, root, "a", "", false)
			m := s.ReadMeta("a")
			m.Task.Category = "c"
			if err := s.WriteMeta("a", m); err != nil {
				t.Fatal(err)
			}
			_, err := editTask(s, "/", "#1", store.TaskEdit{Categories: &[]string{}})
			return err
		}, `category "c" of task #1 in no project has pages (a)`, `タスク #1（プロジェクト未設定）のカテゴリ "c" にはページがあります`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "S")
			err := c.write(t, store.New(root), root)
			if err == nil {
				t.Fatal("written")
			}
			en := err.Error()
			ja := usererr.Tell(i18n.New("ja"), err).Render("")
			if !strings.Contains(en, c.en) || strings.Contains(en, "/") {
				t.Errorf("in English: %q, want %q", en, c.en)
			}
			if !strings.Contains(ja, c.ja) || strings.Contains(ja, "/") || strings.Contains(ja, "no project") {
				t.Errorf("in Japanese, after English: %q, want %q", ja, c.ja)
			}
		})
	}
}

// mustEdit writes ed to the task name of no project.
func mustEdit(t *testing.T, s *store.Store, name string, ed store.TaskEdit) {
	t.Helper()

	if _, err := editTask(s, "/", name, ed); err != nil {
		t.Fatal(err)
	}
}
