package cli

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"floe/internal/project"
	"floe/internal/server"
	"floe/internal/store"
	"floe/internal/usererr"
)

// workFlags are the flags of floe work.
type workFlags struct {
	task, project, category, state, url, title, categories, parent string
	pin, unpin, clear, forget                                      bool
}

// A task of pages: what a page names by its project and the name given, or its own.
type taskOf struct {
	project, name string
}

func (a *app) workCmd() *cobra.Command {
	var f workFlags
	cmd := &cobra.Command{
		Use: "work [<slug>...]",
		Short: "Tell pages' task, category and pin, or change them, or a task's state, URL, title, parent or " +
			"categories' order",
		Long: `With slugs, put the pages on a task and change that task; without, change the task --task names, of
--project (the project of the working directory when left out; "" for none). With slugs and no flag, tell each page as
it is, writing nothing.

The tasks are written first, once for each different task the pages are on, and the pages then, in order, stopping at
the first failure. The same command given again finishes what a failure left. Each task and page written is told on a
line of its own, as the store left it: a task made, changed or unchanged with the fields given, a page with its task,
category and pin. A page's category is where the task's page places it among its pages, which the user decides.

  floe work design-notes --task "#130"
    task #130 in project floe: made
    page design-notes: task #130 in project floe
  floe work --task "#130" --state doing
    task #130 in project floe: changed; state doing
  floe work --task "#130" --url https://example.com/issues/482
  floe work design-notes --category designs
  floe work design-notes
    page design-notes: task #130 in project floe, category "designs"
  floe work --task "#130" --categories proposals,designs,samples
  floe work --task "#130" --parent "#123"
  floe work design-notes --task "#131" --parent "#130"
  floe work design-notes --pin
  floe work --task "#130" --forget`,
		PreRunE: flagGroups,
		RunE: func(cmd *cobra.Command, slugs []string) error {
			// A category of spaces is the empty one from here on, as the server reads it.
			f.category = strings.TrimSpace(f.category)
			if len(slugs) == 0 {
				return a.workTask(cmd, f)
			}
			return a.workPages(cmd, f, slugs)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&f.task, "task", "", `the task's ID; with slugs, "" takes the pages off their task`)
	flags.StringVar(&f.project, "project", "", `the task's project, without slugs; "" for none`)
	flags.StringVar(&f.category, "category", "", `the pages' category on their task, the user's choice; "" takes it off`)
	flags.StringVar(&f.state, "state", "", "the task's state: "+store.StateNames())
	flags.StringVar(&f.url, "url", "", `the task's one outside link, http or https; "" takes it off`)
	flags.StringVar(&f.title, "title", "", `the task's title; "" takes it off`)
	flags.StringVar(&f.categories, "categories", "", `the task's categories in order, comma-separated; "" takes it off`)
	flags.StringVar(&f.parent, "parent", "", `the task's parent, of its project; with slugs, with --task; "" takes it off`)
	flags.BoolVar(&f.pin, "pin", false, "pin the pages to the top of the task list")
	flags.BoolVar(&f.unpin, "unpin", false, "take the pages' pin off")
	flags.BoolVar(&f.clear, "clear", false, "take the pages off their task and take their pin off; the task stays")
	flags.BoolVar(&f.forget, "forget", false, "remove the task, which no page or child task may be on")
	cmd.MarkFlagsMutuallyExclusive("pin", "unpin")
	for _, other := range []string{"task", "category", "state", "url", "title", "categories", "pin", "unpin"} {
		cmd.MarkFlagsMutuallyExclusive("clear", other)
	}
	cmd.MarkFlagsMutuallyExclusive("clear", "parent")
	for _, other := range []string{"state", "url", "title", "categories", "parent"} {
		cmd.MarkFlagsMutuallyExclusive("forget", other)
	}
	return cmd
}

// changed is the flags of names that were given.
func changed(cmd *cobra.Command, names ...string) []string {
	return slices.DeleteFunc(names, func(name string) bool { return !cmd.Flags().Changed(name) })
}

// taskEdit is what the flags change of a task.
func taskEdit(cmd *cobra.Command, f workFlags) server.TaskEdit {
	var ed server.TaskEdit
	if cmd.Flags().Changed("title") {
		ed.Title = &f.title
	}
	if cmd.Flags().Changed("state") {
		ed.State = &f.state
	}
	if cmd.Flags().Changed("url") {
		ed.URL = &f.url
	}
	if cmd.Flags().Changed("categories") {
		// LIMIT: a category's name with a comma cannot be written here, and comes after the categories written. Take a
		// list of the flag if such names come to matter.
		categories := strings.Split(f.categories, ",")
		ed.Categories = &categories
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
	if onPages := changed(cmd, "category", "pin", "unpin", "clear"); len(onPages) > 0 {
		return usageOf(usererr.New("cli_work_needs_pages", onPages[0]))
	}
	t, err := a.namedTask(ctx, cmd, f)
	if err != nil {
		return err
	}
	ed := taskEdit(cmd, f)
	sent, err := encodeSent(ed)
	if err != nil {
		return err
	}
	c, err := a.client(ctx, true)
	if err != nil {
		return err
	}
	var written store.TaskWritten
	if f.forget {
		err = c.do(ctx, http.MethodDelete, taskQuery(t), nil, nil)
	} else {
		err = c.do(ctx, http.MethodPatch, taskQuery(t), sent, &written)
	}
	if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusNotFound {
		return usererr.New("cli_work_no_row", t.name, t.project)
	}
	if err != nil {
		return err
	}
	if f.forget {
		a.print("%s", english.Msg("cli_work_forgotten", t.name, inProject(t.project)))
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
	line := english.Msg(key, w.Task, inProject(w.Project))
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
	if ed.URL != nil {
		fields = append(fields, fmt.Sprintf("url %q", row.URL))
	}
	if ed.Categories != nil {
		fields = append(fields, fmt.Sprintf("categories %q", strings.Join(row.Categories, ",")))
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

// tellPage says the page as the store has it, written or read: its task and project, category and pin.
func (a *app) tellPage(page store.Edited) {
	work := page.Work
	var parts []string
	if work == nil || work.Task == "" {
		parts = append(parts, english.Msg("cli_work_page_off"))
	} else {
		parts = append(parts, english.Msg("cli_work_page_task", work.Task, inProject(page.TaskProject)))
	}
	if work != nil && work.Category != "" {
		parts = append(parts, fmt.Sprintf("category %q", work.Category))
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
		return taskOf{}, flagOf(err, "--task")
	}
	projectName, from := f.project, "--project"
	if !cmd.Flags().Changed("project") {
		projectName, from = project.Name(ctx, a.env.Dir), sentAt["/project"]
	}
	key, err := store.TaskProject(projectName)
	if err != nil {
		return taskOf{}, flagOf(err, from)
	}
	return taskOf{project: key, name: name}, nil
}

// flagOf is err of the store's check of a value from (a flag, or where it was given): one that is no UTF-8 is told
// by from, as encode tells a body's.
func flagOf(err error, from string) error {
	if e, ok := errors.AsType[*usererr.Error](err); ok && e.Key == "store_task_utf8" {
		return usererr.New("cli_not_utf8", from)
	}
	return err //nolint:wrapcheck // the store's words
}

// workPages puts the pages slugs on a task and changes it, or with no flag tells them as they are. What the CLI can
// check (the pages listed, on a task, of one project for a parent) is checked before anything is written, so that a
// page of no task, or none at all, leaves the rest as they were; the store checks a parent as it writes the task. Flags
// of the pages alone ask nothing of tasks.json: an empty write would make a row for a task that has none. A parent goes
// with --task alone, on the one task the pages go on: on each page's own, a page of the parent itself would stop the
// writes halfway.
func (a *app) workPages(cmd *cobra.Command, f workFlags, slugs []string) error {
	ctx := cmd.Context()
	named := cmd.Flags().Changed("task")
	if cmd.Flags().NFlag() == 0 {
		return a.readPages(ctx, slugs)
	}
	onPages, onTask, needTask, name, err := pageFlags(cmd, f)
	if err != nil {
		return err
	}
	ed := taskEdit(cmd, f)
	sent, err := encodeSent(ed)
	if err != nil {
		return err
	}
	// The pages' body too is refused before a task is written: a category is in it alone.
	pageSent, err := encodeSent(pageWorkEdit(cmd, f))
	if err != nil {
		return err
	}
	c, list, err := a.pageList(ctx)
	if err != nil {
		return err
	}
	tasks, err := tasksOfPages(list, slugs, name, named, len(needTask) > 0, len(onTask) > 0)
	if err != nil {
		return err
	}
	if cmd.Flags().Changed("parent") && len(tasks) > 1 {
		// --task names one task, so that tasks differ by their projects alone
		return usageOf(usererr.New("cli_work_parent_projects"))
	}
	if err = a.writeTasks(ctx, c, tasks, ed, sent); err != nil {
		return err
	}
	if len(onPages) == 0 {
		return nil
	}
	return a.eachPage(cmd, slugs, http.MethodPatch, pageSent, a.tellPage)
}

// pageFlags refuses the flags of a floe work with slugs that cannot go together, and returns those that change the
// pages, those that change their task, those that need the pages on a task (the task's, and a category: setWork drops
// one of a page of no task), and the task's name --task gives.
func pageFlags(cmd *cobra.Command, f workFlags) (onPages, onTask, needTask []string, name string, err error) {
	if off := changed(cmd, "forget", "project"); len(off) > 0 {
		return nil, nil, nil, "", usageOf(usererr.New("cli_work_no_pages", off[0]))
	}
	named := cmd.Flags().Changed("task")
	if cmd.Flags().Changed("parent") && !named {
		return nil, nil, nil, "", usageOf(usererr.New("cli_work_parent_task"))
	}
	onPages = changed(cmd, "task", "category", "pin", "unpin", "clear")
	onTask = changed(cmd, "state", "url", "title", "categories", "parent")
	if name, err = store.TaskName(f.task); err != nil {
		return nil, nil, nil, "", flagOf(err, "--task")
	}
	needTask = slices.Clone(onTask)
	if f.category != "" {
		needTask = append(needTask, "category")
	}
	if named && name == "" && len(needTask) > 0 {
		return nil, nil, nil, "", usageOf(usererr.New("cli_work_task_off", needTask[0]))
	}
	return onPages, onTask, needTask, name, nil
}

// writeTasks writes ed, sent as encodeSent made it, to each of tasks in turn, stopping at the first failure. One case
// of not found is passed: ed taking the parent off alone, of a task of no row, which has no parent to take off and is
// made by the pages going on.
func (a *app) writeTasks(ctx context.Context, c client, tasks []taskOf, ed server.TaskEdit, sent jsontext.Value) error {
	var none server.TaskEdit
	rest := ed
	rest.Parent = nil
	parentOff := ed.Parent != nil && *ed.Parent == "" && rest == none
	for _, t := range tasks {
		var written store.TaskWritten
		err := c.do(ctx, http.MethodPatch, taskQuery(t), sent, &written)
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

// readPages tells each of the pages slugs as tellPage does, writing nothing: one not listed is refused before any is
// told.
func (a *app) readPages(ctx context.Context, slugs []string) error {
	_, list, err := a.pageList(ctx)
	if err != nil {
		return err
	}
	pages := make([]store.Edited, 0, len(slugs))
	for _, slug := range slugs {
		page, err := listedPage(list, slug)
		if err != nil {
			return err
		}
		pages = append(pages, store.Edited{Slug: page.Slug, TaskProject: page.TaskProject, Work: page.Work})
	}
	for _, page := range pages {
		a.tellPage(page)
	}
	return nil
}

// pageList is the client of the server, started when it is not running, and the pages it lists.
func (a *app) pageList(ctx context.Context) (client, []store.Listed, error) {
	c, err := a.client(ctx, true)
	if err != nil {
		return client{}, nil, err
	}
	var list []store.Listed
	if err = c.do(ctx, http.MethodGet, "/api/artifacts", nil, &list); err != nil {
		return client{}, nil, err
	}
	return c, list, nil
}

// listedPage is the page slug of list, refused when it is not there.
func listedPage(list []store.Listed, slug string) (store.Listed, error) {
	at := slices.IndexFunc(list, func(l store.Listed) bool { return l.Slug == slug })
	if at < 0 {
		return store.Listed{}, usererr.New("store_no_page", slug)
	}
	return list[at], nil
}

// tasksOfPages is the different tasks the pages slugs are on, the first of each first: the name given, or each page's
// own where the flags write the tasks (write). Each of slugs must be listed, and be on a task where the flags need one
// (needTask).
func tasksOfPages(list []store.Listed, slugs []string, named string, isNamed, needTask, write bool) ([]taskOf, error) {
	var tasks []taskOf
	for _, slug := range slugs {
		page, err := listedPage(list, slug)
		if err != nil {
			return nil, err
		}
		name := ""
		if page.Work != nil {
			name = page.Work.Task
		}
		if isNamed {
			name = named
		}
		if name == "" {
			if needTask {
				return nil, usererr.New("cli_work_page_no_task", slug)
			}
			continue
		}
		if !isNamed && !write {
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
		work.Task, work.Category, work.Pin = &empty, &empty, &off
	}
	if cmd.Flags().Changed("task") {
		work.Task = &f.task
	}
	if cmd.Flags().Changed("category") {
		work.Category = &f.category
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
