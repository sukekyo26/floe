# Commands

Reference for every command of the `floe` binary. How to write a data file is in [pages.md](pages.md).

## Quick reference

| Command | Purpose |
|---|---|
| `floe publish <data.yaml>` | Store the page drawn from a data file (or `--link` a file in place) and print its URL |
| `floe check <data.yaml>...` | Check data files as their pages are drawn, without writing them |
| `floe render <data.yaml>...` | Write the page of each data file to an HTML file, to keep in a repository |
| `floe rebuild` | Draw every stored page again with the templates, parts and themes of now |
| `floe template list` | List the templates |
| `floe template show <name>` | Show a template: its themes, and its schema, checks and example as written |
| `floe template new <name>` | Make the smallest template of your own, to fill in |
| `floe template check` | Check the templates of your own |
| `floe part list` | List the parts, looks and shared schemas |
| `floe part show <name>` | Show a part, look or shared schema, and its schema and checks as written |
| `floe part new <name>` | Make the smallest part of your own, to fill in |
| `floe part check` | Check the parts, looks, shared schemas and themes of your own |
| `floe list` | List pages, newest first |
| `floe rm <slug>...` | Delete pages |
| `floe serve` | Run the server in the foreground |
| `floe stop` | Stop the background server |
| `floe diagram render <spec.yaml>...` | Check diagram specs and print them as inline SVG |
| `floe diagram inspect <spec.yaml>...` | Check diagram specs and print where each part went |
| `floe diagram example [kind]` | List the example specs, or print one |
| `floe icons fetch <set> [zip-or-url]` | Keep an icon set (`aws`) for the `icon` parts of diagrams |
| `floe icons search <word>...` | List the fetched icons whose name contains every word |
| `floe completion <shell>` | Print the completion script for a shell (Cobra builtin) |
| `floe version` | Print the version (also `floe --version`) |
| `floe help [command]` | Print help (Cobra builtin) |

---

## `floe publish <data.yaml>` / `floe publish --link <file>`

Draw the page of a data file with the template it names, store the page as it was drawn, and print its URL (`http://127.0.0.1:7325/a/<slug>/`) on stdout. If the server is not running, it is started in the background first. A new slug is also opened in the browser (`$BROWSER`, else `open`, `wslview` or `xdg-open`); a republished one is not, because the open tab reloads itself.

Pages are drawn from data files (`.yaml`, `.yml`); [pages.md](pages.md) tells how to write one. The data file is copied to `<slug>/source-<id>/data.yaml` under `FLOE_DIR`, with the images it shows at the paths it names them by; take it back from there if the original is lost. Each publish writes a new copy and then removes the one before, so a publish that fails leaves the page as it was. A relative path is resolved against the working directory.

The stored page stays as it was drawn: changing the template, a part or a theme changes it only when `floe rebuild` draws it again. Another file, such as an HTML file, is not stored: link it with `--link` to show it as it is.

### Flags

