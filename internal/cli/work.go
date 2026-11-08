package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/project"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

// workFlags are the flags of floe work.
type workFlags struct {
	task, project, step, state, url, title, steps, parent string
	pin, unpin, clear, forget                             bool
}

// A task of pages: what a page names by its project and the name given, or its own.
type taskOf struct {
	project, name string
}

func (a *app) workCmd() *cobra.Command {
	var f workFlags
	cmd := &cobra.Command{
		Use:   "work [<slug>...]",
		Short: "Put pages on a task of the task list, pin them, or change a task's state, step, URL, title or steps' order",
		Long: `With slugs, put the pages on a task and change that task; without, change the task --task names, of
--project (the project of the working directory when left out; "" for none).

The tasks are written first, once for each different task the pages are on, and the pages then, in order, stopping at
the first failure. The same command given again finishes what a failure left. Each task and page written is told on a
line of its own, as the store left it: a task made, changed or unchanged with the fields given, a page with its task,
step and pin.

  floe work design-notes --task "#130" --step design --state waiting
    task #130 in project floe: changed; state waiting, step "design"
    page design-notes: task #130 in project floe, step "design"
  floe work --task "#130" --url https://example.com/issues/482
  floe work --task "#130" --steps design,build,review
  floe work --task "#130" --parent "#123"
  floe work design-notes --task "#131" --parent "#130" --step design
  floe work design-notes --pin
  floe work --task "#130" --forget`,
		PreRunE: flagGroups,
		RunE: func(cmd *cobra.Command, slugs []string) error {
			// A step of spaces is the empty one from here on, as the server reads it.
			f.step = strings.TrimSpace(f.step)
			if len(slugs) == 0 {
				return a.workTask(cmd, f)
			}
			return a.workPages(cmd, f, slugs)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&f.task, "task", "", `the task's ID; with slugs, "" takes the pages off their task`)
	flags.StringVar(&f.project, "project", "", `the task's project, without slugs; "" for none`)
	flags.StringVar(&f.step, "step", "", `the pages' step on the task; "" takes it off them`)
	flags.StringVar(&f.state, "state", "", "the task's state: "+store.StateNames())
	flags.StringVar(&f.url, "url", "", `the task's one outside link, http or https; "" takes it off`)
	flags.StringVar(&f.title, "title", "", `the task's title; "" takes it off`)
	flags.StringVar(&f.steps, "steps", "", `the order of the task's steps, separated by commas; "" takes it off`)
	flags.StringVar(&f.parent, "parent", "", `the task's parent, of its project; with slugs, with --task; "" takes it off`)
	flags.BoolVar(&f.pin, "pin", false, "pin the pages to the top of the task list")
	flags.BoolVar(&f.unpin, "unpin", false, "take the pages' pin off")
	flags.BoolVar(&f.clear, "clear", false, "take the pages off their task and take their pin off; the task stays")
	flags.BoolVar(&f.forget, "forget", false, "remove the task, which no page or child task may be on")
	cmd.MarkFlagsMutuallyExclusive("pin", "unpin")
	for _, other := range []string{"task", "step", "state", "url", "title", "steps", "pin", "unpin"} {
		cmd.MarkFlagsMutuallyExclusive("clear", other)
	}
	cmd.MarkFlagsMutuallyExclusive("clear", "parent")
	for _, other := range []string{"state", "url", "title", "steps", "parent"} {
		cmd.MarkFlagsMutuallyExclusive("forget", other)
	}
	return cmd
}

// changed is the flags of names that were given.
func changed(cmd *cobra.Command, names ...string) []string {
	return slices.DeleteFunc(names, func(name string) bool { return !cmd.Flags().Changed(name) })
}

// taskEdit is what the flags change of a task: an empty --step, which changes the pages alone, leaves it out.
func taskEdit(cmd *cobra.Command, f workFlags) server.TaskEdit {
	var ed server.TaskEdit
	if cmd.Flags().Changed("title") {
		ed.Title = &f.title
	}
	if cmd.Flags().Changed("state") {
		ed.State = &f.state
	}
	if f.step != "" {
		ed.Step = &f.step
	}
	if cmd.Flags().Changed("url") {
		ed.URL = &f.url
	}
	if cmd.Flags().Changed("steps") {
		// LIMIT: a step's name with a comma cannot be written here, and comes after the steps written. Take a list of the
		// flag if such names come to matter.
		steps := strings.Split(f.steps, ",")
		ed.Steps = &steps
	}
	if cmd.Flags().Changed("parent") {
		ed.Parent = &f.parent
	}
	return ed
}

func taskQuery(t taskOf) string {
	return "/api/tasks?" + url.Values{"project": {t.project}, "task": {t.name}}.Encode()
}

// workTask changes the task --task names, or forgets it.
func (a *app) workTask(cmd *cobra.Command, f workFlags) error {
	ctx := cmd.Context()
	if !cmd.Flags().Changed("task") {
		return usageOf(usererr.New("cli_work_no_task"))
	}
	if onPages := changed(cmd, "step", "pin", "unpin", "clear"); len(onPages) > 0 {
		return usageOf(usererr.New("cli_work_needs_pages", onPages[0]))
	}
	c, err := a.client(ctx, true)
	if err != nil {
		return err
	}
	t, err := a.namedTask(ctx, cmd, f)
	if err != nil {
		return err
	}
	ed := taskEdit(cmd, f)
	var written store.TaskWritten
	if f.forget {
		err = c.do(ctx, http.MethodDelete, taskQuery(t), nil, nil)
	} else {
		err = c.do(ctx, http.MethodPatch, taskQuery(t), ed, &written)
	}
	if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusNotFound {
		return usererr.New("cli_work_no_row", t.name, t.project)
	}
	if err != nil {
		return err
	}
	if f.forget {
		a.print("%s", english.Msg("cli_work_forgotten", t.name, projectOf(t.project)))
		return nil
	}
	a.tellTask(written, ed)
	return nil
}

// tellTask says the task written: made, changed or as it was, and the fields ed asked for as the store left them.
func (a *app) tellTask(w store.TaskWritten, ed server.TaskEdit) {
	key := "cli_work_unchanged"
	switch {
	case w.Made:
		key = "cli_work_made"
	case w.Wrote:
		key = "cli_work_changed"
	}
	line := english.Msg(key, w.Task, projectOf(w.Project))
	if fields := taskFields(w.Row, ed); fields != "" {
		line += "; " + fields
	}
	a.print("%s", line)
}

// taskFields is the fields of row that ed asked for, in one order, the text ones quoted.
func taskFields(row store.Task, ed server.TaskEdit) string {
	var fields []string
	if ed.Title != nil {
		fields = append(fields, fmt.Sprintf("title %q", row.Title))
	}
	if ed.State != nil {
		fields = append(fields, "state "+row.State)
	}
	if ed.Step != nil {
		fields = append(fields, fmt.Sprintf("step %q", row.Step))
	}
	if ed.URL != nil {
		fields = append(fields, fmt.Sprintf("url %q", row.URL))
	}
	if ed.Steps != nil {
		fields = append(fields, fmt.Sprintf("steps %q", strings.Join(row.Steps, ",")))
	}
	if ed.Parent != nil {
		parent := "no parent"
		if row.Parent != "" {
			parent = fmt.Sprintf("parent %q", row.Parent)
		}
		fields = append(fields, parent)
	}
	return strings.Join(fields, ", ")
}

// projectOf is a project of tasks.json (ProjectKey) as floe work says it.
func projectOf(key string) string {
	if key == store.ProjectKey("") {
		return english.Msg("cli_work_no_project")
	}
	return english.Msg("cli_work_project", key)
}

// tellPage says the page as it was written: its task and project, step and pin.
func (a *app) tellPage(page store.Edited) {
	work := page.Work
	var parts []string
	if work == nil || work.Task == "" {
		parts = append(parts, english.Msg("cli_work_page_off"))
	} else {
		parts = append(parts, english.Msg("cli_work_page_task", work.Task, projectOf(page.TaskProject)))
	}
	if work != nil && work.Step != "" {
		parts = append(parts, fmt.Sprintf("step %q", work.Step))
	}
	if work != nil && work.Pin {
		parts = append(parts, "pinned")
	}
	a.print("%s", english.Msg("cli_work_page", page.Slug, strings.Join(parts, ", ")))
}

// namedTask is the task of --task and --project, whose project is the working directory's when left out, its name
// as the store keeps it.
func (a *app) namedTask(ctx context.Context, cmd *cobra.Command, f workFlags) (taskOf, error) {
	name, err := store.TaskName(f.task)
	if err != nil {
		return taskOf{}, err //nolint:wrapcheck // the store's words
	}
	projectName := f.project
	if !cmd.Flags().Changed("project") {
		projectName = project.Name(ctx, a.env.Dir)
	}
	return taskOf{project: store.ProjectKey(projectName), name: name}, nil
}

// workPages puts the pages slugs on a task and changes it: what the CLI can check (the pages listed, on a task, of one
// project for a parent) is checked before anything is written, so that a page of no task, or none at all, leaves the
// rest as they were; the store checks a parent as it writes the task. Flags of the pages alone ask nothing of
// tasks.json: an empty write would make a row for a task that has none. A parent goes with --task alone, on the one
// task the pages go on: on each page's own, a page of the parent itself would stop the writes halfway.
func (a *app) workPages(cmd *cobra.Command, f workFlags, slugs []string) error {
	ctx := cmd.Context()
	named := cmd.Flags().Changed("task")
	onPages, onTask, name, err := pageFlags(cmd, f)
	if err != nil {
		return err
	}
	c, err := a.client(ctx, true)
	if err != nil {
		return err
	}
	var list []store.Listed
	if err = c.do(ctx, http.MethodGet, "/api/artifacts", nil, &list); err != nil {
		return err
	}
	tasks, err := tasksOfPages(list, slugs, name, named, len(onTask) > 0)
	if err != nil {
		return err
	}
	if cmd.Flags().Changed("parent") && len(tasks) > 1 {
		// --task names one task, so that tasks differ by their projects alone
		return usageOf(usererr.New("cli_work_parent_projects"))
	}
	if err = a.writeTasks(ctx, c, tasks, taskEdit(cmd, f)); err != nil {
		return err
	}
	if len(onPages) == 0 {
		return nil
	}
	return a.eachPage(cmd, slugs, http.MethodPatch, pageWorkEdit(cmd, f), a.tellPage)
}

// pageFlags refuses the flags of a floe work with slugs that cannot go together, and returns those that change the
// pages, those that change their task, and the task's name --task gives.
func pageFlags(cmd *cobra.Command, f workFlags) (onPages, onTask []string, name string, err error) {
	if off := changed(cmd, "forget", "project"); len(off) > 0 {
		return nil, nil, "", usageOf(usererr.New("cli_work_no_pages", off[0]))
	}
	named := cmd.Flags().Changed("task")
	if cmd.Flags().Changed("parent") && !named {
		return nil, nil, "", usageOf(usererr.New("cli_work_parent_task"))
	}
	onPages = changed(cmd, "task", "step", "pin", "unpin", "clear")
	onTask = changed(cmd, "state", "url", "title", "steps", "parent")
	if len(onPages)+len(onTask) == 0 {
		return nil, nil, "", usageOf(usererr.New("cli_work_nothing"))
	}
	if name, err = store.TaskName(f.task); err != nil {
		return nil, nil, "", err //nolint:wrapcheck // the store's words
	}
	if f.step != "" {
		onTask = append(onTask, "step")
	}
	if named && name == "" && len(onTask) > 0 {
		return nil, nil, "", usageOf(usererr.New("cli_work_task_off", onTask[0]))
	}
	return onPages, onTask, name, nil
}

// writeTasks writes ed to each of tasks in turn, stopping at the first failure. One case of not found is passed: ed
// taking the parent off alone, of a task of no row, which has no parent to take off and is made by the pages going on.
func (a *app) writeTasks(ctx context.Context, c client, tasks []taskOf, ed server.TaskEdit) error {
	var none server.TaskEdit
	rest := ed
	rest.Parent = nil
	parentOff := ed.Parent != nil && *ed.Parent == "" && rest == none
	for _, t := range tasks {
		var written store.TaskWritten
		err := c.do(ctx, http.MethodPatch, taskQuery(t), ed, &written)
		if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusNotFound && parentOff {
			continue
		}
		if err != nil {
			return err
		}
		a.tellTask(written, ed)
	}
	return nil
}

// tasksOfPages is the different tasks the pages slugs are on, the first of each first: of the name given, or each
// page's own. Each of slugs must be listed, and be on a task where the flags change one (wanted).
func tasksOfPages(list []store.Listed, slugs []string, named string, isNamed, wanted bool) ([]taskOf, error) {
	var tasks []taskOf
	for _, slug := range slugs {
		at := slices.IndexFunc(list, func(l store.Listed) bool { return l.Slug == slug })
		if at < 0 {
			return nil, usererr.New("store_no_page", slug)
		}
		page := list[at]
		name := ""
		if page.Work != nil {
			name = page.Work.Task
		}
		if isNamed {
			name = named
		}
		if name == "" {
			if wanted {
				return nil, usererr.New("cli_work_page_no_task", slug)
			}
			continue
		}
		if !isNamed && !wanted {
			continue
		}
		if t := (taskOf{project: page.TaskProject, name: name}); !slices.Contains(tasks, t) {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

// pageWorkEdit is the edit of the flags that change a page. --clear takes it all off.
func pageWorkEdit(cmd *cobra.Command, f workFlags) server.Edit {
	var work store.WorkEdit
	if f.clear {
		off := false
		empty := ""
		work.Task, work.Step, work.Pin = &empty, &empty, &off
	}
	if cmd.Flags().Changed("task") {
		work.Task = &f.task
	}
	if cmd.Flags().Changed("step") {
		work.Step = &f.step
	}
	if cmd.Flags().Changed("pin") {
		work.Pin = &f.pin
	}
	if cmd.Flags().Changed("unpin") {
		off := !f.unpin
		work.Pin = &off
	}
	return server.Edit{
		Title: nil, Description: nil, Project: nil, Slug: nil, Favorite: nil, Archived: nil, Tags: nil, Work: &work,
	}
}
