# Commands

Reference for every command of the `floe` binary.

## Quick reference

| Command | Purpose |
|---|---|
| `floe publish <data.yaml>` | Store a page drawn from a data file (or `--link` an HTML file in place) and print its URL |
| `floe check <data.yaml>...` | Check data files as `floe publish` would, without publishing them |
| `floe render <data.yaml>...` | Write the page of each data file to an HTML file, to keep in a repository |
| `floe template list` | List the templates |
| `floe template show <name>` | Show the items a data file of a template may hold, and its example |
| `floe template new <name> --from <template>` | Make a template of your own as a copy of another, to edit |
| `floe template check <dir>...` | Check templates before you use or share them |
| `floe part list` | List the parts |
| `floe part show <name>` | Show what a part takes, the looks it takes, the parts it holds and its parts themes |
| `floe part new <name> --from <part>` | Make a part of your own as a copy of another, to edit |
| `floe part check` | Check the parts and parts themes of your own before you use or share them |
| `floe list` | List pages, newest first |
| `floe rm <slug>...` | Delete pages |
| `floe serve` | Run the server in the foreground |
| `floe stop` | Stop the background server |
| `floe diagram render <spec.yaml>...` | Check diagram specs and print them as inline SVG |
| `floe diagram example [kind]` | List the example specs, or print one |
| `floe icons fetch <set> [zip-or-url]` | Keep an icon set (`aws`) for the `icon` parts of diagrams |
| `floe icons search <word>...` | List the fetched icons whose name contains every word |
| `floe version` | Print the version (also `floe --version`) |
| `floe help [command]` | Print help (Cobra builtin) |

---

## `floe publish <data.yaml>` / `floe publish --link <file.html>`

Store a data file and print the URL of its page (`http://127.0.0.1:7325/a/<slug>/`) on stdout. The page is drawn each time it is opened. If the server is not running, it is started in the background first. A new slug is also opened in the browser; a republished one is not, because the open tab reloads itself.

Pages are drawn from data files (`.yaml`, `.yml`). The data file is copied to `<slug>/source-<id>/data.yaml` under `FLOE_DIR`, with the images it shows at the paths it names them by; take it back from there if the original is lost. Each publish writes a new copy and then removes the one before, so a publish that fails leaves the page as it was. An HTML file is not stored: link it with `--link` to show it as it is. A relative path is resolved against the working directory.

### Flags

