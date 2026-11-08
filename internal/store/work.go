package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/usererr"
)

// tasksName is the file of the tasks in the store, the one thing that holds a task's state: its pages name it, and it
// names no page.
const tasksName = "tasks.json"

// noProject is the project of a task whose pages have none, in tasks.json, the API and the management page's URLs.
const noProject = "/"

// ProjectKey is the project of a page as tasks.json has it: trimmed, as a directory's name may not be. It is the one
// place that says what a page's project is there: a page's list entry carries it (Listed.TaskProject) for what cannot
// call this.
func ProjectKey(project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return noProject
	}
	return project
}

// TaskName is the name of a page's task as written: trimmed, "" taking the page off its task. A name of spaces alone is
// no way to say that, and is refused.
func TaskName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if name != "" && trimmed == "" {
		return "", usererr.New("store_task_name")
	}
	if !utf8.ValidString(trimmed) {
		return "", usererr.New("store_task_utf8")
	}
	return trimmed, nil
}

// Work is what a page holds of its place on the task list: the task it is on, its step there, whether it is pinned.
type Work struct {
	Task string `json:"task,omitempty"`
	Step string `json:"step,omitempty"`
	Pin  bool   `json:"pin,omitempty"`
}

// WorkEdit is what an edit changes of a page's Work, each left as it is when nil.
type WorkEdit struct {
	Task *string `json:"task,omitzero"`
	Step *string `json:"step,omitzero"`
	Pin  *bool   `json:"pin,omitzero"`
}

// setWork applies ed to m. A step is of its task: a page that moves to another task leaves it, unless ed gives the
// step it takes there; a page off a task has none, and one neither on a task nor pinned has no Work. TaskName has
// checked ed.
// LIMIT: a page moved to another project's task of the same name, by an edit or a publish, keeps its step: names of
// steps mostly hold there. Take it off in the two ways if that comes to mislead.
func setWork(m *Meta, ed *WorkEdit) {
	var w Work
	if m.Work != nil {
		w = *m.Work
	}
	if ed.Task != nil {
		name := strings.TrimSpace(*ed.Task)
		if name != w.Task {
			w.Step = ""
		}
		w.Task = name
	}
	if ed.Step != nil {
		w.Step = strings.TrimSpace(*ed.Step)
	}
	if ed.Pin != nil {
		w.Pin = *ed.Pin
	}
	if w.Task == "" {
		w.Step = ""
	}
	m.Work = &w
	if w.Task == "" && !w.Pin {
		m.Work = nil
	}
}

// Task is a task's row of tasks.json. At is when it was made or its state last changed; Step the last step written of
// it that was not empty; Parent the task it is a child of, of the same project (childrenOfTask).
type Task struct {
	Title  string   `json:"title,omitempty"`
	State  string   `json:"state"`
	Step   string   `json:"step,omitempty"`
	At     string   `json:"at"`
	URL    string   `json:"url,omitempty"`
	Steps  []string `json:"steps,omitempty"`
	Parent string   `json:"parent,omitempty"`
	// Rev counts the writes of the row, which a screen sends back as what it read: 0 of no row.
	Rev int `json:"rev,omitempty"`
}

// Tasks is tasks.json: the tasks by project (ProjectKey), then by name.
type Tasks map[string]map[string]Task

// State is a state a task can be in: whether the list of it shows the longest in it first, whether a task new or of no
// row is in it, and whether the task list opens on it.
type State struct {
	Name   string `json:"name"`
	Oldest bool   `json:"oldest"`
	New    bool   `json:"new"`
	First  bool   `json:"first"`
}

// States is the one table of the states of a task, in the order of the work: what is written is checked by it, and the
// management page reads it, in this order.
var States = []State{
	{Name: "todo", Oldest: false, New: true, First: false},
	{Name: "doing", Oldest: false, New: false, First: false},
	{Name: "waiting", Oldest: true, New: false, First: true},
	{Name: "done", Oldest: false, New: false, First: false},
	{Name: "archived", Oldest: false, New: false, First: false},
}

