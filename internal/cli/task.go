package cli

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"floe/docs"
	"floe/internal/project"
	"floe/internal/store"
	"floe/internal/usererr"
)

// taskFlags are the flags of floe task.
type taskFlags struct {
	task, project, category, state, title, categories, parent string
	slugs, urls, removeURLs                                   []string
	pin, unpin, clear, forget                                 bool
}

// taskFieldFlags are the flags that write a field of a task's row.
var taskFieldFlags = []string{"state", "url", "remove-url", "title", "categories", "parent"}

// A task of pages: what a page names by its project and the name given, or its own.
type taskOf struct {
	project, name string
}

func (a *app) taskCmd() *cobra.Command {
	var f taskFlags
	cmd := &cobra.Command{
		Use: "task",
		Short: "Tell a project's open tasks and pages' task, category and pin, or change them, or a task's state, URL, " +
			"title, parent or categories' order",
		Long: `With no flag but --project, tell the open tasks (todo or doing) of --project (the project of the working
directory when left out; "" for none), the latest change first, a line each as --task tells a task. With --slug, put
the pages on a task and change that task; without, change the task --task names, of --project. With --slug and no
other flag, tell each page as it is, writing nothing; with --task alone, tell the task. floe task guide says how to
use it.

The pages and their tasks are written in one write, checked whole before anything is written, so that a refusal
writes nothing; a page the system cannot write then stops it there, and the same command given again finishes it. A
task a page comes to be on gets its row when it has none, and the page's category is added at the end of its
categories. Each task written is told on a line of its own, as the store left it (made, changed, or unchanged when a
field was given), then each page with its task, category and pin. A page's category is where the task's page places
it among its pages, which the user decides.

  floe task
    task #130 in project floe: state doing, title "Fix login"
  floe task --slug design-notes --task "#130"
    task #130 in project floe: made
    page design-notes: task #130 in project floe
  floe task --task "#130" --state doing
    task #130 in project floe: changed; state doing
  floe task --task "#130" --url https://example.com/issues/482 --url https://example.com/pull/9
    task #130 in project floe: changed; urls "https://example.com/issues/482" "https://example.com/pull/9"
  floe task --task "#130" --remove-url https://example.com/issues/482
  floe task --slug design-notes --category designs
  floe task --slug design-notes
    page design-notes: task #130 in project floe, category "designs"
  floe task --task "#130" --categories proposals,designs,samples
  floe task --task "#130" --parent "#123"
  floe task --slug design-notes --task "#131" --parent "#130"
  floe task --slug design-notes --pin
  floe task --task "#130" --forget`,
		Args:    cobra.NoArgs,
		PreRunE: flagGroups,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// A category of spaces is the empty one from here on, as the server reads it.
			f.category = strings.TrimSpace(f.category)
			if len(f.slugs) == 0 {
				return a.changeTask(cmd, f)
			}
			return a.pagesTask(cmd, f)
		},
	}
	flags := cmd.Flags()
	flags.StringArrayVar(&f.slugs, "slug", nil, "a page to put on a task, change or tell; give it again for more")
	flags.StringVar(&f.task, "task", "", `the task's ID; with --slug, "" takes the pages off their task`)
	flags.StringVar(&f.project, "project", "",
		`the project of the task, or of the tasks told, without --slug; "" for none`)
	flags.StringVar(&f.category, "category", "", `the pages' category on their task, the user's choice; "" takes it off`)
	flags.StringVar(&f.state, "state", "", "the task's state: "+store.StateNames())
	flags.StringArrayVar(&f.urls, "url", nil, "an outside link, http or https, to add to the task; give it again for more")
	flags.StringArrayVar(&f.removeURLs, "remove-url", nil, "a link to take off the task's; give it again for more")
	flags.StringVar(&f.title, "title", "", `the task's title; "" takes it off`)
	flags.StringVar(&f.categories, "categories", "", `the task's categories in order, comma-separated; "" takes it off`)
	flags.StringVar(&f.parent, "parent", "",
		`the task's parent, of its project; with --slug, with --task; "" takes it off`)
	flags.BoolVar(&f.pin, "pin", false, "pin the pages to the top of the task list")
	flags.BoolVar(&f.unpin, "unpin", false, "take the pages' pin off")
	flags.BoolVar(&f.clear, "clear", false, "take the pages off their task and take their pin off; the task stays")
	flags.BoolVar(&f.forget, "forget", false, "remove the task, which no page or child task may be on")
	cmd.MarkFlagsMutuallyExclusive("pin", "unpin")
	for _, other := range append([]string{"task", "category", "pin", "unpin"}, taskFieldFlags...) {
		cmd.MarkFlagsMutuallyExclusive("clear", other)
	}
	for _, other := range taskFieldFlags {
		cmd.MarkFlagsMutuallyExclusive("forget", other)
	}
	cmd.AddCommand(a.docGuideCmd("Print how to read the task list, put pages on a task and change a task", docs.TaskGuide))
	return cmd
}

