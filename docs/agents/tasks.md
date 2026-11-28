# Tasks

The user keeps a task list in the store: the tasks of each project, each with a state, a title, links and the pages on it. `floe task` reads and writes it through the server (started when it is not running). This doc says how to read the list, put pages on a task and change a task when the user asks.

## Rules

- Put a page on a task, take it off, or change a task (its state, title, links, parent or categories) only when the user asks, or when your instructions (a CLAUDE.md, a skill) name the task.
- Take a task's ID, and a new task's title, from the user or from the tasks `floe task` prints. Do not make up an ID.
- Make a task with `--new` only when the user asks for a new task: floe makes a task by `--new` alone, and refuses any other write of a task of no row.
- Quote an ID in the shell (`--task "#130"`): a bare `#130` starts a comment, and the ID is lost.
- Do not give `--category` or `--categories`, and do not change a page's category, unless the user asks: the user decides where a page stands among its task's pages.
- Do not give `--forget` or `--clear` unless the user asks for that.
- When `floe task` says a page `moved from task …`, tell the user which task and category the page left: the user may not have known it was there.

## What a task holds

A task is named by its project and its ID (`#130`, `S12`); floe does not read what either means. A task of one project is another task than one of the same ID in another.

The user may change a task's ID in the task's Edit on the management page: its pages and child tasks move to the new ID with it, and the old ID names no task after. `floe task` does not change an ID.

| Field | Values | Written by |
|---|---|---|
| project | The project of its pages, the one `floe publish` prints (`page in project app`); `""` for pages of none | `--project`, or each page's own with `--slug` |
| state | `todo`, `doing`, `done` or `archived`; a new task is `todo` | `--state` |
| title | Text | `--title`; `""` takes it off |
| links | `http` or `https` URLs, 10 at most, in order | `--url` adds one at the end, `--remove-url` takes one off |
| categories | Names in order: the headings the task's page groups its pages under; printed each quoted (`categories "designs" "samples"`) | `--categories`, given again for each, and a page's `--category` |
| parent | Another task of its project, with no parent of its own (one level) | `--parent`; `""` takes it off |

A page is on one task at most, with a category of that task's and a pin of its own.

## Read

None of these writes anything.

| Command | Prints |
|---|---|
| `floe task` | The open tasks (`todo` and `doing`) of the working directory's project, the latest change first (made, a field changed, or a page come to it or moved to another of its categories), a line each |
| `floe task --project app` | Those of another project; `--project ""` for pages of no project |
| `floe task --task "#130"` | One task, of any state |
| `floe task --slug design-notes` | The page's task with its project, category and pin |

```text
$ floe task
task #130 in project floe: state doing, title "Fix login", urls "https://example.com/issues/482", categories "designs"
task #131 in project floe: state todo, title "Price table"
$ floe task --slug design-notes
page design-notes: task #130 in project floe, category "designs"
$ floe task --slug old-notes
page old-notes: no task, in project floe
```

A page on no task prints its project after `no task` (`page old-notes: no task, in project floe`), and a page of no project prints `page <slug>: no task` alone. A project of no open task prints `no open task in project floe`. A page published by `--link` is in the project of the linked file. `floe info <slug>` prints the rest floe keeps of a page: its title, URL, tags, data file or linked path, and whether it is archived.

## Make a task

```sh
floe task --task "#131" --new --title "Price table"   # a new task of the working directory's project
floe task --slug design-notes --task "#131" --new     # a new task of the page's project, the page on it
```

`--new` makes the task, in the state `todo`, with the fields given, and refuses a task there is. With `--slug`, the pages MUST be of one project (a task is of one project).

## Put pages on a task

```sh
floe task --slug design-notes --task "#130"                         # on a task of the page's project
floe task --slug a --slug b --task "#130"                           # several pages
floe task --slug design-notes --task ""                             # off its task
```

| Flag, with `--slug` | Changes |
|---|---|
| `--task "<ID>"` | The task the pages go on, in each page's own project. `""` takes them off their task and their category off; the task stays |
| `--category <name>` | The pages' category on their task, added at the end of its categories when it is not among them; `""` takes it off |
| `--pin` | Pins the pages on the task list |
| `--unpin` | Takes their pin off |
| `--clear` | Takes them off their task and their pin off; the task stays |

A page moved from another task leaves that task and its category (unless `--category` gives it one on the new task), and floe says where it was after the page: `; moved from task #130, category "designs"`. `--task ""` and `--clear` say `; off task #130` the same way. `floe publish --move-project` takes a page off its task, as a task is of its project, and says so.

