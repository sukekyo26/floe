package store

import (
	"errors"
	"reflect"
	"slices"
	"strings"

	"floe/internal/usererr"
)

// TaskChange is one write of pages and their tasks, all checked under the lock before any is written: the pages' own
// marks, and the fields of each task the pages are on, or of the task TaskProject and Task name without pages. Its task
// fields are TaskEdit's, embedded, which encoding/json/v2 reads as the write's own.
type TaskChange struct {
	TaskEdit

	// Slugs is the pages, none for a task alone.
	Slugs []string `json:"slugs,omitzero"`
	// TaskProject is, without Slugs, the task's project as tasks.json has it ("/" for none).
	TaskProject *string `json:"taskProject,omitzero"`
	// Task is, with Slugs, the task the pages go on in their projects ("" takes them off); without, the task written.
	Task *string `json:"task,omitzero"`
	// Category, Pin and Clear are the pages' own, as PageTaskEdit has them; Clear takes the task, category and pin off.
	Category *string `json:"category,omitzero"`
	Pin      *bool   `json:"pin,omitzero"`
	Clear    bool    `json:"clear,omitzero"`
}

// TaskChangeResult is what a TaskChange left: each task it is on, in the order first met, and each page.
type TaskChangeResult struct {
	Tasks []TaskWritten `json:"tasks"`
	Pages []Edited      `json:"pages"`
}

// taskRef is a task by its project (ProjectKey) and name.
type taskRef struct {
	project, name string
}

// hasFields tells whether ed writes any field of a task: what may make a row, and needs the pages on a task.
func (ed TaskEdit) hasFields() bool {
	return ed.Title != nil || ed.State != nil || ed.URLs != nil || ed.Categories != nil || ed.Parent != nil
}