// changed is the flags of names that were given, in a list of its own: names may be shared.
func changed(cmd *cobra.Command, names ...string) []string {
	return slices.DeleteFunc(slices.Clone(names), func(name string) bool { return !cmd.Flags().Changed(name) })
}

// taskEdit is what the flags change of a task.
func taskEdit(cmd *cobra.Command, f taskFlags) store.TaskEdit {
	var ed store.TaskEdit
	if cmd.Flags().Changed("title") {
		ed.Title = &f.title
	}
	if cmd.Flags().Changed("state") {
		ed.State = &f.state
	}
	if len(changed(cmd, "url", "remove-url")) > 0 {
		ed.URLs = &store.URLsEdit{Add: f.urls, Remove: f.removeURLs, Order: nil}
	}
	if cmd.Flags().Changed("categories") {
		// LIMIT: a category's name with a comma cannot be written here. Take a list of the flag if such names come to
		// matter.
		categories := strings.Split(f.categories, ",")
		ed.Categories = &categories
	}
	if cmd.Flags().Changed("parent") {
		ed.Parent = &f.parent
	}
	return ed
}

func taskQuery(t taskOf) string {
	return "/api/tasks?" + url.Values{"taskProject": {t.project}, "task": {t.name}}.Encode()
}

// changeTask changes the task --task names, or forgets it; with no field to change, it tells the task, writing nothing.
// With no --task, it tells the open tasks of a project when no flag but --project is given (every other flag changes
// a task or pages, which it must name).
func (a *app) changeTask(cmd *cobra.Command, f taskFlags) error {
	ctx := cmd.Context()
	if !cmd.Flags().Changed("task") {
		return a.listTasks(cmd, f)
	}
	if onPages := changed(cmd, "category", "pin", "unpin", "clear"); len(onPages) > 0 {
		return usageOf(usererr.New("cli_task_needs_pages", onPages[0]))
	}
	t, err := a.namedTask(ctx, cmd, f)
	if err != nil {
		return err
	}
	ed := taskEdit(cmd, f)
	var sent jsontext.Value
	writing := !f.forget && len(changed(cmd, taskFieldFlags...)) > 0
	if writing {
		body := store.TaskChange{
			TaskEdit: ed, Slugs: nil, TaskProject: &t.project, Task: &t.name, Category: nil, Pin: nil, Clear: false,
		}
		if sent, err = encodeSent(body); err != nil {
			return err
		}
	}
	c, err := a.client(ctx, true)
	if err != nil {
		return err
	}
	switch {
	case f.forget:
		err = c.do(ctx, http.MethodDelete, taskQuery(t), nil, nil)
	case writing:
		var written store.TaskChangeResult
		if err = c.do(ctx, http.MethodPatch, "/api/tasks", sent, &written); err == nil {
			a.tellTask(written.Tasks[0], ed)
		}
	default:
		return a.readTask(ctx, c, t)
	}
	if re, ok := errors.AsType[*remoteError](err); ok && re.status == http.StatusNotFound {
		return usererr.New("cli_task_no_row", t.name, t.project)
	}
	if err != nil {
		return err
	}
	if f.forget {
		a.print("%s", english.Msg("cli_task_forgotten", t.name, inProject(t.project)))
	}
	return nil
}

// readTask tells the task t as the store has it, its state and the fields it has, writing nothing.
func (a *app) readTask(ctx context.Context, c client, t taskOf) error {
	var all store.Tasks
	if err := c.do(ctx, http.MethodGet, "/api/tasks", nil, &all); err != nil {
		return err
	}
	row, ok := all[t.project][t.name]
	if !ok {
		return usererr.New("cli_task_no_row", t.name, t.project)
	}
	a.print("%s", taskLine(t.name, t.project, row))
	return nil
}

// listTasks tells the open tasks of the project --project names, or the working directory's, a line each as readTask
// tells a task, writing nothing. Which are open, and their order, are the store's (Tasks.Open). Any flag but --project
// changes a task or pages that were not named, and is refused.
// LIMIT: the tasks of every project are read to tell one project's. Send the project to GET /api/tasks if tasks.json
// grows to where that is slow (the user's, 113 tasks, is 31 KB).
func (a *app) listTasks(cmd *cobra.Command, f taskFlags) error {
	ctx := cmd.Context()
	others := cmd.Flags().NFlag()
	if cmd.Flags().Changed("project") {
		others--
	}
	if others > 0 {
		return usageOf(usererr.New("cli_task_no_task"))
	}
	key, err := a.taskProjectOf(ctx, cmd, f)
	if err != nil {
		return err
	}
	c, err := a.client(ctx, true)
	if err != nil {
		return err
	}
	var all store.Tasks
	if err = c.do(ctx, http.MethodGet, "/api/tasks", nil, &all); err != nil {
		return err
	}
	open := all.Open(key)
	if len(open) == 0 {
		a.print("%s", english.Msg("cli_task_none_open", inProject(key)))
	}
	for _, t := range open {
		a.print("%s", taskLine(t.Name, key, t.Task))
	}
	return nil
}