// TaskEdit is what a write changes of a task's row, each left as it is when nil. An empty URL or Steps takes it off; an
// empty Step leaves the last one written.
type TaskEdit struct {
	Title *string
	State *string
	Step  *string
	URL   *string
	Steps *[]string
	// An empty Parent takes it off.
	Parent *string
	// New, when said, is whether the writer takes the task to be new: refused, as Conflict, when it is there, or not.
	New *bool
	// Rev, when said, is the row's Rev as the writer read it: refused, as Conflict, when the row has been written since.
	Rev *int
	// ParentNew and ParentTitle go with Parent: ParentNew whether the parent named is taken to be new, as New is of the
	// task; and ParentTitle, with ParentNew true, the title of the parent made in the same write.
	ParentNew   *bool
	ParentTitle *string
}

func validState(name string) bool {
	return slices.ContainsFunc(States, func(s State) bool { return s.Name == name })
}

// validURL tells a URL a row may hold, which goes to the browser as a link's href: http and https alone.
func validURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// StateNames is the states' names in the words of what is told of them.
func StateNames() string {
	names := make([]string, len(States))
	for i, s := range States {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

func (s *Store) tasksFile() string {
	return filepath.Join(s.root, tasksName)
}

// ReadTasks reads tasks.json, none being no tasks. What cannot be read, or holds what no write may (checkTaskEdit), is
// refused and not made anew as reads.json is: the states and URLs in it are not to be had again, so that it is mended
// by hand.
// It takes no lock: a write replaces the file whole.
// LIMIT: a parent written by hand that childrenOfTask does not count is not refused, unlike a state: the writes that
// would mend it read through here, and the page shows such a task as of no parent. Tell of it if hand edits come to
// hide it.
// LIMIT: a key of a row floe does not know is lost at the next write. Keep them if tasks.json comes to be hand-edited.
func (s *Store) ReadTasks() (Tasks, error) {
	b, _, err := files.Read(s.tasksFile())
	if errors.Is(err, fs.ErrNotExist) {
		return Tasks{}, nil
	}
	if err != nil {
		return nil, usererr.CannotRead(s.tasksFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	broken := func(format string, args ...any) error {
		return usererr.NewAt(s.tasksFile(), 0, 0, "store_tasks_broken", fmt.Sprintf(format, args...))
	}
	var all Tasks
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, broken("%s", err.Error())
	}
	if all == nil {
		return nil, broken("it holds null, not the projects")
	}
	for project, rows := range all {
		if rows == nil {
			return nil, broken("project %s holds null, not its tasks", project)
		}
		for name, t := range rows {
			if !validState(t.State) {
				return nil, broken("task %s of project %s has the state %q", name, project, t.State)
			}
			if timeOf(t.At).IsZero() {
				return nil, broken("task %s of project %s has the time %q, not one like 2026-01-02T15:04:05Z", name, project, t.At)
			}
			if t.URL != "" && !validURL(t.URL) {
				return nil, broken("task %s of project %s has the URL %q, not http or https", name, project, t.URL)
			}
		}
	}
	return all, nil
}

func (s *Store) writeTasks(all Tasks) error {
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", tasksName, err)
	}
	if err := files.WriteAtomic(s.tasksFile(), append(b, '\n'), 0o600, nil); err != nil {
		return usererr.CannotWrite(s.tasksFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	return nil
}

// taskKey is the project and name that name a task, each as a page's is written. An empty project is refused, not read
// as no project's: that is "/", and an empty one is a query left out. A byte that is no UTF-8, which only a query can
// bring, would come to be another name as JSON writes it.
func taskKey(project, raw string) (key, name string, err error) {
	if strings.TrimSpace(project) == "" {
		return "", "", usererr.New("store_task_project")
	}
	if !utf8.ValidString(project) {
		return "", "", usererr.New("store_task_utf8")
	}
	if name, err = TaskName(raw); err != nil {
		return "", "", err
	}
	if name == "" {
		return "", "", usererr.New("store_task_name")
	}
	return ProjectKey(project), name, nil
}

// checkTaskEdit refuses what no row may hold.
func checkTaskEdit(ed TaskEdit) error {
	if ed.State != nil && !validState(*ed.State) {
		return usererr.New("store_task_state", *ed.State, StateNames())
	}
	if ed.URL != nil && *ed.URL != "" && !validURL(*ed.URL) {
		return usererr.New("store_task_url", *ed.URL)
	}
	if !parentFieldsMatch(ed) {
		return usererr.New("store_parent_fields")
	}
	return nil
}

// parentFieldsMatch tells whether ParentNew and ParentTitle go with ed's Parent: neither without one or when it is
// taken off, and a title of more than spaces exactly when the parent is to be made.
func parentFieldsMatch(ed TaskEdit) bool {
	if ed.Parent == nil {
		return ed.ParentNew == nil && ed.ParentTitle == nil
	}
	if strings.TrimSpace(*ed.Parent) == "" {
		return ed.ParentNew == nil && ed.ParentTitle == nil
	}
	made := ed.ParentNew != nil && *ed.ParentNew
	titled := ed.ParentTitle != nil && strings.TrimSpace(*ed.ParentTitle) != ""
	return made == titled && (made || ed.ParentTitle == nil)
}

// childrenOfTask is the names of the children of the task name, in order: a task is a child of the one its parent
// names when that one has a row of the same project and no parent itself, so that a parent of itself, a pair each the
// other's, a grandchild and a parent gone, written by hand, make no child. The management page's tasksOf says the same.
func childrenOfTask(rows map[string]Task, name string) []string {
	var children []string
	if t, ok := rows[name]; !ok || t.Parent != "" {
		return nil
	}
	for child, t := range rows {
		if t.Parent == name {
			children = append(children, child)
		}
	}
	slices.Sort(children)
	return children
}

// checkParent refuses what ed would make of the parent of the task name. Only that row is looked at: another row broken
// by hand does not keep this one from being written. A parent taken off and nothing else, of a task of no row, is
// ErrNotFound: a name mistyped would make a row. A parent of itself checkSeen has refused.
func checkParent(project string, rows map[string]Task, name string, ed TaskEdit) error {
	if ed.Parent == nil {
		return nil
	}
	parent := *ed.Parent
	if parent == "" {
		// What the screen saw is no change: a task of no row taken off its parent, and nothing else, is not found still
		var nothing TaskEdit
		ed.Parent, ed.Rev, ed.ParentNew, ed.ParentTitle = nil, nil, nil, nil
		if _, had := rows[name]; !had && ed == nothing {
			return ErrNotFound
		}
		return nil
	}
	// A parent made (ParentNew true) has no row yet, nor a parent of its own
	if ed.ParentNew == nil || !*ed.ParentNew {
		up, ok := rows[parent]
		if !ok {
			return usererr.New("store_parent_none", parent, project)
		}
		if up.Parent != "" {
			return usererr.New("store_parent_nested", parent, up.Parent)
		}
	}
	if children := childrenOfTask(rows, name); len(children) > 0 {
		return usererr.New("store_parent_kids", name, children[0])
	}
	return nil
}

// checkThere refuses a write that takes the task name to be new while it is there, or there while it is not, as a
// screen read before it says: there is on the task list, of a row or of a page on it, an archived one too, as the
// management page's tasksOf has it. The pages are read only for a name of no row. The pages are written under the same
// lock.
// LIMIT: a new name, of no row, reads every page's meta.json under the lock. Fine at hundreds of pages; at thousands,
// keep an index of the pages' tasks.
func (s *Store) checkThere(project string, rows map[string]Task, name string, taken *bool) error {
	if taken == nil {
		return nil
	}
	there, err := s.taskThere(project, rows, name)
	if err != nil {
		return err
	}
	switch {
	case *taken && there:
		return conflict(usererr.New("store_task_there", name))
	case !*taken && !there:
		return conflict(usererr.New("store_task_gone", name))
	}
	return nil
}

// taskThere reports whether the task name is there, rows being its project's: of a row, or of pages alone, as the task
// list shows it.
func (s *Store) taskThere(project string, rows map[string]Task, name string) (bool, error) {
	if _, ok := rows[name]; ok {
		return true, nil
	}
	count, _, err := s.pagesOfTask(project, name)
	return count > 0, err
}

// checkSeen refuses, before checkParent, what ed says the screen saw that is no longer so: the task's row written since
// (Rev), whatever it wrote, and a parent named there or new (ParentNew). A parent of itself is refused first, which
// would else be said to be there; one taken to be there of no row next, as it can be no parent, before its pages are
// read.
// LIMIT: a write of another field, or of the state the row is in already, is refused as well: the screen reads the
// tasks again. Compare by field if refusals come to be many
func (s *Store) checkSeen(project string, rows map[string]Task, name string, ed TaskEdit) error {
	if ed.Rev != nil && *ed.Rev != rows[name].Rev {
		return conflict(usererr.New("store_task_was", name))
	}
	if ed.Parent == nil {
		return nil
	}
	parent := *ed.Parent
	if parent == "" {
		return nil
	}
	// LIMIT: a parent named that is there is taken as it is now, its rev not compared: only the task's own row is what the
	// screen says it read
	if parent == name {
		return usererr.New("store_parent_self", name)
	}
	// LIMIT: a parent taken to be there and gone meanwhile is refused as no parent (400), not as stale: the screen
	// does not read the tasks again. Tell it apart by its pages, read under the lock, if that comes to mislead
	if _, ok := rows[parent]; !ok && ed.ParentNew != nil && !*ed.ParentNew {
		return usererr.New("store_parent_none", parent, project)
	}
	return s.checkThere(project, rows, parent, ed.ParentNew)
}

// newTaskRow is the row a task starts with: of the state New, from now.
func newTaskRow() Task {
	fresh := States[slices.IndexFunc(States, func(s State) bool { return s.New })]
	return Task{Title: "", State: fresh.Name, Step: "", At: Now(), URL: "", Steps: nil, Parent: "", Rev: 0}
}

// stepsOf is steps as a task keeps them: each trimmed, without the empty and a name twice, in the order given; nil for
// none.
func stepsOf(steps []string) []string {
	var kept []string
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step != "" && !slices.Contains(kept, step) {
			kept = append(kept, step)
		}
	}
	return kept
}

// pastedURL is a URL with the spaces round it gone, as one pasted has them. Of spaces alone it stays, as checkTaskEdit
// refuses it: "" is what takes a URL off.
func pastedURL(raw *string) *string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return raw
	}
	trimmed := strings.TrimSpace(*raw)
	return &trimmed
}

// TaskWritten is a task's row as EditTask left it, under the project (ProjectKey) and name tasks.json keys it by,
// which a screen puts in place of the row it read. Made is whether the task was not there before this write (no row
// and no page, as checkThere has it), and Wrote
// whether tasks.json was written: not when the row came out as it was, unless a parent was made with it.
type TaskWritten struct {
	Project string `json:"project"`
	Task    string `json:"task"`
	Row     Task   `json:"row"`
	Made    bool   `json:"made"`
	Wrote   bool   `json:"wrote"`
}

// EditTask applies ed to the row of the task (project, name), making it of the state of a new one when there is none,
// but for checkParent's ErrNotFound, and returns the row as it left it: a write that changes nothing writes nothing.
// LIMIT: a row written by hand keeps its Rev, and one forgotten and made again starts it again: a screen that read it
// before may write over it. Count them apart if hand edits come to be done while screens are open
func (s *Store) EditTask(project, name string, ed TaskEdit) (TaskWritten, error) {
	project, name, err := taskKey(project, name)
	if err != nil {
		return TaskWritten{}, err
	}
	ed.URL = pastedURL(ed.URL)
	if err = checkTaskEdit(ed); err != nil {
		return TaskWritten{}, err
	}
	if ed.Parent != nil {
		var parent string
		if parent, err = TaskName(*ed.Parent); err != nil {
			return TaskWritten{}, err
		}
		ed.Parent = &parent
	}
	unlock, err := s.lock(true)
	if err != nil {
		return TaskWritten{}, err
	}
	defer unlock()
	all, err := s.ReadTasks()
	if err != nil {
		return TaskWritten{}, err
	}
	// Made first: a parent made goes in it before checkParent, though the project has no row yet. Refused, none is written
	if all[project] == nil {
		all[project] = map[string]Task{}
	}
	rows := all[project]
	if err = s.checkWrite(project, rows, name, ed); err != nil {
		return TaskWritten{}, err
	}
	return s.writeRow(all, project, name, ed)
}

// writeRow writes ed, checked, to the task name of the project in all: a parent made with it, of rev 1, and the row one
// rev more. A write that changes nothing writes nothing, so that the writes of every step (floe work) do not refuse a
// screen.
func (s *Store) writeRow(all Tasks, project, name string, ed TaskEdit) (TaskWritten, error) {
	rows := all[project]
	parentMade := ed.ParentNew != nil && *ed.ParentNew
	if parentMade {
		up := newTaskRow()
		up.Title = strings.TrimSpace(*ed.ParentTitle)
		up.Rev = 1
		rows[*ed.Parent] = up
	}
	was, had := rows[name]
	if !had {
		was = newTaskRow()
	}
	t := was.edited(ed)
	if had && !parentMade && reflect.DeepEqual(t, was) {
		return TaskWritten{Project: project, Task: name, Row: was, Made: false, Wrote: false}, nil
	}
	// Made is of the task, not the row: one of pages alone was there, as the task list shows it.
	there, err := s.taskThere(project, rows, name)
	if err != nil {
		return TaskWritten{}, err
	}
	t.Rev++
	rows[name] = t
	if err := s.writeTasks(all); err != nil {
		return TaskWritten{}, err
	}
	return TaskWritten{Project: project, Task: name, Row: t, Made: !there, Wrote: true}, nil
}

// checkWrite refuses ed of the task name, rows being the project's now: whether it is there as said, what the screen
// saw, and the parent.
func (s *Store) checkWrite(project string, rows map[string]Task, name string, ed TaskEdit) error {
	if err := s.checkThere(project, rows, name, ed.New); err != nil {
		return err
	}
	if err := s.checkSeen(project, rows, name, ed); err != nil {
		return err
	}
	return checkParent(project, rows, name, ed)
}

// edited is t as ed changes it, which has been checked.
func (t Task) edited(ed TaskEdit) Task {
	if ed.State != nil && *ed.State != t.State {
		t.State, t.At = *ed.State, Now()
	}
	if ed.Title != nil {
		t.Title = strings.TrimSpace(*ed.Title)
	}
	if ed.Step != nil && strings.TrimSpace(*ed.Step) != "" {
		t.Step = strings.TrimSpace(*ed.Step)
	}
	if ed.URL != nil {
		t.URL = *ed.URL
	}
	if ed.Steps != nil {
		t.Steps = stepsOf(*ed.Steps)
	}
	if ed.Parent != nil {
		t.Parent = *ed.Parent
	}
	return t
}

// pagesOfTask counts the pages on the task (project, name), the archived too, and names one of them.
func (s *Store) pagesOfTask(project, name string) (count int, first string, err error) {
	slugs, err := s.slugs()
	if err != nil {
		return 0, "", err
	}
	for _, slug := range slugs {
		m := s.ReadMeta(slug)
		if m != nil && m.Work != nil && m.Work.Task == name && ProjectKey(m.Project) == project {
			if count == 0 {
				first = slug
			}
			count++
		}
	}
	return count, first, nil
}

// ForgetTask removes the row of the task (project, name): ErrNotFound for none, and refused while a page is on it or it
// has a child. It is the one place that says what keeps a task from being forgotten.
func (s *Store) ForgetTask(project, name string) error {
	project, name, err := taskKey(project, name)
	if err != nil {
		return err
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.ReadTasks()
	if err != nil {
		return err
	}
	if _, ok := all[project][name]; !ok {
		return ErrNotFound
	}
	count, first, err := s.pagesOfTask(project, name)
	if err != nil {
		return err
	}
	var kept usererr.List
	if count > 0 {
		kept = append(kept, usererr.New("store_task_pages", name, project, count, first))
	}
	if children := childrenOfTask(all[project], name); len(children) > 0 {
		kept = append(kept, usererr.New("store_task_children", name, project, len(children), children[0]))
	}
	if err = kept.Sorted().Err(); err != nil {
		return err
	}
	delete(all[project], name)
	if len(all[project]) == 0 {
		delete(all, project)
	}
	return s.writeTasks(all)
}
