# Commands

Reference for every command of the `floe` binary. How to write a data file is in [pages.md](pages.md).

## Quick reference

| Command | Purpose |
|---|---|
| `floe publish <data.yaml>` | Store the page drawn from a data file (or `--link` a file or a folder in place) and print its URL |
| `floe check <data.yaml>...` | Check data files as their pages are drawn, without writing them |
| `floe render <data.yaml>...` | Write the page of each data file to an HTML file, to keep in a repository |
| `floe rebuild` | Draw every stored page again with the templates, parts and themes of now |
| `floe template list` | List the templates |
| `floe template show <name> [file]` | Show a template: its themes and the files it reads, or one of them as written |
| `floe template new <name>` | Make a template of your own to fill in: the smallest floe takes, with a viewport |
| `floe template check` | Check the templates of your own |
| `floe part list [--all]` | List the parts a section of floe's templates takes; `--all` every part, look and shared schema |
| `floe part show <name> [file]` | Show a part, look or shared schema, or, for `html`, `any.json`, and the files it reads, or one of them as written |
| `floe part new <name>` | Make the smallest part of your own, to fill in |
| `floe part check` | Check the parts, looks, shared schemas and themes of your own |
| `floe theme list` | List the themes |
| `floe theme show <name> [file]` | Show a theme and the files it reads, or one of them as written |
| `floe list` | List pages, newest first |
| `floe rm <slug>...` | Delete pages |
| `floe archive <slug>...` | Take pages off the lists, keeping them (`--undo` gives them back) |
| `floe tag <slug>... --add <name> --remove <name>` | Put tags on pages and take them off |
| `floe work [<slug>...]` | Put pages on a task of the task list, pin them, or change a task |
| `floe serve` | Run the server in the foreground |
| `floe stop` | Stop the background server |
| `floe diagram render <spec.yaml>...` | Check diagram specs and print them as inline SVG |
| `floe diagram inspect <spec.yaml\|data.yaml>...` | Check diagrams, of specs or of data files, and print where each part went |
| `floe diagram example [kind]` | List the example specs, or print one |
| `floe icons fetch <set> [zip-or-url]` | Keep an icon set (`aws`) for the `icon` parts of diagrams |
| `floe icons search <word>...` | List the fetched icons whose name contains every word, or a short name such as `alb` |
| `floe completion <shell>` | Print the completion script for a shell (Cobra builtin) |
| `floe version` | Print the version (also `floe --version`) |
| `floe help [command]` | Print help (Cobra builtin) |

---

## `floe publish <data.yaml>` / `floe publish --link <file|folder>`

Draw the page of a data file with the template it names, store the page as it was drawn, and print its URL (`http://127.0.0.1:7325/a/<slug>/`) on stdout, and the warnings of the page drawn on stderr, as `floe check` prints them. If the server is not running, it is started in the background first. A new slug is also opened in the browser (`$BROWSER`, else `open`, `wslview` or `xdg-open`); a republished one is not, because the open tab reloads itself.

Pages are drawn from data files (`.yaml`, `.yml`); [pages.md](pages.md) tells how to write one. The data file is copied to `<slug>/source-<id>/data.yaml` under `FLOE_DIR`, with the images it shows at the paths it names them by; take it back from there if the original is lost. Each publish writes a new copy and then removes the one before, so a publish that fails leaves the page as it was. A relative path is resolved against the working directory.

The stored page stays as it was drawn: changing the template, a part or a theme changes it only when `floe rebuild` draws it again. Another file, such as an HTML file, is not stored: link it with `--link` to show it as it is.

### Flags

| Flag | Description |
|---|---|
| `--link` | Register the file or folder in place: an HTML file is shown as it is, a data file is drawn each time its page is opened, a folder lists the HTML files under it (see [links.md](links.md)). |
| `--slug <name>` | The URL name: 1 to 64 lowercase letters, digits and hyphens, starting and ending with a letter or digit. Defaults to the file name; a linked `index.html` takes the name of its directory. |
| `--title <text>` | The title in the list. Defaults to the data file's `title:`, or a linked file's `<title>`, else its first `<h1>`, else the slug. |
| `--description <text>` | A description shown in the list. |
| `--theme <name>` | The theme to draw the page with, its palette and the parts' shape, over the data file's `theme:`. Defaults to `theme:`, else `default`. Refused with `--link` of an HTML file or a folder, which are shown as they are. |
| `--force` | Replace a page published from another project or another file, or a link to another file, keeping nothing of the old one, not even a page's earlier versions. |

### Projects and overwriting

Each page belongs to the project it is published from. The project is the repository name in git's `origin` URL, else the repository's directory (a worktree joins its main checkout), else the working directory's name.

