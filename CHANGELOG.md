# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add a template's `words.yaml`, written as a part's: its `page.html` draws
  the words with `{{word "status_doing"}}` in the page's `lang:`, else in
  `en`, so that a template draws its own words in every language.
  `floe template check` refuses a word `page.html` names that `words.yaml`
  does not have.
- Add `floe:///parts/any.json`, one part of any kind, one key naming it, as
  report's sections hold them: a template of your own lists parts with
  `"items": { "$ref": "floe:///parts/any.json" }` and draws them with one
  line in page.html, `{{range .parts}}{{range .}}{{with .}}{{.}}{{end}}{{end}}{{end}}`.
- Add `floe part`: `list` the parts, floe's and your own, with where they are
  and their descriptions; `show <name>` what a part takes and its items, the
  looks it takes, the parts it holds and the themes with a file of it;
  `new <name> --from <part>` a part of your own as a copy of another, its box
  renamed and the file of the original of each theme copied, writing
  nothing over; and `check` every part and theme of your own, a mistake
  exiting 1. A look alone gives its description in `part.yaml`'s
  `description:`.
- Add parts of your own: a directory each in `FLOE_PARTS_DIR` (default
  `$XDG_CONFIG_HOME/floe/parts`) of the files floe's parts have, referred to
  as `floe:///parts/<name>.json`; its `part.html` draws its items, and its
  schema takes no format of floe's (`html`, `image`, `markdown`), nor
  `any.json`; no schema, a template's too, names a part by
  the `$ref` at its root. One of the
  name of one of floe's takes its place whole, and floe's themes do not
  style it. A part takes the look
  of floe's, and those it takes, by its `part.yaml` (`looks: [panel]` draws
  it in every theme's panel), and a look alone of your own is a
  `part.css` without a schema. Themes of your own go in `FLOE_THEMES_DIR`
  (default `$XDG_CONFIG_HOME/floe/themes`), a directory each, a theme of the
  name of one of floe's taking the place of its files of their names, but for
  its `palette.css`, which goes over floe's and changes what it writes. A part
  or theme that cannot be used is refused to the pages that use it and warned
  of by `floe template check`; a linked page is drawn again when a file of a
  part or a theme is saved, and a publish to a server of other parts or
  themes is refused.
- Add twenty-two parts, which the built-in `report` takes and a template
  refers to as `floe:///parts/<name>.json`: `stats` (large numbers with a unit and a
  note in a tone), `progress` (bars of `done` of `of`), `steps` (numbered,
  each `done`, `doing` or `todo`), `checklist` (items ticked when `done`, with
  their count), `compare` (rows `before` and `after` under two headings, a
  row the same both sides drawn faint), `options` (cards of `pros` and `cons`,
  the one to `pick` ringed under its badge), `facts` (names with a value or a
  pill), `diff` (a diff as git writes it, its added and removed lines
  coloured and counted), `tree` (paths nested into their folders, each with a
  note and an `added`, `removed` or `changed` mark) and `links` (cards of a
  title, a URL and a few words); `findings` (a review's, each a `level`,
  where, what, why it was left and a pill of what became of it), `decision`
  (a title over why and the options set aside), `risks` (how `likely` and its
  `impact`, coloured by both), `matrix` (cells of ✓, △ or × by row and
  column), `fields` (a setting's or a command's, each its type, whether it is
  required and its default), `timeline` (dates down a line), `bars` (values as
  bars), `faq` (questions opening their answers), `glossary`, `keys`
  (keycaps and what they do), and `details` and `tabs`, which fold the parts
  they hold under a title or show one tab's at a time, a `stack` of `text`,
  `code`, `terminal`, `diff`, `note` or `table`. A template's own items may
  hold a `cell` (a line of Markdown or a pill) or a `stack` too. They take
  the look of tables, cards and code in every theme, and draw their few
  words, such as the headings of `risks`, in the page's language;
  `--code-add` and `--code-del` colour a diff's lines.
- Add `floe render <data.yaml>...`, which writes the page of each data file
  to `<name>.html` beside it, or in `--out-dir`, for a repository to keep the
  data file for agents and its page for people. The page is the download of
  the page `floe publish` stores: it opens without floe, with its CSP in a
  `<meta>`. A page is written only when it changes, and nothing is written
  while a data file has mistakes. `--check` writes nothing and fails when a
  page is missing or not what its data file draws, for CI.
