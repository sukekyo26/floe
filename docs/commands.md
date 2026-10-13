# Commands

Reference for every command of the `floe` binary.

## Quick reference

| Command | Purpose |
|---|---|
| `floe publish <data.yaml>` | Store a page drawn from a data file (or `--link` an HTML file in place) and print its URL |
| `floe check <data.yaml>...` | Check data files as `floe publish` would, without publishing them |
| `floe template list` | List the templates |
| `floe template show <name>` | Show the items a data file of a template may hold, and its example |
| `floe template new <name> --from <template>` | Make a template of your own as a copy of another, to edit |
| `floe template check <dir>...` | Check templates before you use or share them |
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

Draw a page from a data file, store it and print its URL (`http://127.0.0.1:7325/a/<slug>/`) on stdout. If the server is not running, it is started in the background first. A new slug is also opened in the browser; a republished one is not, because the open tab reloads itself.

Pages are drawn from data files (`.yaml`, `.yml`). An HTML file is not stored: link it with `--link` to show it as it is. A relative path is resolved against the working directory.

### Flags

| Flag | Description |
|---|---|
| `--link` | Register an HTML file in place and show it as it is (see [Linked files](#linked-files)). |
| `--slug <name>` | The URL name. Lowercase letters, digits and hyphens, up to 64 characters. Defaults to the file name; a linked `index.html` takes the name of its directory. |
| `--title <text>` | The title shown in the list. Defaults to the data file's `title:`, or a linked file's `<title>`, else its first `<h1>`, else the slug. An empty value is a usage error. |
| `--description <text>` | A description shown in the list. |
| `--lang <tag>` | The language of the page drawn from a data file (`<html lang>`), as a BCP 47 tag (`en`, `ja`, `pt-BR`). Defaults to `en`, or to the last one given for the slug. Refused with `--link` of an HTML file, which is shown as it is. |
| `--theme <name>` | The template's theme to draw the page with, over the data file's `theme:`, which is then not used. Defaults to `theme:`, else `default`; a republish without it goes back to `theme:`. An empty value is a usage error. Refused with `--link` of an HTML file. |
| `--force` | Replace a page published from another project, or a link to another file. |

### Data files

A data file names a template, and the template draws the page from its data. The built-in template is `report`: a header over sections, each a heading over parts of Markdown text, with `html` parts for what Markdown cannot say.

```yaml
template: report               # the template that draws the page
title: Moving the team wiki
eyebrow: Plan · 2030-04        # a short line above the title (optional)
lead: The wiki moves in two weekends.   # a sentence under the title (optional)
sections:
  - heading: Steps
    parts:
      - text: |                # Markdown, tables included; raw HTML in it is left out
          1. Freeze edits on **Friday evening**.
          2. Copy the pages.
      - html: <p class="note warn">Edits during the freeze are lost.</p>   # HTML as it is
```

- `theme:` picks a theme of the template (`default`).
- An image the template takes (an item of `"format": "image"`) is a path under the data file's directory; the image goes into the page.
- A template of your own, in `FLOE_TEMPLATES_DIR`, takes the place of a built-in one of its name. [templates.md](templates.md) tells how to write one.
- The data is checked against the template's schema before the page is drawn. Every mistake is printed at once as `file:line:column: message`, and nothing is published until all are fixed (exit code 1): a missing or unknown template, an item the template does not have (a typo), a value of the wrong type, and the like.
- A data file is one YAML document of keys and values: one with `---` between documents, or with anchors, aliases or tags, is refused.
- The page is drawn once, when it is published: changing the data file later changes nothing until it is published again. A data file linked with `--link` is drawn each time it is opened instead.

### Projects and overwriting

Each page belongs to the project it is published from. The project is the repository name in git's `origin` URL, else the repository's directory, else the working directory's name. A link belongs to its file's directory instead.

- The same slug from the same project overwrites the page. stderr says so (`updated "plan" (last published …, title "…")`). If the title changed, a warning suggests another `--slug`. The exit code stays 0.
- A slug published from another project is refused unless `--force`. `--force` starts the page afresh: nothing of the old one is kept.
- A republish keeps what was set on the management page (title, project, favourite), the description, the language and the creation time. `--title`, `--description` and `--lang` override them.

### Linked files

`--link` serves an HTML file where it is, as it is: floe adds nothing around it, so a page of your own or one another tool made shows as written. It is read on every request, so saving the file updates the open tab. The files under its directory are served too, for its relative references (CSS, scripts, images).

- Only a regular `.html` or `.htm` file, or a data file (`.yaml`, `.yml`), can be linked.
- A file in a hidden directory right under your home (settings and keys live there) is refused.
- A file directly under `/` or under your home itself is refused, because the whole directory would be served.
- A link without `--slug` reuses the slug of the same file, else takes the first free one of `name`, `name-2`, `name-3` …
- The same slug for another file is refused unless `--force`.

A linked data file is drawn each time its page is opened, with `--lang` and `--theme` as given when it was linked, so that a page written in a repository can be read while it is written:

- It is checked when it is linked, as `floe publish` checks it, and refused with its mistakes.
- Saving the data file, an image it shows or a file of its template reloads the open tab.
- When it has mistakes, the tab shows them instead of the page, and comes back to the page once they are fixed and saved.
- Nothing of its directory is served: its images are in its page. Its download is the page drawn.
- Its title in the list is its `title:`, read each time.

---

## `floe check <data.yaml>...`

Check each data file as `floe publish` does before it stores the page: its YAML, its template and theme, the template's schema, checks and computed values, and drawing the page. A relative path is resolved against the working directory.

- Every mistake of every file is printed at once as `file:line:column: message`, in the order the files were given (exit code 1).
- Nothing is printed when every file is right (exit code 0).
- Nothing is published, and the server is not started, so a repository's CI can run it over its data files.
- A file that is not a data file (`.yaml`, `.yml`) is refused.

---

## `floe template list`

Print the templates, one per line in five tab-separated columns: the name, where it is (the directory of a template of your own, or `built-in` in every language, for scripts), its themes (comma-separated), the floe it needs (from its `template.yaml`; empty without one) and the description of its `schema.json`. A template of your own takes the place of the built-in one of its name. [templates.md](templates.md) tells how templates are written.

## `floe template show <name>`

Print what a data file of the template may hold, to read before writing one: where the template is, its themes, the floe it needs, its items (from `schema.json`, each with what it takes and its description, the items under it indented) and its `example.yaml`.

## `floe template new <name> --from <template>`

Make the template `<name>` of your own in `$FLOE_TEMPLATES_DIR/<name>` as a copy of `<template>`, built-in or your own, to edit.

- Its `template.yaml` names the floe it needs: that of `<template>`, or this floe's for a copy of a built-in one.
- Its `example.yaml` names `<name>` as its `template:`.
- A name is lower case letters, digits, `-` and `_`. A copy named like a built-in template takes its place.
- A template of your own of that name is left as it is: the command stops (exit code 1).

## `floe template check <dir>...`

Check each directory as the template of its name, before you use or share it. A relative path is resolved against the working directory.

- Its files first: `template.yaml`, `schema.json` (a `format` floe does not know is a mistake), `checks.yaml` (the paths of `for` and `field` must be declared in `schema.json`), `computed.yaml`, `page.html`, `themes/default.css` and the images of `assets/`.
- Then its `example.yaml` through every stage of `floe publish`, drawing the page included. It must name the template of the directory's name: the template drawn is always the one in the directory, never another of its name.
- Every mistake is printed at once, as `file:line:column: message` where it has a place (exit code 1). Nothing is printed when every template is right (exit code 0).
- Warnings go to stderr and leave the exit code 0: no `template.yaml`, or an object of `schema.json` that does not refuse a misspelt key (no `"additionalProperties": false`).

[templates.md](templates.md) tells what each file holds.

---

## `floe list`

Print the pages, newest first. Each line holds four tab-separated columns: the update time (local, `YYYY-MM-DD HH:MM`), the slug, the title and the linked path (empty for a stored page).

## `floe rm <slug>...`

Delete the pages in order, stopping at the first failure (such as a page that does not exist). A linked file itself is left alone; only its registration is removed.

## `floe serve`

Run the server in the foreground on `127.0.0.1` until SIGTERM or SIGINT. `floe publish` starts the same server in the background, so this is mainly for seeing its errors.

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

- `floe publish` starts the server when it is not running, and keeps using it otherwise. Two publishes racing to start it both end up on the one that comes up.
- After floe is reinstalled (another version, or a rebuilt binary), the next publish replaces the running server.
- The server keeps running until `floe stop`, logout or reboot.
- It listens on `127.0.0.1` only. It refuses other host names (against DNS rebinding) and writes from other sites.
- Pages run in a sandbox, so their scripts cannot reach the management page's API.

## Environment variables

| Variable | Description |
|---|---|
| `FLOE_DIR` | Where pages are stored. Defaults to `$XDG_DATA_HOME/floe`, else `~/.local/share/floe`. The layout is the same as the artifact skill's, so either can read the other's. |
| `FLOE_PORT` | The port (default `7325`). A decimal number from 1 to 65535. |
| `FLOE_ICONS_DIR` | Where `floe icons fetch` keeps icon sets, a directory each (`aws`). Defaults to `$XDG_DATA_HOME/floe-icons`, else `~/.local/share/floe-icons`. |
| `FLOE_TEMPLATES_DIR` | Where templates of your own are, a directory each ([templates.md](templates.md)). Defaults to `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`. The server draws linked data files with the one of the publish that started it, and a publish with another restarts it. |
| `FLOE_LANG` | The language of messages (`en`, `ja`). Falls back to `LC_ALL`, `LC_MESSAGES`, then `LANG`; a language without a catalog is skipped. English is the default. |
| `BROWSER` | The command that opens a new page. Defaults to `open` on macOS, else `wslview` or `xdg-open`. |

The management page follows the browser's language instead. Cobra's own errors (an unknown flag or command) and `--help` are in English.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Failure (a refused publish, a missing page, a port in use, a data file or diagram spec with mistakes …) |
| `2` | Usage error (an unknown flag or command, missing arguments, an empty `--title` or `--theme`, an unknown example, a `--lang` that is not a BCP 47 tag) |
