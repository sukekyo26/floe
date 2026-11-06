# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add one form to the buttons of words on the management page: square corners, a thin edge, 2rem high and 2.25rem at a modal's foot; round stays for the buttons of an icon. The Set the task and child task dialogs name their fields Task ID and Title, with an example, and a new task needs a title; a task saved from a screen read before another change is refused, and the tasks are read again.
- Add the task state `todo`. A new task starts in it, whichever way it is made. The list's column goes in the order of the work, To do, In progress, Waiting, Done, Archive, opening on Waiting as before, and To do lists the latest to come to it first.
- Add child tasks. `floe work --task "#130" --parent "#123"`, or the parent field of the Set task dialog, makes a task the child of another of its project, one level deep, and "+ Add" on a task's page makes a new one under it or takes one that is there; `--parent ""` takes it off, and `--forget` is refused while a task has children. On the task list a child's row names its parent and a parent's row says how many children it has; a task's page lists its children, or its parent, over its pages. A parent with no page of its own moves to the archive by its children's pages.
- Add the task screens. `floe work` says which task a page is on, its step and pin, and a task's state, outside link, title and order of steps, held in `tasks.json` in the store; pages keep it when published again, renamed or drawn again. The task list at `/tasks` shows one project's tasks by state, longest waiting first, with their step, pages, unread and link, that project's pinned pages over them, and a switch to it beside the logo once a page is on a task or pinned. A row opens the task's page: its pages under their steps, its state to change, and a button that archives them once it is done. A page's `…` menu pins it, opens its task and sets its task, step and URL, and the page list's rows show a page's task and pin.
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
- Add what changed between two versions of a stored page to the management page: by the words under each heading, side by side, all
  together or the changes alone, with the two versions to choose, contents to resize or hide, and `n` `p` to go through the changes. `Diff` over a page
  and `d` open it; a page newly at another version says how many sections changed since the one read. Opening it marks nothing read.
  A page's HTML is never brought in: the server reads each version into words, which the page writes as text.
- Add `icon: { name, icon, label }` to the nodes of a diagram's `graph`, so that an architecture diagram with icons, an AWS one say, needs no coordinate but the graph's `x` and `y`. An icon has no box round it and stays 48 square; the n edges at a side of it end at the points that part the side into n+1 equal parts. A `group` or a lane takes an `icon` too, in its frame's corner. `floe diagram example aws-graph` shows them.
- Add a modal that searches the pages, opened by `Ctrl+K` (`⌘K`) anywhere, by `/` on a page open whole, or by the magnifier on a
  phone's whole page: it searches by title, description, slug, project, path, template and tag, the archived last, opens a row
  whole, and acts on the row reached with the buttons of a row. Add a modal of the keyboard shortcuts, opened by `?` or the button in the
  bar. On a page open whole, `h` goes back to the list. With the focus in a page, `j`, `k`, `h`, `?`, `/` and `Ctrl+K` now work too (`Ctrl+K` alone in a link's HTML and a folder's,
  which may use the letters themselves).
- Add marking a stored page unread: a toast offers it for 8 seconds as a page you open becomes read, and "Mark as unread" stays in
  the page's menus. It goes back to how it looked before you opened it, by `DELETE /api/artifacts/{slug}/read`; opening it again reads it.
  `POST /api/artifacts/{slug}/read` now answers `{"cleared": true}` when it read the page up to its last version.
- Add what a viewer has read of the stored pages, kept in `reads.json` in the store. A page opened in its frame tells
  the frame the version it served, which the frame sends to `POST /api/artifacts/{slug}/read`, and `GET /api/artifacts`
  gives each stored page `news` (`new` for a page not seen, `version` for a version after the one read) and `version`
  (the number of its last). Pages from before floe first writes `reads.json` count as read. `GET /api/artifacts` also
  gives the name of the `template` of a stored page and a linked data file; `floe rebuild` writes it to the meta.json of
  pages published before.
- Add the versions of a stored page: a publish of a data file, or images it reads, other than the last version's
  keeps the page before as a version and makes the new page the next one (a rebuild, or a publish of a theme only,
  draws the last again and adds none). A page keeps 20, the oldest going first, those before the last gzipped under
  its directory. `GET /api/artifacts/{slug}/versions` lists them, and `/a/{slug}/?raw&v=<n>` serves version `n` as the
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
  A graph holds 200
  boxes and groups and 400 edges, and the edges of all the graphs of a diagram
  may pass 4000 ranks between their ends, all told. The diagram examples
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
  a bar's track, progress's ground and a keycap read it.
- Add `--radius`, `--radius-small` and `--radius-pill` to the palette, one
  length each: the round of a box's corners, a small box's within one and a
  pill's. Parts, templates, floe's base and themes read them, and a theme sets
  its corners in its palette alone.
