# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add `FLOE_INSTALL_TIMEOUT` to `install.sh` and `install.ps1`: how many seconds to wait for an answer while nothing comes, for each request (default 120).

### Changed

- Change an item that `prefixItems` takes in a list whose schema has `items` beside it (draft 2020-12): it now comes to `page.html` and `part.html` as written, as an item under `prefixItems` alone does, and a key the schema declares but the item leaves out is not there, not nil. Read such a key with `index` (`{{with index . "more"}}…{{end}}`): a page that read it as `.more` is now refused with `map has no entry`.
- Change a YAML mistake told on the line right after a `|` or `>` text, as when the text's first line is deeper than a line after it: after the YAML library's words, floe now says that the text ends before that line and how to mend it (indent the text's lines alike, or write its indentation as in `|2`). The words before and the place are as they were.
- Change `floe part check` and `floe template check` to refuse a `$ref` that leads back to itself through `$ref`s alone (a part's root naming a part whose root names it, a `$defs` naming itself), in the schema's file, at the `$ref`. No value passes such a `$ref`, which was told only once a data file wrote a value, at the data file's line. A schema of yours that holds one where no value reached it, or under `not`, `anyOf` or `if`, drew before and is refused now, by every command that reads your parts and templates.
- Change `floe template check` and `floe part check` to read a schema's `type` in checking a `for` of `checks.yaml`: where the `type`s on the way leave the value only keys or only a list, the path is followed as that shape alone, and a key that shape may not hold is refused, where it was let pass as the value might have been of the other shape.
- Change the words of a refused `for` to say that no value the schema allows there holds the key, and to ask for a path of keys such values may hold, as a declared key may be refused too.

### Fixed

- Fix `items` beside `prefixItems` drawing the items `prefixItems` takes too: a part or a format of floe's in `items` drew every item of the list, though JSON Schema takes `items` only for those after `prefixItems`, and a value it did not check stopped the drawing at the data file's line. It now draws only the items after them.
- Fix a `for` of a template's or a part's `checks.yaml` refused as a path of keys the schema does not allow, where it goes through a list whose first items `prefixItems` takes and `items` does not. Such a path is no longer refused, and a path of a key no item may hold is not refused there either.
- Fix `install.sh` and `install.ps1` waiting forever when the server or a proxy on the way went silent: they give up after 120 seconds without an answer, and say what they were fetching and what to do. The whole download has no time limit, so a slow one still finishes. A download that stops midway is given up on after 5 minutes by Windows PowerShell 5.1 and never by PowerShell 7.0 to 7.3.
- Fix a key pressed just after a modal of the management page closed reaching the closed modal: Enter right after Escape could save the change Escape cancelled in the edit modal, or open the page picked in the search modal. The focus now leaves the modal as it closes.
- Fix Enter pressed right after Yes in the ask to move a page to another task: Yes could be lost, the page left where it was and the modals left open.
- Fix a modal of the management page closed while it saved: the save went on and what came after it acted on whatever modal was open by then. It could close a modal opened again for another page, send that modal's next save to the page the first save renamed, so that page was renamed and retitled, or say a failure where it could not be seen. The modal now holds still while it saves and reads the list after: Escape, Cancel, a press outside and its buttons do nothing till the save is answered, and a failure is said in it. A save that never answers holds it till the page is loaded again (F5, Ctrl+R).
- Fix a press begun in a modal's field and let go outside it closing the modal as a press outside does: only a press that goes down outside closes it now.
- Fix floe crashing (`fatal error: stack overflow`) while drawing a page with a list whose schema has `prefixItems` beside an `items` naming a part of yours, when that part's root and another's lead back to each other by `$ref`: such parts are now refused when they are read.
- Fix a `for` of a template's or a part's `checks.yaml` refused where the value may be keys or a list: under `items` beside `"additionalProperties": true` or no `additionalProperties` (`{}` let it pass), and under `"additionalProperties": false` with no `type`. From a schema of no `type` on the way, or of a `type` that lets the value be keys and a list, the path is not checked further: write `type` on every schema on it, the root too, to have it checked.

## [0.2.0] - 2026-11-17

### Added

- Add Windows 11 (amd64, arm64) builds, with `install.ps1` to install them (`irm https://raw.githubusercontent.com/sukekyo26/floe/main/install.ps1 | iex`, into `%USERPROFILE%\.local\bin`). floe keeps its places under `%USERPROFILE%` as it does under `~` on Linux, and opens only what is on a local drive under an ordinary name: a network share or drive, a device, a pipe or a stream's name is refused, and so is linking a file in `AppData` (`%TEMP%` too). docs/commands.md's Windows section tells what else differs.

### Changed

- Change the refusal of a command when the server on the port serves a store that is not an absolute path on this system (a server of another system, such as floe in WSL seen from Windows): it now says only to set `FLOE_PORT` to another port, and no longer to set `FLOE_DIR` to that store or to run `floe stop`.
- Change `floe licenses` on every system to print the license of `github.com/inconshreveable/mousetrap` too, which the Windows builds hold.
- Change `install.sh` on a system other than Linux and macOS to say that Windows installs with `install.ps1`, and how.
- Change the refusal of an image path a data file names to say that on Windows a `\`, a `:` or a device's name such as `NUL` or `CON` is not taken either; an image name holding a NUL byte is now refused with it.
- Change a linked folder and the directory of a linked file to serve no file or directory whose name is not UTF-8: its URL now gets a 404.
- Change the refusal of a link in a hidden directory of your home to name the folders of application data too, and to say where to put the file instead.
- Change the refusal of a link that is not its real path, which named Go functions to call: it now asks for the whole path with its links and junctions followed, as the system names it.
- Change what `install.sh` says when the install directory is not on `PATH`: it shows the command that adds it, to paste as it is, for zsh (`~/.zshrc`), bash on Linux (`~/.bashrc`) and fish (`fish_add_path`), and the line alone for other sh-like shells; it still changes neither `PATH` nor a file. A directory given as a relative path is told and compared by its absolute path, so a directory already on `PATH` is no longer told of.
- Change `floe guide` to tell agents to fix a mistake without a place that is about what they gave the command (a path, a flag, a slug, a task, a `FLOE_…` they set themselves), and to tell the user of any other, changing none of the user's settings and stopping no server. It called every mistake without a place a failure to tell the user of.

### Fixed

- Fix a change of a page's slug that could not move the page: it saved the description, tags, project and title all the same and lost the rest. The page is now moved first and nothing is saved when it cannot be; one moved whose `meta.json` cannot then be written is moved back, or, when that fails too, refused with a mistake that says only the slug changed and the other changes are to be saved again.
- Fix the server not starting when its log over 1 MiB could not be moved aside to `server.log.1` (a directory there, an older server still holding it): it now writes on in the log, with a line of why it was not moved, and tries again at the next start.

## [0.1.1] - 2026-11-16

### Changed

- **BREAKING**: Change `floe task --categories` to a flag given again for each category, as `--url` is (`--categories designs --categories samples`), so a name may hold a comma; `--categories ""` still takes them all off. `floe task` now prints the categories each quoted, as the URLs: `categories "a" "b"`, not `categories "a,b"`, and `no categories`, not `categories ""`, after a write took the last off. A refusal of a category with pages quotes its name (`category "designs" of task …`). Its lines, and `floe info`'s values (title, task, category, tags and the rest), print a full-width space as it is, not as `\u3000`.
- Change the task mode's ask in `floe guide`: agents ask the user in a dialog of choices (a reply of only the question where they have none) which task to put each new page on, the first two open tasks, a new task or none, then which of its categories.
- Change the order of `floe task` and the task list: the task changed last comes first, a field changed or a page put on it or moved to another of its categories, not only its state; the time a task shows is of that change. `floe task --slug … --task …` now tells such a task `changed` on a line of its own when the page alone was written.

### Fixed

- Fix refusals of a task of no project, which said `project /`: `floe task` and the management page now say `no project` (in Japanese `プロジェクト未設定`, after the task list's `未設定`; a named project is now `プロジェクト「floe」` there), and `--parent` naming a task of no row says `no task #42 in project app can be a parent`.
- Fix `floe publish --link` with `--move-project`, refused without `run floe --help` and with exit code 1: the refusal now ends with `run floe --help` and exits 2, as other command line mistakes do.

## [0.1.0] - 2026-11-16

### Added

- Add `floe publish`, which draws a page from a data file (YAML) with a template and shows it on a local server (`http://127.0.0.1:7325`), started when it is not running; `--link` shows an HTML file or a folder of your own as it is.
- Add `floe check` and `floe render`, which check data files and write pages to keep in a repository, and `floe list`, `floe info`, `floe tag`, `floe archive`, `floe rm`, `floe rebuild`, `floe serve` and `floe stop` for the stored pages.
- Add mistakes told at the file, line and column of what is wrong, with how to mend it, and warnings that advise a better way to write.
- Add floe's templates `report`, `design`, `handoff`, `adr`, `postmortem`, `runbook` and `slides`, and templates of your own (`floe template new`).
- Add floe's parts (text, code, terminal, diff, table, entries, steps, stats, bars, chart, checks, asks, options, columns, image and more) and parts of your own (`floe part new`).
- Add twelve themes and themes of your own (`floe theme new`); people pick the default and a page's theme on the management page.
- Add diagrams drawn as checked inline SVG: frames, boxes, edges, tables, trees, a `graph` laid out from its edges, and examples (`floe diagram example`).
- Add icons for diagrams: Lucide built in, and the AWS, Google Cloud and Azure sets kept by `floe icons fetch` and found by `floe icons search`.
- Add the management page: the pages by project, search (`Ctrl+K`), tags, pins, archive, a page's versions and what changed between them, unread pages, and settings.
- Add projects, named after the git repository a page is first published from (`--move-project` moves it).
- Add tasks: a task list by project and state, with child tasks, outside links in order, and a page's category on its task; `floe task` reads and writes them.
- Add the task mode, which has `floe guide` tell agents to ask the user, after publishing a new page, whether to put it on a task.
- Add `floe guide`, the guide for agents, with `floe task guide`, `floe diagram guide`, `floe part guide`, `floe template guide` and `floe theme guide`, and `floe skill install`, which points an agent's skill to it.
- Add a page's CSP and sandbox, which keep its scripts to floe's own.
- Add `floe licenses`, which prints the licenses of floe and of what is built into it.

[Unreleased]: https://github.com/sukekyo26/floe/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/sukekyo26/floe/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/sukekyo26/floe/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/sukekyo26/floe/releases/tag/v0.1.0
