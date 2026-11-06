package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/usererr"
)

// workName is the file of the tasks in the store, the one thing that holds a task's state: its pages name it, and it
// names no page.
const workName = "work.json"

// noProject is the project of a task whose pages have none, in work.json, the API and the management page's URLs.
const noProject = "/"

// ProjectKey is the project of a page as work.json has it: trimmed, as a directory's name may not be. It is the one
// place that says what a page's project is there: a page's list entry carries it (Listed.TaskProject) for what cannot
// call this.
func ProjectKey(project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return noProject
	}
	return project
}

// PageItem is the name of a page's task as written: trimmed, "" taking the page off its task. A name of spaces alone is
// no way to say that, and is refused.
func PageItem(item string) (string, error) {
	trimmed := strings.TrimSpace(item)
	if item != "" && trimmed == "" {
		return "", usererr.New("store_task_item")
	}
	if !utf8.ValidString(trimmed) {
		return "", usererr.New("store_task_utf8")
	}
	return trimmed, nil
}

// Work is what a page holds of its place on the dev screen: the task it is on, its step there, whether it is pinned.
type Work struct {
	Item string `json:"item,omitempty"`
	Step string `json:"step,omitempty"`
	Pin  bool   `json:"pin,omitempty"`
}

// WorkEdit is what an edit changes of a page's Work, each left as it is when nil.
type WorkEdit struct {
	Item *string `json:"item,omitzero"`
	Step *string `json:"step,omitzero"`
	Pin  *bool   `json:"pin,omitzero"`
}

// setWork applies ed to m. A step is of its task: a page that moves to another task leaves it, unless ed gives the
// step it takes there; a page off a task has none, and one neither on a task nor pinned has no Work. PageItem has
// checked ed.
func setWork(m *Meta, ed *WorkEdit) {
	var w Work
	if m.Work != nil {
		w = *m.Work
	}
	if ed.Item != nil {
		item := strings.TrimSpace(*ed.Item)
		if item != w.Item {
			w.Step = ""
		}
		w.Item = item
	}
	if ed.Step != nil {
		w.Step = strings.TrimSpace(*ed.Step)
	}
	if ed.Pin != nil {
		w.Pin = *ed.Pin
	}
	if w.Item == "" {
		w.Step = ""
	}
	m.Work = &w
	if w.Item == "" && !w.Pin {
		m.Work = nil
	}
}

// Task is a task's row of work.json. At is when it was made or its state last changed; Step the last step written of
// it that was not empty. Parent is read and written back alone: nothing sets it yet.
type Task struct {
	Title  string   `json:"title,omitempty"`
	State  string   `json:"state"`
	Step   string   `json:"step,omitempty"`
	At     string   `json:"at"`
	URL    string   `json:"url,omitempty"`
	Steps  []string `json:"steps,omitempty"`
	Parent string   `json:"parent,omitempty"`
}

// Tasks is work.json: the tasks by project (ProjectKey), then by name.
type Tasks map[string]map[string]Task

// State is a state a task can be in: whether the list of it shows the longest in it first, and whether a task new or
// of no row is in it.
type State struct {
	Name   string `json:"name"`
	Oldest bool   `json:"oldest"`
	New    bool   `json:"new"`
}

// States is the one table of the states of a task: what is written is checked by it, and the management page reads it.
var States = []State{
	{Name: "waiting", Oldest: true, New: false},
	{Name: "doing", Oldest: false, New: true},
	{Name: "done", Oldest: false, New: false},
}