- Add the tone `muted`, a quiet one in `--muted` on `--muted-soft`, wherever
  a tone is written (note, a pill, a table's pills, bars, progress, stats,
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
  `floe template show` and `floe theme show` print every file a part, a look,
  a shared schema, a template or a theme reads, as written, each under
  `==> name <==` after a `files:` line naming them; with a file named after,
  that file alone, as written but the byte order mark at its start
  (`floe part show code part.html`).
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
  `PATCH /api/artifacts/{slug}` take a page off `floe list` and the management
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
  or as `tags` in `PATCH /api/artifacts/{slug}`; they show under a page's
  title, are searched, and stay when the page is published again. What is
  reached by keyboard shows one thin grey ring, and a button pressed with the
  mouse shows none.
- Add floe's template `report`: a header over sections, each a heading over
  parts and then subsections, one level of a heading (an `<h3>`) over parts (a
  heading, when written, is not empty, and a section with subsections has
  one); and thirty parts a template refers to as
  `floe:///parts/<name>.json` (`floe part list` lists them): twenty-eight that a
  section of report takes beside Markdown `text` and `html`, from `code`,
  `terminal`, `table` and `image` (an image of the data file's directory,
  such as a screenshot) to `findings`, `decision`, `entries`, `timeline`,
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
  `floe part list`, `show`, `new` (the smallest part, to fill in) and `check`.
  A part's schema takes floe's formats (`html`, `markdown`, `image`) as a
  template's does, and a shared schema holding them. A shared schema is
  checked from its root, whether a schema names it or not. A part's
  `part.html` may call the functions floe's parts draw with (`copyButton`,
  `bars`, `diff`, `tree`, `diagram`, `script`); `bars` refuses an item not
  written as keys and values, and a value that is not a number or is below 0.
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
  `inspect` and `example`.
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

- Change where a `group`'s `label` takes room in a `graph` laid out down: it stands above the group's first rank, a box no edge reaches from above standing under it if it comes before those an edge reaches in the rank, so that nested frames start together and the graph is narrower. A few graphs grow, as boxes start elsewhere, and an edge written by hand that comes to run through a `label` is told: go round it by `via`.
- Change `floe icons search` to find a service by its common short name, listed first: `alb`, `sqs`, `s3`, `ecs` and 26 more, from a table floe keeps. `s3` used to list only icons with `s3` in their names, and not Amazon S3 itself, and `alb` found nothing. When nothing is found, the message now says to try fewer words or a word of the full name, and that `floe icons search aws` lists them all.
- Show a project's name as a pill in the color of its chip on the shelf: on a row, in the search modal, at the head of a task's page, and on the task list's filter button and its head with the column folded. Put the button that archives a page, or gives it back, on its row beside the star, in place of the item in the row's menu, and offer Restore on the toast after archiving.
- Show the way back on a page open whole at every width: beside the logo it says where it goes, the task's name, the task list or the page list, whichever opened the page; on a narrow screen its mark alone, as before.
- Change `report`'s `lead` to a line of Markdown, as the `summary` of `design`, `handoff`, `postmortem` and `runbook`: code in backticks, bold and links are drawn, while a heading, a list or a table stays text. What looks like a tag (`<prefix>`, `Vec<u8>`) is left out and marks such as `*` and `_` are read, so write those as text in backticks. A stored page takes this when published again or at `floe rebuild`, and a linked data file as soon as floe is updated. `floe rebuild` does not draw earlier versions again, so comparing one with the latest shows the `lead` as changed.
- Warn of a fenced block of code in the `text` of an item of `entries`, as in a `text` part: a page that has one now gets a warning, and is drawn as before with the same exit code.
- Change `floe diagram inspect` to begin each diagram with a line of where it is written, its title and its canvas (`page.yaml:9:11: diagram "T" 560x190`, `spec.yaml: diagram "T" 560x190`), and to print where the parts went for a diagram the checks refuse too, as a guide, on stdout with its mistakes on stderr and the exit code still 1.
- `GET /api/artifacts/{slug}/download` takes `?v=<n>` to save that version of a stored page as `<slug>-v<n>.html`, and answers
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
- Change the Markdown a part holds (a note's text, an entry's, a faq's answer, a card's text in options and decision,
  a step's and a timeline item's, an item of format `markdown` in a part's own value) to stand in a text's box, as a
  text's does: its paragraphs, tables, quotes and lists are `--floe-room` (12px) apart in every part, where each part
  spaced them its own way before (2.4px in steps and timeline, 5.6px in entries, 6.4px in options, 8px in note, faq and
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
  design, runbook and handoff. The look `rows` pads its rows by 1.3rem at the sides (entries, keys, checklist and faq
  from 1.2rem; design's decided list from 1rem; an element of your own HTML that takes it from none); faq's question
  still opens across its row. A part or a template of your own reads `--floe-pad-block` and `--floe-pad-inline`. A part of your
  own that takes `rows` drops the padding it writes at the sides of an element in a row (a `summary`, a `div`), which
  now sits twice as far in, and widens an element that opens the row across by a negative margin, as faq does; a
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
- Change progress's ground to the quiet ground, a little lighter (the line's
  colour 45% into the surface, as bars' track, not 60%), and a keycap's to it
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
  notebook, 6px in app and ink, 3px in editorial and washi); progress's bar and
  handoff's and runbook's pills take the theme's pill; runbook's box of rolling
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
- Change findings' nit to be a pill of the tone `muted`, drawn as the theme
  draws its other pills: with no ground in blueprint, editorial and washi,
  filled in ink, in the theme's text colour in brutal, notebook and swiss.
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
  part's CSS has no colours of its own, so the CSS of `floe part show diagram`
  alone draws black diagrams in a page without a palette. floe's palettes are
  written on `:scope`.

### Fixed

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
  wide table stood in a list's item or a quote (in a text, a faq's answer, a
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
