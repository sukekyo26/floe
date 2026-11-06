package cli

import (
	"context"
	"errors"
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
the first failure. The same command given again finishes what a failure left.

  floe work design-notes --task "#130" --step design --state waiting
  floe work --task "#130" --url https://example.com/issues/482
  floe work --task "#130" --steps design,build,review
  floe work --task "#130" --parent "#123"
  floe work design-notes --pin
  floe work --task "#130" --forget`,
		// Asked here, as the flags are read by now: cobra's own asking comes after, unmarked as a usage error.
		Args: usage(func(cmd *cobra.Command, _ []string) error { return cmd.ValidateFlagGroups() }),
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
	flags.StringVar(&f.task, "task", "", `the task's name; with slugs, "" takes the pages off their task`)
	flags.StringVar(&f.project, "project", "", `the task's project, without slugs; "" for none`)
	flags.StringVar(&f.step, "step", "", `the pages' step on the task; "" takes it off them`)
	flags.StringVar(&f.state, "state", "", "the task's state: "+store.StateNames())
	flags.StringVar(&f.url, "url", "", `the task's one outside link, http or https; "" takes it off`)
	flags.StringVar(&f.title, "title", "", `the task's title; "" takes it off`)
	flags.StringVar(&f.steps, "steps", "", `the order of the task's steps, separated by commas; "" takes it off`)
	flags.StringVar(&f.parent, "parent", "", `the task's parent, of its project, without slugs; "" takes it off`)
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
	t := a.namedTask(ctx, cmd, f)
	if !f.forget {
		return c.do(ctx, http.MethodPatch, taskQuery(t), taskEdit(cmd, f), nil)
	}
	err = c.do(ctx, http.MethodDelete, taskQuery(t), nil, nil)
	if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusNotFound {
		return usererr.New("store_no_task", t.name, t.project)
	}
	return err
}

// namedTask is the task of --task and --project, whose project is the working directory's when left out.
func (a *app) namedTask(ctx context.Context, cmd *cobra.Command, f workFlags) taskOf {
	name := f.project
	if !cmd.Flags().Changed("project") {
		name = project.Name(ctx, a.env.Dir)
	}
	return taskOf{project: store.ProjectKey(name), name: f.task}
}

// workPages puts the pages slugs on a task and changes it: all checked before anything is written, so that a page of
// no task, or none at all, leaves the rest as they were. Flags of the pages alone ask nothing of tasks.json: an empty
// write would make a row for a task that has none.
func (a *app) workPages(cmd *cobra.Command, f workFlags, slugs []string) error {
	ctx := cmd.Context()
	if off := changed(cmd, "forget", "project", "parent"); len(off) > 0 {
		return usageOf(usererr.New("cli_work_no_pages", off[0]))
	}
	onPages := changed(cmd, "task", "step", "pin", "unpin", "clear")
	onTasks := changed(cmd, "state", "url", "title", "steps")
	if len(onPages)+len(onTasks) == 0 {
		return usageOf(usererr.New("cli_work_nothing"))
	}
	named := cmd.Flags().Changed("task")
	name, err := store.TaskName(f.task)
	if err != nil {
		return err //nolint:wrapcheck // the store's words
	}
	onTask := onTasks
	if f.step != "" {
		onTask = append(onTask, "step")
	}
	if named && name == "" && len(onTask) > 0 {
		return usageOf(usererr.New("cli_work_task_off", onTask[0]))
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
	for _, t := range tasks {
		if err = c.do(ctx, http.MethodPatch, taskQuery(t), taskEdit(cmd, f), nil); err != nil {
			return err
		}
	}
	if len(onPages) == 0 {
		return nil
	}
	return a.eachPage(cmd, slugs, http.MethodPatch, pageWorkEdit(cmd, f))
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
