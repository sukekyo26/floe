# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

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
  without options, what it touches, and sections of your own as in `report`.
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
  when it is saved.
- Add `floe check` and `floe render`: check data files without writing them,
  and write each page to an HTML file that opens without floe (`--check` for
  CI), the mistakes printed as `file:line:column: message`, every one of a
  stage of the checks at once.
- Add `floe rebuild`, which draws every stored page again with the templates,
  parts and themes of now, keeping the page it had where it no longer draws.
- Add `floe list`, `floe rm`, `floe serve` and `floe stop`, and a management
  page in English or Japanese, by the browser's language, that lists, edits,
  opens and downloads pages: a shelf on the left picks all, the favourites
  (which head the list of all), the archived, a tag and the project (picked
  again, let go), a row of buttons on a narrow screen; the bar over the list
  picks the kind and the order (from a menu of the page's own, by mouse or
  keys), and holds the actions on the pages selected
  while there are any; the list groups its pages by project in round cards,
  each project in a color of its own, those with the most pages first, a row's
  menu holding its edit, opening, download, archive and delete; and the bar over
  an open page stars and archives it with buttons of icons alone, filled in while
  on, and its title opens the pages of the list it was opened from, to search and
  pick from. Tags are given in a page's edit dialog, to those selected,
  or as `tags` in `PATCH /api/artifacts/{slug}`; they show under a page's
  title, are searched, and stay when the page is published again. What is
  reached by keyboard shows one thin grey ring, and a button pressed with the
  mouse shows none.
- Add floe's template `report`: a header over sections, each a heading over
  parts; and thirty-two parts a template refers to as
  `floe:///parts/<name>.json` (`floe part list` lists them): thirty that a
  section of report takes beside Markdown `text` and `html`, from `code`,
  `terminal`, `table` and `image` (an image of the data file's directory,
  such as a screenshot) to `findings`, `decision`, `risks`, `timeline`,
  `tabs`, `diagram` and `script`, and `cell` and `stack`, which other parts
  hold.
- Add nine themes a data file picks with `theme:` (`default`, `soft`,
  `editorial`, `ink`, `brutal`, `blueprint`, `notebook`, `swiss` and `washi`),
  each a palette of CSS variables and the shape of the parts. Pages in
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
  A mistake of a part, shared schema, theme or template of your own stops
  every page drawn until it is fixed. The directories of parts, themes and
  templates of your own that are one, or one in the other, are refused for
  their settings.
- Add diagrams drawn as inline SVG from a spec of parts on a canvas (frames,
  boxes, tables, icons, edges joining named parts, dots, bars, axes, circles,
  labels, lines), their text in one language or several (`{en: …, ja: …}`),
  checked for overflowing text, overlaps, edges through boxes and crossings
  in every language before they are drawn: a `diagram` part in a data file,
  or `floe diagram render`, `inspect` and `example`.
- Add `floe icons fetch` and `floe icons search`, which keep the AWS
  Architecture Icons for the `icon` parts of diagrams. Icons not fetched, or a
  directory for them that cannot be used, are told once, at no place in a
  data file, however many diagrams and files need them.
- Add a page's CSP, which keeps the scripts of a page drawn from a data file
  from sending what it holds out, on the server and in a page `floe render`
  writes or a download gives. Such a page holds one charset, floe's, first in
  its head, in place of those `page.html` writes; `floe render --check` tells a
  page written before with two.