- Run scripts in data files: the built-in `report` takes a `script` part, and
  a template refers to the part `floe:///parts/script.json`, each `js`, a function the
  page calls with an empty element in its place and `data`, any value. A `js`
  holding `</script` or `<!--` is refused, pointing at the `js`.
- Write tables, cards and notes in data files: floe's parts `table`
  (`columns`, a `num` one aligned right, and `rows`, each cell a line of
  Markdown, a number in quotes, or a pill, `{ pill, tone }`, as many in each
  row as there are columns), `cards` (each a `title` over Markdown `text`) and
  `note` (Markdown `text` with a `tone`, under a `title` if one is given), each `tone` `accent`, `good`, `warn`
  or `bad`. The built-in `report` takes them, and a template of one's own
  refers to one with `"$ref": "floe:///parts/table.json"`: floe checks the item,
  draws it, and puts the part's style once into a page that names it, as a
  part's element does (`data-floe-part="table"`, the box of one part), a
  table of Markdown (in the table's look) and HTML that names a part so
  (`<span class="good" data-floe-part="cell">`) or takes a part's look without
  being its box (`data-floe-look="panel"`) included. A part takes the look of
  others by its `part.yaml` (`looks: [table]`), and the looks those take:
  `panel` and `rows`, looks alone, are the box most parts are drawn in. A
  part's style reaches the elements that have its look, not a part held in
  them, and comes after the style of the looks it takes. A template's item of
  `"format": "markdown"` is Markdown text, which floe draws as HTML.
  `floe template show` tells a part's items.
  A template's `page.html` takes `markdownLine`, which draws a line of
  Markdown without a paragraph around it, keeping a cell like `-` or `1.` as
  text.
- Write code and commands in data files: floe's parts `code` (`text` as it is,
  under its `file` with its `lang` at the right) and `terminal` (`steps`, each a command,
  `run`, with what it printed, `out`, under the name of its `shell` and at its
  prompt: `bash` or `sh` `$`, `zsh` `%`, `powershell` `PS>` or `cmd` `>`),
  which the built-in `report` takes. The prompt is drawn by the style, so a
  copy of the commands leaves it out. Both are dark in light mode too, in the
  colours of `--code-bg`, `--code-fg` and `--code-prompt`. A `run` whose first
  word is a prompt (ending in `$`, `#`, `%`, `>` or `❯`, such as `~/app$` or
  `C:\work>`) is refused. `report` refuses a fenced block of code in `text`,
  pointing at the parts to write code and commands in.
