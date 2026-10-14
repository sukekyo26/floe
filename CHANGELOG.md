# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Run scripts in data files: the built-in `report` takes a `script` part, and
  a template takes an item of `"format": "script"`, each `js`, a function the
  page calls with an empty element in its place and `data`, any value. A `js`
  holding `</script` or `<!--` is refused, pointing at the `js`.
- Write tables, cards and notes in data files: the built-in `report` takes
  `table` (`columns`, a `num` one aligned right, and `rows`, each cell a line of
  Markdown, a number in quotes, or a pill, `{ pill, tone }`, as many in each
  row as there are columns),
  `cards` (each a `title` over Markdown `text`) and `note` (Markdown `text`,
  with a `tone` of `warn` or `bad`) parts. A template's `page.html` takes
  `markdownLine`, which draws a line of Markdown without a paragraph around
  it, keeping a cell like `-` or `1.` as text.
- Draw diagrams in data files: the built-in `report` takes a `diagram` part,
  and a template takes an item of `"format": "diagram"`, each a spec as
  `floe diagram render` takes it, written in place. A diagram is checked as
  `floe diagram render` checks one, its ids across the page, and every
  mistake is reported at the data file's line and column. The page gets the
  diagrams' style and zoom once. `floe template check` warns of a theme that
  leaves out the CSS variables diagrams take their colours from.
- Add `floe diagram render <spec.yaml>...`, which draws diagrams from YAML
  specs of parts placed at absolute coordinates and prints them as one HTML
  fragment of inline SVG. It first checks for text that does not fit, parts
  outside the canvas, overlaps, edges through boxes or labels, and crossing
  edges, and reports every mistake as `file:line:column`. The fragment
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
  prints one to copy. The examples come in English and Japanese and follow the
  language of messages; `--lang` picks another.
- Add `floe publish <data.yaml>`, which draws a page from a data file with the
  template it names, stores it and prints its URL on `http://127.0.0.1:7325`.
  The built-in template `report` lays out a header and sections of Markdown
  text, with HTML parts for what Markdown cannot say. The data is checked
  against the template's schema first, and every mistake is reported at its
  line and column (`file:line:column`). The scripts of a page drawn from a
  data file cannot send what it holds out: its CSP blocks `fetch`, XHR,
  images from elsewhere and forms, in its download too. A page is in English
  unless `--lang` names another language. It starts the server in the
  background when needed and opens a new page in the browser. An HTML file is
  not stored: `--link` registers it in place and shows it as it is, served
  with the files under its directory. A data file linked with `--link` is
  drawn each time its page is opened instead of stored: saving it, an image
  it shows or a file of its template, or fetching the icons its diagrams
  show, reloads the open tab, and while it has mistakes the tab shows them in
  place of the page.
  `--theme` picks another theme of the template, and `--slug`, `--title`,
  `--description` and `--force` are also available.
  Republishing a slug says so on stderr, with a warning when the title
  changed.
- Use templates of your own: a directory in `FLOE_TEMPLATES_DIR` (default
  `~/.config/floe/templates`), a symlink included, takes the place of a
  built-in template of its name. Besides `page.html`, `schema.json` and
  themes, a template may check relations between items with `checks.yaml`
  (on the items of a list, lists on the way gone through: `sections.parts`)
  and compute values for the page with `computed.yaml`, in
  [expr](https://expr-lang.org/), each failure reported at the data file's
  line and column, and a mistake in those files at their own;
  `page.html` writes numbers with `num` and `bytes`.
  Images go into the page as data URIs: an item of `"format": "image"`
  names an image under the data file's directory, and `page.html` shows one
  of the template's `assets/` with `asset`. An image is a png, jpeg, gif,
  webp or svg file up to 1 MiB, checked by its name and by what it holds,
  and a mistake in one of the data is reported at its line and column.
  `template.yaml` names the floe a template needs. See
  [docs/templates.md](docs/templates.md).
- Add `floe template list`, which lists the templates with where they are,
  their themes and the floe they need; `floe template show <name>`, which
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
  management page with live reload, which reads and writes the same store
  format as the artifact skill. The server keeps running until `floe stop`.
  They are configured with `FLOE_DIR` and `FLOE_PORT` (default 7325).
- Show messages in English or Japanese. The command line follows `FLOE_LANG`,
  then `LC_ALL`, `LC_MESSAGES` and `LANG`; the management page follows the
  browser's language. English is the default.
- Add the `floe` command with `floe version` (also `floe --version`), which
  prints the installed version.
- Add prebuilt binaries for linux and macOS (amd64 / arm64) on GitHub Releases
  and `install.sh`, which installs the latest release (or `FLOE_VERSION`) into
  `FLOE_INSTALL_DIR` (default `~/.local/bin`) after verifying its SHA-256.