| Flag | Description |
|---|---|
| `--link` | Register an HTML file in place and show it as it is (see [Linked files](#linked-files)). |
| `--slug <name>` | The URL name. Lowercase letters, digits and hyphens, up to 64 characters. Defaults to the file name; a linked `index.html` takes the name of its directory. |
| `--title <text>` | The title shown in the list. Defaults to the data file's `title:`, or a linked file's `<title>`, else its first `<h1>`, else the slug. An empty value is a usage error. |
| `--description <text>` | A description shown in the list. |
| `--theme <name>` | The template's theme to draw the page with, over the data file's `theme:`, which is then not used. Defaults to `theme:`, else `default`; a republish without it goes back to `theme:`. An empty value is a usage error. Refused with `--link` of an HTML file. |
| `--parts-theme <name>` | The theme to draw floe's parts (and Markdown's quotes) with, over the data file's `parts-theme:`, which is then not used. Defaults to `parts-theme:`, else `default`; a republish without it goes back to `parts-theme:`. An empty value is a usage error. Refused with `--link` of an HTML file. |
| `--force` | Replace a page published from another project, or a link to another file. |

### Data files

A data file names a template, and the template draws the page from its data. The built-in template is `report`: a header over sections, each a heading over parts: Markdown text, `code` and `terminal` parts for code and commands, `table`, `cards` and `note` parts, parts for numbers, steps, what changes, options, diffs, file trees, review findings, decisions, risks and more, `tabs` and `details` that hold parts (all told in [Parts](templates.md#parts)), `diagram` parts for diagrams, `script` parts for what moves, and `html` parts for what they cannot say.

```yaml
template: report               # the template that draws the page
lang: en                       # the page's language
title: Moving the team wiki
eyebrow: Plan · 2030-04        # a short line above the title (optional)
lead: The wiki moves in two weekends.   # a sentence under the title (optional)
sections:
  - heading: Steps
    parts:
      - text: |                # Markdown, tables included; raw HTML in it is left out
          1. Freeze edits on **Friday evening**.
          2. Copy the pages.
      - terminal:              # commands and what they printed; floe draws the prompt (shell: bash, zsh, sh, powershell, cmd)
          steps:
            - { run: wiki export --out pages/, out: 1840 pages written }
      - code:                  # code as it is, its file and language above it
          file: old-wiki.conf
          text: |
            read_only = true
      - diagram:               # a diagram spec, as floe diagram render takes it
          id: move
          width: 640
          height: 120
          title: The pages go from the old wiki to the new host
          parts:
            - box: { x: 16, y: 32, w: 184, h: 56, rows: [Old wiki] }
            - box: { x: 440, y: 32, w: 184, h: 56, tone: good, rows: [New host] }
            - edge: { d: "M200,60 H440" }
      - table:                 # each cell is a line of Markdown, a number in quotes, or a pill
          columns:
            - title: Risk
            - { title: Pages, num: true }   # numbers: aligned right
            - title: State
          rows:
            - [Broken links, '120', { pill: watching, tone: warn }]   # tone: accent, good, warn, bad, or none
      - cards:                 # cards side by side, each a title over Markdown text
          - { title: Copy, text: Copies the pages on **Saturday**. }
      - note: { tone: warn, text: Edits during the freeze are lost. }   # tone: accent, good, warn, bad, or none
      - script:                # a function the page calls with an element to draw in and data
          data: { pages: 1840 }
          js: |
            function (el, data) { el.textContent = `${Math.ceil(data.pages / 300)} hours`; }
      - html: <details><summary>What the freeze covers</summary><p>Edits, comments and uploads.</p></details>   # HTML as it is
```

- `lang:` is the page's language (`<html lang>`), a BCP 47 tag (`en`, `ja`, `pt-BR`). Every data file names one.
- `theme:` picks a theme of the template, which styles the page; `report` has `default`.
- `parts-theme:` picks a theme of floe's parts, which styles the parts and Markdown's quotes, the same in every template: `default` (faces set in panels, notes in their tone's colour), `soft` (faces with round corners), `editorial` (rules and thin frames, as in print), `ink` (a table's heading in ink, states filled with their colour), `brutal` (thick black frames, primary faces and hard shadows), `blueprint` (blue ink on squared paper, every cell ruled), `notebook` (ruled paper, marker strokes and sticky notes), `swiss` (heavy black rules, large numbers and red, states told by shape) and `washi` (sumi and vermilion on washi in a Mincho face, seals and quotes ruled in vermilion).
- An image the template takes (an item of `"format": "image"`) is a path under the data file's directory; the image goes into the page. `floe publish` sends the server the images it asks for: one that is not there is told at its line and column, while one floe will not send (through a symlink, larger than 1 MiB, not a file, unreadable or not holding its type) is told without them.
- A diagram (the part `diagram`, in `report` or a template's own) is a spec written in place, with the parts and checks of [`floe diagram render`](#floe-diagram-render-specyaml), its mistakes at the data file's lines and columns. The ids of the diagrams of a page must differ. Its icons come from `FLOE_ICONS_DIR`; for a published page, from that of the server, which draws it.
- A script (the part `script`, in `report` or a template's own) is `js`, a function the page calls with an empty element in its place and `data`. A mapping in `data` comes in the order of its keys' names (write a list where the order matters), and the function runs where it stands, before the rest of the page is read. A `js` holding `</script` or `<!--` is refused; a mistake inside the function shows in the browser.
- A template of your own, in `FLOE_TEMPLATES_DIR`, takes the place of a built-in one of its name. [templates.md](templates.md) tells how to write one.
- The data is checked against the template's schema and its checks (`checks.yaml`) before the page is drawn. Every mistake is printed at once as `file:line:column: message`, and nothing is published until all are fixed (exit code 1): a missing or unknown template, a missing `lang:` or one that is not a BCP 47 tag, an item the template does not have (a typo), a value of the wrong type, a row of a `table` with more or fewer cells than its columns, and the like. `report` refuses a fenced block of code (a line starting with ```` ``` ```` or `~~~`, indented or in a list or a quote too) in `text`: write code in `code` and commands in `terminal`. A `terminal`'s `run` whose first word is a prompt, ending in `$`, `#`, `%`, `>` or `❯` (`$ `, `~/app$ `, `root@host:~# `, `C:\work> `, `❯ `; after `(venv) `, and `[me@host app]$ ` and `PS C:\work> ` too), is refused.
- A data file is one YAML document of keys and values: one with `---` between documents, or with anchors, aliases or tags, is refused.
- The page is drawn each time it is opened, from the copy of the data file kept when it was published: changing the data file changes nothing until it is published again, but changing its template does. A data file linked with `--link` is drawn from its place instead.

### Projects and overwriting

Each page belongs to the project it is published from. The project is the repository name in git's `origin` URL, else the repository's directory, else the working directory's name. A link belongs to its file's directory instead.

- The same slug from the same project overwrites the page. stderr says so (`updated "plan" (last published …, title "…")`). If the title changed, a warning suggests another `--slug`. The exit code stays 0.
- A slug published from another project is refused unless `--force`. `--force` starts the page afresh: nothing of the old one is kept.
- A republish keeps what was set on the management page (title, project, favourite), the description and the creation time. `--title` and `--description` override them.

### Linked files

`--link` serves an HTML file where it is, as it is: floe adds nothing around it, so a page of your own or one another tool made shows as written. It is read on every request, so saving the file updates the open tab. The files under its directory are served too, for its relative references (CSS, scripts, images).

- Only a regular `.html` or `.htm` file, or a data file (`.yaml`, `.yml`), can be linked.
- A file in a hidden directory right under your home (settings and keys live there) is refused.
- A file directly under `/` or under your home itself is refused, because the whole directory would be served.
- A link without `--slug` reuses the slug of the same file, else takes the first free one of `name`, `name-2`, `name-3` …
- The same slug for another file is refused unless `--force`.

A linked data file is drawn each time its page is opened, with `--theme` and `--parts-theme` as given when it was linked, so that a page written in a repository can be read while it is written:

- It is checked when it is linked, as `floe publish` checks it, and refused with its mistakes.
- Saving the data file, an image it shows or a file of its template, or fetching the icons its diagrams show, reloads the open tab.
- When it has mistakes, the tab shows them instead of the page, and comes back to the page once they are fixed and saved.
- Nothing of its directory is served: its images are in its page. Its download is the page drawn, its CSP kept in a `<meta>`.
- Its title in the list is its `title:`, read each time.
- It is drawn with the server's templates of your own (`FLOE_TEMPLATES_DIR` of the `floe publish`, `floe list`, `floe rm` or `floe serve` that started it), as every page is.

---

## `floe check <data.yaml>...`

Check each data file as `floe publish` does before it stores the page: its YAML, its template and themes, the template's schema, checks and computed values, and drawing the page. A relative path is resolved against the working directory.

- Every mistake of every file is printed at once as `file:line:column: message`, in the order the files were given (exit code 1).
- Nothing is printed when every file is right (exit code 0).
- Nothing is published, and the server is not started, so a repository's CI can run it over its data files.
- A file that is not a data file (`.yaml`, `.yml`) is refused.

---

## `floe render <data.yaml>...`

Draw each data file as `floe publish` does and write its page to `<name>.html`, beside the data file, so that a repository keeps the data file for agents to read and edit, and its page for people to read. The page is the one the management page downloads: it opens without floe, what it shows is in it (images, the diagrams' style and zoom), and its CSP, which keeps its scripts from sending what it holds out, is in a `<meta>`.

```sh
floe render docs/spec.yaml                 # writes docs/spec.html
floe render docs/*.yaml --out-dir site     # writes site/<name>.html
floe render --check docs/*.yaml            # in CI: fails when a page was not written again
```

| Flag | Description |
|---|---|
| `--out-dir <dir>` | Write the pages in this directory, made when missing, instead of beside each data file. |
| `--check` | Write nothing: print each page that is missing or is not what its data file draws (exit code 1), or nothing when every page is (exit code 0). |

- A page is written only when it changes, and its path is printed; a page that would not change is left as it is, so running it again does nothing.
- A page there is written over: keep the pages in version control to get one back. A page that is a symlink is written where it leads, and stays a symlink.
- Every mistake of every file is printed at once as `file:line:column: message`, as `floe check` prints them, and nothing is written until all are fixed (exit code 1). Nothing is written either when two data files would write the same page (`a/spec.yaml` and `b/spec.yaml` with one `--out-dir`, `spec.yaml` and `spec.yml`, or the same through a symlinked directory).
- The page's language, themes and title are the data file's `lang:`, `theme:`, `parts-theme:` and `title:` (else the slug `floe publish` would give it, else, for a name of no letter a slug takes, the name), so that writing a page and checking it take the same arguments.
- A data file named twice (by two globs, through a symlinked directory, or in another case on macOS) is written once; a symlink of another name is a page of its own.
- Nothing is published, and the server is not started.
- A page depends on the floe, the templates and the icons that draw it: in CI, use the floe version and the templates the pages were written with, and write the pages again when you upgrade.
- A `script` of the page runs without the sandbox of the server's pages when the file is opened; its CSP still cuts off `fetch`, XHR, images from elsewhere and forms. A link between pages (`/a/<slug>/`) does not work in a file: link the files relatively.

---

## `floe template list`

Print the templates, one per line in five tab-separated columns: the name, where it is (the directory of a template of your own, or `built-in` in every language, for scripts), its themes (comma-separated), the floe it needs (from its `template.yaml`; empty without one) and the description of its `schema.json`. A template of your own takes the place of the built-in one of its name. [templates.md](templates.md) tells how templates are written.

## `floe template show <name>`

Print what a data file of the template may hold, to read before writing one: where the template is, its themes, the parts themes, the floe it needs, its items (the keys floe reads from every data file, `lang` and `template` required, `parts-theme`, `theme` and `title`, and those of `schema.json`; each with what it takes and its description, the items under it indented) and its `example.yaml`.

## `floe template new <name> --from <template>`

Make the template `<name>` of your own in `$FLOE_TEMPLATES_DIR/<name>` as a copy of `<template>`, built-in or your own, to edit.

- Its `template.yaml` names the floe it needs: that of `<template>`, or this floe's for a copy of a built-in one.
- Its `example.yaml` names `<name>` as its `template:`.
- A name is lower case letters, digits, `-` and `_`. A copy named like a built-in template takes its place.
- A template of your own of that name is left as it is: the command stops (exit code 1).

## `floe template check <dir>...`

Check each directory as the template of its name, before you use or share it. A relative path is resolved against the working directory.

- Its files first: `template.yaml`, `schema.json` (a `format` floe does not know is a mistake, as is an item, declared or required, named as a key floe reads from every data file: `lang`, `parts-theme`, `template`, `theme`, `title`), `checks.yaml` (the paths of `for` and `field` must be declared in `schema.json`), `computed.yaml`, `page.html`, `themes/default.css` and the images of `assets/`.
- Then its `example.yaml` through every stage of `floe publish`, drawing the page included. It must name the template of the directory's name: the template drawn is always the one in the directory, never another of its name.
- Every mistake is printed at once, as `file:line:column: message` where it has a place (exit code 1). Nothing is printed when every template is right (exit code 0).
- Warnings go to stderr and leave the exit code 0: no `template.yaml`, or an object of `schema.json` that does not refuse a misspelt key (no `"additionalProperties": false`).

[templates.md](templates.md) tells what each file holds.

## `floe part list`

Print the parts, floe's and those of your own in `$FLOE_PARTS_DIR`, one per line in three tab-separated columns: the name, where it is (the directory of a part of your own, or `built-in` in every language, for scripts) and its description (from its `schema.json`, or a look's `part.yaml`). The parts floe draws in Go (`diagram`, `script`) and the looks alone (`panel`, `rows`) are among them. A part of your own takes the place of floe's of its name. One that cannot be used is left out, and its mistake printed after the others (exit code 1).

## `floe part show <name>`

Print what a part takes, to read before writing it in a schema or making a copy of it: where it is (and whether it takes the place of floe's), what it takes (keys and values, a list of them, or text), the looks it takes (from its `part.yaml`, and those they take in turn, in the order they go into a page), the parts its schema holds, the parts themes with a file of it, and its items, each with what it takes and its description, the items under it indented. A look alone takes nothing, and a part floe draws in Go has no items to tell: floe checks its shape where it draws it.

## `floe part new <name> --from <part>`

Make the part `<name>` of your own in `$FLOE_PARTS_DIR/<name>` as a copy of `<part>`, floe's or your own, to edit.

- Every file of `<part>` is copied, its `part.yaml` included, so the copy takes the looks it takes. Its `part.html` names `<name>` where it named `<part>` as its box, `data-floe-part="<part>"`; the parts it holds and the looks it takes keep their names.
- The file of `<part>` of each parts theme that has one, floe's or your own, is copied into `$FLOE_PARTS_DIR/themes/<theme>/<name>.css`, so the copy starts with the original's look in every theme. What you copy is yours: floe's later changes to its part or its themes do not reach it.
- A name is lower case letters, digits, `-` and `_`, and not `diagram`, `script`, `tone`, `base` or `themes`. A copy named like one of floe's parts takes its place. A part floe draws in Go has no files to copy.
- Nothing is written over: with a part of that name, or a file of a parts theme for it in any theme of yours, the command stops and makes nothing (exit code 1). It stops too at a parts theme of yours that cannot be used and may have a file of `<part>`, which the copy would lack: fix it first.
- The copy is read as a page would read it in its place, before it is put there. When the copy, or a part that takes its look or holds it, could not be used (a look alone in place of one of floe's parts that draws, a copy that would hold itself, as `cell` copied from `table`), the command makes nothing and tells why. It stops too when your parts themes, or one of them, cannot be read at all.

## `floe part check`

Check every part and parts theme of your own in `$FLOE_PARTS_DIR`, as a page would read them, before you use or share them. To check another directory, set `FLOE_PARTS_DIR` to it: a part holds others and takes their looks by name, so a part is checked with the rest of its directory.

- Every mistake is printed at once, at its line where it has one (exit code 1): of its files, its names, the looks it takes, the parts it holds, a part that holds itself or takes its own look, and the CSS of a parts theme. Nothing is printed when they are right, or when there are none (exit code 0).
- Warnings go to stderr and leave the exit code 0: a file that goes into no page, such as a file of a parts theme named for no part.
- A part is not drawn, as it has no data: `floe template check` draws the example of a template that uses it.

[templates.md](templates.md) tells what a part holds.

---

## `floe list`

Print the pages, newest first, as the server lists them (it is started when it is not running). Each line holds four tab-separated columns: the update time (local, `YYYY-MM-DD HH:MM`), the slug, the title and the linked path (empty for a stored page).

## `floe rm <slug>...`

Delete the pages in order through the server (it is started when it is not running), stopping at the first failure (such as a page that does not exist). A linked file itself is left alone; only its registration is removed.

## `floe serve`

Run the server in the foreground on `127.0.0.1` until SIGTERM or SIGINT. `floe publish`, `floe list` and `floe rm` start the same server in the background, so this is mainly for seeing its errors.

## `floe stop`

Stop the background server. Without one, it prints `server is not running` and exits 0.

---

## `floe diagram render <spec.yaml>...`

Check each diagram spec and print the diagrams, in order, as one HTML fragment of inline SVG on stdout, to paste into an HTML page of your own: a whole document with `<!doctype html>` and `<html lang>`, since `floe publish --link` shows the file as it is. A relative path is resolved against the working directory.

- A mistake in the shape of a spec (an unknown key, a wrong type, a value that is not allowed) stops that spec at the first one.
- Layout mistakes are all reported: text too wide or too tall for its box, parts sticking out of the canvas, overlapping, or straddling the edge of a frame, edges running through boxes, labels or frame headings, and edges crossing like an X.
- Every mistake of every spec is printed as `file:line:column: message`, and nothing is drawn until all are fixed (exit code 1).
- The ids of the diagrams drawn together must differ, because they name the diagrams' markers and icons. Draw all the diagrams of a page with one `render`.
- The fragment starts with the assets: the diagrams' style, which takes its colours from the CSS variables the `report` theme defines (so dark mode follows; a page without them gets that theme's light colours), and the script that zooms them. Each diagram sits in a frame that first shows it whole; readers zoom with the buttons, Ctrl / ⌘ + wheel or a pinch, and drag or use the arrow keys to move while zoomed. The toolbar is worded in the page's language (`<html lang>`). A page needs the assets once; holding them more than once does no harm.

| Flag | Description |
|---|---|
| `--no-assets` | Print the diagrams without the assets, for a page that already holds them. |

### Specs

A spec is a YAML file holding one diagram: a canvas and the parts placed on it at absolute coordinates (the top left is 0, 0).

```yaml
id: api                        # lowercase letters, digits and hyphens
width: 640
height: 200
title: How the API is laid out # read aloud by screen readers
parts:
  - frame: { x: 16, y: 16, w: 608, h: 168, label: VPC }
  - box: { x: 40, y: 72, w: 200, h: 56, tone: accent, rows: [[ALB, main code], public entry] }
  - box: { x: 400, y: 72, w: 200, h: 56, rows: [API server, ECS] }
  - edge: { d: "M240,100 H400", label: HTTPS, at: [320, 100] }
```

Each part is one key naming its kind. Defaults are in parentheses; `tone` is `accent`, `good`, `warn` or `bad`.

| Part | Keys |
|---|---|
| `frame` | `x` `y` `w` `h`, `label`, `accent`, `square` (no rounded corners), `icon` (an icon name such as `aws/group/region`: the frame is drawn in the AWS group style, with square corners and the 32px icon in its corner, which other parts and edges may not cover or run through). Its label sits inside the top left corner, so place its contents 32 or more below its top (40 with an icon). |
| `box` | `x` `y` `w` `h`, `rows`, `tone`, `shape` (`rect`; `pill`, `hex` for a decision, `state`). A row is text, or `[text, style]` with the style `main`, `sub`, `main code` or `sub code`; without a style the first row is `main` and the rest `sub`. |
| `table` | `x` `y` `w`, `title`, `rows`, `tone`. A row is a name, `[name]`, `[name, type]`, or `---` for a separator. The height is 30, plus 22 a row and 9 a separator, plus 6 when it has rows; row i (from 0, without separators) is centred at `y + 41 + 22i`. |
| `icon` | `x` `y` (the top left), `name`, `label` (below the icon), `size` (48). `name` is the icon's set and its name in the set, like `aws/amazon-ec2` (see `floe icons search`); an icon with a variant for dark backgrounds switches with the colour scheme. Edges stop at its sides. An unknown name is a mistake that suggests close names. |
| `edge` | `d`, `label` with `at: [x, y]`, `dashed`, `bold`, `tone`, `start` (`none`), `end` (`arrow`), `crossing`. `d` takes the absolute `M`, `H`, `V` and `L` only. The ends are `arrow`, `triangle`, `diamond`, `odiamond`, `one`, `many`, `zero-one`, `zero-many` or `none`. `crossing: true` lets this edge cross others. |
| `dot` | `cx` `cy`, `r` (7), `tone`, `hollow`. Edges may reach its centre. |
| `bar` | `x1` `x2` `y`, `tone`, `label` (right of the bar). |
| `axis` | `x1` `x2` `y`, `ticks: [[x, text], ...]`, `grid` (the top of the grid lines). |
| `circle` | `cx` `cy` `r`, `tone` (`accent`). Circles may overlap. |
| `label` | `text`, `x` `y`, `anchor` (`middle`; `start`, `end`), `plain` (no background), `strong`. |
| `line` | `x1` `y1` `x2` `y2`, `kind` (`line`; `lifeline`, `divider`). |
| `raw` | `svg`: SVG that no part can draw, written out as it is and not checked. |

## `floe diagram example [kind]`

Without a kind, list the built-in examples, one per kind of diagram (architecture, AWS architecture, class, ER, flow, Git branches, sequence, state, timeline, Venn): each line holds the kind and a summary, tab-separated. With a kind, print its spec; its comments explain the grid it was laid out on. Every example passes the checks. The AWS example draws AWS icons, so render it after `floe icons fetch aws`.

The examples come in English and Japanese, text and comments alike. They follow the language of messages (`FLOE_LANG`, then `LC_ALL`, `LC_MESSAGES` and `LANG`), so copying an example in a Japanese environment starts a diagram in Japanese. An example not written in the language is printed in the language without its region (`ja-JP` to `ja`), else in English.

| Flag | Description |
|---|---|
| `--lang <tag>` | The language of the examples, as a BCP 47 tag (`en`, `ja`). A region falls back to its language (`ja-JP` to `ja`), and a language without examples to English. |

## `floe icons fetch <set> [zip-or-url]`

Keep an icon set in `$FLOE_ICONS_DIR/<set>` for the `icon` parts of diagrams. floe does not ship icons; each machine fetches a set once. The sets:

| Set | Icons |
|---|---|
| `aws` | The [AWS Architecture Icons](https://aws.amazon.com/architecture/icons/): the 48px service and resource icons and the 32px group icons, with their variants for dark backgrounds. The Icon package zip is about 14MB. |

- Without a zip, the set's package is found on its official page and downloaded. A zip downloaded in a browser, or its URL, can be given instead; a relative path is resolved against the working directory.
- A version already there is kept. `--force` fetches it again.
- The icons are replaced as a whole: if that fails, the previous ones stay.
- A download gives up when no response comes in 30 seconds or no data for 60 seconds; a slow one that keeps going is let finish. A package over 256MB, or an icon in it over 1MB, is refused.

| Flag | Description |
|---|---|
| `--force` | Fetch the icons even if that version is already there. |

## `floe icons search <word>...`

List the fetched icons whose name contains every word (case does not matter), one per line: the name to give an `icon` part (`aws/…`) and its category, tab-separated.

---

## The server

- `floe publish`, `floe list` and `floe rm` start the server when it is not running, and keep using it otherwise. Two of them racing to start it both end up on the one that comes up.
- They ask the server to check, save, list and delete pages: the server is the only one that writes the store, and a publish is checked with the server's templates, parts and icons.
- After floe is reinstalled (another version, or a rebuilt binary), the next of them replaces the running server.
- The server keeps running until `floe stop`, logout or reboot.
- It listens on `127.0.0.1` only. It refuses other host names (against DNS rebinding) and writes from other sites.
- Pages run in a sandbox, so their scripts cannot reach the management page's API.
- The scripts of a page drawn from a data file cannot send what it holds out with `fetch`, XHR, images from elsewhere or forms, its download included; see [templates](templates.md#pagehtml). Linked HTML loads what it loads.

## Environment variables

| Variable | Description |
|---|---|
| `FLOE_DIR` | Where pages are stored. Defaults to `$XDG_DATA_HOME/floe`, else `~/.local/share/floe`. A relative one is under the working directory; a publish that means another than the running server's is refused (run `floe stop` first). |
| `FLOE_PORT` | The port (default `7325`). A decimal number from 1 to 65535. |
| `FLOE_ICONS_DIR` | Where `floe icons fetch` keeps icon sets, a directory each (`aws`). Defaults to `$XDG_DATA_HOME/floe-icons`, else `~/.local/share/floe-icons`. A relative one is under the working directory. The diagrams of a published page show the icons of the server, which takes the one of the `floe publish` or `floe serve` that started it; a publish that means another is refused (run `floe stop` first). |
| `FLOE_TEMPLATES_DIR` | Where templates of your own are, a directory each ([templates.md](templates.md)). Defaults to `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`. A relative one is under the working directory. A published page is drawn with the server's, the one of the `floe publish` or `floe serve` that started it; a publish that means another is refused (move the templates there, or run `floe stop` first). |
| `FLOE_PARTS_DIR` | Where parts and parts themes of your own are, a directory each ([templates.md](templates.md#parts-of-your-own)). Defaults to `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`. A relative one is under the working directory. A published page is drawn with the server's, as with `FLOE_TEMPLATES_DIR`; a publish that means another is refused. |
| `FLOE_LANG` | The language of messages (`en`, `ja`). Falls back to `LC_ALL`, `LC_MESSAGES`, then `LANG`; a language without a catalog is skipped. English is the default. |
| `BROWSER` | The command that opens a new page. Defaults to `open` on macOS, else `wslview` or `xdg-open`. |

In `FLOE_DIR`, `FLOE_ICONS_DIR`, `FLOE_TEMPLATES_DIR`, `FLOE_PARTS_DIR`, `XDG_DATA_HOME` and `XDG_CONFIG_HOME`, a `~` or `~/` at the start stands for `HOME`, so that one written in quotes (a `.env` file, a service unit) is not a directory named `~`; a `FLOE_` one starting with `~name`, or with `~` without `HOME`, is refused. A relative `XDG_DATA_HOME` or `XDG_CONFIG_HOME` is ignored, as the XDG spec says, and so is one starting with `~name`, or with `~` without `HOME`.

The management page follows the browser's language instead. Cobra's own errors (an unknown flag or command) and `--help` are in English.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Failure (a refused publish, a missing page, a port in use, a data file or diagram spec with mistakes …) |
| `2` | Usage error (an unknown flag or command, missing arguments, an empty `--title`, `--theme` or `--parts-theme`, an unknown example, a `--lang` that is not a BCP 47 tag) |