// taskLine is the task name of project (a key of tasks.json) as the store has it: its state, and its title, URL,
// categories and parent when it has them.
func taskLine(name, project string, row store.Task) string {
	fields := []string{"state " + row.State}
	if row.Title != "" {
		fields = append(fields, fmt.Sprintf("title %q", row.Title))
	}
	if len(row.URLs) > 0 {
		fields = append(fields, urlsField(row.URLs))
	}
	if len(row.Categories) > 0 {
		fields = append(fields, fmt.Sprintf("categories %q", strings.Join(row.Categories, ",")))
	}
	if row.Parent != "" {
		fields = append(fields, fmt.Sprintf("parent %q", row.Parent))
	}
	return english.Msg("cli_task_page_task", name, inProject(project)) + ": " + strings.Join(fields, ", ")
}

// tellTask says the task written: made, changed or as it was, and the fields ed asked for as the store left them, with
// its categories when a page's category was added to them.
func (a *app) tellTask(w store.TaskWritten, ed store.TaskEdit) {
	key := "cli_task_unchanged"
	switch {
	case w.Made:
		key = "cli_task_made"
	case w.Wrote:
		key = "cli_task_changed"
	}
	line := english.Msg(key, w.Task, inProject(w.TaskProject))
	if fields := taskFields(w.Row, ed, len(w.Added) > 0); fields != "" {
		line += "; " + fields
	}
	a.print("%s", line)
}