// ChangeTasks writes w under one lock: the pages are read, each one's task as it comes to be, and the tasks it comes to
// be on (the task named without pages), are checked, and only then tasks.json and the pages' meta.json are written, in
// that order, so that a refusal writes nothing, and a task a page names always has a row and the page's category among
// its categories. A row that changes moves its Rev on.
// LIMIT: a row written by hand keeps its Rev, and one forgotten and made again starts it again: a screen that read it
// before may write over it. Count them apart if hand edits come to be done while screens are open
func (s *Store) ChangeTasks(w TaskChange) (TaskChangeResult, error) {
	w.URLs = pastedURLs(w.URLs)
	if err := checkTaskEdit(w.TaskEdit); err != nil {
		return TaskChangeResult{}, err
	}
	if w.Parent != nil {
		parent, err := TaskName(*w.Parent)
		if err != nil {
			return TaskChangeResult{}, err
		}
		w.Parent = &parent
	}
	var alone taskRef
	if len(w.Slugs) == 0 {
		project, name, err := taskKey(deref(w.TaskProject), deref(w.Task))
		if err != nil {
			return TaskChangeResult{}, err
		}
		alone = taskRef{project: project, name: name}
	} else if w.Task != nil {
		name, err := TaskName(*w.Task)
		if err != nil {
			return TaskChangeResult{}, err
		}
		w.Task = &name
	}
	unlock, err := s.lock(true)
	if err != nil {
		return TaskChangeResult{}, err
	}
	defer unlock()
	all, err := s.ReadTasks()
	if err != nil {
		return TaskChangeResult{}, err
	}
	pages, err := s.changedPages(w)
	if err != nil {
		return TaskChangeResult{}, err
	}
	refs := []taskRef{alone}
	if len(w.Slugs) > 0 {
		if refs, err = pagesTasks(w, pages); err != nil {
			return TaskChangeResult{}, err
		}
	}
	if err = s.checkChange(all, w, refs, pages); err != nil {
		return TaskChangeResult{}, err
	}
	return s.applyChange(all, w, refs, pages)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// changedPage is a page of a TaskChange: its meta as it comes to be, the task it comes to be on, and its task as read
// before the change.
type changedPage struct {
	slug string
	meta *Meta
	ref  taskRef
	was  PageTask
}

// changedPages reads the pages of w, each once, and makes their task as w leaves it.
func (s *Store) changedPages(w TaskChange) ([]changedPage, error) {
	var pages []changedPage
	for _, slug := range w.Slugs {
		if slices.ContainsFunc(pages, func(p changedPage) bool { return p.slug == slug }) {
			continue
		}
		if _, ok := s.dir(slug); !ok {
			return nil, usererr.New("store_no_page", slug)
		}
		m := s.ReadMeta(slug)
		if m == nil {
			return nil, usererr.New("store_no_page", slug)
		}
		var was PageTask
		if m.Task != nil {
			was = *m.Task
		}
		if w.Clear {
			empty, off := "", false
			setPageTask(m, &PageTaskEdit{Name: &empty, Category: &empty, Pin: &off})
		}
		setPageTask(m, &PageTaskEdit{Name: w.Task, Category: w.Category, Pin: w.Pin})
		ref := taskRef{project: ProjectKey(m.Project), name: ""}
		if m.Task != nil {
			ref.name = m.Task.Name
		}
		pages = append(pages, changedPage{slug: slug, meta: m, ref: ref, was: was})
	}
	return pages, nil
}

// pagesTasks is the different tasks the pages come to be on, the first of each first. A task field or a category of a
// page that comes to be on no task is refused: setPageTask would drop the category, and the field would be of no task.
func pagesTasks(w TaskChange, pages []changedPage) ([]taskRef, error) {
	needs := w.hasFields() || (w.Category != nil && *w.Category != "")
	if needs && w.Task != nil && *w.Task == "" {
		return nil, usererr.New("store_task_off")
	}
	var refs []taskRef
	for _, p := range pages {
		if p.ref.name == "" {
			if needs {
				return nil, usererr.New("store_page_no_task", p.slug)
			}
			continue
		}
		if !slices.Contains(refs, p.ref) {
			refs = append(refs, p.ref)
		}
	}
	return refs, nil
}

// checkChange refuses w before anything is written: what the screen saw (New, Rev), which is of one task, the parent,
// a task of nothing to write and no row, and a category with pages left out of the categories written.
func (s *Store) checkChange(all Tasks, w TaskChange, refs []taskRef, pages []changedPage) error {
	if projects := projectsOf(refs); w.Parent != nil && projects > 1 {
		return usererr.New("store_parent_projects", projects)
	}
	if (w.New != nil || w.Rev != nil) && len(refs) != 1 {
		// The tasks come of the pages as read under the lock: a write of one task, of pages moved meanwhile
		name := deref(w.Task)
		if len(refs) > 0 {
			name = refs[0].name
		}
		return conflict(usererr.New("store_task_was", name))
	}
	for _, ref := range refs {
		if err := s.checkTask(all[ref.project], w, ref, pages); err != nil {
			return err
		}
	}
	return nil
}

// checkTask refuses w of the task ref, rows being its project's: as checkChange says.
func (s *Store) checkTask(rows map[string]Task, w TaskChange, ref taskRef, pages []changedPage) error {
	if err := checkThere(rows, ref.name, w.New); err != nil {
		return err
	}
	if err := checkSeen(ref.project, rows, ref.name, w.TaskEdit); err != nil {
		return err
	}
	// A parent taken off a task of no row: not found of the task alone, nothing for the pages, which make the row
	err := checkParent(ref.project, rows, ref.name, w.TaskEdit)
	if err != nil && (len(pages) == 0 || !errors.Is(err, ErrNotFound)) {
		return err
	}
	if err := checkURLCount(rows[ref.name], ref.name, w.URLs); err != nil {
		return err
	}
	if _, had := rows[ref.name]; !had && len(pages) == 0 && !w.hasFields() {
		return ErrNotFound
	}
	if w.Categories == nil {
		return nil
	}
	return s.checkPaged(ref, categoriesOf(*w.Categories), pages)
}

// checkURLCount refuses ed when it adds URLs to the row t of the task name beyond maxURLs. A row over it already,
// written by hand, still takes the other writes, and those that take URLs off.
func checkURLCount(t Task, name string, ed *URLsEdit) error {
	if ed == nil {
		return nil
	}
	// Counted each once, as a write leaves them: a row written by hand may hold one twice
	before, after := withURLs(t.URLs, URLsEdit{Add: nil, Remove: nil, Order: nil}), withURLs(t.URLs, *ed)
	if len(after) > len(before) && len(after) > maxURLs {
		return usererr.New("store_task_urls_max", name, len(after), maxURLs)
	}
	return nil
}

// checkPaged refuses categories written of the task ref that leave out a category another page has on it: the pages of
// the write come to be in them (applyChange adds theirs). Archived pages count.
func (s *Store) checkPaged(ref taskRef, categories []string, pages []changedPage) error {
	var refused error
	err := s.eachPageOn(ref.project, ref.name, func(slug string, m *Meta) bool {
		if slices.ContainsFunc(pages, func(p changedPage) bool { return p.slug == slug }) {
			return true
		}
		if c := m.Task.Category; c != "" && !slices.Contains(categories, c) {
			refused = usererr.New("store_category_paged", c, ref.name, ref.project, slug)
			return false
		}
		return true
	})
	if err != nil {
		return err
	}
	return refused
}

// projectsOf is the number of different projects of refs.
func projectsOf(refs []taskRef) int {
	var projects []string
	for _, ref := range refs {
		if !slices.Contains(projects, ref.project) {
			projects = append(projects, ref.project)
		}
	}
	return len(projects)
}

// applyChange writes w, checked: each task's row made when there is none, its fields, the categories of its pages it
// has not added at the end, and its Rev moved on when it changed; tasks.json, then the pages.
// LIMIT: a page the system refuses to write after tasks.json stops the write there: the rows and the pages before it
// stay written, which keeps every task a page names of a row, and the same write given again finishes it. Write the
// pages to new files first and rename them after if a half-done write comes to mislead.
func (s *Store) applyChange(all Tasks, w TaskChange, refs []taskRef, pages []changedPage) (TaskChangeResult, error) {
	out := TaskChangeResult{Tasks: make([]TaskWritten, 0, len(refs)), Pages: make([]Edited, 0, len(pages))}
	changed := false
	for _, ref := range refs {
		if all[ref.project] == nil {
			all[ref.project] = map[string]Task{}
		}
		rows := all[ref.project]
		parentMade := w.ParentNew != nil && *w.ParentNew
		if parentMade {
			up := newTaskRow()
			up.Title = strings.TrimSpace(*w.ParentTitle)
			up.Rev = 1
			rows[*w.Parent] = up
		}
		was, had := rows[ref.name]
		t := rowAfter(was, had, w, ref, pages)
		// Compared with the URLs as a write leaves them, so that a row written by hand ([] or one twice) is not taken
		// to change, and its rev move, by a write that changes nothing of it
		wrote := !had || parentMade || !reflect.DeepEqual(t.tidied(), was.tidied())
		added := slices.DeleteFunc(slices.Clone(t.Categories), func(c string) bool {
			return slices.Contains(was.Categories, c) || (w.Categories != nil && slices.Contains(categoriesOf(*w.Categories), c))
		})
		if wrote {
			t.Rev++
			rows[ref.name] = t
			changed = true
		}
		out.Tasks = append(out.Tasks, TaskWritten{
			TaskProject: ref.project, Task: ref.name, Row: t, Made: !had, Wrote: wrote, Added: added,
		})
	}
	if changed {
		if err := s.writeTasks(all); err != nil {
			return TaskChangeResult{}, err
		}
	}
	for _, p := range pages {
		if err := s.WriteMeta(p.slug, p.meta); err != nil {
			return TaskChangeResult{}, err
		}
		out.Pages = append(out.Pages, p.edited())
	}
	return out, nil
}

// edited is the page p as written, with the task and category it was on when it left that task, for another or none.
func (p changedPage) edited() Edited {
	e := Edited{Slug: p.slug, TaskProject: p.ref.project, Task: p.meta.Task, OffTask: "", OffCategory: ""}
	if p.was.Name != "" && p.was.Name != p.ref.name {
		e.OffTask, e.OffCategory = p.was.Name, p.was.Category
	}
	return e
}

// rowAfter is the row of the task ref as w leaves it: was, or a new one when it had none, with w's fields, and the
// categories of its pages it has not, at the end in the pages' order.
func rowAfter(was Task, had bool, w TaskChange, ref taskRef, pages []changedPage) Task {
	t := was
	if !had {
		t = newTaskRow()
	}
	t = t.edited(w.TaskEdit)
	for _, p := range pages {
		if p.ref != ref || p.meta.Task == nil {
			continue
		}
		if c := p.meta.Task.Category; c != "" && !slices.Contains(t.Categories, c) {
			t.Categories = append(slices.Clone(t.Categories), c)
		}
	}
	return t
}
