# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add a task's ID to its Edit on the management page: changed, the task's pages (the archived too) and the parent of its child tasks move to the new ID in the same write, and the screen goes to the task's page by it. An ID of another task of the project is refused. The old ID names no task after.
- Add the theme `pebble`: rows as round stones on a tray, over a warm ground, each part in a form of its own, with a brown accent; a section's heading is a band from edge to edge.
- Add variables a theme of one's own sets on `:scope` in `base.css` for the headings of sections in the templates `report` (`--floe-heading-top`, the padding above) and `design` (`--floe-heading-start` and `--floe-heading-edge`, the padding and the line at the left), which `page.css` alone set. A theme that sets none is drawn as before.
- Add a way to make a new category in the dialog that adds a page on a task's page: "Add a category" beside the categories asks its name, and it is created as the page is put (not at all when the dialog is closed without one). A name already there is refused, in the task's Edit too, with "The category X is already there".
- Add trees that grow to the right, `direction: right` in a diagram's `tree`, and to both sides of the root with `left`, as a mind map does, where floe places every node; and an example of it, `floe diagram example mindmap`. A tree that grows down is written and drawn as before.
- Add `floe repository guide`, which prints how to change the documents a repository keeps (data files and the pages drawn) and how to set a repository up so that CI checks them; `floe guide` points to it for such documents.

### Changed

- Change `floe --version` and `floe version` of a floe built from a checkout (`go build`, `go install ./cmd/floe`, `just build`) to add the commit it was built from after a `+`, as `0.3.1+09feb55824ca`, and `.dirty` when the tree had changes, so that it is told from a release, which says its version alone as before.
- Change the theme `swiss` so that its boxes are seen to their edges: each (a table, a note, a card, an option, a fold and the like) is white in a black frame, wherever it stands, where it had no ground and a rule at its top or its left alone; a quote stands on white and a diagram on grey, the grey of a warning is darker, and so, slightly, is the page's ground.
- Change the mistake of a `for` of `checks.yaml` that does not lead to a list: there is one for each value in the data file that the path stops at, at its line and column, naming the `for` and its `checks.yaml`, where there was one for the check, at its `for`.
- Change the bar's "Page list" and "Task list" into a switch: the one of the screen shown stands on a white thumb in a grey track, and on a page shown whole, the one of the list it was opened from does. At 56rem or less (it was 40rem), a page shown whole has the back link's mark in their place. They are links as before, and go to the same places.
- Change the management page's list to show a long one sooner (with 300 pages, the rows come up about 80 ms earlier), and a page open whole a little sooner too: the items of a "…" menu are made as it opens, so the browser's find in page no longer finds the words of a closed "…" menu.
- Change a task's page in the management page. The way back to the task list, taken off in 0.3.0, is there again, as the mark `‹` on the left of the title; it goes where the bar's Task list and `h` go. "Edit" is on the right of the title, and the button that archives a task done is by its state. "Add a page" is now "Add" beside a heading over the pages that says how many they are, in a pill ("Pages 2"); the heading of the child tasks says its number in the same pill.
- Change `floe publish` to refuse, without `--force`, a page stored before 0.1.0, which does not say which file it is of, from any file, its own too; it was taken over by another file of its project, or by any when it had no project.
- **BREAKING**: Change `floe template check` and `floe part check` to refuse a `for` of `checks.yaml` that goes on past a value whose `type` takes neither keys nor a list (`"type": "string"`, and the items of a list of it), where the check found no item and passed unseen, or the data was refused. Mend the path, or remove the check: until then every page is stopped, as by any mistake in a template or part of yours.
- Change the skill of `floe skill install` to be used for writing and changing a repository's documents too. Those who installed it before replace it with `floe skill install <agent> --force`.

### Fixed

