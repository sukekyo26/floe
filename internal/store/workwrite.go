package store

import (
	"errors"
	"reflect"
	"slices"
	"strings"

	"floe/internal/usererr"
)

// WorkWrite is one write of pages and their tasks, all checked under the lock before any is written: the pages' own
// marks, and the fields of each task the pages are on, or of the task Project and Task name without pages. Its task
// fields are TaskEdit's, embedded, which encoding/json/v2 reads as the write's own.
type WorkWrite struct {
	TaskEdit

	// Slugs is the pages, none for a task alone.
	Slugs []string `json:"slugs,omitzero"`
	// Project is, without Slugs, the task's project as tasks.json has it ("/" for none).
	Project *string `json:"project,omitzero"`
	// Task is, with Slugs, the task the pages go on in their projects ("" takes them off); without, the task written.
	Task *string `json:"task,omitzero"`
	// Category, Pin and Clear are the pages' own, as WorkEdit has them; Clear takes the task, category and pin off.
	Category *string `json:"category,omitzero"`
	Pin      *bool   `json:"pin,omitzero"`
	Clear    bool    `json:"clear,omitzero"`
}

// WorkWritten is what a WorkWrite left: each task it is on, in the order first met, and each page.
type WorkWritten struct {
	Tasks []TaskWritten `json:"tasks"`
	Pages []Edited      `json:"pages"`
}

// taskRef is a task by its project (ProjectKey) and name.
type taskRef struct {
	project, name string
}

// hasFields tells whether ed writes any field of a task: what may make a row, and needs the pages on a task.
func (ed TaskEdit) hasFields() bool {
	return ed.Title != nil || ed.State != nil || ed.URL != nil || ed.Categories != nil || ed.Parent != nil
}

