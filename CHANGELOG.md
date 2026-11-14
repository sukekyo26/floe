# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add `floe publish`, which draws a page from a data file (YAML) with a template and shows it on a local server (`http://127.0.0.1:7325`), started when it is not running; `--link` shows an HTML file or a folder of your own as it is.
- Add `floe check` and `floe render`, which check data files and write pages to keep in a repository, and `floe list`, `floe info`, `floe tag`, `floe archive`, `floe rm`, `floe rebuild`, `floe serve` and `floe stop` for the stored pages.
- Add mistakes told at the file, line and column of what is wrong, with how to mend it, and warnings that advise a better way to write.
- Add floe's templates `report`, `design`, `handoff`, `adr`, `postmortem`, `runbook` and `slides`, and templates of your own (`floe template new`).
- Add floe's parts (text, code, terminal, diff, table, entries, steps, stats, bars, chart, checks, asks, options, columns, image and more) and parts of your own (`floe part new`).
- Add twelve themes and themes of your own (`floe theme new`); people pick the default and a page's theme on the management page.
- Add diagrams drawn as checked inline SVG: frames, boxes, edges, tables, trees, a `graph` laid out from its edges, and examples (`floe diagram example`).
- Add icons for diagrams: Lucide built in, and the AWS, Google Cloud and Azure sets kept by `floe icons fetch` and found by `floe icons search`.
- Add the management page: the pages by project, search (`Ctrl+K`), tags, pins, archive, a page's versions and what changed between them, unread pages, and settings.
- Add projects, named after the git repository a page is first published from (`--move-project` moves it).
- Add tasks: a task list by project and state, with child tasks, outside links in order, and a page's category on its task; `floe task` reads and writes them.
- Add the task mode, which has `floe guide` tell agents to ask the user, after publishing a new page, whether to put it on a task.
- Add `floe guide`, the guide for agents, with `floe task guide`, `floe diagram guide`, `floe part guide`, `floe template guide` and `floe theme guide`, and `floe skill install`, which points an agent's skill to it.
- Add a page's CSP and sandbox, which keep its scripts to floe's own.
- Add `floe licenses`, which prints the licenses of floe and of what is built into it.