- Fix the title of a task's page in the management page wrapping under the task's ID: the ID is a line of its own, and the title starts on the next, each of its lines where the first starts.
- Fix the management page's star being hard to see on its grounds: a star pressed or pointed at, the shelf's favourites and the star of a row of the search or of Add a page are a darker gold, at a contrast of 3:1 or more on every ground.
- Fix a key held on Enter in Add a page sending again over a refusal, before it was read.
- Fix the rest of a press or a key that opens or closes a modal of the management page (the second press of a double press, a key held down) going through to the modal opened, to the backdrop or to what is under a modal closed: a double press on a row of another task's page under Add a page could move it unasked.
- Fix the bar of a page shown whole running off the side of a window of about 640 to 700 pixels, in Japanese, when the page is archived and has more than one version.
- Fix a task's page scrolling sideways when the task's title has a long run with no place to break: the title goes over lines.

## [0.3.1] - 2026-11-24

### Fixed

- Fix the management page's links among words showing no underline, so that they read as words, as they have since 0.3.0: Open the latest version and See what differs from the latest over an older version, See what changed, Open another project and Back to the page list where the task list has nothing to show, Back to the task list of a task not there, the link of a toast, and the page's URL on the printed frame. Back to the task list is as dark as the words over it again.
- Fix the management page's bar running off the side of a narrow window (in Japanese, 303px wide or less), as it has since 0.3.0: at 24rem (384px) or less, the logo is its mark alone on every screen.
- Fix the management page showing no icon in the browser tab, as it has since 0.3.0.

## [0.3.0] - 2026-11-24

### Added

- Add the licenses of the management page's build (React and the other packages built into it, and Tailwind CSS) to what `floe licenses` prints.
- Add `FLOE_INSTALL_TIMEOUT` to `install.sh` and `install.ps1`: how many seconds to wait for an answer while nothing comes, for each request (default 120).

### Changed

