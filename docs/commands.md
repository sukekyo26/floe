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
| `FLOE_LANG` | The language of messages (`en`, `ja`). Falls back to `LC_ALL`, `LC_MESSAGES`, then `LANG`; a language without a catalog is skipped. English is the default. |
| `BROWSER` | The command that opens a new page. Defaults to `open` on macOS, else `wslview` or `xdg-open`. |

The management page follows the browser's language instead. Cobra's own errors (an unknown flag or command) and `--help` are in English.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Failure (a refused publish, a missing page, a port in use …) |
| `2` | Usage error (an unknown flag or command, missing arguments, an empty `--title`) |