// Work writes w under one lock: the pages are read, each one's work as it comes to be, and the tasks it comes to be on
// (the task named without pages), are checked, and only then tasks.json and the pages' meta.json are written, in that
// order, so that a refusal writes nothing, and a task a page names always has a row and the page's category among its
// categories. A row that changes moves its Rev on.
func (s *Store) Work(w WorkWrite) (WorkWritten, error) {
	w.URL = pastedURL(w.URL)
	if err := checkTaskEdit(w.TaskEdit); err != nil {
		return WorkWritten{}, err
	}
	if w.Parent != nil {
		parent, err := TaskName(*w.Parent)
		if err != nil {
			return WorkWritten{}, err
		}
		w.Parent = &parent
	}
	var alone taskRef
	if len(w.Slugs) == 0 {
		project, name, err := taskKey(deref(w.Project), deref(w.Task))
		if err != nil {
			return WorkWritten{}, err
		}
		alone = taskRef{project: project, name: name}
	} else if w.Task != nil {
		name, err := TaskName(*w.Task)
		if err != nil {
			return WorkWritten{}, err
		}
		w.Task = &name
	}
	unlock, err := s.lock(true)
	if err != nil {
		return WorkWritten{}, err
	}
	defer unlock()
	all, err := s.ReadTasks()
	if err != nil {
		return WorkWritten{}, err
	}
	pages, err := s.workPages(w)
	if err != nil {
		return WorkWritten{}, err
	}
	refs := []taskRef{alone}
	if len(w.Slugs) > 0 {
		if refs, err = pagesTasks(w, pages); err != nil {
			return WorkWritten{}, err
		}
	}
	if err = s.checkWork(all, w, refs, pages); err != nil {
		return WorkWritten{}, err
	}
	return s.writeWork(all, w, refs, pages)
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// workPage is a page of a WorkWrite: its meta as it comes to be, and the task it comes to be on.
type workPage struct {
	slug string
	meta *Meta
	ref  taskRef
}

// workPages reads the pages of w, each once, and makes their work as w leaves it.
func (s *Store) workPages(w WorkWrite) ([]workPage, error) {
	var pages []workPage
	for _, slug := range w.Slugs {
		if slices.ContainsFunc(pages, func(p workPage) bool { return p.slug == slug }) {
			continue
		}
		if _, ok := s.dir(slug); !ok {
			return nil, usererr.New("store_no_page", slug)
		}
		m := s.ReadMeta(slug)
		if m == nil {
			return nil, usererr.New("store_no_page", slug)
		}
		if w.Clear {
			empty, off := "", false
			setWork(m, &WorkEdit{Task: &empty, Category: &empty, Pin: &off})
		}
		setWork(m, &WorkEdit{Task: w.Task, Category: w.Category, Pin: w.Pin})
		ref := taskRef{project: ProjectKey(m.Project), name: ""}
		if m.Work != nil {
			ref.name = m.Work.Task
		}
		pages = append(pages, workPage{slug: slug, meta: m, ref: ref})
	}
	return pages, nil
}

// pagesTasks is the different tasks the pages come to be on, the first of each first. A task field or a category of a
// page that comes to be on no task is refused: setWork would drop the category, and the field would be of no task.
func pagesTasks(w WorkWrite, pages []workPage) ([]taskRef, error) {
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

// checkWork refuses w before anything is written: what the screen saw (New, Rev), which is of one task, the parent,
// a task of nothing to write and no row, and a category with pages left out of the categories written.
func (s *Store) checkWork(all Tasks, w WorkWrite, refs []taskRef, pages []workPage) error {
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

// checkTask refuses w of the task ref, rows being its project's: as checkWork says.
func (s *Store) checkTask(rows map[string]Task, w WorkWrite, ref taskRef, pages []workPage) error {
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
	if _, had := rows[ref.name]; !had && len(pages) == 0 && !w.hasFields() {
		return ErrNotFound
	}
	if w.Categories == nil {
		return nil
	}
	return s.checkPaged(ref, categoriesOf(*w.Categories), pages)
}

// checkPaged refuses categories written of the task ref that leave out a category another page has on it: the pages of
// the write come to be in them (writeWork adds theirs). Archived pages count.
func (s *Store) checkPaged(ref taskRef, categories []string, pages []workPage) error {
	var refused error
	err := s.eachPageOn(ref.project, ref.name, func(slug string, m *Meta) bool {
		if slices.ContainsFunc(pages, func(p workPage) bool { return p.slug == slug }) {
			return true
		}
		if c := m.Work.Category; c != "" && !slices.Contains(categories, c) {
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

// writeWork writes w, checked: each task's row made when there is none, its fields, the categories of its pages it
// has not added at the end, and its Rev moved on when it changed; tasks.json, then the pages.
// LIMIT: a page the system refuses to write after tasks.json stops the write there: the rows and the pages before it
// stay written, which keeps every task a page names of a row, and the same write given again finishes it. Write the
// pages to new files first and rename them after if a half-done write comes to mislead.
func (s *Store) writeWork(all Tasks, w WorkWrite, refs []taskRef, pages []workPage) (WorkWritten, error) {
	out := WorkWritten{Tasks: make([]TaskWritten, 0, len(refs)), Pages: make([]Edited, 0, len(pages))}
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
		wrote := !had || parentMade || !reflect.DeepEqual(t, was)
		added := slices.DeleteFunc(slices.Clone(t.Categories), func(c string) bool {
			return slices.Contains(was.Categories, c) || (w.Categories != nil && slices.Contains(categoriesOf(*w.Categories), c))
		})
		if wrote {
			t.Rev++
			rows[ref.name] = t
			changed = true
		}
		out.Tasks = append(out.Tasks, TaskWritten{
			Project: ref.project, Task: ref.name, Row: t, Made: !had, Wrote: wrote, Added: added,
		})
	}
	if changed {
		if err := s.writeTasks(all); err != nil {
			return WorkWritten{}, err
		}
	}
	for _, p := range pages {
		if err := s.WriteMeta(p.slug, p.meta); err != nil {
			return WorkWritten{}, err
		}
		out.Pages = append(out.Pages, Edited{Slug: p.slug, TaskProject: p.ref.project, Work: p.meta.Work})
	}
	return out, nil
}

// rowAfter is the row of the task ref as w leaves it: was, or a new one when it had none, with w's fields, and the
// categories of its pages it has not, at the end in the pages' order.
func rowAfter(was Task, had bool, w WorkWrite, ref taskRef, pages []workPage) Task {
	t := was
	if !had {
		t = newTaskRow()
	}
	t = t.edited(w.TaskEdit)
	for _, p := range pages {
		if p.ref != ref || p.meta.Work == nil {
			continue
		}
		if c := p.meta.Work.Category; c != "" && !slices.Contains(t.Categories, c) {
			t.Categories = append(slices.Clone(t.Categories), c)
		}
	}
	return t
}
