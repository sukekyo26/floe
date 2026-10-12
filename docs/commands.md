# Commands

Reference for every command of the `floe` binary.

## Quick reference

| Command | Purpose |
|---|---|
| `floe publish <file.html>` | Store a page (or `--link` a file in place) and print its URL |
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

## `floe publish <file.html>`

Store the page and print its URL (`http://127.0.0.1:7325/a/<slug>/`) on stdout. If the server is not running, it is started in the background first. A new slug is also opened in the browser; a republished one is not, because the open tab reloads itself.

A relative `<file.html>` is resolved against the working directory.

### Flags

| Flag | Description |
|---|---|
| `--link` | Register the file in place instead of copying it (see [Linked files](#linked-files)). |
| `--slug <name>` | The URL name. Lowercase letters, digits and hyphens, up to 64 characters. Defaults to the file name; `index.html` takes the name of its directory. |
| `--title <text>` | The title shown in the list. Defaults to the page's `<title>`, else its first `<h1>`, else the slug. An empty value is a usage error. |
| `--description <text>` | A description shown in the list. |
| `--lang <tag>` | The language to wrap body-only HTML in, as a BCP 47 tag (`en`, `ja`, `pt-BR`). Defaults to `en`, or to the last one given for the slug. |
| `--force` | Replace a page published from another project, or a link to another file. |

### Body-only HTML

A file without a doctype, `<html>`, `<head>`, `<body>` or `<title>` is wrapped in the built-in template, with `<html lang>` set to `--lang`. A whole document is stored as it is.

### Projects and overwriting

Each page belongs to the project it is published from. The project is the repository name in git's `origin` URL, else the repository's directory, else the working directory's name. A link belongs to its file's directory instead.

- The same slug from the same project overwrites the page. stderr says so (`updated "plan" (last published …, title "…")`). If the title changed, a warning suggests another `--slug`. The exit code stays 0.
- A slug published from another project is refused unless `--force`. `--force` starts the page afresh: nothing of the old one is kept.
- A republish keeps what was set on the management page (title, project, favourite), the description, the language and the creation time. `--title`, `--description` and `--lang` override them.

### Linked files

`--link` serves the file where it is. It is read on every request, so saving the file updates the open tab. The files under its directory are served too, for its relative references (CSS, scripts, images).

- Only a regular `.html` or `.htm` file can be linked.
- A file in a hidden directory right under your home (settings and keys live there) is refused.
- A file directly under `/` or under your home itself is refused, because the whole directory would be served.
- A link without `--slug` reuses the slug of the same file, else takes the first free one of `name`, `name-2`, `name-3` …
- The same slug for another file is refused unless `--force`.

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

Check each diagram spec and print the diagrams, in order, as one HTML fragment of inline SVG on stdout, to paste into a page. A relative path is resolved against the working directory.

- A mistake in the shape of a spec (an unknown key, a wrong type, a value that is not allowed) stops that spec at the first one.
- Layout mistakes are all reported: text too wide or too tall for its box, parts sticking out of the canvas, overlapping, or straddling the edge of a frame, edges running through boxes, labels or frame headings, and edges crossing like an X.
- Every mistake of every spec is printed as `file:line:column: message`, and nothing is drawn until all are fixed (exit code 1).
- The ids of the diagrams drawn together must differ, because they name the diagrams' markers. Draw all the diagrams of a page with one `render`.
- The fragment starts with the assets: the diagrams' style, which takes its colours from the template's CSS variables (so dark mode follows; a page outside the template gets the template's light colours), and the script that zooms them. Each diagram sits in a frame that first shows it whole; readers zoom with the buttons, Ctrl / ⌘ + wheel or a pinch, and drag or use the arrow keys to move while zoomed. The toolbar is worded in the page's language (`<html lang>`, from `floe publish --lang`). A page needs the assets once; holding them more than once does no harm.

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
| `frame` | `x` `y` `w` `h`, `label`, `accent`, `square` (no rounded corners), `icon` (an icon name such as `aws/group/region`: the frame is drawn in the AWS group style, with square corners and the 32px icon in its corner). Its label sits inside the top left corner, so place its contents 32 or more below its top (40 with an icon). |
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
| `FLOE_LANG` | The language of messages (`en`, `ja`). Falls back to `LC_ALL`, `LC_MESSAGES`, then `LANG`; a language without a catalog is skipped. English is the default. |
| `BROWSER` | The command that opens a new page. Defaults to `open` on macOS, else `wslview` or `xdg-open`. |

The management page follows the browser's language instead. Cobra's own errors (an unknown flag or command) and `--help` are in English.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Failure (a refused publish, a missing page, a port in use, a diagram spec with mistakes …) |
| `2` | Usage error (an unknown flag or command, missing arguments, an empty `--title`, an unknown example, a `--lang` that is not a BCP 47 tag) |
