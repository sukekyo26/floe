# floe

[日本語版 README](docs/README.ja.md)

A local page server for agents, shipped as a single Go binary. Agents write a
page as data (YAML) and floe draws it with a template, or link an HTML file to
show it as it is; floe serves them at `http://127.0.0.1:7325` with live reload
and a management page. Diagrams are drawn as inline SVG from a spec, and
checked for overflowing text, overlaps and crossing edges before they are drawn.

## Install

linux and macOS (amd64 / arm64):

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

Set `FLOE_VERSION` to pin a release and `FLOE_INSTALL_DIR` to change the
destination (default `~/.local/bin`). The binary is verified against the
release's `SHA256SUMS`.

## Quick start

```sh
floe template list          # floe's templates (report, design, handoff, adr, postmortem, runbook, slides) and your own
floe template show report   # report's files: what a data file holds and an example first
floe check plan.yaml        # check a data file: prints its mistakes and warnings, or nothing
floe publish plan.yaml      # store its page, print the URL and open it
floe publish --link out/index.html   # show an HTML file as it is
floe publish --link docs             # list the HTML files under a folder
floe list                   # pages, newest first
floe archive old-plan       # take a page off the lists, keeping it
floe stop                   # stop the background server
```

The server starts on the first publish and keeps running until `floe stop`.
Open `http://127.0.0.1:7325` for the management page.

## Documentation

| To | Read | 日本語 |
|---|---|---|
| Write pages (data files) | [pages.md](docs/pages.md) | [pages.ja.md](docs/pages.ja.md) |
| Make a template of your own | [templates.md](docs/templates.md) | [templates.ja.md](docs/templates.ja.md) |
| Make a part or theme of your own | [parts.md](docs/parts.md) | [parts.ja.md](docs/parts.ja.md) |
| Look up a command or a setting | [commands.md](docs/commands.md) | [commands.ja.md](docs/commands.ja.md) |
| See what changed | [CHANGELOG.md](CHANGELOG.md) | [CHANGELOG.ja.md](docs/CHANGELOG.ja.md) |

## i18n

The command line speaks English, whatever the environment's language, as an
agent reads it. What a browser shows, the management page and the page of a
data file's mistakes, is in English or Japanese, by the browser's language.

## Development

`just ci` runs what CI runs. The browser tests find Chrome at `FLOE_CHROME`, else on `PATH`, and are skipped without it.

## License

[MIT](LICENSE)