- Change the management page (the page list `/`, the task list and a task's page `/tasks`, and a page open whole `/a/<slug>/`) to screens built anew, the same in what they do. What looks or acts different:
  - Every screen: the bar's Page list and Task list are two plain links, the one of the screen shown darker, where they were a switch with the one shown in a box; the server's lost connection is said in the bar a little further from the logo; a screen that cannot be drawn says "Failed to show the screen (…)"; a screen reader reads Select and Copy the path in a page's `…` with the page's title, and the row of the page open beside the list as the current one (`aria-current`); in the tags dialog, a tag's whole name typed that begins other tags' (`Foo` beside `Foobar`) and Enter marks that tag, where nothing happened; the toast of a page read and the note of a newer version show for a page that is read again as soon as it is opened too; in the help, the two switches (about the screen, the keyboard shortcuts) are tabs with a line under the one shown, which ← and → move between, where they were buttons, the one shown on a black ground.
  - The page list: the bar of the selected pages ends with the word "Clear", where it had an x; the words that the version picked cannot be opened go away when that page is renamed; after saving an Edit opened in the search, itself opened with the focus on a row of the list, the focus goes back to that row.
  - The tasks: what a press on a task's page changes is drawn at once, not at the next read of the list; a press on a row of a task's page (or `j` `k`) that folds the list leaves the focus on Show the list, not nowhere; in a task's Edit, "Add a URL" and "Add a category" are buttons under the list that open a small dialog to type it in; a URL that is not http or https is refused there as it is added, not when the task is saved, and at 10 URLs the button opens no dialog but says so under the list; the hint under the categories no longer speaks of a +; a task's page has one "Add a page", beside "Edit", whose dialog chooses the category (No category at first), where each category's heading had one, and heads its pages of no category with "No category" while there are any; it has no link back to the task list of its own, the bar's Task list going where it went (the list at the task's state, the task's row focused).
  - A page whole: on a window wider than 40rem, the bar has the links Page list and Task list, neither darker, in place of the back button; the way back is the link of the list it was opened from, and for a page opened from a task's page, a plain back link with the task's ID stays beside the logo, whose words alone darken under the pointer. With the diff open, the bar's button that closes it is an x on a black ground, where it was the diff's icon. Escape in the search field of a page not found empties it and leaves it, where it only emptied it.
- Change `level: true` edges of a `graph` to join boxes and icons of any two groups, as between availability zones (they were refused unless in one group or directly in two lanes). The lowest group holding both is laid out as lanes are: the groups, boxes and icons directly in it stand as written, and the ends face each other. Something standing between the two after layout is told by name; the heading of any group, not only of a lane, is checked laid out `right`.
- **BREAKING**: Change an item that `prefixItems` takes in a list whose schema has `items` beside it (draft 2020-12): it now comes to `page.html` and `part.html` as written, as an item under `prefixItems` alone does, and a key the schema declares but the item leaves out is not there, not nil. Read such a key with `index` (`{{with index . "more"}}…{{end}}`): a page that read it as `.more` is now refused with `map has no entry`.
- Change a YAML mistake told on the line right after a `|` or `>` text, as when the text's first line is deeper than a line after it: after the YAML library's words, floe now says that the text ends before that line and how to mend it (indent the text's lines alike, or write its indentation as in `|2`). The words before and the place are as they were.
- **BREAKING**: Change `floe part check` and `floe template check` to refuse a `$ref` that leads back to itself through `$ref`s alone (a part's root naming a part whose root names it, a `$defs` naming itself), in the schema's file, at the `$ref`. No value passes such a `$ref`, which was told only once a data file wrote a value, at the data file's line. A schema of yours that holds one where no value reached it, or under `not`, `anyOf` or `if`, drew before and is refused now, by every command that reads your parts and templates.
- Change how much room an icon of a `graph` takes along the ranks: it grows by 12 (6 on each side of the icon) for each edge past the first at the busier of its top and its bottom (with `right`, of its left and its right), so an icon with 3 edges at its bottom and 2 at its top grows by 24. A diagram whose icons have many edges is taller (with `right`, wider) than before, even where no edges shared a run.
- Change `floe template check` and `floe part check` to read a schema's `type` in checking a `for` of `checks.yaml`: where the `type`s on the way leave the value only keys or only a list, the path is followed as that shape alone, and a key that shape may not hold is refused, where it was let pass as the value might have been of the other shape. The words of a refused `for` now say that no value the schema allows there holds the key, and to ask for a path of keys such values may hold, as a declared key may be refused too.
- **BREAKING**: Change expr, the library of `checks.yaml` and `computed.yaml`, to 1.18.1: an expression can use `merge` and `log2`, and slice a value that may be nil with `?.[a:b]`. A computed value named `merge` or `log2` is now refused as a function of expr, and an item of the data so named is no longer seen by name in `computed.yaml`: rename it. Two values may change without a mistake: the third argument of `sortBy` and `reduce` is now computed outside the items, so one that reads `#` there gives another value, and `sum([a, b, c])` adds what is written from the left, which may change the last digit of a decimal. Some of expr's words change, as for `take` with a negative number and `join` of a list that is not text.
- **BREAKING**: Change what counts toward expr's budget of one run of an expression: the text it makes now counts, a byte as 1 for each it adds to its longest input (`join` all but its longest part, `+` the shorter text, `toJSON` of keys and values or of a list its whole text), beside its lists, an item as 1, up to 1,000,000 together. Past it, expr says `memory budget exceeded`: `toJSON` of keys and values or of a list near 1 MiB, `join` of many texts adding up to 1,000,000 bytes, and an expression that compares each of 300 items with each other one by `toJSON` are now refused. Compare items by a key; `toJSON` writes text longer than the data, by its indentation and escapes.

### Fixed

- Fix a press lost, with nothing said, when the same was asked the other way and back before the management page had read its list again (as on a slow connection): taking a task off its parent, undoing it and taking it off again; archiving and restoring a task; archiving, restoring and archiving a page. Each is written now. A press for what is being written already is still dropped.
- Fix a press written twice, with two toasts, when a star, a pin, a page taken off its task, or the archive or the delete of the selected pages was pressed again before the management page had read its list again (as on a slow connection). It is written once.
- Fix a name of `computed.yaml` that expr reads as other than one name, as `not`, `let`, `if`, `in` or `1a`: `floe check` and `floe template check` told expr's mistake in reading it as an expression (`unexpected token EOF`), and now tell that it is not a name an expression and `page.html` can use, as for `a-b`.
- Fix the mistake of a name in `data-floe-look` that is no part or look: it said `names the part nope`, read as of `data-floe-part`. It now names the attribute and the name in it, for both: `the page drawn from this file names nope in data-floe-look, which there is none of; use …`, and only that name where the attribute holds others too.
- Fix `floe rebuild` leaving a stored page whose copy has no `page.html` (as a store of an older floe can have) as it was, told to `fix the data file it was published from` though nothing was wrong with it: there is no page to compare with, so it is drawn again and written, its versions and news as they were. A `page.html` that is there but cannot be read still keeps the page, told as before.
- Fix a schema's mistake against JSON Schema's own rules where the rule takes one of some forms, as `type` (a name, or a list of names): `floe check`, `floe template check` and `floe part check` now tell the library's words for each form, where it is, in place of `'anyOf' failed` alone. `"type": "strin"` now tells `#/properties/a/type: value must be one of 'array', 'boolean', 'integer', 'null', 'number', 'object', 'string'` (and `got string, want array`, of the list form). A data file's mistake against an `anyOf` or `oneOf` of the schema is told as before, as one.
- Fix floe stopping with `laid out a mistake of its own, diagram_runs_through` on some `graph`s with icons and labelled edges (mostly laid out `right`): a labelled straight edge between two icons no longer moves onto one icon's point where the turn back to the other icon's point would cross a label or a box. A few graphs with icons that drew before now run that edge or the edges beside it another way, their labels elsewhere.
- Fix `floe diagram inspect` of a `graph` laid out `right` whose level edge is refused for running through a heading: an edge written by hand now goes in at the sides the refused edge took, which it went round though that edge is not drawn.
- Fix a page drawn of any size: one larger than 80 MiB of HTML is now refused at its data file, by `floe check`, `floe render`, `floe template check` and the server alike (`the page drawn is 96 MiB, over the 80 MiB a page may be`).
- Fix the server stopping, with every page, on a page whose expressions or `page.html` repeat a long text for each item: it draws each page in a process of its own, which may hold up to 512 MiB of memory and take up to 30 seconds; past either, that page is refused at its data file and the server serves on. This holds for publishing, opening a linked data file's page, picking a theme for a page and `floe rebuild`, which draws the other pages on. A server left running while floe is installed again tells so in place of drawing, till a floe command (`floe list`) starts it again. Stopping `floe rebuild` (Ctrl+C) now stops the drawing too, the pages not drawn again yet keeping the page they had. Publishing and opening a linked data file's page take about 0.1 seconds longer.
- Fix `floe check`, `floe render`, `floe template check` and `floe diagram render` and `inspect` taking gigabytes of memory, or stopping with `fatal error: out of memory`, on a page whose expressions or `page.html` repeat a long text for each item: drawing may now hold up to 512 MiB of memory (in one `floe render`, all its pages together), and past it floe stops and says so at the file it was drawing (exit code 1).
- Fix the diff of a version of a page cutting a line with characters of more than one byte (Japanese, for one) at the wrong place when a character is added next to the same one: when `はい あ漢` became `はい ああ漢`, it was shown as `はい あ`{+あ+}`漢`, and is now shown as `はい `{+あ+}`あ漢`, the added word starting where it does, as in a line of single-byte characters.
- Fix `floe part new`, `floe theme new` and `floe template new` making a part, a theme or a template inside a directory that `FLOE_TEMPLATES_DIR`, or a default directory itself (such as `~/.config/floe/templates`), leads to as a symlink to a directory not there yet: once it was made, every command reading both directories stopped, telling that they overlap. A symlink to a directory not there is now compared where it leads, so they refuse before making anything; `floe check` and the other commands tell the overlap before it is made too, when the other directory is there.
- Fix `items` beside `prefixItems` drawing the items `prefixItems` takes too: a part or a format of floe's in `items` drew every item of the list, though JSON Schema takes `items` only for those after `prefixItems`, and a value it did not check stopped the drawing at the data file's line. It now draws only the items after them.
- Fix a `for` of a template's or a part's `checks.yaml` refused as a path of keys the schema does not allow, where it goes through a list whose first items `prefixItems` takes and `items` does not. Such a path is no longer refused, and a path of a key no item may hold is not refused there either.
- Fix `install.sh` and `install.ps1` waiting forever when the server or a proxy on the way went silent: they give up after 120 seconds without an answer, and say what they were fetching and what to do. The whole download has no time limit, so a slow one still finishes. A download that stops midway is given up on after 5 minutes by Windows PowerShell 5.1 and never by PowerShell 7.0 to 7.3.
- Fix a key pressed just after a modal of the management page closed reaching the closed modal: Enter right after Escape could save the change Escape cancelled in the edit modal, or open the page picked in the search modal. The focus now leaves the modal as it closes.
- Fix Enter pressed right after Yes in the ask to move a page to another task: Yes could be lost, the page left where it was and the modals left open.
- Fix a modal of the management page closed while it saved: the save went on and what came after it acted on whatever modal was open by then. It could close a modal opened again for another page, send that modal's next save to the page the first save renamed, so that page was renamed and retitled, or say a failure where it could not be seen. The modal now holds still while it saves and reads the list after: Escape, Cancel, a press outside and its buttons do nothing till the save is answered, and a failure is said in it. A save that never answers holds it till the page is loaded again (F5, Ctrl+R).
- Fix a press begun in a modal's field and let go outside it closing the modal as a press outside does: only a press that goes down outside closes it now.
- Fix floe crashing (`fatal error: stack overflow`) while drawing a page with a list whose schema has `prefixItems` beside an `items` naming a part of yours, when that part's root and another's lead back to each other by `$ref`: such parts are now refused when they are read.
- Fix edges at one side of an icon of a `graph` turning beside the icon on one line, where they drew as one thick line that hid which edge went where (as three edges from one icon to boxes in two groups, laid out `right`). They now turn on lines 6 apart.
- Fix an edge of a large `graph` left running through a frame that holds neither of its ends, where floe gave up looking for a way round it too soon. It now goes round in more diagrams; one that floe finds no way round still runs through.
- Fix a `for` of a template's or a part's `checks.yaml` refused where the value may be keys or a list: under `items` beside `"additionalProperties": true` or no `additionalProperties` (`{}` let it pass), and under `"additionalProperties": false` with no `type`. From a schema of no `type` on the way, or of a `type` that lets the value be keys and a list, the path is not checked further: write `type` on every schema on it, the root too, to have it checked.
- Fix an edge of a `graph` that ends at a group ending off the group's frame, by its corner, or beside a heading: as floe moved such an edge to meet an icon at its other end (straight onto the icon's point, or slid with its label), the end at the group could move past where the edge was first placed. That end now stays on the frame's side, 12 or more in from its corners, clear of the headings of the group and the groups round it as they are drawn, and not close beside another edge. The edge bends where it cannot run straight.

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

[Unreleased]: https://github.com/sukekyo26/floe/compare/v0.3.1...HEAD
[0.3.1]: https://github.com/sukekyo26/floe/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/sukekyo26/floe/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/sukekyo26/floe/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/sukekyo26/floe/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/sukekyo26/floe/releases/tag/v0.1.0