## Change a task

```sh
floe task --task "#130" --state doing
floe task --task "#130" --url https://example.com/pull/9
floe task --task "#130" --remove-url https://example.com/issues/482
floe task --task "#131" --parent "#130"
```

| Flag | Changes |
|---|---|
| `--state <state>` | `todo`, `doing`, `done` or `archived` |
| `--title <title>` | The title; `""` takes it off |
| `--url <url>` | Adds a link at the end; give it again for more. One the task has stays where it is |
| `--remove-url <url>` | Takes a link off; give it again for more. One the task does not have is no mistake |
| `--parent "<ID>"` | The parent; `""` takes it off |
| `--categories <name>` | A category of the task, in order; give it again for each, the whole order (`--categories designs --categories samples`); `""` takes them all off |
| `--forget` | Removes the task; without `--slug` |

Without `--slug`, `--task` names the task in `--project` (the working directory's project when left out). With `--slug` and `--task`, these but `--forget` change the task the pages go on; with `--slug` alone, each page's own task, but for `--parent`, which needs `--task`.

## What floe refuses

floe checks the whole command before it writes, so a refused command writes nothing. It says why:

| Given | floe says |
|---|---|
| An 11th link | `task #130 would hold 11 URLs; a task holds 10 at most` |
| A link not `http` or `https` | `"ftp://x" is not an http or https URL` |
| `--task ""` with `--slug` and a flag that changes a task, or `--category` | `--task "" takes the pages off their task, so it cannot go with --state: run floe --help` |
| `--category` for a page on no task | `page "design-notes" is on no task: name one with --task` |
| `--category`, `--pin` or `--unpin` with `--task` and no `--slug` | `--pin changes pages: name them with --slug: run floe --help` |
| `--forget` or `--project` with `--slug` | `--forget goes with a task alone, not with pages: give it without --slug: run floe --help` |
| `--task` of a task of no row, without `--new` | `no task #130 in project floe: check the ID, name its project with --project, or make it with --new` |
| The same, of a task of no project (`--project ""`) | `no task #99 in no project: check the ID, name its project with --project, or make it with --new` |
| The same, with `--slug` | `no task #130 in project floe: check the ID, or make it with --new` |
| `--new` of a task there is | `task #130 is there already` |
| `--new` with `--slug` of pages of 2 projects | `a new task is made in one project, and the pages are of 2 projects: put them on it a project at a time` |
| A category with pages left out of `--categories` | `category "designs" of task #130 in project floe has pages (design-notes): take them off it first, or keep it in the order` |
| `--forget` of a task a page or a child task is on | `task #130 of project floe still has 1 page(s), such as design-notes: take them off it or delete them first` |
| `--parent` naming a task of no row | `no task #42 in project floe can be a parent` |
| `--parent` naming a task with a parent | `task #130 has a parent itself (#120), so it cannot be a parent: tasks have one level of children` |
| `--parent` given to a task with children | `task #42 has child tasks, such as #41, so it cannot have a parent: tasks have one level of children` |
| `--parent` or `--new` with `--slug` but no `--task` | `--parent with --slug goes with --task: name the one task the pages go on: run floe --help` |
| A flag but `--project` with neither `--task` nor `--slug` | `name a task with --task, or pages with --slug; to tell the open tasks, give no flag but --project: run floe --help` |
| An argument (`floe task design-notes`) | `unknown command "design-notes" for "floe task": run floe --help` |
| A `--slug` of no page | `no page "design-notes" (see floe list)` |

## What it prints

Each task and page written is printed on a line of its own, as the store left it: a task `made` (it had no row), `changed` (a field changed, or a page came to it or moved to another of its categories) or `unchanged` (a field was given, its value as it was), then each page with its task, category and pin, and, when it left a task, the task and category it was on.

```text
$ floe task --slug api-notes --task "#132" --new --title "API notes"
task #132 in project floe: made; title "API notes"
page api-notes: task #132 in project floe
$ floe task --slug design-notes --task "#132"
task #132 in project floe: changed
page design-notes: task #132 in project floe; moved from task #130, category "designs"
$ floe task --task "#132" --state doing
task #132 in project floe: changed; state doing
$ floe task --task "#132" --categories designs --categories "Designs, v2"
task #132 in project floe: changed; categories "designs" "Designs, v2"
$ floe task --slug api-notes --slug design-notes --task ""
page api-notes: no task, in project floe; off task #132
page design-notes: no task, in project floe; off task #132
$ floe task --task "#132" --forget
task #132 in project floe: forgotten
```