- A page belongs to the file it is published from: its project, and its place in it, the path from its repository's root when the file is in the repository of the working directory (a worktree of it too), else its real path. `docs/plan.yaml` is one place whether it is published from the repository's root, from `docs`, or from another worktree.
- The same slug from the same file overwrites the page. stderr says so (`updated "plan" (last updated …, title "…")`). If the title changed, a warning says from what to what. The exit code stays 0.
- A slug published from another project or another file is refused unless `--force`. `--force` starts the page afresh: nothing of the old one is kept. `--force` drops the page's history: its versions, its pinned title, tags and favourite. To move a data file or rename it and keep its page's URL, publish it with `--force`.
- A page renamed on the management page (`o` to `d`) is its file's still: publish that file under the new name to keep its history (`floe publish o.yaml --slug d`). Another file publishing to `d` is refused unless `--force`.
- A page and a link cannot take each other's slug, even with `--force`: `floe rm` it first.
- A republish keeps what was set on the management page (title, project, favourite, tags), the description and the creation time. `--title` and `--description` override them. An archived page comes back to the lists.
- A republish of the same content, with another theme or not, draws the page again, but its update time and its place in the list stay; other content moves them up. A linked file is listed at the later of the file's time and when it was linked, so that linking it again brings it up.

### Linked files

`--link` registers a file or a folder where it is, instead of storing a page; [links.md](links.md) tells how each is shown.

- Only a regular `.html` or `.htm` file, a data file (`.yaml`, `.yml`) or a folder can be linked. A data file is checked as `floe check` checks it, and refused with its mistakes.
- Refused: a hidden directory right under your home (settings and keys live there), and a file or folder in one; a file directly under `/` or under your home itself, and those two as folders, as the whole directory would be served; `--theme` with an HTML file or a folder.
- A link without `--slug` reuses the slug of the same file, else takes the first free one of `name`, `name-2`, `name-3` …; a folder in a repository is named by its project and its place in it (`api-docs-docs`), so that folders of one name in several repositories differ by their URL, and a folder outside one by its name (`handbook`). The same slug for another file is refused unless `--force`.

---

## `floe check <data.yaml>...`

Check each data file as its page is drawn: its YAML, its template and theme, the template's schema, its checks and computed values, and the page drawn. A relative path is resolved against the working directory.

