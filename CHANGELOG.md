# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add `lucide`, an icon set built into floe: the 2,052 icons of [Lucide](https://lucide.dev) 1.72.0 (ISC), such as users, a browser, a phone, a server, a database, a file, a key and mail (`lucide/users`, `lucide/database`), each in the category Lucide gives it, for the `icon` parts of diagrams. It needs no fetch and no `FLOE_ICONS_DIR`: `floe icons search lucide user` finds its names, with short names such as `db`, `person` and `browser`, and lists first, in any set, an icon whose name but for its folder is the word (`aws/res/server` for `server`), and `floe icons fetch lucide` is refused, saying so. Its lines draw in the diagram's text colour, or in the colour of a `tone`, which the `icon` part and a graph's `icon` now take; an icon of the clouds' sets keeps its own colours, and a `tone` on one is refused. The license is in `internal/icons/lucide/LICENSE`, named by the README (S338).
- Add the Google Cloud and Azure icon sets to `floe icons fetch`, beside AWS's, for the `icon` parts of diagrams (`gcp/cloud-run`, `gcp/category/networking`, `azure/virtual-machine`). `floe icons fetch gcp` downloads the two zips of Google Cloud's page, the core product icons and the product category icons, which Google's guide has every other product take; it does not fetch the legacy icons Google has deprecated. Azure's page lets its zip be downloaded only after agreeing to its terms, so `floe icons fetch azure` downloads nothing and says where to agree: download the zip there and pass it or its URL. `floe icons fetch` takes one zip or URL for each package of a set (two for `gcp`) and refuses more. `floe icons search` takes the set first and searches it alone (`floe icons search gcp storage`), so that a diagram of one cloud does not take another's icon; a set not fetched is told with how to fetch it, and a short name such as `k8s` finds the set's own service (S337).
- Add `floe part guide`, `floe template guide` and `floe theme guide`, which print how to make a part, a template and a theme of your own: the docs themselves, as `floe diagram guide` prints its doc, so that an agent outside floe's repository reads them. `floe part --help`, `floe template --help` and `floe theme --help` point to them. The themes are now in a doc of their own, `docs/agents/themes.md`, out of `docs/agents/parts.md`; the docs floe prints name another doc by the command that prints it, not by a link (S329).
- Add `floe theme new <name>`, which makes a theme of your own in `FLOE_THEMES_DIR`: an empty `palette.css` to fill in, over floe's `default` palette, as `floe part new` makes a part; nothing is written over. Add `floe theme check`, which does what `floe part check` does: a theme's `<part>.css` needs its part, so parts and themes are checked together. `floe guide` says that templates, parts and themes can be made of your own, and that their `--help` tells how (S329).
- Add telling a project's open tasks to `floe work`: with no slug and no flag but `--project`, it prints the tasks that are todo or doing, the latest change first, a line each as `--task` prints a task, and writes nothing (it was refused). `--project` names the project, the working directory's when left out; a project with no open task is told so, which is not a refusal. A flag but `--project` with no `--task` and no slug is still refused, and the refusal says how to tell the open tasks. `floe guide` tells the command.
- Add `server.log` to the store, written by the server alone: that it starts (with its pid and build), that it stops and why (`floe stop`, a command of another build, SIGTERM or SIGINT), that it stopped, a request that panicked with its stack, and what the runtime wrote when the server crashed. A `server.log` over 1 MiB is moved to `server.log.1` at the next start, which replaces the one before; one that is not a regular file (a FIFO, a directory) keeps `floe serve` from starting. A command that stops a running server of another build says so on stderr (`stopping the server on port 7325 (pid 1234), which was of another build of floe, to start this build's`), also when no new server starts after it or the server does not stop (S229, S273).
- Add reading a page's task to `floe work`: with slugs and no flag, it prints each page's task, category and pin as a write does, and writes nothing (it was refused).
- Add `floe publish --move-project`, which puts the page in the project of the directory floe publish runs in, though it is in another: publish again from the repository the page is about to mend a page that went into the wrong project. Without it a page keeps its project. It is refused with `--link` and where there is no project. It moves nothing of the page's task. `floe guide` tells an agent to do so, in place of telling the user.
- Add `floe guide`, the guide for agents: how to write, check and publish a page, pick the template and the parts by what the page holds, write YAML that floe reads as meant and read its mistakes, a summary of the docs in the binary. `floe --help` points agents to it.
- Add `floe skill install <agent>` (claude, codex, copilot, cursor, gemini), which writes a skill that points the agent to `floe guide` where it reads the skills of all your projects: `$CLAUDE_CONFIG_DIR/skills` or `~/.claude/skills` for claude, `~/.agents/skills` for the others. It leaves this floe's skill as it is, keeps another unless `--force`, and follows links named `floe` and `SKILL.md` only to a directory and a file of those names.
- Add Theme to a page's menu (…) on the management page, for a page drawn from a data file, stored or linked: the theme picked there, from the same cards as the settings', draws that page alone again and wins over `--theme` and `theme:`, kept when the page is published again or rebuilt and dropped when another file's page replaces it. Remove this page's theme, its first card, gives the page back to the theme it was stored with, which the card names. A theme the page cannot be drawn in leaves it as it was.
- Add the management page's settings (the gear in its bar), to choose the default theme from cards of a sample page drawn in each theme, floe's and your own, each described (floe's in the page's language). A page that names no theme takes the default when it is first stored and keeps it: choosing another changes the pages stored from then on only. `floe theme default` prints the default chosen, nothing for none. `floe check` and `floe render` still draw in `default`.
- Add cards for the rows of the page list, a task's page and the task list: each row a card with an edge, round corners and a light shadow, apart on the list's ground, a card chosen edged in blue; a folder open is one card with its files. The rows are lower too: less room above and below each, and their buttons 26px (the room between the title and the line under it is kept, for the ring of a button reached by the keys).
- Add your own parts to what a section of floe's templates takes: a part in `FLOE_PARTS_DIR` joins `any`, so report's `parts` (and those of every template of floe's, and a schema of your own naming `any.json`) take it as they take floe's, and `floe part list` lists it with them. It does not join what `details`, `tabs`, `columns`, `options`, `entries` and `checks` hold, as a part of your own may hold a form or HTML, which those would break on.
- Add a description to a theme: `description:` in a `theme.yaml` in its directory, which `floe theme list` and `floe theme show` print. Each of floe's themes has one.
- Add the shape to the lines of `floe template list`, `part list`, `theme list` and `diagram example`, between where an entry is and its description: the keys a value of it takes, `[] ` before those of a list's items, `one of ` when exactly one is written, the name of its format or floe part, or `-` where the list cannot tell it right (`floe part show <name>` names in its `files:` line what to read). `floe diagram example` lists its examples in the same four columns.
- Add `floe tag <slug>... --add <name> --remove <name>`, which puts tags on pages and takes them off from the command line, in order, stopping at the first page not there. A name is one as written, commas and all; the names to remove are taken off first, so a name given to both stays on. The management page's tag dialog now sends only the tags changed in it, so that tags written meanwhile by `floe tag` or another tab stay.
- Add every part but `tabs`, `columns` and `html` to what `details`, `tabs`, `columns` and the `parts` of an item of `entries`, `checks` or `options` hold: `options`, `steps`, `facts`, `stats`, `details` in `details` and the rest. An `html` part stays out, as its `</form>` would close the form of `tabs` around it.
- Add `parts` to an option of `options`: parts under its title, above its text, such as a picture, a diagram or code of the option. The cards on a line start their parts and their text at one height, the parts `--floe-room` from the title and from the text, a small picture kept at its size; on paper such a card goes to the next page whole. An option's `text` is a `text` part now, its fenced code warned of as in `entries`. Options without `parts` look as before. A theme changes the room in a card by `--cards-gap`.

- Add contents to `report`: on a page of 4 sections with a heading or more, or as its new key `toc` says, the sections and their subsections stand beside them on a screen 1000px wide or more, where the page takes the whole window, the section being read marked as the page scrolls; on a narrower screen the sections alone stand over them, and paper holds none. A template of your own takes them by the look `toc` and the function `contents`, and `markdownLabel` writes a line of Markdown with its links as their words.
- Add `tags` to the part `steps`: short values under a step's title, each a line of Markdown or a pill, as entries' tags are, such as the PR a step is done in. Both draw them by a new look `tags`, which one's own parts and templates may take too. A page writing none looks as it did, but for entries' tags on a face a template paints (a band, a face of one's own template), which take the face's text colour where they were grey.
- Add `title`, `start` and `mark` to the part `code`: `title` says what the code shows, in words, above it beside its `file`; `start` numbers the lines from that number, and `mark` marks lines by those numbers, with a line at their side, a ground to the end of the longest line and a word a screen reader reads. The copy button copies the code as written, without the numbers. A page writing none of them looks as it did.
- Add rows written by the columns' ids to the part `table`: a column takes an `id`, and a row is `{ state: done, who: walter }`, a column left out drawn empty and a key that is no column's `id` refused. Rows in the order of the columns stay as they are, and a table may hold both. Parts and formats may now stand under `additionalProperties` in a schema of one's own, where floe draws them; not at the root's, which would draw `lang` and `title` too.
- Add `from` and `better` to the part `stats`: `from` is the number before, drawn as `3.1 → 1.3` with the change under it, exact and in its colour, for the better or the worse as `better` says. Pages without `from` are drawn as before.
- Add `unit` to a bar of the part `bars`, drawn after its value (`240 s`, `1,200 / 1,840 pages`), apart by a margin as `stats`' unit is, in the text face and the muted colour, and in the `aria-valuetext` of a bar with `of`. A short unit in Japanese is kept whole, and a long one wraps in the value's column. A `unit` that is not text is refused. Bars without `unit` are drawn as before.
- Add the task state `archived`, so that a task of no page can be archived: "Archive" now shows on any task that is done, and `floe work --task X --state archived` writes it. **A task's place on the list is now its state alone**: archiving all a task's pages with `floe archive` no longer moves the task, and a task of the archive with a page published again stays there, with its unread shown. The button archives the task alone, as `--state archived` does: its pages stay as they are, and are archived on their own, by their rows or `floe archive`.
- Add a × to each pin on the task list, which takes the pin off there, without going to the page list or the page.
- Add the part `chart`: numbers over a row of names, as a line or as upright bars, with up to 4 series, on one scale from 0 (`min` moves a line's start), at the page's text size at every width, with the numbers in a folded table. Each theme has its own four colours, `--chart-1` to `--chart-4`; brutal, blueprint, notebook, retro and washi draw their lines and bars in their own shapes.
- Add the part `checks`: what was checked and what came of it, in a survey or a test, each with ok, ng or unchecked, the actual result and how it was checked, in the order written, the count of each result on top.
- Add "+ Task" (Create task) to the task list, under the projects of its left column (in Filter on a narrow screen): it makes a task of the project shown from a task ID and a title, then goes to To do with the new row marked; an ID already there offers "Open the task". The screen of no task anywhere has the same button and asks for a project. The "Page list | Task list" switch now shows with no task too, and the words of the screen of no task say how to make one. The screens of no task and of no page no longer show a command
- Add the part `asks`: questions put to the reader, numbered Q1, Q2 and on, with their options, what is recommended and why, who decides, and `decided` for what was. `handoff` draws its `asks` by it, so that each question is now a card, an option recommended is ringed among the options, `decided` may be written there too, and the count at the head of the page is of the questions still to decide.
- Add a parent to a task from its page: "+ Add" over its parent task takes a task there or makes a new one in the same write, "Change" moves it, and a button on a parent's or child's row takes the child off at once, with "Undo" in the toast. A task written from a screen is refused, and read again, when another session changed its parent meanwhile.
- Add `caption` to the part `image`: a line of Markdown shown under the image. An image drawn smaller than its size, to fit the column, now zooms as a diagram does: the toolbar over it, Ctrl / ⌘ + wheel, a pinch and the keys `+ - 0`, with a drag or the arrow keys to move while zoomed; it shows whole on paper. Parts of one's own zoom the same way with the function `{{zoom}}` and the look `zoom`.
- Add one form to the buttons of words on the management page: square corners, a thin edge, 2rem high and 2.25rem at a modal's foot; round stays for the buttons of an icon. The Set the task and child task dialogs name their fields Task ID and Title, with an example, as the step field has too, and a new task needs a title; a task saved from a screen read before another change is refused, and the tasks are read again.
- Add the part `columns`: parts side by side, to see at once, such as a screen before and after. Each column is an optional `title` over `parts`, which hold what `details` holds; they are of one width, and fold under one another where they don't fit.
- Add the task state `todo`. A new task starts in it, whichever way it is made. The list's column goes in the order of the work, To do, In progress, Done, Archive, opening on In progress, each state the latest changed first.
- Add child tasks. `floe work --task "#130" --parent "#123"` (with pages too: `floe work design-notes --task "#130" --parent "#123"`) makes a task the child of another of its project, one level deep, and "+ Add" on a task's page makes a new one under it or takes one that is there; `--parent ""` takes it off, and `--forget` is refused while a task has children. On the task list a child's row names its parent and a parent's row says how many children it has; a task's page lists its children, or its parent, over its pages. A parent with no page of its own moves to the archive by its children's pages.
- Add the task screens. `floe work` says which task a page is on, its step and pin, and a task's state, outside link, title and order of steps, held in `tasks.json` in the store, and tells each task and page it wrote on a line as the store left them (a task made, changed or unchanged); pages keep it when published again, renamed or drawn again. The task list at `/tasks` shows one project's tasks by state, latest changed first, with their step, pages, unread and link, that project's pinned pages over them, and a switch to it beside the logo once a page is on a task or pinned. A row opens the task's page: its pages under their steps, the archived after the rest of each, dimmed and marked Archived, its state to change, and a button that archives the task once it is done. A page's `…` menu pins it, opens its task and sets its task and step, a task's page edits the task's title, URL and steps (reordered, the names kept, by a drag of a step's grip or its up and down buttons, added by the last row, and removed when of no page, archived or not), puts a page of the project under a step by the step's Add a page, and a page's `…` takes it off its task, with a way back in the toast, and the page list's rows show a page's task and pin. A task written from a screen, its state, archive, parent, Edit, the Set task dialog or Add a page, is refused when it was written after the screen read it, and the tasks are read again.
- Add one axis to the points of an edge's `via` in a diagram: `{ x: 710 }` or `{ y: 218 }`, the other axis being the point before's, or that of the centre of the edge's start, so that the point does not copy where a part stands. The docs now tell to write only the turns you choose, and which points cannot be left out.
- Add `parts` to the items of the part `entries`: the parts `details` holds, under the item's `text`, so that code in an item is drawn by `code` and `terminal`, under its file or prompt with a copy button.
- Add `with: <name>` to the boxes and icons of a diagram's `graph`: it stands one in the rank of the box or icon named, with no edge drawn, of another group too, and takes the edges from boxes into it out of the ranks, so that an edge runs the way it means, up from the servers to their gateway. An AWS diagram of 25 edges goes from 2136×1440 to 1704×1407 with it, as boxes outside a group's frame no longer stand beside it.
- Add data files to `floe diagram inspect`: a file with a key `template` at its top has its diagrams inspected in the order written, in `details` and `tabs` too, after the template's schema, without drawing the page. Add a line for each frame's heading, `the heading of frame "F" x,y WxH`, which tells how far to move an edge told it runs through one.
- Add straight edges between facing sides that share a height, instead of a bend by the difference of their middles: an edge written by hand, when the straight line would touch nothing, and the edge of a graph at an icon that cannot move far enough, whose end alone on its side stands up to 12 off its point. If that leaves a mistake, a label with no place written landing on something say, all the diagram's edges stay as they were.
- Add a choice of versions to the right of a stored page's title on the management page. An earlier version shows with the
  choice in yellow and words over the page that lead to the latest and to what differs from it; viewing it does not mark the
  latest as read. The frame's URL takes `?v=<n>`, and a version that is not there shows the latest and says so.
- Add equal parts for edges written by hand: the n ends on a side of a box, an icon, a table or a frame now stand at the points that part it into n+1 equal parts, instead of all at its middle. An edge running straight stays straight. An edge that would run into something stays in the middle: move a label in the way to part the edges. To branch from one point, write `via` or use a `tree`.
- Add equal parts for the edges of a diagram's `graph`: the n ends on a side of a box now stand at the points that part it into n+1 equal parts, instead of on the columns the edges were routed by. A straight edge stays straight, and an end that would land on something, cross more edges or run close beside another stays where it is. A box's loops and level edges do not count.
- Add what changed between two versions of a stored page to the management page, shown on the later version's page itself, in its
  theme and parts: words added underlined, words gone struck through where they were, what is gone of a list, a section or a part
  put back in its shape by the part it was in (before it when it came first), a table's rows and cells gone in their place in it and an ol's items with their
  numbers, a diagram or image that changed framed with its earlier version to open under it. A block is framed only where its words
  cannot show what changed. The two versions to choose, contents to resize or hide, and `n` `p` to go through the changes. `Diff`
  over a page and `d` open it; a page newly at another version says how many sections changed since the one read. Opening it marks
  nothing read. The page is shown in the sandbox as any page; the management page writes the contents as text. The same versions
  answer the same however busy floe is: the comparison is bounded by its size, not its time.
- Add `icon: { name, icon, label }` to the nodes of a diagram's `graph`, so that an architecture diagram with icons, an AWS one say, needs no coordinate but the graph's `x` and `y`. An icon has no box round it and stays 48 square; the n edges at a side of it end at the points that part the side into n+1 equal parts. A `group` or a lane takes an `icon` too, in its frame's corner. `floe diagram example aws-graph` shows them.
- Add a modal that searches the pages, opened by `Ctrl+K` (`⌘K`) anywhere, by `/` on a page open whole, or by the magnifier on a
  phone's whole page: it searches by title, description, slug, project, path, template and tag, the archived last, opens a row
  whole, and acts on the row reached with the buttons of a row. Add a help modal of two parts: the keyboard shortcuts, opened by `?`, and About the page list or About the task list, which says what that screen is and what its words mean, opened by the `?` button in the bar (on a window wider than 40rem). On a page open whole, `h` goes back to the list. With the focus in a page, `j`, `k`, `h`, `?`, `/` and `Ctrl+K` now work too (`Ctrl+K` alone in a link's HTML and a folder's,
  which may use the letters themselves).
- Add marking a stored page unread: a toast offers it for 8 seconds as a page you open becomes read, and "Mark as unread" stays in
  the page's menus. It goes back to how it looked before you opened it, by `DELETE /api/pages/{slug}/read`; opening it again reads it.
  `POST /api/pages/{slug}/read` now answers `{"cleared": true}` when it read the page up to its last version. An archived page is never
  unread: its row has no dot, bold title or new version mark, nothing counts it, and nothing offers to mark it unread.
- Add what a viewer has read of the stored pages, kept in `reads.json` in the store. A page opened in its frame tells
  the frame the version it served, which the frame sends to `POST /api/pages/{slug}/read`, and `GET /api/pages`
  gives each stored page `news` (`new` for a page not seen, `version` for a version after the one read) and `version`
  (the number of its last). Pages from before floe first writes `reads.json` count as read. `GET /api/pages` also
  gives the name of the `template` of a stored page and a linked data file; `floe rebuild` writes it to the meta.json of
  pages published before.
- Add the versions of a stored page: a publish of a data file, or images it reads, other than the last version's
  keeps the page before as a version and makes the new page the next one (a rebuild, or a publish of a theme only,
  draws the last again and adds none). A page keeps 20, the oldest going first, those before the last gzipped under
  its directory. `GET /api/pages/{slug}/versions` lists them, and `/a/{slug}/?raw&v=<n>` serves version `n` as the
  page is served.
- Add `labels: between` to the diagram part `axis`: each tick's text stands halfway between it and the next tick on
  its right, or `x2` past the last, as a week's name in the middle of its week (the default `labels: at` keeps it at
  the tick).
- Add the column a template centres its page in, on an element of the class `floe-column`: `--column-width` wide (the
  palette's `--page-width` unless the template sets it) within `--page-pad` each side, under the template's
  `page.css`. A page of one column sets `:scope { --column-width: var(--text-width); }`. floe's templates take it in
  place of writing the column themselves.
- Add a theme's `markdown.css`, the look of Markdown (quotes, headings)
  wherever it stands: on the page, in a text and in what the parts hold.
  floe's themes draw their quotes, and retro its stripes under headings, from
  it. A part or look is not named `markdown`.
- Add the diagram part `graph`: write boxes and the edges joining them by
  name, and floe lays them out in ranks (`direction: down` or `right`), orders
  each rank to cross fewer edges and draws each edge at right angles between
  points of its boxes' sides of its own, straight where it can: a box a
  column short of an edge's way widens into the room beside it, and an edge
  passing ranks turns once where it must leave its column. An edge's `label` stands on it, on the diagram's
  ground and cutting the line, at the middle of its longest run where that is
  clear, where floe makes room if there is none without bending the edge; an edge from a box to itself is a loop out of the side the
  graph's edges do not use. A `group` of its nodes keeps its boxes side by
  side in every rank, framed as a `frame` with its `label` (groups nest), and
  an edge may end at a group, at its frame's top or bottom. Written as
  `lanes` in place of `nodes`, its groups are lanes, bands from end to end of
  the graph in the order written, their tops and bottoms in line. An edge written `level: true`
  stands its two boxes in one rank, the edge straight between their facing
  sides: two boxes of one group, side by side as written, or directly in two
  lanes, each at its lane's end towards the other; floe refuses one whose
  boxes something stands between, or that takes the side of a box's loops.
  A graph holds 200 boxes, icons, groups and lanes together and 400 edges,
  and the edges of all the graphs of a diagram may pass 4000 ranks between
  their ends, all told. The diagram examples
  `flow`, `state` and `swimlane` are drawn with it.
- Add `marks:` to a part's or look's `part.yaml`, which of its elements are
  rows with a mark at their left: `box`, `items` (each `li`) or `joined`
  (with a line to the next). floe lays the rows out in one layer,
  `floe.marks`, where the mark stands and how big; the part draws the mark.
  `floe part check` refuses `marks:` written twice down a part's looks.
- Add the look `marked`, a letter before each item of a list as a row's
  mark, which decision, options, checklist and handoff's done work take.
- Add `data-floe-mark` to an item of the look `numbered`: `ring` rings it in
  its tone, `solid` fills it and its line in the tone.
- Add `--quiet` to the palette, the quiet ground within a box: a table's head,
  a bar's track and a keycap read it.
- Add `--radius`, `--radius-small` and `--radius-pill` to the palette, one
  length each: the round of a box's corners, a small box's within one and a
  pill's. Parts, templates, floe's base and themes read them, and a theme sets
  its corners in its palette alone.
- Add the tone `muted`, a quiet one in `--muted` on `--muted-soft`, wherever
  a tone is written (note, a pill, a table's pills, bars, stats,
  timeline, a diagram); adr's deprecated and superseded, postmortem's low
  severity and fields' type are drawn in it.
- Add `--tone-fill`, a theme's own fill of a tone, from `--<tone>-fill` in its
  palette (`--good-fill`), or else the tone's `-soft`.
- Add a diagram's `width` and `height` left out: the canvas then reaches as
  far right (or down) as what is drawn, and 20 more, its top left still 0, 0.
  Raw SVG is not measured, so write both with a `raw` part.
- Add one padding above and below the rows of what is drawn as rows (the
  looks table and rows, the parts that take them, the templates' tables and
  lists in them, and facts' tiles): 0.4rem, set by `--floe-row-pad` in
  floe's base.css, in the page's line height; a part of your own takes it by
  the look rows, or by the variable.
- Add the line and column of a mistake about a value of a YAML file at the
  value's first character, though floe's YAML library places it off after a
  tab, after a value with spaces at its line's end, and after a comment or
  quotes over CRLF lines.
- Add a table's columns written as their titles alone (`columns: [Risk, Owner]`),
  an object `{ title, num, pills }` only for a column holding more; and a
  column's `pills`, words with their tones (`{ done: good, open: bad }`), for a
  cell of the column that is one of them, whole, to be drawn as that pill
  without `{ pill, tone }` written in each cell.
- Add the function `written` to a part's `part.html`: the part's value as
  written, its parts and formats not drawn, to compare a cell with a word
  (floe's `table` matches a column's `pills` so).
- Add the part `entries`: records one under another, each a heading with its
  tags (a line of Markdown or a pill, as a table's cells) beside it, and
  Markdown text the full width under them, for a name and its longer text,
  such as requirements, risks and what is done about them, or terms, a state
  in a tag. It goes where parts go, and in `details` and `tabs` too.
- Add the refusal of a `|` or `>` text whose first line not of spaces begins
  with spaces and a tab (a tab alone too), which floe's YAML library counts
  into the indentation, dropping or shortening the lines after it.
- Add a note to print from the management page: printing a page's frame,
  which would put only one screen of the page on paper, prints instead a note
  to open the page alone, with its URL, and print it from there.
- Add printing every ground of a page as the screen draws it, the page's own
  too, so that a light text on a fill is not lost on white paper; and the CSS
  variables `--face` and `--on-face`, which a face filling itself with a colour
  (a band, swiss's bad and grey warning notes, a quote in ink, code) sets to
  its ground and its text's colour, for floe's base to draw the links, a line's
  code and the focus ring in it in that colour.
- Add `floe theme list` and `floe theme show`. `floe part show`,
  `floe template show` and `floe theme show` list on a `files:` line the files
  a part, a look, a shared schema, a template or a theme reads; with a file
  named after, they print that file alone, as written but the byte order mark
  at its start (`floe part show code part.html`).
- Add floe's template `slides`, slides to show one at a time: a cover of the
  title on a band, then slides as tall as the screen, each a heading over
  parts as in `report`, where the scroll stops at each one, numbered in the
  corner; a slide of a heading alone stands between parts of the talk, and
  in print each slide is a page. With `direction: across` the slides stand
  side by side instead, moved by the ← and → keys or a swipe.
- Add floe's look `numbered`, the items of a list numbered in rings with a
  line joining each to the next: the part `steps` takes it, and a template
  takes it with `data-floe-look="numbered"` on an `<ol>`, as runbook's steps
  do.
- Add floe's template `runbook`, steps to follow with the commands to run:
  what to check before starting, a caution, the steps numbered under a line
  to the next, each with who does it, the commands with a copy button and
  what to do if it fails, and how to roll back, in a frame of its own.
- Add floe's template `postmortem`, an incident review written without blame:
  a summary on a band with its severity (high, medium or low, red for high
  alone), the impact, a timeline drawn with the part `timeline`, the cause,
  what went well, badly and by luck, and the actions with their owners and
  states (to do, in progress, done).
- Add floe's look `band`, a band filled with the accent whose text, links,
  code and focus ring are in the colour on a fill: a template takes it with
  `data-floe-look="band"`, as design's conclusion and the heads of handoff and
  postmortem do.
- Add floe's template `adr`, a decision record on a sheet: its status stamped
  under the title (proposed, accepted, rejected, deprecated or superseded, a
  superseded record naming the one that superseded it), the context, the
  decision, the options set aside and why, and what it gains and costs.
- Add floe's template `handoff`, a handoff of work or a status update with
  what to decide first: the date, who hands to whom, counts of what is done,
  in progress, to decide and next, questions numbered Q1, Q2 with their
  options and a recommendation, who is doing what and whether they wait or are
  blocked, what is next and what to watch out for.
- Add floe's template `design`, a design document with its conclusion first:
  the background and goals it needs, then the non-goals, decisions each a
  question over the part `options` (the option marked with `pick` is put at
  the top of the page, one without is shown as undecided), what is decided
  without options, what it touches, and sections of your own, each a heading
  over parts. Each decision's question is a heading over its options.
- Add a copy button over the top right of `code`, `terminal` and `diff`: it
  copies the code as written, a terminal's commands without their prompt and
  output, and a diff as written, and says it copied, in the page's language.
  It copies where the page may not write to the clipboard too (VS Code's
  Simple Browser and the like). Scrolled to its end, a long line ends left of
  the button.
- Add warnings: advice on a better way to write a page, which `floe check`,
  `floe render`, `floe template check` and `floe publish` print after the
  mistakes as `file:line:column: warning: message`, leaving the page drawn and
  the exit code as it is. The first is a fenced block of code in a `text` part,
  in a section (report's, `more` of design, handoff, adr, postmortem and
  runbook, runbook's `text` of a step, and a slide of slides) or in `details`
  and `tabs`: it is drawn as plain code, and the warning says to write it in
  `code` or `terminal`. `text` is a part of its own, so a part of your own
  named `text` is refused.
- Add the diagram part `tree`: write the nodes of a tree and their children,
  and floe places each under its parent and draws the edges; and the diagram
  example `org` (`floe diagram example org`), the departments of a company
  drawn with it.
- Add the diagram example `gantt` (`floe diagram example gantt`): bars for
  how long tasks take and arrows for which task waits for which, here the
  plan of one version of an app over six weeks.
- Add the diagram example `quadrant` (`floe diagram example quadrant`):
  two axes that split items into four areas, here a notes app's feature
  ideas by their value and the effort they take.
- Add the diagram example `swimlane` (`floe diagram example swimlane`): a
  flow split into a lane for each who acts, here one pull request between the
  user, the architect and the engineer.
- Add a button over `code`, `terminal` and `diff` that copies them: the code,
  the commands without their prompt or output, the diff as written. It says
  "Copied" in the page's language, and still copies where the page may not
  write the clipboard, such as in VS Code's Simple Browser.
- Add archiving: `floe archive <slug>...` (`--undo` to give back), "Archive"
  on the management page, one page or those selected, and `archived` in
  `PATCH /api/pages/{slug}` take a page off `floe list` and the management
  page's list without deleting it; it is still served at its URL. The
  management page shows the archived under "Archive" on its shelf, and
  `floe list --archived` lists them too, with a fifth column. Publishing an
  archived page again gives it back to the lists.
- Add `floe publish`, which draws the page of a data file (YAML) with the
  template it names, stores the page as drawn and prints its URL on a local
  server at `http://127.0.0.1:7325`, started in the background when it is not
  running; `--link` registers a file in place instead: an HTML file shown as it
  is, or a data file drawn each time its page is opened, the open tab reloading
  when it is saved. Every command that uses the server (`floe rebuild`,
  `list`, `rm` and `archive` too) and cannot use the one on the port (of
  another store, drawing with other directories, outdated and not stopping, or
  a port that does not answer) is refused with what to do, then to run that
  command again; `floe stop` returns once the server is gone. A server is
  stopped by asking it (not by a signal to the pid it tells, which may be
  another process), and it stops once the requests under way are done.
- Add `floe check` and `floe render`: check data files without writing them,
  and write each page to an HTML file that opens without floe (`--check` for
  CI), the mistakes printed as `file:line:column: message` (`file: message`
  for one of the whole file, such as a key it lacks at the top), every one of
  a stage of the checks at once.
- Add `floe rebuild`, which draws every stored page again with the templates,
  parts and themes of now, keeping the page it had where it no longer draws.
- Add `floe list`, `floe rm`, `floe serve` and `floe stop`, and a management
  page in English or Japanese, by the browser's language, that lists, edits,
  opens and downloads pages: the bar over the list holds the actions on the
  pages selected while there are any, each project has a color of its own, a
  row's menu holds its edit, opening, download, archive and delete, and the bar
  over an open page stars and archives it with buttons of icons alone, filled in
  while on. Tags are given in a page's edit dialog, to those selected,
  or as the names to `add` and `remove` in `tags` of `PATCH /api/pages/{slug}`; they show under a page's
  title, are searched, and stay when the page is published again. What is
  reached by keyboard shows one thin grey ring, and a button pressed with the
  mouse shows none.
- Add floe's template `report`: a header over sections, each a heading over
  parts and then subsections, one level of a heading (an `<h3>`) over parts (a
  heading, when written, is not empty, and a section with subsections has
  one); and twenty-six parts a template refers to as
  `floe:///parts/<name>.json` (`floe part list --all` lists them): twenty-four that a
  section of report takes beside Markdown `text` and `html`, from `code`,
  `terminal`, `table` and `image` (an image of the data file's directory,
  such as a screenshot) to `decision`, `entries`, `timeline`,
  `tabs`, `diagram` and `script`, and `cell` and `stack`, which other parts
  hold. A part draws no heading, as it knows not the level it is put at: the
  titles of `options` and `decision` are not headings, so that the headings of
  a page keep their levels wherever the parts are.
- Add twelve themes a data file picks with `theme:` (`default`, `soft`,
  `editorial`, `ink`, `brutal`, `blueprint`, `notebook`, `retro`, `swiss`,
  `washi`, `aurora` and `app`), each a palette of CSS variables and the shape
  of the parts. A quote in a part (a note, a step, a card, details or tabs)
  is drawn as floe's, a rule at its left, not in the theme's shape of the
  page's quotes; and on paper, where every tab shows under its title, no
  theme fills the title of the tab picked on the screen. `aurora` lays faces
  of frosted white glass on a wash of lavender, peach and mint, with a violet
  accent and round corners. `app`
  raises white cards from a cool grey ground with soft shadows, its Latin in
  the UI face of Windows or Android, with a raspberry accent. Pages in
  `default`, and in the themes that take its colours (`soft`, `editorial`), have
  the warm grey ground and text of the management page, with lines of the same
  warmth, and a terminal's prompt, the language of code and a diff's hunk heads
  in yellow-green.
- Add templates of your own in `FLOE_TEMPLATES_DIR`: `page.html`,
  `schema.json` and `example.yaml`, and `page.css`, `checks.yaml`,
  `computed.yaml`, `words.yaml` and `assets/` if they like; and
  `floe template list`, `show`, `new` (the smallest template, to fill in, its
  `page.html` with a viewport for phones) and `check`.
- Add parts, looks, shared schemas and themes of your own in `FLOE_PARTS_DIR`
  and `FLOE_THEMES_DIR`, beside floe's and never named as one of them; and
  `floe part list` (what a section of floe's templates takes, `html` too; with
  `--all`, every part, look and shared schema a `$ref` can name, your own too),
  `show` (`floe part show html any.json` prints `any.json`, which holds `html`), `new` (the
  smallest part, to fill in) and `check`.
  A part's schema takes floe's formats (`html`, `markdown`, `image`) as a
  template's does, and a shared schema holding them. A shared schema is
  checked from its root, whether a schema names it or not. A part's
  `part.html` may call the functions floe's parts draw with (`copyButton`,
  `bars`, `diff`, `tree`, `diagram`, `script`); `bars` refuses an item not
  written as keys and values, a value that is not a number or is below 0, an
  `of` that is not a number or not above 0, and a value over its `of`, telling
  every item's mistake together, each with its bar's number and label.
  text/template's mistake of running a `part.html` (a key its schema does not
  declare) is told at its line and column, as one of a `page.html` is; a
  function refusing a value (`num`, `bars`) is told at the value.
  A theme's `base.css` reaches the page as a template's `page.css` does, not
  what a part holds nor a text's Markdown, and under the template's
  `page.css` and the looks: its root is `:scope`, `:root` and `html` being
  outside it, and a rule for what a part holds goes in the theme's file of
  that part. A theme's `markdown.css` is the look of Markdown wherever it
  stands, in what the parts hold too, under every part's, look's and
  template's CSS.
  A mistake of a part, shared schema, theme or template of your own stops
  every page drawn until it is fixed. The directories of parts, themes and
  templates of your own that are one, or one in the other, are refused for
  their settings.
- Add diagrams drawn as inline SVG from a spec of parts on a canvas (frames,
  boxes, tables, icons, edges joining named parts, dots, bars, axes, circles,
  labels, lines), their text in one language or several (`{en: …, ja: …}`),
  checked for overflowing text, overlaps, edges through boxes and crossings
  in every language before they are drawn: a `diagram` part in a data file,
  in a section and in `details` and `tabs` too, or `floe diagram render`,
  `inspect`, `example` and `guide`.
- Add `floe icons fetch` and `floe icons search`, which keep the AWS
  Architecture Icons for the `icon` parts of diagrams. Icons not fetched, or a
  directory for them that cannot be used, are told once, at no place in a
  data file, however many diagrams and files need them.
- Add a page's CSP, which keeps the scripts of a page drawn from a data file
  from sending what it holds out, on the server and in a page `floe render`
  writes or a download gives. Such a page holds one charset, floe's, first in
  its head, in place of those `page.html` writes.
- Add a folder as a link (`floe publish --link <folder>`, or “Add a link or
  folder” on the management page): one row of the list, which opens a tree of the
  folder's directories and HTML files, reading only what is opened, and frames
  each page under the folder's title. Names starting with a dot are left out; one
  directory shows its first 1000 entries. A folder's page has the folder's tree on its left, whose edge is
  dragged or keyed to widen it, and which hides at a button; the browser keeps
  both.

### Changed

- Change `floe theme check`, `floe part check` and `floe template check`, and every command that reads your own themes, parts and templates (`floe check`, `floe render`, the server's publish), to refuse a `--radius-small` or `--radius-pill` of more than one item in their CSS, `5px 3px` or `4px / 2px`: floe's parts read each as one corner, a bar's `rx` and a pill's `min()`, which such a value voided without a word. A length or one function (`var()`, `calc()`, `min()`) passes, and `--radius` is not checked. A template's `page.css` is checked by the commands that read templates, not by `floe theme check` and `floe part check`.
- Change how `floe work` writes: the pages and their tasks in one write (`PATCH /api/work`), checked whole first, so that a refusal writes nothing and a page left a task's row behind no more. A task a page goes on gets its row in the same write, made then, and the page's category is added at the end of the task's categories; `--categories` keeps a category with pages, and adds a page's at its end. `floe work --task X` with no field to change tells the task and writes nothing (it made an empty row); an empty write of a task of no row is not found. A page whose project changes, by `floe publish --move-project` or the page's Edit, is taken off its task, and publish says so (`page in project app, off task #9`).
- Change a page's category to be the page's alone: a task no longer keeps the last category written, so the task list no longer shows one on each task, `floe work --category` without `--task` writes no task, and with `--task` writes the task named as before, with nothing of it changed (`made`, `changed` or `unchanged`). `PATCH /api/tasks` refuses `category`.
- Change the name of the place a page has among its task's pages from step to category: `floe work --category` and `--categories` (once `--step` and `--steps`, which are now refused as unknown flags), `category` and `categories` in `meta.json`, `tasks.json` and the API, and Category on the management page. The help and docs no longer give a category on the same line as a task's state, and `floe guide` says that the user decides a page's category, and that an agent gives `--category` only when asked.
- Change the management page's header bar: the page's whole screen (`/a/<slug>/`) no longer shows the settings and help buttons (`?` still opens the help, on its shortcuts), and the list's bar now ends with the settings, after the help. The server's state moves from the bar's end to beside the logo and shows only while the connection is lost (a red label with its words, or only its dot when the window is 40rem wide or less); while connected there is no green dot.
- Change `floe guide` and the docs to teach writing every text value of a data file in `'…'` (several lines in a `|` block) without judging whether it needs quotes, and not in `"…"`, where a `\` is read as an escape and mostly with no mistake (`\_` is a no-break space); a number, `true`, `false` and a word of ASCII letters, digits, `_ . / -` starting with a letter or digit stay bare. A text that reads as a number, `null` or `~` is quoted too. All the YAML that floe shows agents is written so: what `floe template show`, `floe part show`, `floe theme show` and `floe diagram example` print, the one `floe template new` makes, and the YAML in the docs and the guide. What it says is as it was.
- Change `floe publish` to say the project a page is in, after its URL on stderr (`page in project floe`, or `page in no project`), when the page is new or its project changed, as when it replaces another file's with `--force` or was emptied of its project. A publish that keeps the project, and `--link`, say nothing; stdout is still the URL alone. The server's answer to a publish now holds the page's `taskProject` and its replaced page's.
- Change the management page to show the project column at the left even with one project to choose, or only "no project", and to put a project's pill on each row of the page list and of the search with one project too; a task's page, with its project in its head, still has none on its rows. It showed them with two or more projects only, so that a store of one project told nowhere which it was in.
- Change `floe guide` to tell an agent to quote a text that starts with a backtick (YAML refuses it unquoted), and to run `floe publish` in the repository it works in: a page's project is set by where it is first published from, whatever directory holds the data file.
- Change `report`'s example (`floe template show report example.yaml`) to a short one of the parts used most (text, terminal, code, entries, table, note, columns), 63 lines from 104: `floe part list` lists the others. `floe guide` now has agents run `floe part list` to pick from the parts the template takes, and its table of parts sends short values before and after to `table` and a screen or code before and after to `columns`, as the descriptions of both parts now say.
- Change the part `tree` to start the notes of all its rows at one edge, whatever the depth or the length of the names, as a column of its own. A note keeps 6rem (or its longest word) at least, so that in a tree of long names the notes narrow and fold line by line, and on a phone scroll sideways to be reached.
- Change the management page's folder row to show no time (nor in its tooltip or the search): the time was when the link was added, which never moves when a file in the folder is written, unlike a linked file's time. The folder keeps its place in the list, by that time. The rows of a folder's directories, and the note of an empty one, no longer show "just now" a minute after they are drawn.
- Change the management page's words for deleting a task and a step to Delete, from Remove: "Delete the task", "Really delete it", "Delete <step>", "Deleted <step>", "cannot be deleted" and "was deleted elsewhere"; a task refused for its pages or child tasks says to take them off it or delete them first, at `floe work --forget` too. Making a task is Create, from Add and Make: "Create task", "Created task <ID>", "Created the child task <ID>", "Create" in the dialog and "Create <ID> as a new task"; finding or making a tag is Add: "Find or add a tag", "Add the tag <name>". The Japanese words for adding a step, and the toast for taking a page off favorites, change too.
- Change "+ Add a link or folder" on the page list to "+ Link or folder", and its place: from the head of the list to its left column, under the kinds (page, link, folder); in Filter on a narrow screen. It stays while rows are selected and with the list folded, and goes with the left column folded; a list with no page at all has only the button in its middle
- Change the tables of four themes, each still in its frame: soft's rows are striped faintly in the accent with no rules, its head on a tint of the accent in the accent; app's head is a band of the code's dark blue in light text, its rows striped with the ground; aurora's head stays clear over a line of its wash's three colours, deepened, with lines of light between the rows; swiss's table, ruled only above and below before, is framed in black, its head white on black. Markdown's tables, a chart's numbers, `fields` and the tables of the templates design and postmortem take the same look. A table's head is painted on its row now, so that a gradient in `--table-head-bg` runs once across it.
- Change the management page's "Edit" of a page to fold its slug and project under "Slug and project", which shows what they are now and opens when one of them is refused or fails to save, with the field focused; each says what changing it changes on the screen. Japanese words for what a person does say 変更する ("名前は変更できない" on a task's steps).
- Change the words of the management page's dialogs and failures: a dialog that saves what is typed closes by "Cancel" ("Set task", "Edit task" and the dialog adding a task, a child or a parent, which said "Close"), one with no button that saves by "Close"; a failure says "Failed to …: reason" ("…に失敗しました：理由") throughout, the reason in brackets when words follow it. Saving "Edit task" or "Add a page" when the server is not reached, or a version that changed meanwhile, now says "Failed to save: …" ("Add a page" said nothing when not reached), and marking a page read, when the server refuses it, says "Failed to mark as read: …".
- Refuse a comment right after a text value not in quotes, or after a `|` or `>` text, when it is deeper than the value's key (or `-`), on a line after the value too. A `# Heading` line shallower than a `|` text's indentation, which YAML leaves out of the text as a comment, a `#` line of a plain value over lines, and a comment cutting a value in a file of CR line breaks are refused now. The mistake tells how to mend a `|` or `>` text (indent all its lines alike, the `#` line too, or write its indentation, like `|2`) and how deep to write a comment.
- Change the mistake of `min` on a bar chart and of `nodes` beside `lanes` in a graph from `false schema` to `additional properties 'min' not allowed` (and `'nodes'`), naming the key, at the chart or graph that has it.
- Change a stored page to belong to its data file, as a link belongs to its path, whatever directory publishes it: the file is named by its repository and its path from the repository's root, else by its real path. The same file published again from another directory, another worktree, another project or from outside its repository overwrites its page, renamed on the management page too, and keeps the page's project; it was refused from another project, and a renamed page got a second page at the file's name. Another file publishing to its slug (`notes/plan.yaml` over `docs/plan.yaml`) is refused unless `--force`, naming the file whose page it is, where it replaced the page with no word. A page published before keeps the file of its next publish from its project. A link linked again keeps its project too.
- Change Markdown to read `**`, `*` and `~~` beside Japanese, Chinese or Korean letters across a mark too, as markdown-cjk-friendly reads them, everywhere floe draws Markdown: `**「題」**に` is bold, where CommonMark left the marks as text. Fullwidth letters and marks count as such letters too, and `_` is read so as well, though in a sentence as before. A `*` meant as a mark beside such a letter (`*.yaml`) is written as `\*` or in backticks.
- Change raw HTML in Markdown to be drawn as the text written, everywhere floe draws Markdown: what looks like a tag (`/logs/<prefix>`, `Vec<u8>`, a heading `<T>`) is no longer dropped without a word, and a block of HTML shows as a paragraph of its lines. HTML in Markdown is still not drawn as HTML: write a tag as text in backticks, and a line break in Markdown.
- Change the refusals of a slug in use (by another project's page, another file's link, a page or link of the other kind, or on a rename) to say what uses it and to choose another slug, no longer suggesting `--force` or `floe rm`, which drop that page and its versions; `floe publish --help` and the docs tell an owner how to replace or remove theirs. The warning of a changed title says from what to what, and no longer suggests another `--slug`.
- Change Markdown's strikethrough to two tildes alone, everywhere floe draws Markdown: `~~text~~` is struck through as before, while a single `~` stays text, so that a range (`2026-11-01~11-08`) or a path (`~/work`) is no longer struck through unasked.
- Change `report`'s `eyebrow` to a line of Markdown, as `lead` and the section headings are: code in backticks, bold and links are drawn, and a bare URL or email address becomes a link, but not a URL of an IP address or localhost. What looks like a tag (`<prefix>`, `Vec<u8>`) is left out and marks such as `*` and `_` are read, so write those as text in backticks.
- Change how a page is tagged on the management page: a page open beside the list, in the frame or on a task's page shows its tags under its bar, with a button that opens the tag dialog of that page alone, and the menu of a row or a bar has "Tags" for it. The one dialog, which tags the selected pages too, is the only place tags are changed: the edit dialog has no tags field any more, so a tag with a comma in it is no longer split when another field is edited. Enter in the dialog's find field takes the one tag it leaves, else makes the name a tag. The line is left out of a frame's print.
- Change `options`, `facts`, `stats`, `bars`, `keys`, `timeline` and `asks` to lay out by the width of their place, so that they stay within a narrow one, such as an option's card: a card's column is never wider than its place, `keys` and `timeline` turn to their narrow layout by their own width (so in a window up to about 640px wide, where they turned at 576px), a pill of `asks` wraps, and the label and the value of `bars` take up to 40% each, wrapping, so that a long label no longer leaves its bar a sliver. The narrow layout of `timeline` keeps the room of the wide one between its dot and its text, 4px less than before.
- Change a task's page so that its parent or its children stand in a card, each row headed by its state, apart from the steps of pages under it: the states are as wide as the widest, the IDs stand in line, a long ID is cut at a fifth of the card, a long title at what is left, and pointing at a row shows both whole.

- Change a diagram's `graph` so that an edge with both ends outside a group goes round the group's frame, a column off it, instead of running through it, back against the ranks if it must: it runs longer and may cross more edges. It still runs through where floe finds no way round, where many edges end on one side of a box, or where floe gives up looking in a crowded graph, a lane is not gone round, and a level edge stays straight. The ends moved to the equal parts of a side no longer take an edge into a frame that holds neither end, so a few graphs keep an end off its equal part. A way round that runs into an edge or part written by hand is told at that one, written by hand: add `crossing: true` to the edge, or move it.

- Change the room between what a card holds in `stats`, `facts`, `decision`, `asks` and `options` to the look `cards`' gap (`--cards-gap`, 0.4rem): stats' lines and facts' name and value stand a little further apart, decision's and asks' rows a little closer, and a theme's `--cards-gap` changes them too. In options holding parts, a card of no parts on a line of its own, and every card in a theme that draws the number out of the rows, stand 0.4rem taller.
- Change where a part sets a variable a theme may change: on its root (`:scope`), so that a theme of one's own setting it on the `:scope` of the part's or look's file, as floe's themes do, wins (`--floe-mark` and `--floe-mark-gap` of numbered, marked, timeline and links, `--floe-mark-gap` of checklist, the number of cards on `:scope.numbered`, `--checks-gap`, `--entries-row-gap`). floe's pages look as before.

- The docs and the diagram's schema tell how raw SVG takes the theme's colours: the palette's variables (`fill="var(--accent)"`), and `var(--tone)` under `data-floe-tone`. A part's classes are not for raw, and the class `band`, which only the example `sequence` drew with, is gone: raw that wrote it draws black.
- Change `report`'s section headings to a line of Markdown, as `lead` is: their ids are made of the words drawn, so a tag, a mark such as `__init__`, a backslash, a character reference or a link's address changes them.
- Change `design`'s contents to the look `toc`: a quiet list on a line, the section being read marked, and none on paper.
- Change the part `links` from two columns of cards to rows: a sign, the title and a few words on a line and the URL under them, whole rather than cut, on paper too. The title alone is a link now, so the words and the URL can be selected. A theme's decoration of a card no longer reaches `links`.
- Stop placing the `::after` of a row of `marks: box` or `items` over its mark: floe's own parts no longer draw one. In your own part, one with no `position` is an item of the row's grid: it adds a row under the text, in the mark's column, and one wider than the mark moves the text too. To lay it over the mark, write `position: absolute` and `position-anchor: --floe-mark`.
- Show the zoom toolbar of a diagram or an image as its buttons alone: how to zoom moves from a line over the frame to the frame's tooltip, and to a line under the frame while the keyboard is on it, which is also its description (`aria-describedby`), so that images side by side in `columns` no longer stand under two lines of tools. A touch screen no longer shows how to zoom.
- Change a diagram's zoom to be offered only while the diagram is drawn smaller than its actual size, or zoomed: a diagram shown at its size or larger, on a wide screen, has no toolbar or tab stop, and a finger can pinch the page over it. The zoomable frame is named Zoomable figure to a screen reader, not after the diagram, which its svg names already. The frame and toolbar of `floe diagram render` are now of the classes `floe-zoom` and `floe-zoom-tools` (were `dg-wrap` and `dg-tools`), and the English toolbar says figure for diagram.
- Change where a `group`'s `label` takes room in a `graph` laid out down: it stands above the group's first rank, a box no edge reaches from above standing under it if it comes before those an edge reaches in the rank, so that nested frames start together and the graph is narrower. A few graphs grow, as boxes start elsewhere, and an edge written by hand that comes to run through a `label` is told: go round it by `via`.
- Change `floe icons search` to find a service by its common short name, listed first: `alb`, `sqs`, `s3`, `ecs` and 26 more, from a table floe keeps. `s3` used to list only icons with `s3` in their names, and not Amazon S3 itself, and `alb` found nothing. When nothing is found, the message now says to try fewer words or a word of the full name, and that `floe icons search aws` lists them all.
- Show a project's name as a pill in the color of its chip on the shelf: on a row, in the search modal, at the head of a task's page, and on the task list's filter button and its head with the column folded. Put the button that archives a page, or gives it back, on its row beside the star, in place of the item in the row's menu, and offer Restore on the toast after archiving.
- Show the way back on a page open whole at every width: beside the logo it says where it goes, the task's name, the task list or the page list, whichever opened the page; on a narrow screen its mark alone, as before.
- Change `report`'s `lead` to a line of Markdown, as the `summary` of `design`, `handoff`, `postmortem` and `runbook`: code in backticks, bold and links are drawn, while a heading, a list or a table stays text. What looks like a tag (`<prefix>`, `Vec<u8>`) is left out and marks such as `*` and `_` are read, so write those as text in backticks.
- Warn of a fenced block of code in the `text` of an item of `entries`, as in a `text` part: a page that has one now gets a warning, and is drawn as before with the same exit code.
- Change `floe diagram inspect` to begin each diagram with a line of where it is written, its title and its canvas (`page.yaml:9:11: diagram "T" 560x190`, `spec.yaml: diagram "T" 560x190`), and to print where the parts went for a diagram the checks refuse too, as a guide, on stdout with its mistakes on stderr and the exit code still 1.
- `GET /api/pages/{slug}/download` takes `?v=<n>` to save that version of a stored page as `<slug>-v<n>.html`, and answers
  404 for a version that is not there. The page's download follows the version shown.
- Change the bar over a page of the management page to diff, download, open alone, archive, star and `…`, whose menu no longer
  repeats what the bar shows. At 40rem or less a page open whole keeps diff and `…` in the bar, the rest in the menu, favorites
  too, and the logo gives its room to the title; a folder's page keeps its path and open alone in the bar.
- Move the button that folds a column of the management page (the filters, the list, a folder's tree, a diff's contents) to a
  foot at the column's bottom. Folded, the column, its foot and its line go, and only an icon to open it stays in the button's
  place, over the page beside it. The narrow column a diff's contents left behind is gone. Whichever columns are open, one line
  parts each from the next, and none stands at the window's edge.
- Change the gap between an icon and its label in a diagram to 3px from 7: the label, and with it the icon's bottom side,
  stand 4 higher, and a diagram with its height left out and a label lowest is 4 shorter. A side floe chooses at such an
  icon may change, which a part close by can make an edge refused; write the side. A label written by hand to go with an
  icon's label, beside it or stacked under it, stands off by 4; move it.
- Change the management page into a list of pages: newest first under its days (today, yesterday, the last 7 days, then by
  month), a blue dot on a stored page not seen, the mark `v3 · new version` on one with a version after the one read, and
  the template, project and tags under each title. A row opens its page beside the list (`j` and `k` or the arrows go on
  to the next and the one before), and its title, `Enter` or pressing the row again (or double) opens it whole, where `j` and `k` go through the list that opened
  it. The shelf picks a view (unread, all, favourites, archive), and narrows by project, kind, template and tag, all in the
  URL. "All" is lit only with no view and no picks: pressed, it lets go of them, and a view pressed again lets go of itself alone. Above 70rem, the edges of the filters and the list drag to widen them and each of them folds, kept in the browser. The sort, the pages of the list, the tabs of kinds, the groups by project and the frame's title that picked a page
  are gone, and the check boxes show only while rows are selected (`x`, or Select in a row's menu).
- Change a stored page's `updatedAt` to move only when a publish adds a version: publishing the same content again, or
  with another theme only, no longer moves it. `floe publish` says `last updated` in place of `last published`.
- Change the time a linked file is listed at to the later of the file's time and when it was linked, so that linking
  it again brings it up the list.
- Refuse a diagram's `title` empty in any language (`title: ""`, `{ en: "", ja: … }`, `{ en: …, ja: "" }`): it told a
  reader who cannot see the diagram nothing (`aria-label=""`).
- Refuse an empty label in a diagram (`label: ""`, the `text` of the part `label`, or `{ en: "", ja: … }`), in the
  parts `edge`, `bar`, `icon`, `frame` and `label` and a graph's edges and groups. All but a frame's took room, and
  unless plain drew an empty box. For no label, leave the key out. A tick's empty text is still a mark alone.
- Refuse a comment in `{ }` or `[ ]`, in every YAML file floe reads, a comment on a line of its own and one in a
  flow over lines too: YAML reads a `#` there as a comment to the line's end, the `}` or `]` on it too, which the
  YAML library told only as missing, at the `[` or the next line. floe tells it at the `#`, and where the innermost
  `{ }` or `[ ]` it is in opens.

- Change how a template's own Markdown, an item of format markdown and `{{markdown}}` in `page.html`, is drawn: in a
  `<div data-floe-markdown>`, its blocks `--floe-room` apart and its headings as large as a text's (1.2em and 1.1em).
  `page.css` and the theme's `base.css` still reach in it; a `page.css` that names its blocks as the children of the
  element around them (`section > p`) names them in it now (`section > [data-floe-markdown] > p`).
- Change the management page's corners and text to three steps each: a box's 20px, a middle box's 14px (a menu, the
  suggestions, the error) and a small square's 6px, and 0.72rem for labels, 0.82rem for small text and 0.875rem for
  the body, which was a fixed 14px and now follows the browser's text size (14px at its default). Most sizes move by a few hundredths of a rem; a field's corners are 6px (9px before), the room in a pop-up and a
  menu is 6px, a side tree's links and the keys' words are a little larger.
- Change parts' small text, written before in nine sizes from 0.72rem to 0.92rem, half a stat's number for its unit
  and 12px in a diagram's toolbar, to two steps of floe's base: `--floe-text-small` (0.8rem) for pills, chips, a ring's
  number, keys, a table's heading, terms (a bar's name too), dates, counts, a note beside a value and a diagram's
  toolbar, and `--floe-text-aside` (0.9rem) for a card's or a step's words, a note's title, a small title over a list
  (decision's set-aside options), a tab's name and a value's unit. A language's chip, a field's `required` and
  decision's set-aside title are a little larger than before, a table's heading, stats' terms and a bar's name a little
  smaller; the text of code keeps code's size. A theme does not set the two. A theme's small text that fell below a
  part's step now reads the step, 0.8rem: blueprint's code head, table heading, note title and card number (0.7rem and
  0.72rem before), and default's card number and ink's note title (0.75rem before).
- Change the Markdown a part holds (a note's text, an entry's, a card's text in options and decision,
  a step's and a timeline item's, an item of format `markdown` in a part's own value) to stand in a text's box, as a
  text's does: its paragraphs, tables, quotes and lists are `--floe-room` (12px) apart in every part, where each part
  spaced them its own way before (2.4px in steps and timeline, 5.6px in entries, 6.4px in options, 8px in note and
  decision). A part's own `part.html` that draws `{{markdown .x}}` gets the box too: its CSS reaches the blocks in it by
  `:scope p`, not `:scope > p`, and writes no room between them. A template's own Markdown is drawn as before.
- Change where a right-angled edge with `via` turns at a via when going on the way it came would run back over
  itself: it turns the other way first, so its corner moves, and an edge drawn before may now be refused as running
  through a part beside the via. `from: [0, 0], via: [[200, 0]], to: [100, 100]` ran `H200 H100 V100` and now runs
  `H200 V100 H100`, through a box at x 180 to 220, y 30 to 70; another via gives another way.
- Change what a screen reader reads of the marks CSS draws: no mark is read as
  its shape (a check mark, plus, a prompt's `$`, a quote's “), and what a mark
  alone meant is read in words of the page's language: a step or a checklist's
  item done or not, an option's pros and cons, a tree's file added, removed or
  changed, a terminal's command, a note's tone and a runbook's caution. A
  numbered step and a numbered card read their number; a slide's number and a
  checklist's count are shown alone. A part of
  one's own writes `/ ""` after a mark's `content`, and tells its meaning in
  `<span class="floe-hidden">` with a word of its `words.yaml`.
- Change an open page's tab not shown to reload when shown, no longer while hidden: switched to, it shows the page
  it had for a moment, and a linked data file's page is drawn again then.
- Change a right-angled edge with `via` that repeats a point (a via on the point before it, or on the edge's start)
  to keep the way it was going over that point: `from: [10, 100], via: [[10, 100], [50, 150]], to: [200, 150]`
  goes across first (`H50 V150`) where it went down first (`V150 H50`).
- Change code's look to keep the room at a line's sides on each line, an element
  `pre` holds, rather than on `pre`: a part of one's own that takes it makes
  each line an element, as text `pre` holds outside one sits on its edge, and
  reads `--floe-code-pad` and `--floe-code-room` to move the room onto lines
  of its own rather than writing them. It makes a block of a `<code>` that
  `pre` holds, and takes the inline look off it, and of no other: a `<code>`
  elsewhere in such a part (its heading, an element `pre` holds) keeps the
  inline look of floe's base. A long line of such a part without a copy button keeps
  the room after it, scrolled to the end.
- Change the labels of a Venn diagram's regions (`in`) to have no box unless
  they say `plain: false`: one that wrote no `plain` loses the box it drew on
  its circle. A diagram's ground, `--bg`, is painted in its frame, so a
  diagram on a panel or a theme's patterned page stands as a box of the page's
  ground, and a fragment of `floe diagram render` on another page as a box of
  `default`'s `--bg`, light on a dark page.
- Change brutal's and notebook's numbered cards (options', or a part or template of your own that takes the look
  `cards` with `class="numbered"`) to draw what they hold in its own colours on their filled faces, no longer
  painting every text in the card's ink: options' + and − signs take the face's colour by `var(--on-face, …)`. Text of
  your own in such a card written in `--muted`, `--accent` or a tone with no `var(--on-face, …)` no longer reads on
  their yellow, blue or pink faces; write it with `var(--on-face, …)`. A pill there keeps its own colour.
- Change every page to start with no margin on the body, set by floe under the template's `page.css`: a template of
  your own that writes none loses the browser's 8px, and a margin your theme's `base.css` gives the body no longer
  reaches it (write it in the template's `page.css`). floe's templates set no font size, so their text takes the size
  its reader's browser sets, as their spacing and headings already did (at the browser's default of 16px nothing
  changes). A theme of your own whose `--mono` is `monospace` alone draws a line's code at about 11px rather than
  13.76px, as a browser sizes text in `monospace` alone from 13px: name a face before `monospace`.
- Change a table's head, the tab picked and code's heading to be painted by their parts from variables a theme sets,
  in place of rules for their selectors, the state picked and print: `--table-head-bg`, `--table-head-fg`,
  `--tabs-picked-bg`, `--tabs-picked-fg`, `--tabs-picked-ring`, `--code-head-bg`, `--code-head-fg`, `--code-head-add`
  and `--code-head-del` (a diff's counts on code's heading), beside `--table-rule` and `--code-ground`. A terminal's
  heading is faded by its colour, not its opacity: its ground and rule are a little stronger in every theme but
  brutal, and a theme of your own that writes the heading's colour by a rule draws a terminal's at that colour,
  not fainter. A shadow a theme of your own gives every tab title is not drawn on the tab picked, unless set as
  `--tabs-picked-ring`. brutal's tab titles are 2px smaller on every side, and the tab picked holds its text 2px
  nearer its black frame: the frame is drawn within it, not as a border every tab kept room for. notebook's ring of
  focus on a tab not picked has the hand-drawn corners of the tab picked, not an oval.
- Change the padding of a box among parts to one pair, 1.1rem above and below and 1.3rem at the sides, in every
  theme: cards (options', stats' and decision's from 1.15rem above and below; links' from 0.9rem and 1.1rem at the
  left), bars, tree (default's white box within its frame too), note (editorial's too), details, and the boxes of
  design, runbook and handoff. The look `rows` pads its rows by 1.3rem at the sides (entries, keys and checklist
  from 1.2rem; design's decided list from 1rem; an element of your own HTML that takes it from none). A part or a template of your own reads `--floe-pad-block` and `--floe-pad-inline`. A part of your
  own that takes `rows` drops the padding it writes at the sides of an element in a row (a `summary`, a `div`), which
  now sits twice as far in, and widens an element that opens the row across by a negative margin; a
  decoration in the `::after` of its box has the sides' padding too.
- Change a long pill that wraps (entries' tags) to a label of round corners, not
  an oval stretched up and down; a pill of one line stays round at both ends
  (notebook keeps its own corners). A part or template of your own lets its
  pills wrap by `--floe-pill-wrap: normal` on an element of its own.
- Change the text of a part on a ground of its own (a note, a part on a panel)
  to the palette's text colour: it takes no colour a template's `page.css` sets
  around it.
- Change the part text to draw its Markdown in a box of no part
  (`data-floe-text`): a template's `page.css` and a theme's `base.css` stop at
  it, so that a template's heading marks (design's rule, report's line) no
  longer fall on a text's headings, and it takes the face and tone around it.
  A heading in a part or a text is 1.2em (`#`) or 1.1em (`##`), no larger than
  a template's own.
- Change a theme's quotes and retro's stripes to be drawn in what the parts
  hold too (details, tabs, notes, entries), as on the page. default's quote is
  white in a panel; swiss's quote is a heavy red rule at the left of its bold
  text, with no “; washi's quote is shorter (less padding and line height).
- Change steps', timeline's and cards' muted text to take the colour on a
  quote's face, as a quote with a face of its own (ink's) now stands in them.
- Change steps' step in progress to a ring 1.5px wide, runbook's lines to
  its rings' colour (the accent, and bad in its rollback, its frame and
  heading too), and brutal's and notebook's filled rings to the tone's own
  fill.
- Change runbook's and adr's column to the width of prose (3rem and 6rem
  narrower).
- Change a letter mark (decision's ×, options' + and −, checklist's – and
  ✓, handoff's ✓) to stand in a column of its own, centred, the lines of its
  text wrapping under the text, not under the mark.
- Change a diagram to refuse a coordinate past a million either way, a size
  (`w`, `h`, `r`) past a million, and a canvas (`width`, `height`) past ten
  million, in the schema's words (`maximum: got 1×10¹⁷, want 1×10⁰⁶`): a
  bound on size, so that what floe works out from them stays finite and of a
  sane size. A diagram written at 1e17 was drawn with its boxes on one another
  and its edges of no length, with no mistake told.
- Change floe's themes to draw upright and write no word: notebook's and
  washi's pills and notebook's tape no longer tilt; blueprint's quote takes a
  short rule for QUOTE and its cards number 01 for FIG. 01; washi's note seals
  take the note's mark for their kanji and its card seals count 1, 2.
- Change the cards of retro, washi, notebook and swiss so that their
  decorations take no room: retro's stripes are thinner (2px each) inside its
  frame; washi's inner rule lies over the padding, its seal is smaller,
  upright and laid on the card's corner, and no card keeps the room at its
  right for it any more; notebook's tape lies over a sticky note's padding; and
  swiss leaves a card's padding to its part, so that its first card's text
  stands in from the column's left as the others do. A card's number keeps the
  terms of a row's mark: sized by `--floe-mark` (on the card or the number),
  hidden by `content: none`, plain text by `--floe-mark: auto`.
- Change a keycap's ground to the quiet ground
  (from the page's ground, or aurora's surface): keys no longer melt into
  swiss's panel, brutal's are a mid grey as its tracks, and default's are white
  on its grey panels and grey on its white rows. A table's head where the theme
  draws none of its own (soft, a theme of your own) and soft's quotes
  take it too (45%, not 40%).
- Change the corners of parts and templates to the palette's: in default a box
  is 20px round (a note, code and a quote were 18px), and a face along a
  panel's edge is rounded about it (a card in cards 8px, the code in a code
  part 14px); small boxes within one (keys, bars, images, a diagram's frame,
  fields' types, code in a line and its language, links' and cards' marks) are
  6px, square in blueprint, brutal and swiss, 3px in editorial and washi; a
  tab's title is drawn as a pill (round in default, aurora, retro, soft and
  notebook, 6px in app and ink, 3px in editorial and washi); handoff's and runbook's pills take the theme's pill; runbook's box of rolling
  back takes the theme's box; notebook's note and code take its panel's
  hand-drawn corners; a note's mark drawn as a rounded square is 6px (washi's
  seal 3px); the ring round a focused title of details follows its box's
  corners.
- Change the statuses and cautions of floe's templates to be drawn as the
  parts cell and note: adr's status, postmortem's severity and actions' states,
  handoff's kind and states are pills, and adr's record superseding it (now
  after the head), runbook's caution and what to do if a step fails are notes,
  drawn as the theme draws its pills and notes. A pill is in the text's own
  face wherever it stands (retro's pick in options, in its card's title, no
  longer in Georgia). notebook's pills are filled solid, the paper's lines no
  longer through them.
- Change a tone to be marked by `data-floe-tone`, not by a class, in parts,
  themes and templates of one's own and in a data file's html (a pill written
  as `<span data-floe-tone="good" data-floe-part="cell">`). floe gives what is
  marked the tone's colours as `--tone`, `--tone-soft` and `--tone-fill` (see
  Added), which a part, a theme or a template draws in with one rule, and a
  part's box accent's; a value of no tone is as if not written.
- Change the room between parts that stand one after another to
  `--floe-room` (0.75rem) everywhere: in a stack (details' and tabs' panels),
  between the cards of stats, options, decision and html's cards (1rem), all
  cards in brutal and notebook (1.5rem and 1.75rem; facts' and links' were
  0.75rem in the other themes already), and in report's, design's, runbook's,
  handoff's and slides' sections (1rem or 0.8rem), where the paragraphs and
  headings of a text stand too.
- Change a list (`ul`, `ol` or `menu`) that is a part's box or takes a look to
  draw no marks, padding or room between its items, laid out in a grid, in parts
  and templates of one's own too; a template's `page.css` or a theme's `base.css`
  that writes a rule for every list no longer changes a part's box's display,
  marks, padding or room between items, nor a theme's such rule those of an
  element that takes a look.
- Change the diagrams of `floe diagram render`'s fragment to take no variable or
  color-scheme from the page around them, its `:root` or an element holding them;
  they draw in floe's fonts, and in a shadow root too. They take their colours
  through `@scope`, which a browser that does not read it drops. The diagram
  part's CSS has no colours of its own, so the CSS of `floe part show diagram part.css`
  alone draws black diagrams in a page without a palette. floe's palettes are
  written on `:scope`.

### Fixed

- Fix the mistakes of an image that cannot be read (not there, a link out of the data file's directory, unreadable), which named the image's path alone (`x/missing.png: no such file or directory`), not the item: they are now told at each item that names the image, as `d.yaml:7:24: the image "missing.png" cannot be read (no such file or directory)`, one line for each place it is written, as the other mistakes of an image are. `floe publish` now tells every mistake of an image at its item too, where it told those it read on the command line with no place (`floe: "dir.png" is not a file of an image…`). An image of a template's `assets/` that cannot be read when the page is drawn is told at its file, not at `page.html` (S335, S68).
- Fix the management page's buttons whose words broke over lines where it is narrow (a done task's "Edit" and "Archive the task", the help modal's switch, the foot of the modal that sets a page's task): their words keep to one line, and the row of buttons goes onto another. The task's archive button now says "Archive" and "Restore", as a page's does, and every button on the screen has an icon, the buttons in a modal, the bar of the selected and a toast keeping to words. The page shown for a page not found draws its picture at its size again, its button back in sight.
- Fix `floe publish` of a file whose name has no letter a to z or digit (`設計.yaml`, a `--link` of `資料/index.html`), which was refused as an `invalid slug ""` the publisher never wrote: it now says that no slug can be made of the name, at the file, and to pass `--slug` or rename it. A page published before under its own `--slug` is still updated (S339).
- Fix `floe icons search` of a word with spaces (`floe icons search gcp "api gateway"`), which found nothing: it is searched as its words. When `gcp` finds nothing, the message also says that a product without an icon of its own is drawn with its category's icon and its name as the label, and that `floe icons search gcp category` lists them (S340, S345).
- Fix a request that panics ending in `EOF` on the command line and in nothing at all in the server: it is answered with a 500 that says where it panicked and where the stack is (`server.log`), and the server goes on (S229).
- Fix "Take off the task" in the "…" menu of a page opened whole doing nothing: it now takes the page off its task, and the toast's Undo puts it back.
- Fix a task named by pages alone, which the management page showed from its pages and made a row of at the time of its first write, jumping on the task list (S286): every task a page names has its row, made with the page's write, and the management page shows tasks of their rows alone; a page's categories are always among its task's, so a category no longer shows up or goes with its pages. The management page writes pages and tasks by `PATCH /api/work` alone, in one write, which `PATCH /api/tasks` and a page edit's `work` were.
- Fix a mistake of a whole object or list (`missing property` and the like) told at the line of its first key, where it read as that key's: it is now told at the key that holds it (`a:` or `-`), or at its `{` or `[` in `[ ]`, where no key holds it. A mistake naming one key it does not know (`additional properties 'zz' not allowed`, `source: true` in a diagram's spec too) is now told at the line of that key, at the top too (at the value's column for text, at the key's for an object or a list). The mistakes of a file may come in another order.
- Fix the dates of the part `timeline` folding into two lines (`2030-05-14 10:42`, `11 月 2 日 10:12`): the column of the dates is now as wide as the longest date of the timeline, and the dots and lines of all its rows stand in one line, where it was 7rem whatever the date. A date up to 14rem (about 29 characters, as `2030 年 5 月 14 日（火）10:42` or `2030-05-14T10:42:07+09:00`) stays in one line; a longer one folds within 14rem, and one with no space in it breaks anywhere. One long date widens the column of every row of its timeline, narrowing the text beside them. The part narrower than 36rem is as before, the date over the title.
- Fix the lead of `report` with contents stopping at 56rem on a screen wider than about 930px, narrower than the title and the sections beside it: it now takes the same width as the title, as the lead of a page without contents does.
- Fix publishing a data file again without `--slug` after its page was renamed on the management page making a second page at the old slug: it now writes to the file's own page, found by the project and the file it was published from, as a link without `--slug` finds its file's. A title made of the slug follows the rename, so publishing again tells no title changed. `--force` without `--slug` no longer replaces another file's page when the file has one of its own. A store whose pages cannot be listed now stops a publish without `--slug`, of a link too, rather than take the name.
- Fix a command failing with "the server did not start" when the server it started took over 3 seconds to answer, on a busy machine: it now waits 10 seconds. A server that does not start, `floe stop` with a server that keeps answering or does not answer, and a port held by a program that does not answer are now told after 10 seconds, not 3.
- Fix the management page keeping an old list after a draw of it failed: the next look drew nothing until the list changed. The next read now shows the list again, and the page says "Could not show the list (…). It is shown again at the next read." until it does; a mistake of a read, of a draw and of the tasks are said one at a time, in that order, and none of them covers or clears the mistake of an action, such as a failed delete.
- Fix the management page drawing an older list over a newer one: an answer that came late, or one read while an archive, a read mark or another change was on its way, could bring back a row just archived or removed, for a moment or until the next look. The list is now read one load at a time, only while no change is on its way, and read again when a change is sent meanwhile; a read that does not answer in 10 seconds fails and says so, rather than holding the reads after it.
- The theme blueprint draws the heads of code, diffs and terminals, a note's title and a table's heads, and the template report its eyebrow, in the case they are written in, not in capitals: a browser copies the capitals, so a file name or code copied from them came out as other text.
- Fix where a row of too many or too few cells of a `table` is told: at each such row, not once at the start of `rows`.
- Fix changing a task's state on its page after the task was removed elsewhere: it made the task again, and now refuses, says so and reads the tasks again.
- Fix going back from a page opened whole, by the back button, `h` or the browser's own: the list that opened it, the page list or a task's page, now has that page's row focused and the page beside the list, or the pin focused on the task list. A page left unread stays unread: only its row has the focus.
- Fix a Venn diagram's label placed by its region standing 2 off the axis of a region symmetric about one: the
  points tried for it are now counted from the middle of where its circles are, not from their edge. Every label
  placed by its region may move, in the drawing and in `floe diagram inspect`: mostly by 2 or less across and down,
  further in a diagram past 2000 wide or tall, or in a region other circles cut into.
- Fix a diagram's mistakes and `floe diagram inspect` naming an edge's ends in two ways: an edge written by hand and an
  edge of a graph now quote the names at their ends, as a tree's did (`edge "b"→"c"`), and a graph's mistakes quote
  the names of the boxes they speak of too.
- Fix a diagram refusing an edge's label on a run it shares with another edge written by hand, as where two edges
  join at one part or part from one. Such an edge is no longer told as running through the label, when it runs on
  exactly the same line from side to side of the label; `edge "b"→"c" runs through label "SQL"` stays for any other edge.
- Fix a page nested past 512 open elements crashing `floe render` and `floe check`, and cutting the connection (`EOF`)
  in `floe publish`, `floe rebuild`, opening a linked data file and downloading a page. It is now refused with
  `html: open stack of elements exceeds 512 nodes`, at the data file, or at the value of a diagram's `raw` that goes that
  deep by itself. What the HTML library will not write (`<svg><col>x</col></svg>`) is told in its words as a mistake of
  the page, without `draw: write the page:`, `diagram: read raw SVG:` or `diagram: write raw SVG:`. The diff of versions
  answers such a page with 400 in the library's words, not 500 with `read a version of the page:`. `floe render` and the
  download of a linked data file write the page checked, without reading it again.
- Fix an edge at the bottom side of an icon with a label being refused as running through the label: the side now stands
  under the label, so an edge named from one icon to the one below it is drawn, and a diagram can flow downwards. An edge
  turning between an icon and its label, or with no side and the other part's top beside the label's, is now refused; write
  the side or move it below the label.
- Fix a diagram measuring bold text as regular: a table's title, a box's `main` row, a frame's heading, a graph
  group's heading and a `strong` label are now counted as wide as bold is drawn (0.64 of the size for a narrow
  character, from 0.6), and a `main code` row at the 12px it is drawn in, from 13. A table, box or graph with no `w`
  grows, the more the longer its text (a title of 49 characters from 416 to 440), and a diagram that passed may now be
  refused: a written `w` too narrow, a part at written coordinates overlapping a box that grew or a frame's heading, an
  edge by hand running through a box that grew, a `level` edge running through a lane's heading, a box straddling the
  edge of a group's frame, a part sticking out of a written `width` or `height`, or a bold `label` no longer inside its
  `in`. Level edges keep off the round corner of a box they end at, which grows for them where it was just short. Mistakes and
  `floe diagram inspect` may name a part in another language, the one now measured widest. Capitals still run wider
  than counted, so a long title of capitals can pass the edge of its box; only later than before.

- Fix a diagram table measuring its rows' types in code, though they are drawn in the plain font: a table with no
  `w` is sized and a written `w` is checked with the name and the two spaces after it in code and the type not, so a
  long type needs a narrower table than before.

- Fix a heading written in a template's Markdown field (adr's `context` and `decision`, design's `background`,
  postmortem's `cause`) drawn as large as the page's title, or as a section's heading with its line: such a heading
  is drawn as in a text, and floe's templates name their own headings outside the box of their Markdown.
- Fix a tree scrolled sideways on screen ending its ground at the box's width: in default, a long name ran out of the
  white ground onto the grey frame. The tree's list now reaches to its longest line, and a theme's ground on it with
  it; scrolled to the end, the room at the tree's right edge stays.
- Fix a Venn diagram's labels moving with a float's rounding: a label floe places in a region symmetric about a
  line went to one side or the other as the circles stood, a label at `x` and `y` touching a circle's edge was told
  outside its region, and `floe diagram inspect` named a label whose centre is on an edge `in` the circle, or not. Room
  and the circles a label is in are now told within 1e-9 px.
- Fix a diagram's `axis` tick of empty text (`""`) taking room as a label 10 wide: it was listed by
  `floe diagram inspect`, told overlapping a part, sticking out of the canvas or run through by an edge, and widened a
  canvas left out. It is now a mark alone.
- Fix a diagram's edge running along a grid line, an axis or a `line` where floe placed its run (between facing
  sides, round two sides of one way, the two turns into a side): the run now keeps 4 off the lines of `axis` and `line`
  parts, moved to the nearest place where the edge makes no new mistake, and stays where there is none.
- Fix a diagram's mistakes, and `floe diagram inspect`, naming a part with no name by its English text, though
  another language's text decided its width, a frame with no name nor label as `frame ""`, and a dot by its centre
  though it had a name. A part with no name
  is named by its text in the language that decides its width (the widest, English of two as wide), and with no text
  either by where it stands (`frame 10,10 250x150`, a bar with no label `bar 200,37 80x14`); a tree's node so too, in
  its box and in its edges.
- Fix a diagram's placement mistake between two parts written by hand (an overlap, a crossing, an edge through a
  box, a part across a frame's edge) told at the line of the one compared first, often the part that was there
  before: it is told at the part written later in `parts` now, which was added or moved last most often. One between
  a node of a tree and a part written by hand is told at the part written by hand, as one with a graph's is. Two
  parts named in the same words (two boxes of one text) told at one line are told once.
- Fix parts a template of one's own places in a band, which could not be read there. In swiss, whose panels, cards
  and notes have no ground, nearly every part's text and focus ring stayed in its own colour on the band's; they are
  on the surface with the page's text in a band now, as in other themes, a table's first column clear of the
  surface's edge, and stay as before in any other face. In every theme, in any filled face (a band, or a face a
  template paints itself), tabs' titles and focus ring and the focus ring around links' cards are in the face's text
  colour now, and the picked tab in the face's two colours turned over, told from the others and from the face.
- Fix an edge to a diagram's dot named in `from` or `to` drawn to the dot's centre, its arrow hidden under the dot.
  It is drawn to the dot's outline now, towards the edge's first point off the centre, the tip of its arrow (or
  other marker) on the outline. It is still drawn to the centre when that run is too short for the marker, between
  two dots that touch or overlap, and when the end is written as a point at the centre. The edge is checked, and its
  label placed, as before. The built-in gantt example draws its arrows into the release again.
- Refuse a diagram's label straddling the edge of a frame, as a box or a dot there is, when it is written where it
  stands (at an `x` and `y`, or an edge's at `at`) or belongs to a part that may not straddle a frame itself (an icon,
  by hand or in a `graph`, or a bar); it was drawn across it. A label is checked by its box, 5 wider each side than its
  text. The other labels (an edge's with no `at`, an axis's, a `graph` edge's, and one in its region with `in` alone)
  are not checked against frames. The mistake is told once for each edge crossed, with where that edge stands and where
  the part spans on that axis (`label "…" straddles the edge of frame "…" at x 140, spanning 114.2 to 145.8`), for boxes
  and dots too.
- Fix printing cutting what scrolls sideways on screen: a long line of code, a diff or a terminal, Markdown's code, a
  wide table and a deep tree were cut at the paper's right edge, with a scrollbar drawn on paper. On paper they fold
  their lines now (a table shares the paper's width among its columns, its words kept whole); a table whose longest
  words are wider than the paper, or a field's long name, still widens the page, which Chrome prints smaller, and such
  a table in a card side by side with others reaches past its card, over the next one.
- Fix an edge written by hand to a box of a graph or a tree, or a group's or a lane's frame, with no side written,
  which could end at the middle of a side one of their edges ends at and run over it. With `via` or without, it takes
  a side their edges leave, with the via next to it (or the other end) outside it, across first; with none, the side
  it took. `from: [100, 0], via: [[100, 60]], to: b`, b under a in a graph a→b, ran into b's top over a→b
  (`V60 H44 V73`) and now into its right (`V60 V87.5 H68`); some edges refused as running through a box or crossing
  one of the graph's are drawn now.
- Fix swiss's cards keeping their left line on the first card of a wrapped line while the first card of all had none:
  every card has the same line now (the first card's text stands 1px further right, past it). In a band, swiss's
  cards' lines and numbers are in the band's text colour, not black on its red.
- Fix a right-angled edge with `via` running back over itself at a via when the next point lay behind the way it
  came: it turns the other way first there. `from: a, via: [[200, 20]], to: [100, 0]` from a box below ran back along
  y 20 (`V20 H200 H100 V0`) and now turns up at the via (`V20 H200 V0 H100`).
- Fix the next page waiting long to show its content while many pages' tabs were open, and the pages opened after
  the sixth not reloading when changed: each open tab held a connection to the server, and Chrome opens six at most
  to one server. A page holds none now, and asks whether it changed every second while it is shown.
- Fix an edge with `via` whose end is a box, table, row, icon, frame or bar named without a side, which ran along the
  end's edge or back over itself after its last via, or was refused as running through the box it started from. Such
  an end, at either end of the edge and on a `straight` edge too, takes the side towards the via next to it when that
  via is beside it, above or below it; with the via off both ways, the last end of a right-angled edge takes the side
  across the way the edge came. The last run of a right-angled edge meets its end's side square, a side written too
  when the via is outside it, with two turns halfway when one turn would run back over the edge.
- Fix faint text drawn by `opacity`, which could lie over a scrolling code's
  focus ring: a code's file name, a terminal's heading and output, a diff's
  lines outside a hunk, a design's undecided pick and a diagram's disabled
  button are faded by their colour. A diff's lines outside a hunk are as faint
  as a terminal's output, 70% rather than 65%.
- Fix a diagram on a panel, in details or on a theme's patterned page, whose
  labels' boxes and hollow ends and dots stood out in the page's ground: a
  diagram paints its own ground, `--bg`, in its frame.
- Fix washi's cards losing their inner rule while picked (options) or focused (links): the rule is drawn within the
  card, by `--floe-inset`, no longer by the outline that the ring takes. A card of your own in washi drawn on a ground
  other than `--surface` shows a band of it between the rules, and one that takes no `panel` has the outer rule alone.
- Fix the ring of the option picked sinking into brutal's and notebook's filled cards (2.3:1 on brutal's blue): it
  takes the face's text colour there, as a focus ring does.
- Fix swiss's parts in a band (a note, details, a table) and a template's own
  panels and tables there, whose text and rules were dark on the red band and
  whose links, titles, marks and focus rings red on it: they draw in the band's
  text colour.
- Fix a page that grew wider than a phone's screen when a long line of code or a
  wide table stood in a list's item or a quote (in a text, a
  card of options or of a decision, a step, a tab): on screen, code, tables and
  trees now scroll inside their box at any place, and widen nothing around
  them. Print is as before.
- Fix washi's facts, whose pills lay over the inner rule of their tile, and
  swiss's first card picked in options, whose frame touched its text.
- Fix pills unreadable on a filled face (a band, a note): blueprint's,
  editorial's, washi's and swiss's, which have no ground, drew in the tone's
  colour on it; they now take the face's text colour, the tone told by the
  word alone, and notebook's no longer let the face through.
- Fix decision marking a list in its text with the × of the options it set
  aside.
- Fix an edge told running through a box it does not touch: the check
  measured a box by the rectangle around it, so an edge past a hex's points
  or a rounded corner (a pill's or a state's end, a box's, a table's or a
  bar's corner) was refused. It now measures the shape drawn, which floe
  decides in one place for drawing and checking alike. Corners are circles,
  of a radius no more than half the shorter side, which changes what is
  drawn: a tall pill's ends are half circles, and the corners of a box
  narrower than twice their radius (a narrow state, a bar 4 wide, now
  radius 2, not 3) are round, where a browser drew ellipses; a table's
  heading takes its outline's arcs at the corners, not curves of its own.
- Fix the diagrams of `floe diagram render`'s fragment taking the variables of the
  page they are pasted into: a page's `--bg` filled the edges' labels, so that
  they could not be read. The fragment carries `default`'s palette on the diagrams
  alone, and the page outside them keeps its variables and color-scheme.
- Fix shadows reaching other parts and the page's edge: app's and aurora's
  soft shadows fell on the part below, aurora's past the page's edge and onto
  the card beside, and notebook's tilted sticky notes onto the part below.
  What a box draws outside itself now stays within `--floe-room` (0.75rem)
  on every side, the room floe leaves between parts, in a size that
  does not follow the box's, and takes no room. What changes on a page:
  - app's, aurora's and notebook's shadows are smaller; app's close shadow is
    gone; notebook's sticky notes no longer tilt, and its cards no longer
    leave room round them.
  - brutal's boxes and cards are 6px wider and no longer leave 6px below
    them, as its shadows take no room; soft's faint shadow on panels and code
    is gone.
  - The parts in adr's and postmortem's sections, in runbook's steps, swiss's
    cards, and a tab's title and its panel stand `--floe-room` apart (runbook
    put no room between a step's text and its terminal).
  - ink's cards keep their 4px shadow, which falls within the page's side and
    the cards' gap.
  - A theme of one's own draws a shadow by `--floe-shadow-x`, `-y`, `-blur`
    and `-spread` in its palette and `--floe-shadow-color` on the box, and a
    shadow within a box by `--floe-inset`; `--shadow` alone draws none, and a
    `box-shadow` written on a part's box drops them.
  - A part's box writes no margin of its own: a template's `page.css` or a
    theme's `base.css` that sets a margin on `ul`, `ol`, `dl`, `figure` or
    `form` moves the parts drawn in them too (checklist, links, steps,
    timeline, facts, keys, entries, stats, code, image, diff, terminal, tabs).

- Fix a row's mark left behind when a theme changes the row's padding (retro's
  links, whose mark stood above the link's line): the marks of links, note,
  timeline and the look `numbered` (steps, runbook) stand on the baseline of
  their row's first line in every theme, and the line to the next mark starts
  0.35rem below the mark, however big the mark or the title. What changes on a
  page:
  - A row whose mark is taller than its first line grows by the difference:
    steps' title and text stand 7px further apart, a note with a big mark
    (default, brutal, washi, blueprint, notebook) is up to 9px taller, and
    ink's note 7px shorter.
  - timeline's date stands on its title's baseline, a few px lower, and its
    line starts 5.6px below the dot (was 4px).
  - A note's text keeps `--floe-mark-gap` from a mark with a border (brutal,
    notebook, blueprint), 2 to 4px further right.
  - swiss's note has its mark within the note's padding, as the other themes
    do, its text 18px further right; the first card of swiss's links has its
    mark at the card's edge, as its first cards have their text.
  - The item of an `<ol>` that takes `numbered` breaks when it holds text
    written straight in it: write its text in an element
    (`<li><p>…</p></li>`).
  - An older browser that reads no anchor positioning draws the line to the
    next mark out of place.

- Fix the outer losing where looks and parts nest: a look's style for an
  element in it that takes another look, and a part's for the box of a part in
  it, now win over that element's own, whatever the selectors; a look still
  loses to the box of a part in it. What changes on a page:
  - A card in an element of `cards` that takes `panel` itself
    (`<div data-floe-look="cards"><div class="card" data-floe-look="panel">`)
    is drawn as the cards of a part are, in every theme (in default it was
    the panel's grey; in editorial, ink, notebook, swiss, washi and brutal it
    lost the theme's line, shadow or margin of a card).
  - A part's box placed in a note's box by format html is spaced from what is
    before it by the note's 0.5rem, and keeps its other margins (brutal's room
    for its shadow).
  - Markdown's `####`–`######`, `---`, and a `<figure>` or `<dl>` of format
    html have no margins of a browser's own anywhere, as headings, paragraphs
    and lists already had none: what holds them spaces them (a section's gap,
    a note's 0.5rem). On a page's sections, a `####` and a `---` stand a
    section's gap from the next, not more.

### Removed

- Remove the Japanese versions of the docs that `floe guide` and `floe diagram|part|template|theme guide` print (guide, diagrams, parts, templates and themes): floe prints English only. The English docs are now in `docs/agents/` (S331).