- Pick the look of a page, in any template, by one of floe's themes
  (`theme:` in a data file, `floe publish --theme`): `default`
  (faces set in panels, notes in their tone's colour), `soft` (faces with
  round corners), `editorial` (rules and thin frames, as in print), `ink` (a
  table's heading in ink, states filled with their colour), `brutal` (thick
  black frames, primary faces and hard shadows), `blueprint` (blue ink on
  squared paper, every cell ruled), `notebook` (ruled paper, marker strokes
  and sticky notes), `swiss` (heavy black rules, large numbers and red, states
  told by shape) or `washi` (sumi and vermilion on washi in a Mincho face,
  seals and quotes ruled in vermilion), each in light and dark mode.
  `blueprint`, `brutal`, `ink`, `notebook`, `swiss` and `washi` give the
  whole page their colours too, its ground and its rules (blueprint's squared
  ground, notebook's ruled one in a rounded face, washi's double rules in a
  Mincho face), and `default`, `editorial` and `soft` draw it in default's. A
  theme restyles each part and look in a file of each (`panel.css` every
  panel), its style reaching the elements that have the look but not a part
  held in them, and Markdown's quotes, and goes into every page after the
  parts' style; the template's `page.css` places the page, in a layer under
  them, and a template has no themes of its own. A
  theme's `palette.css` sets the page's colours and faces as CSS variables, in
  the bottom layer over `default`'s, which defines them all: the parts,
  diagrams, the template and the themes' files of the parts draw with them
  (`--rule` the page's rules), and floe's base draws the page's ground, colour
  and face with them (`--page-bg`, `--fg`, `--font`) on its root under any
  template; a template keeps colours of its own, those among them, by setting
  them on `:scope`, as a ground it draws on `body` covers the body's box alone,
  and the parts take them in every theme but for the colours a
  theme keeps of its own, which the palette has no name for (brutal's
  primaries, notebook's markers); `--on-fill` is the text on a face filled
  with the accent or a tone's colour, `--shadow` the colour of shadows.
  `palette.css` holds those variables, a theme's of its own names too, alone,
  without `!important`, on `:root` and then in `@media (prefers-color-scheme:
  dark)`, and is warned of where it sets a colour for light mode alone, as is a
  template that sets one so on `:scope`. `floe part check` and `floe template
  check` warn of a colour a part's `part.css`, or a theme's `base.css` or file
  of a part, writes as a value, and of a variable those files of a theme set.
  A part of your own cannot be named `palette`. `floe
  template show` lists the themes.
- Draw diagrams in data files: the built-in `report` takes a `diagram` part,
  and a template refers to the part `floe:///parts/diagram.json`, each a spec as
  `floe diagram render` takes it, written in place. A diagram is checked as
  `floe diagram render` checks one, its ids across the page, and every
  mistake is reported at the data file's line and column. The page gets the
  diagrams' style and zoom once.
- Add `floe diagram render <spec.yaml>...`, which draws diagrams from YAML
  specs of parts placed at absolute coordinates and prints them as one HTML
  fragment of inline SVG. It first checks for text that does not fit (frame
  headings too), parts outside the canvas (edges, lines and axes too),
  overlaps (with frame headings too), edges through boxes or labels, and
  crossing edges, and reports every mistake as `file:line:column`, saying to
  quote text that YAML read as something else, as a data file's does. The fragment
  carries the diagrams' style and a zoom toolbar (buttons, Ctrl / ⌘ + wheel,
  pinch, drag, arrow keys) worded in the page's language; `--no-assets` leaves them out.
- Add `floe icons fetch <set> [zip-or-url]`, which keeps an icon set (`aws`:
  the AWS Architecture Icons from the official Icon package) in `FLOE_ICONS_DIR` (default
  `~/.local/share/floe-icons`), and `floe icons search <word>...`, which lists
  their names. The `icon` part of a diagram and the `icon` of a frame draw
  them, switching to an icon's dark variant with the colour scheme, and an
  unknown name suggests close ones.
- Add `floe diagram example [kind]`, which lists the built-in example specs
  (architecture, AWS, class, ER, flow, Git, sequence, state, timeline, Venn) or
  prints one to copy. The examples come in English and Japanese: English, or
  the language `--lang` picks.
- Add `floe publish <data.yaml>`, which stores a data file with the images it
  shows (`<slug>/source-<id>/` in `FLOE_DIR`, where it can be taken back from) and
  prints the URL of its page on `http://127.0.0.1:7325`, drawn with the
  template it names each time it is opened.
  The built-in template `report` lays out a header and sections of Markdown
  text, with HTML parts for what Markdown cannot say. The data is checked
  against the template's schema first, and every mistake is reported at its
  line and column (`file:line:column`). Where YAML read text as something
  else (a `: ` or a ` #` in plain text, written over lines too, a value
  beginning with `` ` `` or `@`, a `computed.yaml` or `checks.yaml` expression a
  `#` cut short), the mistake
  says to put the text in quotes, at where the text was split or the comment
  began. The scripts of a page drawn from a
  data file cannot send what it holds out: its CSP blocks `fetch`, XHR,
  images from elsewhere and forms, and a `<base>`, in its download too. A page is in the
  language its data file names (`lang:`, a BCP 47 tag such as `en` or `ja`),
  which every data file holds. It starts the server in the
  background when needed, which checks and saves the page (the server is the
  only one that writes the store), and opens a new page in the browser. An HTML file is
  not stored: `--link` registers it in place and shows it as it is, served
  with the files under its directory. A data file linked with `--link` is
  drawn from its place instead of stored. Saving a data file, an image it
  shows or a file of its template, or fetching the icons its diagrams show,
  reloads the open tab, and while it has mistakes the tab shows them in place
  of the page.
  `--theme` picks another theme, and `--slug`, `--title`,
  `--description` and `--force` are also available.
  Republishing a slug says so on stderr, with a warning when the title
  changed.
- Use templates of your own: a directory in `FLOE_TEMPLATES_DIR` (default
  `~/.config/floe/templates`), a symlink included, takes the place of a
  built-in template of its name. A published page is drawn with the server's
  templates; a publish whose `FLOE_TEMPLATES_DIR` or `FLOE_ICONS_DIR` is not
  the running server's is refused, pointing at `floe stop`. Besides
  `page.html`, `page.css` and `schema.json`, a template may check relations between items with `checks.yaml`
  (on the items of a list, lists on the way gone through: `sections.parts`)
  and compute values for the page with `computed.yaml`, in
  [expr](https://expr-lang.org/), each failure reported at the data file's
  line and column, and a mistake in those files at their own;
  `page.html` writes numbers with `num` and `bytes`.
  floe puts a page's CSS together, the template's `page.css` in a
  `<style>` of its own, in a cascade layer over floe's base (the page's ground, colour and
  face, text, lists, links, code and quotes) and the looks of elements that take one without being a part's
  box, and under the parts' style and the theme's, which they do not
  restyle whatever their selectors. The template's CSS is scoped to the page,
  `:scope` its root: it reaches a part's element but not what the part holds.
  `page.html` holds a `</head>`, before which floe puts the page's styles, or
  is refused, as is a drawing an action of it leaves it out of. It holds
  no `<style>` or `<base>`, nor an item of `"format": "html"` a `<style>`, `<link>`,
  `</head>`, `<base>` or `<meta>`, or what changes the page around it as a browser reads it
  (an `<svg>`, a comment or a `<form>` it leaves open, an end tag of the page's; a `page.html` that puts one outside its `<head>` and `<body>` or leaves out `<!doctype html>`), nor the template's CSS an `@import`, `:root`, `html` or a `}` that closes no block, each refused
  at its line, and `floe template check` refuses a part `page.html` names that floe
  does not have.
  Images go into the page as data URIs: an item of `"format": "image"`
  names an image under the data file's directory, and `page.html` shows one
  of the template's `assets/` with `asset`. An image is a png, jpeg, gif,
  webp or svg file up to 1 MiB, checked by its name and by what it holds,
  and a mistake in one of the data is reported at its line and column, but
  for one `floe publish` will not send the server (through a symlink, over
  1 MiB, not a file, unreadable or not holding its type).
  A schema's mistakes, a format of floe's or a part's `$ref` where floe does
  not follow to it (under `anyOf`, `$defs` and the like), a format floe does
  not know, a `$ref` out of floe or into a part, or a keyword beside a `$ref`
  to a part but annotations (`description`, `title`, `default`…), are refused at their
  JSON pointers, by `floe template check` and wherever the template is used,
  and a part's by `floe part check`; a `$ref` into the schema's own file
  (`#/$defs/row`) is followed by JSON Schema alone.
  `template.yaml` names the floe a template needs. A schema may not declare
  or require `lang`, `template`, `theme` or `title`, which floe reads from
  every data file itself. See [docs/templates.md](docs/templates.md).
- Add `floe template list`, which lists the templates with where they are
  and the floe they need; `floe template show <name>`, which
  prints the items a data file of a template may hold and its example, to
  read before writing one; `floe template new <name> --from <template>`,
  which copies a template into a template of your own to edit; and
  `floe template check <dir>...`, which checks a template before you use or
  share it: its files (a `format` floe does not know, and paths of
  `checks.yaml` that `schema.json` does not declare, among them), then its
  `example.yaml` through every stage of a publish.
- Add `floe check <data.yaml>...`, which checks data files as `floe publish`
  does and prints every mistake as `file:line:column`, without publishing them
  or starting the server, for the CI of a repository that keeps data files.
- Add `floe list`, `floe rm`, `floe serve` and `floe stop`, as well as the
  management page with live reload. `floe list` and `floe rm` ask the server,
  starting it when needed, as `floe publish` does. The server keeps running
  until `floe stop`.
  They are configured with `FLOE_DIR` and `FLOE_PORT` (default 7325). A
  relative `FLOE_DIR`, `FLOE_ICONS_DIR` or `FLOE_TEMPLATES_DIR` is under the
  working directory, and a `~` or `~/` at its start stands for `HOME`
  (`~name` is refused). The defaults are under `XDG_DATA_HOME` and
  `XDG_CONFIG_HOME`, whose `~` stands for `HOME` too; a relative one of
  those is ignored, as the XDG spec says.
  The management page may be framed by itself and by VS Code's Simple
  Browser alone (`frame-ancestors 'self' vscode-webview: vscode-file:`), so
  that another site, or a page, cannot have its buttons clicked unseen.
- Show what a browser shows, the management page and the page of a data
  file's mistakes, in English or Japanese by the browser's language. The
  command line speaks English alone, whatever the environment's language.
- Add the `floe` command with `floe version` (also `floe --version`), which
  prints the installed version.
- Add prebuilt binaries for linux and macOS (amd64 / arm64) on GitHub Releases
  and `install.sh`, which installs the latest release (or `FLOE_VERSION`) into
  `FLOE_INSTALL_DIR` (default `~/.local/bin`) after verifying its SHA-256.
