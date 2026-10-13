# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

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
  line and column (`file:line:column`). A page is in English
  unless `--lang` names another language. It starts the server in the
  background when needed and opens a new page in the browser. An HTML file is
  not stored: `--link` registers it in place and shows it as it is, served
  with the files under its directory.
  `--theme` picks another theme of the template, and `--slug`, `--title`,
  `--description` and `--force` are also available.
  Republishing a slug says so on stderr, with a warning when the title
  changed.
- Use templates of your own: a directory in `FLOE_TEMPLATES_DIR` (default
  `~/.config/floe/templates`), a symlink included, takes the place of a
  built-in template of its name. Besides `page.html`, `schema.json` and
  themes, a template may check relations between items with `checks.yaml`
  and compute values for the page with `computed.yaml`, in
  [expr](https://expr-lang.org/), each failure reported at the data file's
  line and column, and a mistake in those files at their own;
  `page.html` writes numbers with `num` and `bytes`.
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
