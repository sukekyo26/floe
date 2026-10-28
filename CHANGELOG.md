# Changelog

All notable changes to floe are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Add a button over `code`, `terminal` and `diff` that copies them: the code,
  the commands without their prompt or output, the diff as written. It says
  "Copied" in the page's language, and still copies where the page may not
  write the clipboard, such as in VS Code's Simple Browser.
- Add archiving: `floe archive <slug>...` (`--undo` to give back), "Archive"
  on the management page, one page or those selected, and `archived` in
  `PATCH /api/artifacts/{slug}` take a page off `floe list` and the management
  page's list without deleting it; it is still served at its URL. The
  management page shows the archived under "Archive" on its shelf, and
  `floe list --archived` lists them too, with a fifth column. Publishing an
  archived page again gives it back to the lists.
- Add `floe publish`, which draws the page of a data file (YAML) with the
  template it names, stores the page as drawn and prints its URL on a local
  server at `http://127.0.0.1:7325`, started in the background when it is not
  running; `--link` registers a file in place instead: an HTML file shown as it
  is, or a data file drawn each time its page is opened, the open tab reloading
  when it is saved.
- Add `floe check` and `floe render`: check data files without writing them,
  and write each page to an HTML file that opens without floe (`--check` for
  CI), the mistakes printed as `file:line:column: message`, every one of a
  stage of the checks at once.
- Add `floe rebuild`, which draws every stored page again with the templates,
  parts and themes of now, keeping the page it had where it no longer draws.
- Add `floe list`, `floe rm`, `floe serve` and `floe stop`, and a management
  page in English or Japanese, by the browser's language, that lists, edits,
  opens and downloads pages: a shelf on the left picks all, the favourites
  (which head the list of all), the archived, a tag and the project (picked
  again, let go), a row of buttons on a narrow screen; the bar over the list
  picks the kind and the order (from a menu of the page's own, by mouse or
  keys), and holds the actions on the pages selected
  while there are any; the list groups its pages by project in round cards,
  each project in a color of its own, those with the most pages first, a row's
  menu holding its edit, opening, download, archive and delete; and the bar over
  an open page stars and archives it with buttons of icons alone, filled in while
  on, and its title opens the pages of the list it was opened from, to search and
  pick from. Tags are given in a page's edit dialog, to those selected,
  or as `tags` in `PATCH /api/artifacts/{slug}`; they show under a page's
  title, are searched, and stay when the page is published again. What is
  reached by keyboard shows one thin grey ring, and a button pressed with the
  mouse shows none.
- Add floe's template `report`: a header over sections, each a heading over
  parts; and thirty-two parts a template refers to as
  `floe:///parts/<name>.json` (`floe part list` lists them): thirty that a
  section of report takes beside Markdown `text` and `html`, from `code`,
  `terminal`, `table` and `image` (an image of the data file's directory,
  such as a screenshot) to `findings`, `decision`, `risks`, `timeline`,
  `tabs`, `diagram` and `script`, and `cell` and `stack`, which other parts
  hold.
- Add nine themes a data file picks with `theme:` (`default`, `soft`,
  `editorial`, `ink`, `brutal`, `blueprint`, `notebook`, `swiss` and `washi`),
  each a palette of CSS variables and the shape of the parts. Pages in
  `default`, and in the themes that take its colours (`soft`, `editorial`), have
  the warm grey ground and text of the management page, with lines of the same
  warmth.
- Add templates of your own in `FLOE_TEMPLATES_DIR`: `page.html`,
  `schema.json` and `example.yaml`, and `page.css`, `checks.yaml`,
  `computed.yaml`, `words.yaml` and `assets/` if they like; and
  `floe template list`, `show`, `new` (the smallest template, to fill in) and
  `check`.
- Add parts, looks, shared schemas and themes of your own in `FLOE_PARTS_DIR`
  and `FLOE_THEMES_DIR`, beside floe's and never named as one of them; and
  `floe part list`, `show`, `new` (the smallest part, to fill in) and `check`.
  A part's schema takes floe's formats (`html`, `markdown`, `image`) as a
  template's does, and a shared schema holding them. A shared schema is
  checked from its root, whether a schema names it or not. A mistake
  of a part, shared schema, theme or template of your own stops every page
  drawn until it is fixed. The directories of parts, themes and templates of
  your own that are one, or one in the other, are refused for their settings.
- Add diagrams drawn as inline SVG from a spec of parts on a canvas (frames,
  boxes, tables, icons, edges joining named parts, dots, bars, axes, circles,
  labels, lines), their text in one language or several (`{en: …, ja: …}`),
  checked for overflowing text, overlaps, edges through boxes and crossings
  in every language before they are drawn: a `diagram` part in a data file,
  or `floe diagram render`, `inspect` and `example`.
- Add `floe icons fetch` and `floe icons search`, which keep the AWS
  Architecture Icons for the `icon` parts of diagrams. Icons not fetched, or a
  directory for them that cannot be used, are told once, at no place in a
  data file, however many diagrams and files need them.
- Add a page's CSP, which keeps the scripts of a page drawn from a data file
  from sending what it holds out, on the server and in a page `floe render`
  writes.