| Flag | Description |
|---|---|
| `--link` | Register the file in place: an HTML file is shown as it is, a data file is drawn each time its page is opened (see [Linked files](#linked-files)). |
| `--slug <name>` | The URL name: 1 to 64 lowercase letters, digits and hyphens, starting and ending with a letter or digit. Defaults to the file name; a linked `index.html` takes the name of its directory. |
| `--title <text>` | The title in the list. Defaults to the data file's `title:`, or a linked file's `<title>`, else its first `<h1>`, else the slug. |
| `--description <text>` | A description shown in the list. |
| `--theme <name>` | The theme to draw the page with, its palette and the parts' shape, over the data file's `theme:`. Defaults to `theme:`, else `default`. Refused with `--link` of an HTML file, which is shown as it is. |
| `--force` | Replace a page published from another project, or a link to another file. |

### Projects and overwriting

Each page belongs to the project it is published from. The project is the repository name in git's `origin` URL, else the repository's directory (a worktree joins its main checkout), else the working directory's name.

- The same slug from the same project overwrites the page. stderr says so (`updated "plan" (last published …, title "…")`). If the title changed, a warning suggests another `--slug`. The exit code stays 0.
- A slug published from another project is refused unless `--force`. `--force` starts the page afresh: nothing of the old one is kept.
- A republish keeps what was set on the management page (title, project, favourite), the description and the creation time. `--title` and `--description` override them.

### Linked files

`--link` registers a file where it is, instead of storing a page.

- Only a regular `.html` or `.htm` file, or a data file (`.yaml`, `.yml`), can be linked.
- A file in a hidden directory right under your home (settings and keys live there) is refused.
- A file directly under `/` or under your home itself is refused, because the whole directory would be served.
- A link without `--slug` reuses the slug of the same file, else takes the first free one of `name`, `name-2`, `name-3` …
- The same slug for another file is refused unless `--force`.

An HTML file is shown as it is: floe adds nothing around it, so a page of your own or one another tool made shows as written. The files under its directory are served too, for its relative references (CSS, scripts, images). Saving the file reloads the open tab.

A linked data file is drawn each time its page is opened, with `--theme` as given when it was linked, so that a page written in a repository can be read while it is written:

- It is checked when it is linked, as `floe check` checks it, and refused with its mistakes.
- Saving the data file, a part, theme or template of your own, or fetching icons, reloads the open tab. Saving an image it shows does not: save the data file again, or reload the tab.
- When it has mistakes, the tab shows them instead of the page, and comes back to the page once they are fixed and saved.
- Nothing of its directory is served: its images are in its page. Its download is the page drawn, its CSP kept in a `<meta>`.
- Its title in the list is its `title:`, read each time.

---

## `floe check <data.yaml>...`

Check each data file as its page is drawn: its YAML, its template and theme, the template's schema, its checks and computed values, and the page drawn. A relative path is resolved against the working directory.

- The mistakes of every file are printed as `file:line:column: message`, in the order the files were given (exit code 1): every mistake of a stage of the checks at once, a stage running only when the one before it has none. [pages.md](pages.md#reading-mistakes) tells how to read them.
- Nothing is printed when every file is right (exit code 0).
- Nothing is written, and the server is not started, so a repository's CI can run it over its data files.
- A file that is not a data file (`.yaml`, `.yml`) is refused.
- A data file named twice (by two globs, through a symlinked directory, or in another case on macOS) is checked once, and told by the path it was first given as.

## `floe render <data.yaml>...`

Draw each data file and write its page to `<name>.html`, beside the data file, so that a repository keeps the data file for agents to read and edit, and its page for people to read. The page is the one the management page downloads: it opens without floe, what it shows is in it (images, the diagrams' style and zoom), and its CSP, which keeps its scripts from sending what it holds out, is in a `<meta>`.

```sh
floe render docs/spec.yaml                 # writes docs/spec.html
floe render docs/*.yaml --out-dir site     # writes site/<name>.html
floe render --check docs/*.yaml            # in CI: fails when a page was not written again
```

| Flag | Description |
|---|---|
| `--out-dir <dir>` | Write the pages in this directory, made when missing, instead of beside each data file. |
| `--check` | Write nothing: print each page that is missing or is not what its data file draws (exit code 1), or nothing when every page is (exit code 0). |

- A page is written only when it changes, and its path is printed; running it again does nothing.
- A page there is written over: keep the pages in version control to get one back.
- The mistakes are printed as `floe check` prints them, and nothing is written until all are fixed (exit code 1). Nothing is written either when two data files would write the same page (`a/spec.yaml` and `b/spec.yaml` with one `--out-dir`, or `spec.yaml` and `spec.yml`).
- Nothing is published, and the server is not started.
- A page depends on the floe, the templates and the icons that draw it: in CI, use the floe version and the templates the pages were written with, and write the pages again when you upgrade.
- A `script` of the page runs without the sandbox of the server's pages when the file is opened; its CSP still cuts off `fetch`, XHR, images from elsewhere and forms. A link between pages (`/a/<slug>/`) does not work in a file: link the files relatively.

## `floe rebuild`

Have the server draw every stored page again from the copy of its data file and images, with its templates, parts and themes of now, and keep the page where it changed. Run it after you change a template, part or theme of your own, or upgrade floe.

- A page that no longer draws keeps the page it had, and its mistakes are told (exit code 1): fix the data file it was published from and publish it again, or fix the template.
- Linked files are drawn each time they are opened, and are left alone.
- The server is started when it is not running.

---

## `floe template list`

Print the templates, floe's and your own, one per line in three tab-separated columns: the name, where it is (`floe`, or the directory of a template of your own) and the description of its `schema.json`. A template of your own hides floe's of its name. [templates.md](templates.md) tells how templates are written.

## `floe template show <name>`

Print a template, to read before writing a data file of it: its description, where it is, the themes a data file may pick (`themes:`), and, as written, its `schema.json`, its `checks.yaml` and its `example.yaml` to start a data file from.

## `floe template new <name>`

Make the template `<name>` in `FLOE_TEMPLATES_DIR`: a `page.html`, a `schema.json` and an `example.yaml`, the smallest that `floe template check` takes, to fill in.

- A name is lower case letters, digits, `-` and `_`, starting with a letter or a digit.
- Nothing is written over: with anything of that name there, the command stops and makes nothing (exit code 1).

## `floe template check`

Read the templates in `FLOE_TEMPLATES_DIR` with the parts and themes of your own, as `floe part check` does, then draw the `example.yaml` of each template of your own as `floe check` draws a data file.

- The mistakes are printed (exit code 1): those of the files, and the examples drawn once the files have none. Nothing is printed when they are right (exit code 0).
- A file floe does not read in a template (a `template.yaml`, say) is refused: a template holds `page.html`, `schema.json` and `example.yaml`, and `page.css`, `computed.yaml`, `checks.yaml`, `words.yaml` and `assets/` if it likes.

[templates.md](templates.md) tells what each file holds.

## `floe part list`

Print the parts, looks and shared schemas, floe's and your own, one per line in three tab-separated columns: the name, where it is (`floe`, or the directory of one of your own) and its description.

## `floe part show <name>`

Print a part, a look or a shared schema: its description, where it is, the looks it takes, and the files that say what a data file may hold of it, as written (the `schema.json` and `checks.yaml` of a part, the `<name>.json` of a shared schema).

## `floe part new <name>`

Make the part `<name>` in `FLOE_PARTS_DIR`: a `schema.json`, a `part.html` of its box and an empty `part.css`, the smallest that `floe part check` takes, to fill in.

- Nothing is written over: with anything of that name there, the command stops and makes nothing (exit code 1).

## `floe part check`

Read the parts, looks and shared schemas in `FLOE_PARTS_DIR` and the themes in `FLOE_THEMES_DIR`, over floe's own, and print every mistake of them (exit code 1). Nothing is printed when they are right (exit code 0).

A mistake of a part, theme or template of your own stops `floe check`, `floe render` and `floe publish` too, whether the page uses it or not: run `floe part check` and `floe template check` to see it. [parts.md](parts.md) tells what a part and a theme hold.

---

## `floe list`

Print the pages, newest first, as the server lists them (it is started when it is not running). Each line holds four tab-separated columns: the update time (local, `YYYY-MM-DD HH:MM`), the slug, the title and the linked path (empty for a stored page).

## `floe rm <slug>...`

Delete the pages in order through the server (it is started when it is not running), stopping at the first failure (such as a page that does not exist). A linked file itself is left alone; only its registration is removed.

## `floe serve`

Run the server in the foreground on `127.0.0.1` until SIGTERM or SIGINT. `floe publish`, `floe rebuild`, `floe list` and `floe rm` start the same server in the background, so this is mainly for seeing its errors. A background server that does not start is told as `the server did not start; run "floe serve" to see the error`.

## `floe stop`

Stop the background server. Without one, it prints `server is not running` and exits 0.

---

## `floe diagram render <spec.yaml>...`

Check each diagram spec and print the diagrams, in order, as one HTML fragment of inline SVG on stdout, to paste into an HTML page of your own: a whole document with `<!doctype html>` and `<html lang>`, since `floe publish --link` shows the file as it is. In a data file, write the spec in a `diagram` part instead ([pages.md](pages.md#diagrams), which tells how a spec is written). A relative path is resolved against the working directory.

- A spec is checked against the diagram's schema, then laid out once for every language its text is written in, and checked: text that overflows, parts that overlap or stick out of the canvas, edges through boxes, crossings … ([pages.md](pages.md#diagrams) lists the checks).
- Every mistake of every spec is printed as `file:line:column: message`, in the order the specs were given, and nothing is drawn until all are fixed (exit code 1).
- The text is written in the language of `--lang` (`en` when left out). A language a text does not have falls back to English.
- The fragment starts with the assets: the diagrams' style, which takes its colours from the palette's CSS variables (a page without them gets `default`'s colours), and the script that zooms them. Readers zoom with the buttons, Ctrl / ⌘ + wheel or a pinch, and drag or use the arrow keys to move while zoomed. A page needs the assets once; holding them more than once does no harm.

| Flag | Description |
|---|---|
| `--lang <tag>` | The language to write the diagrams' text in (`en`, `ja`). Defaults to `en`. |
| `--no-assets` | Print the diagrams without the assets, for a page that already holds them. |

## `floe diagram inspect <spec.yaml>...`

Check each spec as `floe diagram render` does, and print, a line each, where each part went: a part's area, an edge's path and the ends it joins, a label's area and the circles it is in. It lets one check that edges meet the rows they mean and labels sit in their areas, without seeing the drawing.

## `floe diagram example [kind]`

Without a kind, list the built-in examples, one per kind of diagram (architecture, AWS architecture, class, ER, flow, Git branches, sequence, state, timeline, Venn): each line holds the kind and a summary, tab-separated. With a kind, print its spec; its comments explain the grid it was laid out on. Every example passes the checks. Its text is written in English and Japanese (`{en: …, ja: …}`), as a spec may write it. The AWS example draws AWS icons, so render it after `floe icons fetch aws`.

---

## `floe icons fetch <set> [zip-or-url]`

Keep an icon set in `FLOE_ICONS_DIR/<set>` for the `icon` parts of diagrams. floe does not ship icons; each machine fetches a set once. The sets:

| Set | Icons |
|---|---|
| `aws` | The [AWS Architecture Icons](https://aws.amazon.com/architecture/icons/): the 48px service and resource icons and the 32px group icons. The Icon package zip is about 14MB. |

- Without a zip, the set's package is found on its official page and downloaded. A zip downloaded in a browser, or its URL, can be given instead; a relative path is resolved against the working directory.
- A version already there is kept. `--force` fetches it again.
- The icons are replaced as a whole: if that fails, the previous ones stay.

| Flag | Description |
|---|---|
| `--force` | Fetch the icons again even if that version is already there. |

## `floe icons search <word>...`

List the fetched icons whose name contains every word (case does not matter), one per line: the name to give an `icon` part (`aws/…`) and its category, tab-separated.

---

## The server

- `floe publish`, `floe rebuild`, `floe list` and `floe rm` start the server when it is not running, and keep using it otherwise.
- They ask the server to check, save, draw, list and delete pages: the server is the only one that writes the store. A publish sends the data file (and the images the server asks for), and the server checks and draws it with its own templates, parts, themes and icons.
- After floe is reinstalled (another version, or a rebuilt binary), the next of them replaces the running server.
- The server keeps running until `floe stop`, logout or reboot.
- The server draws with the `FLOE_TEMPLATES_DIR`, `FLOE_PARTS_DIR`, `FLOE_THEMES_DIR` and `FLOE_ICONS_DIR` of the command that started it. A `floe publish` or `floe rebuild` with other ones is refused until `floe stop`, so set yours in the shell's environment, where every command takes them, rather than on one command. `floe list` and `floe rm` do not draw, and take any.
- A server serves one store (`FLOE_DIR`) on its port: while it runs, a command for another store is refused. Give that store another `FLOE_PORT`.
- It listens on `127.0.0.1` only. It refuses other host names (against DNS rebinding) and writes from other sites.
- A stored page is served as it was drawn; its CSP and sandbox are sent with it. Pages run in the sandbox, so their scripts cannot reach the management page's API.
- The scripts of a page drawn from a data file cannot send what it holds out with `fetch`, XHR, images from elsewhere or forms, its download included. Linked HTML loads what it loads.
- A page opens at the part its URL names (`/a/<slug>/#part`), and a move to a part inside it is kept in the URL, so that a copy of the URL or a reload opens that part.
- The management page downloads a page as one file: the stored page, or a linked file, with its CSP in a `<meta>`.

## Environment variables

| Variable | Description |
|---|---|
| `FLOE_DIR` | Where pages are stored. Defaults to `$XDG_DATA_HOME/floe`, else `~/.local/share/floe`. |
| `FLOE_PORT` | The port (default `7325`). A decimal number from 1 to 65535. |
| `FLOE_ICONS_DIR` | Where `floe icons fetch` keeps icon sets, a directory each (`aws`). Defaults to `$XDG_DATA_HOME/floe-icons`, else `~/.local/share/floe-icons`. |
| `FLOE_TEMPLATES_DIR` | Where templates of your own are, a directory each ([templates.md](templates.md)). Defaults to `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`. |
| `FLOE_PARTS_DIR` | Where parts, looks and shared schemas of your own are ([parts.md](parts.md)). Defaults to `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`. |
| `FLOE_THEMES_DIR` | Where themes of your own are, a directory each ([parts.md](parts.md)). Defaults to `$XDG_CONFIG_HOME/floe/themes`, else `~/.config/floe/themes`. |
| `BROWSER` | The command that opens a new page. Defaults to `open` on macOS, else `wslview` or `xdg-open`. |

Each `FLOE_*` directory is an absolute path. floe reads one place wherever it runs, so a relative one is refused, and so is one starting with `~`, which no shell read (`FLOE_PARTS_DIR is ~/x, a relative path; set it to an absolute path`). A relative `XDG_DATA_HOME` or `XDG_CONFIG_HOME` is ignored, as the XDG spec says.

A command is told of the settings it uses alone, when it uses them. A directory set that cannot be used, as it is not a directory or cannot be read or written (or, for your own templates, parts and themes, is not there), is refused by its variable's name:

```text
floe: FLOE_PARTS_DIR is /tmp/t/missing, which cannot be used as a directory (no such file or directory); make it a directory floe can use, or set FLOE_PARTS_DIR to another directory
```

- `FLOE_TEMPLATES_DIR`, `FLOE_PARTS_DIR` and `FLOE_THEMES_DIR` are used by every command that draws a page or reads your own (`floe check`, `floe render`, the `template` and `part` commands, and the server, which draws), so one set that is not there stops them. Their default that is not there is none: floe draws with its own alone. A default that is there but is no directory is refused.
- `FLOE_DIR` is used by the server (`floe serve`, and `floe publish`, `floe rebuild`, `floe list` and `floe rm`, which start it), and made when it is not there. `FLOE_ICONS_DIR` is used by the icons commands, which make it, and by the diagrams that show an icon, at the diagram.
- Without `HOME` (or with one that is not an absolute path), a directory that has no other default is none: `floe check` runs with floe's templates, parts and themes alone, while a command that writes there (`floe template new`, `floe list` for `FLOE_DIR` …) asks for the variable to be set.
- Keep `FLOE_PARTS_DIR` and `FLOE_THEMES_DIR` apart: in one directory, the parts are read as themes and the themes as parts, each refused as such.

The command line speaks English, whatever the environment's language; the management page and the page of a data file's mistakes follow the browser's language.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Failure, and a value floe refuses (a refused publish, a missing page, a port in use, a data file or diagram spec with mistakes, an unknown example …) |
| `2` | A command line floe cannot read, or that lacks what it needs (an unknown flag or command, missing arguments) |
