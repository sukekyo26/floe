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
floe guide                   # how an agent writes, checks and publishes a page
floe diagram guide           # how an agent writes a diagram, by hand or as a graph
floe template list          # floe's templates (report, design, handoff, adr, postmortem, runbook, slides) and your own
floe template show report   # report's files, what a data file holds first
floe template show report example.yaml   # an example to start from
floe check plan.yaml        # check a data file: prints its mistakes and warnings, or nothing
floe publish plan.yaml      # store its page, print the URL and open it
floe publish --link out/index.html   # show an HTML file as it is
floe publish --link docs             # list the HTML files under a folder
floe list                   # pages, newest first
floe archive old-plan       # take a page off the lists, keeping it
floe tag old-plan --add design   # put a tag on a page
floe stop                   # stop the background server
```

The server starts on the first publish and keeps running until `floe stop`.
Open `http://127.0.0.1:7325` for the management page.
Read the pages in a recent Chrome, Safari (26 or later) or Firefox (147 or later).

## For agents

`floe guide` prints the guide an agent follows to write, check and publish a
page. `floe skill install <agent>` writes a skill that points the agent to it,
where the agent reads the skills of all your projects; `floe skill install --help`
lists the agents and their places. If you use Cursor, it reads both
`~/.claude/skills` and `~/.agents/skills`, and shows the skill twice when it is
in both; that does no harm. To remove the skill, remove the `floe` directory
it was written to.

For an agent not listed, put this line in its instructions (AGENTS.md or the
like):

```markdown
To show the user a page, use floe: run `floe guide` first and follow it. It is the guide of the floe installed, so read it each time rather than from memory.
```

## Documentation

| To | Read | 日本語 |
|---|---|---|
| What an agent reads first (`floe guide`) | [guide.md](docs/guide.md) | [guide.ja.md](docs/guide.ja.md) |
| Write pages (data files) | [pages.md](docs/pages.md) | [pages.ja.md](docs/pages.ja.md) |
| Make a template of your own (`floe template guide`) | [templates.md](docs/templates.md) | [templates.ja.md](docs/templates.ja.md) |
| Make a part of your own (`floe part guide`) | [parts.md](docs/parts.md) | [parts.ja.md](docs/parts.ja.md) |
| Make a theme of your own (`floe theme guide`) | [themes.md](docs/themes.md) | [themes.ja.md](docs/themes.ja.md) |
| Draw a diagram (`floe diagram guide`) | [diagrams.md](docs/diagrams.md) | [diagrams.ja.md](docs/diagrams.ja.md) |
| Show an HTML file or a folder as it is | [links.md](docs/links.md) | [links.ja.md](docs/links.ja.md) |
| Look up a command or a setting | [commands.md](docs/commands.md) | [commands.ja.md](docs/commands.ja.md) |
| See what changed | [CHANGELOG.md](CHANGELOG.md) | [CHANGELOG.ja.md](docs/CHANGELOG.ja.md) |

## i18n

The command line speaks English, whatever the environment's language, as an
agent reads it. What a browser shows, the management page and the page of a
data file's mistakes, is in English or Japanese, by the browser's language.

## Development

`just ci` runs CI's checks, its tests without the race detector: CI runs them with it, the browser tests in two jobs of their own (`just test-browser looks` and `rest`), so a race is told by CI alone. Under the race detector the heaviest random tests of `internal/diagram` run a fifth of their graphs: reproduce a graph that the coverage job fails on without it. The browser tests find Chrome at `FLOE_CHROME`, else on `PATH`, and are skipped without it.

## License

[MIT](LICENSE)