- The mistakes of every file are printed as `file:line:column: message` (`file: message` for one of the whole file), in the order the files were given (exit code 1): every mistake of a stage of the checks at once, a stage running only when the one before it has none. [pages.md](pages.md#reading-mistakes) tells how to read them.
- The warnings of the pages drawn, advice on a better way to write them, are printed after the mistakes as `file:line:column: warning: message`; they leave the exit code as it is. [pages.md](pages.md#warnings) tells them.
- Nothing is printed when every file is right and has no warning (exit code 0).
- Nothing is written, and the server is not started, so a repository's CI can run it over its data files.
- A file that is not a data file (`.yaml`, `.yml`) is refused.
- A data file named twice (by two globs, through a symlinked directory, or in another case on macOS) is checked once, and told by the path it was first given as.

## `floe render <data.yaml>...`

Draw each data file and write its page to `<name>.html`, beside the data file, so that a repository keeps the data file for agents to read and edit, and its page for people to read. The page is the one the management page downloads: it opens without floe, what it shows is in it (images, the diagrams' style and zoom), and its CSP, which keeps its scripts from sending what it holds out, is in a `<meta>`.

```sh
floe render docs/spec.yaml                 # writes docs/spec.html
floe render docs/*.yaml --out-dir site     # writes site/<name>.html
floe render --check docs/*.yaml            # in CI: fails when a page was not written again
```

| Flag | Description |
|---|---|
| `--out-dir <dir>` | Write the pages in this directory, made when missing, instead of beside each data file. |
| `--check` | Write nothing: print each page that is missing or is not what its data file draws (exit code 1), or, when every page is, only the warnings of the pages drawn (exit code 0). |

- A page is written only when it changes, and its path is printed; running it again does nothing.
- The warnings of the pages drawn are printed on stderr as `floe check` prints them; they leave the exit code as it is.
- A page there is written over: keep the pages in version control to get one back.
- The mistakes are printed as `floe check` prints them, and nothing is written until all are fixed (exit code 1). Nothing is written either when two data files would write the same page (`a/spec.yaml` and `b/spec.yaml` with one `--out-dir`, or `spec.yaml` and `spec.yml`).
- Nothing is published, and the server is not started.
- A page depends on the floe, the templates and the icons that draw it: in CI, use the floe version and the templates the pages were written with, and write the pages again when you upgrade.
- A `script` of the page runs without the sandbox of the server's pages when the file is opened; its CSP still cuts off `fetch`, XHR, images from elsewhere and forms. A link between pages (`/a/<slug>/`) does not work in a file: link the files relatively.

## `floe rebuild`

Have the server draw every stored page again from the copy of its data file and images, with its templates, parts and themes of now, and keep the page where it changed. Run it after you change a template, part or theme of your own, or upgrade floe.

- A page that no longer draws keeps the page it had, and its mistakes are told (exit code 1): fix the data file it was published from and publish it again, or fix the template.
- Linked files are drawn each time they are opened, and are left alone.
- The server is started when it is not running.

---

## `floe template list`

Print the templates, floe's and your own, one per line in four tab-separated columns: the name, where it is (`floe`, or the directory of a template of your own), its shape (the keys of its data file, below) and the description of its `schema.json`. A template of your own hides floe's of its name. [templates.md](templates.md) tells how templates are written.

## `floe template new <name>`

Make the template `<name>` in `FLOE_TEMPLATES_DIR`: a `page.html`, a `schema.json` and an `example.yaml`, the smallest that `floe template check` takes, to fill in. The `page.html` also holds a viewport `<meta>`, so that a phone draws the page at its own width.

- A name is lower case letters, digits, `-` and `_`, starting with a letter or a digit.
- Nothing is written over: with anything of that name there, the command stops and makes nothing (exit code 1).

## `floe template check`

Read the templates in `FLOE_TEMPLATES_DIR` with the parts and themes of your own, as `floe part check` does, then draw the `example.yaml` of each template of your own as `floe check` draws a data file.

- The mistakes are printed (exit code 1): those of the files, and the examples drawn once the files have none; then the warnings of the examples drawn. Nothing is printed when they are right and have no warning (exit code 0).
- A file floe does not read in a template (a `template.yaml`, say) is refused: a template holds `page.html`, `schema.json` and `example.yaml`, and `page.css`, `computed.yaml`, `checks.yaml`, `words.yaml` and `assets/` if it likes.

[templates.md](templates.md) tells what each file holds.

## `floe part list`

Print the parts a section of floe's templates takes (the keys of floe's `any.json`, and your own parts, which join it), one per line in four tab-separated columns: the name, where it is (`floe`, or the directory of one of your own), its shape and its description. `html` takes HTML as text and has no `schema.json`: `floe part show html any.json` prints `any.json`, which holds the key `html`. Your own parts join `any`, but `floe part show any any.json` prints floe's `any.json` as it is written.

The shape is what a value of it is written as: its keys, by name; `[] ` before those of a list's items; `one of ` before keys of which exactly one is written; the name of the format (`markdown`, `html`) or of the floe part it is; or `-`, which the list does not tell: `floe part show <name>` (`template show`, `theme show`) names in its `files:` line what to read, and `floe diagram example <kind>` prints an example. A template's shape holds `template`, `lang`, `theme` and `title`, which every data file may write.

With `--all`, print every part, look and shared schema, floe's and your own, that a `$ref` of your own schema can name, in the same columns (where it is: `floe`, or the directory of one of your own). `html` is not among them, as there is no `floe:///parts/html.json`.

## `floe part new <name>`

Make the part `<name>` in `FLOE_PARTS_DIR`: a `schema.json`, a `part.html` of its box and an empty `part.css`, the smallest that `floe part check` takes, to fill in.

- Nothing is written over: with anything of that name there, the command stops and makes nothing (exit code 1).

## `floe part check`

Read the parts, looks and shared schemas in `FLOE_PARTS_DIR` and the themes in `FLOE_THEMES_DIR`, over floe's own, and print every mistake of them (exit code 1), a shared schema's too, checked from its root whether a schema names it or not. Nothing is printed when they are right (exit code 0).

[parts.md](parts.md) tells what a part and a theme hold.

## `floe theme list`

Print the themes, floe's and your own, one per line in the same four tab-separated columns as `floe part list`: the name, where it is (`floe`, or the directory of a theme of your own), its shape, `-`, as nothing is written as a theme's value, and its description, its `theme.yaml`'s `description:`, empty without one.

## `floe template show <name> [file]` / `floe part show <name> [file]` / `floe theme show <name> [file]`

Print a template, a part (a look or a shared schema too) or a theme, floe's or your own, to read before writing with it or copying from it: its name and description, where it is and the files it reads (`files:`), not what they hold.

- With a file named after (`floe part show code part.html`), print that file alone, as written but the byte order mark at its start: floe's parts show how `part.html` uses `{{copyButton}}` and the other functions. A file it does not read is refused with the files it does.

| Of | Also prints | The files, in the order of `files:` |
|---|---|---|
| A template | The themes a data file may pick (`themes:`) | What a data file may hold (`schema.json`, `checks.yaml`, `computed.yaml`) and `example.yaml`, then `page.html`, `page.css` and `words.yaml`. Not `assets/`, its images |
| A part | The looks it takes | `schema.json`, `checks.yaml`, `part.yaml`, `part.html`, `part.css`, `words.yaml`. The CSS of the looks it takes is the looks' own: `floe part show panel part.css` prints it |
| A look | The looks it takes | `part.yaml`, `part.css` |
| A shared schema | — | `<name>.json` |
| A theme | — | `theme.yaml`, `palette.css`, `base.css`, `markdown.css`, then the CSS it gives each part or look, by name |

---

## `floe list`

Print the pages, newest first, as the server lists them (it is started when it is not running). Each line holds four tab-separated columns: the update time (local, `YYYY-MM-DD HH:MM`), the slug, the title and the linked path (empty for a stored page). Archived pages are left out; `--archived` lists them too, with a fifth column on every line: `archived`, or empty.

## `floe rm <slug>...`

Delete the pages in order through the server (it is started when it is not running), stopping at the first failure (such as a page that does not exist). A linked file itself is left alone; only its registration is removed.

## `floe archive <slug>...`

Archive the pages in order through the server (it is started when it is not running), stopping at the first failure (such as a page that does not exist). An archived page is left off `floe list` and the management page's list, which shows it under "Archive" on its shelf; it is still served at its URL, downloaded and drawn again by `floe rebuild`. Archiving one again changes nothing. `--undo` gives the pages back to the lists, and so does publishing a page again. This moves the pages alone: a task goes to the archive by its state, `floe work --task X --state archived`.

## `floe tag <slug>... --add <name> --remove <name>`

Put tags on the pages and take tags off them in order through the server (it is started when it is not running), stopping at the first failure (such as a page that does not exist). Give `--add` and `--remove` once for each name, at least one of them. A name is taken as written, trimmed of spaces at its ends, commas and all; one a page has already is kept once, and one it does not have is taken off as nothing. The names given to `--remove` are taken off first, then those given to `--add` are put on, so a name given to both stays on the page, last. Only the names given change: tags written meanwhile from the management page or another `floe tag` stay.

```sh
floe tag design-notes plan --add design --add "cli, api"   # two tags on two pages
floe tag design-notes --remove draft                       # one taken off
```

## `floe work [<slug>...]`

Say which task a page is on, and what a task's state is, through the server (it is started when it is not running). A task is named by its project and its ID; floe does not read what either means. The pages say which task they are on (`work` in each `meta.json`), and `tasks.json` in the store holds each task's title, state, last step, outside link and order of steps, and how many times it was changed (`rev`), which a screen sends back so that a task written since is refused.

```sh
floe work design-notes --task "#130" --step design --state waiting   # the page on the task, and the task's state
floe work --task "#130" --url https://example.com/issues/482         # the task alone
floe work --task "#130" --steps design,build,review                  # the order its steps are shown in
floe work design-notes --pin                                         # pinned, whatever its task
floe work design-notes --task "#131" --parent "#130" --step design   # the page on a child task of #130
floe work --task "#130" --forget                                     # the task's row removed
```

Each task and page written is told on a line of its own, in the order written, as the store left it: a task `made` (it had no row), `changed` or `unchanged` (the same values, not written), with the fields given; a page with its task, step and pin. A task of no project is `in no project`.

```text
$ floe work design-notes --task "#130" --step design --state waiting
task #130 in project floe: changed; state waiting, step "design"
page design-notes: task #130 in project floe, step "design"
$ floe work --task "#130" --forget
task #130 in project floe: forgotten
```

With slugs, the task's project is each page's: `--task` puts the pages on that name in their own projects, and without it each page's own task is changed. `--project` and `--forget` go with a task alone, not with pages, so they go without slugs. `--parent` with slugs goes with pages of one project. Without slugs, `--task` names the task, of `--project` (the working directory's project when left out; `""` for none).

| Flag | Changes | |
| --- | --- | --- |
| `--task <ID>` | pages or the task named | With slugs, `""` takes the pages off their task and takes their step off, and cannot go with what changes a task (`--state`, `--url`, `--title`, `--steps`, `--parent`, a step). A page moved to another task leaves its step, unless `--step` gives it one there. A new ID is a task in the state `todo` |
| `--step <step>` | pages, and the task's last step | With slugs only. `""` takes it off the pages and leaves the task's |
| `--state <state>` | the task | `todo`, `doing`, `waiting`, `done` or `archived` |
| `--url <url>` | the task | Its one outside link, `http` or `https`; `""` takes it off |
| `--parent <task ID>` | the task | With slugs, with `--task`: of the one task the pages go on. A task of its project with a row and no parent of its own; a task with children takes none: one level only. `""` takes it off |
| `--title <title>` | the task | `""` takes it off |
| `--steps a,b,c` | the task | The order of its steps; a step not named comes after them. A step's name cannot hold a comma here. `""` takes it off |
| `--pin`, `--unpin` | pages | With slugs only |
| `--clear` | pages | Off their task, and their pin off; the task stays |
| `--forget` | the task | Without slugs. Refused while a page is on it, an archived one too, or it has a child task |

The tasks are written first, once for each different task the pages are on, and the pages then, in order, stopping at the first failure; a page that does not exist, or is on no task where the flags change one, is told before anything is written. A page that could not be written leaves its task written: the same command given again finishes it. Flags of the pages alone write no task.

The task list, at `/tasks` of the server, lists the tasks of one project by state. Opened without one, it takes the project you last had in this browser while it is there, else the one with the task that has waited longest, else the first on the shelf, and writes it in the URL. Waiting ones the longest first, the others the latest changed first, each with its last step, its pages, how many are unread and its link, under the pages of that project that are pinned, each with a × that takes its pin off. A project with no task, whose last pin is taken off, is then no longer on the list, and its screen says so as for any such project. A task's place on the list is its state alone: `archived` puts it under Archive, whatever its pages are. The switch to it stands beside the logo, with no task too.

**Add task**: "+ Add task" at the top right of the task list makes a task of the project shown, from a task ID and a title. Made, the list goes to To do with the new row marked. An ID already there is said so, and "Open the task" goes to its page. The screen of no task anywhere has the same button, and asks for a project of those the pages have, or "Not set"; for a project no page has, use `floe work --project`. The archived list and a project not on the list, named in the URL, have no such button.

A task with no page and no child has "Remove the task" on its page: two presses remove it and go back to the list, the way `--forget` does. A task's page has "Edit" over its pages, which opens the task's title, URL and steps in one dialog. The steps are reordered, added and removed there, their names kept: moved by a step's grip, dragged, or by its up and down buttons, which a touch screen shows alone; added by "Add a step", the last row, where a name is typed and Enter adds it; and removed by a step's bin button, which a step with pages, archived ones too, has but cannot press: take its pages off it first, by "Take off its task" or "Set task" in a page's `…`, which an archived page has too. The order is written whole, a step of pages alone too, archived ones among them. Only what was changed is sent, so that nothing written meanwhile is undone: one written meanwhile is refused, and the dialog keeps what was changed and reads the rest again; a task removed meanwhile is not made again. A task's row opens the task's page, at `/tasks?project=…&task=…`: its pages under their steps, in the order `--steps` wrote and then as they first came, those of no step first, and within each the archived after the rest, dimmed and marked Archived; a step of no page is headed too, and each step's "Add a page" finds a page of the project, not archived, to put under it, moving one of another task or step; and over them its state to change, its link, and, once it is done, a button that archives the task, as `--state archived` does: its pages stay as they are, archived on their own by their rows or `floe archive`; the same button then gives the task back, to done. A task is written from its page, a dialog or the list as the screen last read it: one written by another session meanwhile is refused, and the screen reads the tasks again. Opening it reads nothing, and a page opened whole from it goes back to it, as one from the pins goes back to the task list. `h` and the link over its rows go to the project's list at the state the task is in now, with the focus on the task's row.

A page open whole has a back button beside the logo, which says where it goes: the task's ID, the task list or the page list, whichever opened it, and `h` goes there too. Back on the list, by that button, `h` or the browser's own way back, that page's row has the focus and the page is open beside it at the same version, unless it was left unread. The logo always goes to the page list. A page that is not among the rows that opened it (from the search, or another tab) goes back to the page list.

A page's `…` menu does the same by hand: it pins the page or takes the pin off, opens its task, `Set the task` names its task's ID, its step, and a new task's title, and `Take off the task` takes the page off its task, the toast's Undo putting it back on the same task and step; a task's own title, URL and order of steps are edited on its page, and its parent in its family card there. The names offered say each task's state, and a page put on a task is told so, with a link to it. An ID of no task is a new task, which needs a title. The page list's rows show a page's task and pin, as a way to them.

**Child tasks**: a parent is set with `--parent`, or on a task's page. A task's page with a row and no family has two parts under its head: "Parent task" and "Child tasks", each with "+ Add"; a task of pages alone, of no row, has the parent's alone. "+ Add" over the parent task takes a task of the project as the parent, or makes a new one with an ID and a title, in the same write; a task with a parent has "Change" there instead, which moves it, saying from which. A button at the right of a parent's or child's row takes the child off its parent at once, shown as the row is pointed at or reached by keyboard (always on a touch screen); the toast says so, with "Undo", and so does a change of parent, while the parent before can be one. "+ Add" over a task's child tasks, on the page of a task with no parent, makes a new task under it, of no page yet, to do, with an ID and a title; a task of that project may be chosen or named instead, which becomes a child with the title it has, out of another parent's too, as the dialog says before it is done; one with children is refused. A parent and a child are separate tasks, each under its own state on the list. A child's row names its parent in words, and a parent's row says how many children it has. A task's page shows its children, or its parent, over its pages in a card, each row headed by its state; pressing one opens that task's page (`j` and `k` do not stop on them). Changing a parent's state changes nothing of its children. Whether a task is archived is its own too: "Archive the task" touches that task alone, and not its pages, its parent or its child tasks. A `parent` written by hand that breaks these (a task of no row, itself, a task with a parent) is not refused: the task shows as having no parent. Changing a page's project does not move its task's row, so the family stays in the first project.

A page keeps its `work` when published again, renamed or drawn again. `tasks.json` that cannot be read, or holds another state or a URL other than `http` or `https`, is refused and not written over: mend it by hand. A key of a row floe does not know is lost at the next write.

## `floe serve`

Run the server in the foreground on `127.0.0.1` until SIGTERM, SIGINT or `floe stop`; it stops once the requests under way are done (10 seconds at most), and a second signal ends it at once. The commands [The server](#the-server) lists start the same server in the background, so this is mainly for seeing its errors. A background server that does not start is told as `the server did not start; run "floe serve" to see the error`.

## `floe stop`

Stop the background server, and return once it is gone, so that a command right after starts its own. Without one, it prints `server is not running` and exits 0.

---

## `floe diagram render <spec.yaml>...`

Check each diagram spec and print the diagrams, in order, as one HTML fragment of inline SVG on stdout, to paste into an HTML page of your own: a whole document with `<!doctype html>` and `<html lang>`, since `floe publish --link` shows the file as it is. In a data file, write the spec in a `diagram` part instead ([diagrams.md](diagrams.md), which tells how a spec is written). A relative path is resolved against the working directory.

- A spec is checked against the diagram's schema, then laid out once for every language its text is written in, and checked: text that overflows, parts that overlap or stick out of the canvas, edges through boxes, crossings … ([diagrams.md](diagrams.md) lists the checks).
- Every mistake of every spec is printed as `file:line:column: message` (`file: message` for one of the whole spec), in the order the specs were given, and nothing is drawn until all are fixed (exit code 1).
- The text is written in the language of `--lang` (`en` when left out). A language a text does not have falls back to English.
- The fragment starts with the assets: `default`'s palette on the diagrams alone, the diagrams' style, which takes its colours from the palette's CSS variables, and the script that zooms them. The page around the diagrams keeps its own colours and variables, and the diagrams take none of the page's: the palette and `color-scheme: light` are set on the diagrams' roots, so a variable or a color-scheme on the page's `:root`, or on an element holding them (a dark panel), does not reach them. Each diagram paints its own ground, the palette's `--bg` (`default`'s, `#f4f3f0`): on a dark page it stands as a light box, unless you set `--bg` on its roots as below. To draw the diagrams in the page's colours, set the palette's variables on their roots, outside any cascade layer (`.floe-zoom-tools, .floe-zoom { --accent: …; }`): the fragment's palette is in the layer `floe.palette`, which comes after the layers the page names first, so a rule in one of the page's layers (Tailwind's `@layer components`) loses to it, and a rule outside layers wins. Keep the assets in the same tree as the diagrams (the document, or one shadow root). In a shadow root the diagrams take their colours, but not the toolbar or the zoom: the script looks for diagrams in the document only. Read the page in a browser the README names: one that does not read `@scope` draws the diagrams without colours. The CSS that `floe part show diagram part.css` prints takes the palette's variables and has none of its own, so a page without a palette takes the fragment's assets, not that CSS. Readers zoom a diagram drawn smaller than its actual size with the buttons, Ctrl / ⌘ + wheel or a pinch, and drag or use the arrow keys to move while zoomed. A page needs the assets once; holding them more than once does no harm.

| Flag | Description |
|---|---|
| `--lang <tag>` | The language to write the diagrams' text in (`en`, `ja`). Defaults to `en`. |
| `--no-assets` | Print the diagrams without the assets, for a page that already holds them. |

## `floe diagram inspect <spec.yaml|data.yaml>...`

Check each diagram as `floe diagram render` does, and print where each part went. It lets one check that edges meet the rows they mean and labels sit in their areas, without seeing the drawing.

```text
page.yaml:9:11: diagram "The API" 560x190
frame "cloud" 20,20 300x150
the heading of frame "cloud" 60,26 74.88x18
the icon of frame "cloud" 20,20 31x31
edge [60,10]→[60,120] M60,10 V120
```

- A file with a key `template` at its top is a data file: its diagrams, in `details`, `tabs`, `columns` and the `parts` of an item of `entries`, `checks` or `options` too, are read after the template's schema, in the order they are written. Any other file is a spec. A mistake off the diagrams that the schema refuses stops the file, as nothing can be said of what is under it.
- Each diagram prints a line of where it is written (`file:line:column:`, or `file:` for a spec), its title and its canvas. Then, a line each: a part's area, a frame's heading, an edge's path and the ends it joins, a label's area and the circles it is in.
- Where is printed on stdout and mistakes on stderr, and the exit code is 1 when there is one. A diagram the checks refuse is printed too, as a guide: fixing it may move its parts and edges, and a `graph` with a mistake has only what it got as far as, or nothing. A diagram that gets no layout (a mistake of its YAML or schema, an `example` there is none of) prints no line.
- The checks of a data file's template and parts do not run, and the rest of its page is not checked (its items of format `html`, its images, what its parts' functions refuse): `floe check` checks the whole page. A diagram that a part of one's own draws with `{{diagram}}` is not among them. A data file lacking `template` is read as a spec, and told so in the words for a spec; a spec with one is read as a page.

## `floe diagram example [kind]`

Without a kind, list the built-in examples, one per kind of diagram (architecture, AWS architecture, AWS architecture laid out by floe, class, ER, flow, Gantt chart, Git branches, organisation chart, quadrant, sequence, state, swimlane, timeline, Venn): each line holds the kind, where it is (`floe`), its shape, `-`, as an example is copied rather than written as a value, and a summary, tab-separated, as `floe part list` prints them. With a kind, print its spec; its comments explain the grid it was laid out on. Every example passes the checks. Its text is written in English and Japanese (`{en: …, ja: …}`), as a spec may write it. The AWS examples draw AWS icons, so render them after `floe icons fetch aws`.

---

## `floe icons fetch <set> [zip-or-url]`

Keep an icon set in `FLOE_ICONS_DIR/<set>` for the `icon` parts of diagrams. floe does not ship icons; each machine fetches a set once. The sets:

| Set | Icons |
|---|---|
| `aws` | The [AWS Architecture Icons](https://aws.amazon.com/architecture/icons/): the 48px service and resource icons and the 32px group icons. The Icon package zip is about 14MB. |

- Without a zip, the set's package is found on its official page and downloaded. A zip downloaded in a browser, or its URL, can be given instead; a relative path is resolved against the working directory.
- A version already there is kept. `--force` fetches it again.
- The icons are replaced as a whole: if that fails, the previous ones stay.

| Flag | Description |
|---|---|
| `--force` | Fetch the icons again even if that version is already there. |

## `floe icons search <word>...`

List the fetched icons whose name contains every word (case does not matter), one per line: the name to give an `icon` part (`aws/…`) and its category, tab-separated. A common short name of a service (`alb`, `s3`, `ecs`) finds the service first, then its resources (`s3 bucket`); floe keeps a short list of these, and any other word is matched only against the names.

---

## The server

- `floe publish`, `floe rebuild`, `floe list`, `floe rm`, `floe archive`, `floe tag` and `floe work` start the server when it is not running, and keep using it otherwise.
- They ask the server to check, save, draw, list and delete pages: the server is the only one that writes the store. A publish sends the data file (and the images the server asks for), and the server checks and draws it with its own templates, parts, themes and icons.
- After floe is reinstalled (another version, or a rebuilt binary), the next of them replaces the running server.
- The server keeps running until `floe stop`, logout or reboot.
- The server draws with the `FLOE_TEMPLATES_DIR`, `FLOE_PARTS_DIR`, `FLOE_THEMES_DIR` and `FLOE_ICONS_DIR` of the command that started it: set them in the shell's environment, where every command takes them, rather than on one command.
- A `floe publish` or `floe rebuild` with other ones is refused until `floe stop`. `floe list`, `floe rm`, `floe archive`, `floe tag` and `floe work` do not draw and take any, but the one that replaces the server after floe is reinstalled is refused with other ones too, as the new server would draw with them.
- A server serves one store (`FLOE_DIR`) on its port: while it runs, a command for another store is refused. Give that store another `FLOE_PORT`.
- It listens on `127.0.0.1` only. It refuses other host names (against DNS rebinding) and writes from other sites.
- A stored page is served as it was drawn; its CSP and sandbox are sent with it. Pages run in the sandbox, so their scripts cannot reach the management page's API. The frame takes from a page only that it was read and the keys that move on or open a modal.
- A publish whose data file or images changed makes a new version of the stored page; publishing the same content, `floe rebuild` and changes on the management page make none. A page keeps its last 20 versions, the oldest going first.
- The store keeps which version of each stored page was read in `reads.json`; links and folders have no such record. A stored page from before floe first wrote it counts as read. When `floe serve` cannot read it, it goes to `reads.json.broken` and a new one takes its place, every page then counting as read; mend it by hand to keep what it held. "Mark as unread", in the toast as a page you open becomes read and in a page's menus, takes it back to how it looked before you opened it, until you open it again. An archived page is never unread: its row has no dot, bold title or new version mark, nothing counts it, and neither the toast nor its menus offer to mark it unread.
- The scripts of a page drawn from a data file cannot send what it holds out with `fetch`, XHR, images from elsewhere or forms, its download included. Linked HTML loads what it loads.
- A page opens at the part its URL names (`/a/<slug>/#part`), and a move to a part inside it is kept in the URL, so that a copy of the URL or a reload opens that part.
- The management page downloads a page as one file: the stored page, or a linked file, with its CSP in a `<meta>`.
- A stored page of two versions or more shows what changed between any two of them, by their words under their headings: side by side, all together or the changes alone. It opens from `Diff` over the page, and a page newly at another version says what changed since the one read. Opening it reads nothing; closing it opens the page, which reads it. A link and a folder have no versions. Bold, a link's target and CSS do not show; a diagram or image whose words are the same is told as changed in shape.
- Printed from its frame, a page comes out one screen long, with a note to open it alone and print it there.
- The management page takes keys, and `?` lists those that work where you are. `Ctrl+K` (`⌘K`) searches the pages. In the list: `j` `k` or `↑` `↓` move over the rows, `Enter` opens a row whole, `x` selects (as does Select in a row's menu), `f` stars, `/` goes to the search; pressing a row whose page is open beside the list, as a double press does, opens it whole; `d` opens what changed in the page open. On a page open whole: `j` `k` go on through the rows that opened it, `h` goes where its back button goes, `f` stars, `d` opens what changed since the version read, whose `n` `p` go to the next and previous change and `d` closes, `/` searches the pages. After a press inside a page, `j` `k` `h` `d` `?` `/` `Ctrl+K` still work on a page floe drew, and `Ctrl+K` alone on a linked HTML file, which may use the letters itself.

## Environment variables

| Variable | Description |
|---|---|
| `FLOE_DIR` | Where pages are stored. Defaults to `$XDG_DATA_HOME/floe`, else `~/.local/share/floe`. |
| `FLOE_PORT` | The port (default `7325`). A decimal number from 1 to 65535. |
| `FLOE_ICONS_DIR` | Where `floe icons fetch` keeps icon sets, a directory each (`aws`). Defaults to `$XDG_DATA_HOME/floe-icons`, else `~/.local/share/floe-icons`. |
| `FLOE_TEMPLATES_DIR` | Where templates of your own are, a directory each ([templates.md](templates.md)). Defaults to `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`. |
| `FLOE_PARTS_DIR` | Where parts, looks and shared schemas of your own are ([parts.md](parts.md)). Defaults to `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`. |
| `FLOE_THEMES_DIR` | Where themes of your own are, a directory each ([parts.md](parts.md)). Defaults to `$XDG_CONFIG_HOME/floe/themes`, else `~/.config/floe/themes`. |
| `BROWSER` | The command that opens a new page. Defaults to `open` on macOS, else `wslview` or `xdg-open`. |

Each `FLOE_*` directory is an absolute path. floe reads one place wherever it runs, so a relative one is refused, and so is one starting with `~`, which no shell read (`FLOE_PARTS_DIR is ~/x, a relative path; set it to an absolute path`). A relative `XDG_DATA_HOME` or `XDG_CONFIG_HOME` is ignored, as the XDG spec says.

A command is told of the settings it uses alone, when it uses them. A directory set that cannot be used, as it is not a directory or cannot be read or written (or, for your own templates, parts and themes, is not there), is refused by its variable's name:

```text
floe: FLOE_PARTS_DIR is /tmp/t/missing, which cannot be used as a directory (no such file or directory); make it a directory floe can use, or set FLOE_PARTS_DIR to another directory
```

- `FLOE_TEMPLATES_DIR`, `FLOE_PARTS_DIR` and `FLOE_THEMES_DIR` are used by every command that draws a page or reads your own (`floe check`, `floe render`, the `template` and `part` commands, and the server, which draws), so one set that is not there stops them. Their default that is not there is none: floe draws with its own alone. A default that is there but is no directory is refused.
- `FLOE_DIR` is used by the server, and so by the commands [The server](#the-server) lists, and made when it is not there. `FLOE_ICONS_DIR` is used by the icons commands, which make it, and by the diagrams that show an icon, at the diagram.
- Without `HOME` (or with one that is not an absolute path), a directory that has no other default is none: `floe check` runs with floe's templates, parts and themes alone, while a command that writes there (`floe template new`, `floe list` for `FLOE_DIR` …) asks for the variable to be set.
- Keep `FLOE_PARTS_DIR`, `FLOE_THEMES_DIR` and `FLOE_TEMPLATES_DIR` apart: two that are one directory, or one in the other (through a symlink too), are refused for their names before anything in them is read, a setting not there yet too. A command compares only the ones it reads, and a default only when it is there; `floe part new` and `floe template new` refuse to make what would be in, or hold, the directory of another kind.

```text
floe: FLOE_PARTS_DIR (/tmp/pp) and FLOE_THEMES_DIR (/tmp/pp) are one directory, or one is in the other, where floe would read the files of one as those of the other; make FLOE_PARTS_DIR and FLOE_THEMES_DIR two directories, neither in the other
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Failure, and a value floe refuses (a refused publish, a missing page, a port in use, a data file or diagram spec with mistakes, an unknown example …) |
| `2` | A command line floe cannot read, or that lacks what it needs (an unknown flag or command, missing arguments) |
