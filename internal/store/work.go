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

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/usererr"
)

// workName is the file of the tasks in the store, the one thing that holds a task's state: its pages name it, and it
// names no page.
const workName = "work.json"

// noProject is the project of a task whose pages have none, in work.json, the API and the management page's URLs.
const noProject = "/"

// ProjectKey is the project of a page as work.json has it. It is the one place that says what a page of no project is
// there: a page's list entry carries it (Listed.TaskProject) for what cannot call this.
func ProjectKey(project string) string {
	if project == "" {
		return noProject
	}
	return project
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

// setWork applies ed to m. A page off a task has no step of it, and one neither on a task nor pinned has no Work.
func setWork(m *Meta, ed *WorkEdit) {
	var w Work
	if m.Work != nil {
		w = *m.Work
	}
	if ed.Item != nil {
		w.Item = strings.TrimSpace(*ed.Item)
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

// State is a state a task can be in, and whether the list of it shows the longest in it first.
type State struct {
	Name   string `json:"name"`
	Oldest bool   `json:"oldest"`
}

// States is the one table of the states of a task: what is written is checked by it, and the management page reads it.
var States = []State{{Name: "waiting", Oldest: true}, {Name: "doing", Oldest: false}, {Name: "done", Oldest: false}}

const newTaskState = "doing"

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

func stateNames() string {
	names := make([]string, len(States))
	for i, s := range States {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

func (s *Store) workFile() string {
	return filepath.Join(s.root, workName)
}

// ReadWork reads work.json, none being no tasks. What cannot be read, or holds a state of no State, is refused and not
// made anew as reads.json is: the states and URLs in it are not to be had again, so that it is mended by hand.
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
	all := Tasks{}
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, usererr.NewAt(s.workFile(), 0, 0, "store_work_broken", err.Error())
	}
	for project, rows := range all {
		for item, t := range rows {
			if !validState(t.State) {
				reason := fmt.Sprintf("task %s of project %s has the state %q", item, project, t.State)
				return nil, usererr.NewAt(s.workFile(), 0, 0, "store_work_broken", reason)
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

// taskKey is the project and name that name a task, as Edit reads a project: each trimmed. An empty project is no
// project's: that is "/", which would otherwise make a task no page's project can reach.
func taskKey(project, item string) (trimmedProject, trimmedItem string, err error) {
	project, item = strings.TrimSpace(project), strings.TrimSpace(item)
	if project == "" {
		return "", "", usererr.New("store_task_project")
	}
	if item == "" {
		return "", "", usererr.New("store_task_item")
	}
	return project, item, nil
}

// checkTaskEdit refuses what no row may hold. A URL goes to the browser as a link's href: http and https alone.
func checkTaskEdit(ed TaskEdit) error {
	if ed.State != nil && !validState(*ed.State) {
		return usererr.New("store_task_state", *ed.State, stateNames())
	}
	if ed.URL != nil && *ed.URL != "" {
		u, err := url.Parse(*ed.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return usererr.New("store_task_url", *ed.URL)
		}
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
		t = Task{Title: "", State: newTaskState, Step: "", At: Now(), URL: "", Steps: nil, Parent: ""}
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