// TaskEdit is what a write changes of a task's row, each left as it is when nil. An empty URL or Steps takes it off; an
// empty Step leaves the last one written.
type TaskEdit struct {
	Title *string
	State *string
	Step  *string
	URL   *string
	Steps *[]string
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

func (s *Store) workFile() string {
	return filepath.Join(s.root, workName)
}

// ReadWork reads work.json, none being no tasks. What cannot be read, or holds what no write may (checkTaskEdit), is
// refused and not made anew as reads.json is: the states and URLs in it are not to be had again, so that it is mended
// by hand.
// It takes no lock: a write replaces the file whole.
// LIMIT: a key of a row floe does not know is lost at the next write. Keep them if work.json comes to be hand-edited.
func (s *Store) ReadWork() (Tasks, error) {
	b, _, err := files.Read(s.workFile())
	if errors.Is(err, fs.ErrNotExist) {
		return Tasks{}, nil
	}
	if err != nil {
		return nil, usererr.CannotRead(s.workFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	broken := func(format string, args ...any) error {
		return usererr.NewAt(s.workFile(), 0, 0, "store_work_broken", fmt.Sprintf(format, args...))
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
		for item, t := range rows {
			if !validState(t.State) {
				return nil, broken("task %s of project %s has the state %q", item, project, t.State)
			}
			if t.URL != "" && !validURL(t.URL) {
				return nil, broken("task %s of project %s has the URL %q, not http or https", item, project, t.URL)
			}
		}
	}
	return all, nil
}

func (s *Store) writeWork(all Tasks) error {
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", workName, err)
	}
	if err := files.WriteAtomic(s.workFile(), append(b, '\n'), 0o600, nil); err != nil {
		return usererr.CannotWrite(s.workFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	return nil
}

// taskKey is the project and name that name a task, each as a page's is written. An empty project is refused, not read
// as no project's: that is "/", and an empty one is a query left out. A byte that is no UTF-8, which only a query can
// bring, would come to be another name as JSON writes it.
func taskKey(project, item string) (key, name string, err error) {
	if strings.TrimSpace(project) == "" {
		return "", "", usererr.New("store_task_project")
	}
	if !utf8.ValidString(project) {
		return "", "", usererr.New("store_task_utf8")
	}
	if name, err = PageItem(item); err != nil {
		return "", "", err
	}
	if name == "" {
		return "", "", usererr.New("store_task_item")
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
	return nil
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

// EditTask applies ed to the row of the task (project, item), making it of the state doing when there is none.
func (s *Store) EditTask(project, item string, ed TaskEdit) error {
	project, item, err := taskKey(project, item)
	if err != nil {
		return err
	}
	if err = checkTaskEdit(ed); err != nil {
		return err
	}
	unlock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.ReadWork()
	if err != nil {
		return err
	}
	if all[project] == nil {
		all[project] = map[string]Task{}
	}
	t, had := all[project][item]
	if !had {
		fresh := States[slices.IndexFunc(States, func(s State) bool { return s.New })]
		t = Task{Title: "", State: fresh.Name, Step: "", At: Now(), URL: "", Steps: nil, Parent: ""}
	}
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
	all[project][item] = t
	return s.writeWork(all)
}

// pagesOfTask counts the pages on the task (project, item), the archived too, and names one of them. It is the one
// place that says what keeps a task from being forgotten.
func (s *Store) pagesOfTask(project, item string) (count int, first string, err error) {
	slugs, err := s.slugs()
	if err != nil {
		return 0, "", err
	}
	for _, slug := range slugs {
		m := s.ReadMeta(slug)
		if m != nil && m.Work != nil && m.Work.Item == item && ProjectKey(m.Project) == project {
			if count == 0 {
				first = slug
			}
			count++
		}
	}
	return count, first, nil
}

// ForgetTask removes the row of the task (project, item): ErrNotFound for none, and refused while a page is on it.
func (s *Store) ForgetTask(project, item string) error {
	project, item, err := taskKey(project, item)
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
	all, err := s.ReadWork()
	if err != nil {
		return err
	}
	if _, ok := all[project][item]; !ok {
		return ErrNotFound
	}
	count, first, err := s.pagesOfTask(project, item)
	if err != nil {
		return err
	}
	if count > 0 {
		return usererr.New("store_task_pages", item, project, count, first)
	}
	delete(all[project], item)
	if len(all[project]) == 0 {
		delete(all, project)
	}
	return s.writeWork(all)
}