// taskFields is the fields of row that ed asked for, and its categories when added, in one order, the text ones quoted.
func taskFields(row store.Task, ed store.TaskEdit, added bool) string {
	var fields []string
	if ed.Title != nil {
		fields = append(fields, fmt.Sprintf("title %q", row.Title))
	}
	if ed.State != nil {
		fields = append(fields, "state "+row.State)
	}
	if ed.URLs != nil {
		fields = append(fields, urlsField(row.URLs))
	}
	if ed.Categories != nil || added {
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

// urlsField is a task's URLs as a line tells them, each quoted, or no urls.
func urlsField(urls []string) string {
	if len(urls) == 0 {
		return "no urls"
	}
	quoted := make([]string, len(urls))
	for i, u := range urls {
		quoted[i] = strconv.Quote(u)
	}
	return "urls " + strings.Join(quoted, " ")
}

// tellPage says the page as the store has it, written or read: its task and project, category and pin.
func (a *app) tellPage(page store.Edited) {
	on := page.Task
	var parts []string
	if on == nil || on.Name == "" {
		parts = append(parts, english.Msg("cli_task_page_off"))
		if page.TaskProject != store.ProjectKey("") {
			parts = append(parts, english.Msg("cli_task_page_in", inProject(page.TaskProject)))
		}
	} else {
		parts = append(parts, english.Msg("cli_task_page_task", on.Name, inProject(page.TaskProject)))
	}
	if on != nil && on.Category != "" {
		parts = append(parts, fmt.Sprintf("category %q", on.Category))
	}
	if on != nil && on.Pin {
		parts = append(parts, "pinned")
	}
	a.print("%s", english.Msg("cli_task_page", page.Slug, strings.Join(parts, ", ")))
}

// namedTask is the task of --task and --project, whose project is the working directory's when left out, its name
// as the store keeps it.
func (a *app) namedTask(ctx context.Context, cmd *cobra.Command, f taskFlags) (taskOf, error) {
	name, err := store.TaskName(f.task)
	if err != nil {
		return taskOf{}, flagOf(err, "--task")
	}
	key, err := a.taskProjectOf(ctx, cmd, f)
	if err != nil {
		return taskOf{}, err
	}
	return taskOf{project: key, name: name}, nil
}

// taskProjectOf is the project --project names, or the working directory's when left out, as tasks.json keeps it: the
// one place the project of a task or of a list of tasks is told.
func (a *app) taskProjectOf(ctx context.Context, cmd *cobra.Command, f taskFlags) (string, error) {
	projectName, from := f.project, "--project"
	if !cmd.Flags().Changed("project") {
		projectName, from = project.Name(ctx, a.env.Dir), sentAt["/taskProject"]
	}
	key, err := store.TaskProject(projectName)
	if err != nil {
		return "", flagOf(err, from)
	}
	return key, nil
}

// flagOf is err of the store's check of a value from (a flag, or where it was given): one that is no UTF-8 is told
// by from, as encode tells a body's.
func flagOf(err error, from string) error {
	if e, ok := errors.AsType[*usererr.Error](err); ok && e.Key == "store_task_utf8" {
		return usererr.New("cli_not_utf8", from)
	}
	return err //nolint:wrapcheck // the store's words
}

// pagesTask puts the pages --slug names on a task and changes it, or with no other flag tells them as they are. The
// pages and their tasks are written in one write (PATCH /api/tasks), which the store checks whole before it writes any.
// What the CLI can say better (the pages listed, on a task, of one project for a parent) it checks first, in its own
// words.
func (a *app) pagesTask(cmd *cobra.Command, f taskFlags) error {
	ctx := cmd.Context()
	slugs := f.slugs
	named := cmd.Flags().Changed("task")
	// --slug names the pages and writes nothing of its own
	if cmd.Flags().NFlag() == 1 {
		return a.readPages(ctx, slugs)
	}
	onTask, needTask, name, err := pageFlags(cmd, f)
	if err != nil {
		return err
	}
	ed := taskEdit(cmd, f)
	body := pagesChange(cmd, f, ed, slugs)
	sent, err := encodeSent(body)
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
		return usageOf(usererr.New("cli_task_parent_projects"))
	}
	var written store.TaskChangeResult
	if err = c.do(ctx, http.MethodPatch, "/api/tasks", sent, &written); err != nil {
		return err
	}
	a.tellChange(written, ed, len(onTask) > 0)
	return nil
}

// tellChange says a write of pages and their tasks: each task made or written, or each when fields were asked, with its
// categories when a page's category was added to them; then each page.
func (a *app) tellChange(written store.TaskChangeResult, ed store.TaskEdit, asked bool) {
	for _, t := range written.Tasks {
		if t.Made || t.Wrote || asked {
			a.tellTask(t, ed)
		}
	}
	for _, page := range written.Pages {
		a.tellPage(page)
	}
}

// pageFlags refuses the flags of a floe task with --slug that cannot go together, and returns those that change their
// task, those that need the pages on a task (the task's, and a category: setPageTask drops
// one of a page of no task), and the task's name --task gives.
func pageFlags(cmd *cobra.Command, f taskFlags) (onTask, needTask []string, name string, err error) {
	if off := changed(cmd, "forget", "project"); len(off) > 0 {
		return nil, nil, "", usageOf(usererr.New("cli_task_no_pages", off[0]))
	}
	named := cmd.Flags().Changed("task")
	if cmd.Flags().Changed("parent") && !named {
		return nil, nil, "", usageOf(usererr.New("cli_task_parent_task"))
	}
	onTask = changed(cmd, taskFieldFlags...)
	if name, err = store.TaskName(f.task); err != nil {
		return nil, nil, "", flagOf(err, "--task")
	}
	needTask = slices.Clone(onTask)
	if f.category != "" {
		needTask = append(needTask, "category")
	}
	if named && name == "" && len(needTask) > 0 {
		return nil, nil, "", usageOf(usererr.New("cli_task_task_off", needTask[0]))
	}
	return onTask, needTask, name, nil
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
		pages = append(pages, store.Edited{Slug: page.Slug, TaskProject: page.TaskProject, Task: page.Task, OffTask: ""})
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
	if err = c.do(ctx, http.MethodGet, pagesPath, nil, &list); err != nil {
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
// (needTask). The CLI checks these first in its own words; the store checks them again as it writes.
func tasksOfPages(list []store.Listed, slugs []string, named string, isNamed, needTask, write bool) ([]taskOf, error) {
	var tasks []taskOf
	for _, slug := range slugs {
		page, err := listedPage(list, slug)
		if err != nil {
			return nil, err
		}
		name := ""
		if page.Task != nil {
			name = page.Task.Name
		}
		if isNamed {
			name = named
		}
		if name == "" {
			if needTask {
				return nil, usererr.New("cli_task_page_no_task", slug)
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

// pagesChange is the write of the flags of floe task with --slug: the pages, the task --task names, the task's fields
// ed, and the pages' own.
func pagesChange(cmd *cobra.Command, f taskFlags, ed store.TaskEdit, slugs []string) store.TaskChange {
	w := store.TaskChange{TaskEdit: ed, Slugs: slugs, TaskProject: nil, Task: nil, Category: nil, Pin: nil, Clear: f.clear}
	if cmd.Flags().Changed("task") {
		w.Task = &f.task
	}
	if cmd.Flags().Changed("category") {
		w.Category = &f.category
	}
	if cmd.Flags().Changed("pin") {
		w.Pin = &f.pin
	}
	if cmd.Flags().Changed("unpin") {
		off := !f.unpin
		w.Pin = &off
	}
	return w
}
