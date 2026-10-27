# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add `floe publish`, which draws the page of a data file (YAML) with the
  template it names, stores the page as drawn and prints its URL on a local
  server at `http://127.0.0.1:7325`, started in the background when it is not
  running; `--link` registers a file in place instead: an HTML file shown as it
  is, or a data file drawn each time its page is opened, the open tab reloading
  when it is saved.
- Add `floe check` and `floe render`: check data files without writing them,
  and write each page to an HTML file that opens without floe (`--check` for
  CI), every mistake printed at once as `file:line:column: message`.
- Add `floe rebuild`, which draws every stored page again with the templates,
  parts and themes of now, keeping the page it had where it no longer draws.
- Add `floe list`, `floe rm`, `floe serve` and `floe stop`, and a management
  page in English or Japanese, by the browser's language, that lists, edits,
  opens and downloads pages.
- Add floe's template `report`: a header over sections, each a heading over
  parts, and twenty-nine parts that report takes and a template refers to as
  `floe:///parts/<name>.json`, from `text`, `code`, `terminal` and `table` to
  `findings`, `decision`, `risks`, `timeline`, `tabs`, `diagram` and `script`
  (`floe part list` lists them).
- Add nine themes a data file picks with `theme:` (`default`, `soft`,
  `editorial`, `ink`, `brutal`, `blueprint`, `notebook`, `swiss` and `washi`),
  each a palette of CSS variables and the shape of the parts.
- Add templates of your own in `FLOE_TEMPLATES_DIR`: `page.html`,
  `schema.json` and `example.yaml`, and `page.css`, `checks.yaml`,
  `computed.yaml`, `words.yaml` and `assets/` if they like; and
  `floe template list`, `show`, `new` (the smallest template, to fill in) and
  `check`.
- Add parts, looks, shared schemas and themes of your own in `FLOE_PARTS_DIR`
  and `FLOE_THEMES_DIR`, beside floe's and never named as one of them; and
  `floe part list`, `show`, `new` (the smallest part, to fill in) and `check`.
  A mistake of one of your own stops every page drawn until it is fixed.
- Add diagrams drawn as inline SVG from a spec of parts on a canvas (frames,
  boxes, tables, icons, edges joining named parts, dots, bars, axes, circles,
  labels, lines), their text in one language or several (`{en: …, ja: …}`),
  checked for overflowing text, overlaps, edges through boxes and crossings
  in every language before they are drawn: a `diagram` part in a data file,
  or `floe diagram render`, `inspect` and `example`.
- Add `floe icons fetch` and `floe icons search`, which keep the AWS
  Architecture Icons for the `icon` parts of diagrams.
- Add a page's CSP, which keeps the scripts of a page drawn from a data file
  from sending what it holds out, on the server and in a page `floe render`
  writes.
