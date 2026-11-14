# Templates

A template is the page a data file is drawn into: a directory that says what the data file may hold, how the page is laid out and what goes with it. A data file names its template with `template:` (`floe guide` tells how to write one). floe's templates are `report`, `design`, `handoff`, `adr`, `postmortem`, `runbook` and `slides` (`floe template list` lists them). This doc says what each file holds, what `page.html` can use, what `page.css` reaches, how a template uses parts, looks and the palette, and what `floe template check` refuses. For a part of your own, see `floe part guide`; for a theme, `floe theme guide`.

## Where templates are

- A template is a directory in `FLOE_TEMPLATES_DIR` (default `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`), named after the template. `floe template new memo` makes the smallest one, `memo`: a `page.html`, a `schema.json` and an `example.yaml` to fill in. A file directly in `FLOE_TEMPLATES_DIR` is ignored without a message: put a template's files in its directory.
- The name MUST be lower case letters, digits, `-` and `_`, starting with a letter or a digit.
- Do not give a template the name of one of floe's, unless it is to replace it (a template of yours hides floe's of that name for every page that names it, and nothing says so).
- A directory in `FLOE_TEMPLATES_DIR`, and anything in a template's directory, MUST NOT be a link to a directory (floe reads a link to a file). Every file there MUST be a regular file: floe refuses a FIFO or a device without reading it.
- `FLOE_TEMPLATES_DIR` MUST NOT be the directory of your parts or themes (`FLOE_PARTS_DIR`, `FLOE_THEMES_DIR`), contain it, or be in it: floe would read the files of one as those of the other.
- A mistake in any template, part, look, shared schema or theme of yours stops every page, whether or not the page uses it (a page of `report` too), until it is fixed: `floe check`, `floe render`, and the server's publishes and linked pages alike. `floe template check` and `floe part check` (`floe theme check` the same) list them all. A published page is drawn with the server's templates (`floe serve --help`); after you change one, `floe rebuild` draws the stored pages again with it.

### Make one in another directory

The server reads your templates again as soon as you save one, so a template half written stops the server's pages for everyone who uses it. Write a new template in another directory, and move it in once it is checked:

```sh
export FLOE_TEMPLATES_DIR=~/.config/floe/staging-templates
mkdir -p "$FLOE_TEMPLATES_DIR"
floe template new memo                      # made ~/.config/floe/staging-templates/memo
floe template check                         # mend memo until it tells no mistake
floe render --out-dir /tmp/memo notes.yaml  # a data file of template: memo; open /tmp/memo/notes.html
unset FLOE_TEMPLATES_DIR
mkdir -p ~/.config/floe/templates
mv ~/.config/floe/staging-templates/memo ~/.config/floe/templates/
```

- `floe template check` and `floe render` draw without the server. Do not `floe publish` while `FLOE_TEMPLATES_DIR` is set to the other directory: a server running refuses it (it draws with the directories it was started with), and with none running, the publish starts one on the other directory.
- Stage the template on the same file system as the templates' directory, so that `mv` moves it in at once.
- To change a template you have, copy it to the other directory, change and check it there, and move it back in place of the old one.
- Parts and themes are made the same way, with `FLOE_PARTS_DIR`, `floe part new` and `floe part check`, and `FLOE_THEMES_DIR`, `floe theme new` and `floe theme check` (`floe part guide`, `floe theme guide`).

## Files

`floe template show <template>` lists the themes and the files a template reads (floe's too), its schema and example first, and `floe template show <template> page.css` prints one.

| File | In a template | What it holds |
|---|---|---|
| `page.html` | required | The page, a Go html/template (*page.html*) |
| `schema.json` | required | What a data file may write of the template, as JSON Schema (*schema.json*) |
| `example.yaml` | required | A data file to start from, which `floe template check` draws (*example.yaml*) |
| `page.css` | optional | The page's CSS, around the parts (*page.css*) |
| `checks.yaml` | optional | Relations between items that a schema cannot say (*checks.yaml*) |
| `computed.yaml` | optional | Values computed from the data (*computed.yaml*) |
| `words.yaml` | optional | The template's words, by language (*words.yaml*) |
| `assets/` | optional | Images `page.html` shows with `asset` (*Images*) |

A file or directory of any other name MUST NOT be in a template: floe refuses it. A name that begins with `.` (`.git/`, `.DS_Store`) is skipped, neither read nor refused. `example.yaml` MUST name the template of its directory (`template: memo`).

## page.html

`page.html` is a Go html/template, which escapes what it inserts. `floe template new memo` writes this one, the smallest page floe takes:

```html
<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
</head>
<body>
</body>
</html>
```

- Start the page with `<!doctype html>` (floe adds none).
- Write the viewport `<meta>`.
- You need not write a charset: a page opened without floe's server (`floe render`, a download) gets one from floe, first in its head, and the server sends it in its header (floe's own templates write one all the same).
- The page drawn MUST NOT hold a `<style>` (write the CSS in `page.css`), a `<base>`, a `<noscript>`, or a `<meta>` other than its charset and viewport, whoever writes it: `page.html`, a part's `part.html` or an item of the data. floe refuses it at the data file's `template:`, as the page is drawn.
- Inside a `<select>`, the page drawn MUST NOT hold an element whose content HTML reads as text (`<script>`, `<textarea>`, `<title>`, `<iframe>` and the like), nor an `<svg>` or a `<math>`.
- The page drawn MUST NOT hold a `<template>` that HTML reads as a `<template>` element inside an `<svg>` or a `<math>`, as in `<svg><foreignObject><template>x</template></foreignObject></svg>`: HTML's reader leaves out the rest of the page past it, and floe refuses the page (`<svg><template>x</template></svg>` is an svg element, and passes).
- `data-floe-part` MUST name one part that exists (`data-floe-part="note"`): floe refuses a name with no part, a look's name (take a look with `data-floe-look`) and several names.

`page.html` sees these, besides html/template's own functions (`printf`, `index`, `eq` and the rest):

| Name | What it is |
|---|---|
| `.Title` | The page's title: the data file's `title:`, else the slug |
| `.Lang` | The page's language: the data file's `lang:`, which every data file has |
| `.Data` | The data file's items, without `lang`, `template`, `theme` and `title`, which floe reads itself. A part comes drawn (*Parts in a template*), as does an item of `"format": "markdown"` (its HTML) or `"format": "image"` (a data URI); an item of `"format": "html"` is the HTML as written |
| `.Calc` | The values of `computed.yaml`, by name |
| `markdown` | Markdown text as HTML (`{{markdown .text}}`), HTML in it drawn as the text written: in `page.html`, in the box of a template's own Markdown (*schema.json*); in a part's `part.html`, in a text's box (`floe part guide`) |
| `markdownLine` | A line of Markdown, such as a table cell, as HTML without a paragraph around it (the item is a plain `string`) |
| `markdownLabel` | A line of Markdown as `markdownLine` writes it, its links (written or found, such as a URL) as their words: a label that goes in a link of the template's own |
| `contents` | The script that marks, in the contents just before it (`<nav data-floe-look="toc">…</nav>{{contents}}`), the link to the part of the page being read: `aria-current="location"` |
| `num` | A number with its digits grouped (`{{.Calc.total \| num}}` is `1,234,567`), or rounded to significant digits (`{{.Calc.rate \| num 3}}` is `0.00648`) |
| `bytes` | A count of bytes in a binary unit (`{{.Calc.size \| bytes}}` is `1.5 KiB`) |
| `asset` | An image of the template's `assets/` as a data URI (`<img src="{{asset "logo.svg"}}">`) |
| `word` | A word of the template's `words.yaml` in the page's language (`{{word "status_doing"}}`) |

- Write `len`, `index` and `slice` of an item the data may leave out inside `with`, as `{{with .Data.done}}{{len .}}{{else}}0{{end}}` (an item the data leaves out, a key of an object in a list too, is nil, and those three refuse nil).
- Link to ids you write, not to those floe makes (floe gives each heading, `<h1>` to `<h6>`, that has no `id` one made of its text, as `#1-what-happens`, and `-1` to the later of two of one text: the id changes with the text).
- A page drawn from a data file holds all it shows, and its scripts cannot send what it holds out: its CSP lets it load only images, fonts and media of data URIs, inline styles and scripts, Google Fonts, and libraries from cdnjs, jsDelivr and unpkg. `fetch`, XHR, an image from elsewhere and a form are blocked. The CSP guards against a slip, not a hostile template: use templates you trust.

## page.css

`page.css` lays the page out around the parts: their places, the page's width, its header.

- floe puts it in the layer `floe.page`, over floe's base, the theme's `base.css` and `markdown.css`, the looks and the layout of the rows with a mark, and under the parts' styles and the theme's files of the parts: it does not restyle a part.
- floe scopes it as `@scope (:root) to ([data-floe-part] > *, [data-floe-text] > *)`: it reaches a part's box and a text's box, so that it places them, but not what they hold, nor a part text's Markdown, which floe draws in a box of its own (`data-floe-text`).
- Write the page's variables and the rules of its root on `:scope` (`:root` and `html` are outside the scope: a rule for them applies to nothing).
- Name `@keyframes`, `@counter-style` and `@font-face` with your template's name (`memo-q`, for a template `memo`): the names are the page's, not held by the scope. Of two files that make one name the higher layer wins: a part's, or a theme's file of a part, over yours, and yours over the theme's `base.css` and `markdown.css`.
- `page.css` MUST NOT hold a `}` that closes no block, an `@import` (load a stylesheet, such as Google Fonts, with a `<link>` in `page.html`), a block, string, comment or `url()` left open at the end of the file, a string that runs past its line, an escape (`\`) outside a string, a `url()` or a comment, or a `url()` that is not one address or one quoted string alone.
- Draw with the palette's variables (`var(--muted)`), so that the page follows the theme the data file picks.

### Space

- Leave `var(--floe-room)` between parts that stand one after another (in a section, in an item: `section { display: grid; gap: var(--floe-room); }`), as every place of floe's does (a part's box has no margin of its own, and a theme's shadow reaches outside a box no further than that space).
- A part text's box (`data-floe-text`) stands there as a part does, and floe spaces its paragraphs and headings by the same space in it.
- The box of your template's own Markdown (`data-floe-markdown`) is spaced so too, but `page.css` reaches in it, as it does not in a text's.
- A rule for an element that is a part's box too (`ul { margin-block: 1rem }`, `figure { … }`) reaches the part: a part writes no margin of its own on its box (asks, checklist and links are `ul`, steps and timeline `ol`, facts, keys, entries and stats `dl`, chart, code, image, diff and terminal `figure`, tabs `form`, details `details`, cell `span`, and the others `div`).
- Do not write a list's display, marks, padding or gap, or a rule for its items (`li`, `::marker`), for a list that is a part's box: floe sets them there, and `page.css` does not reach the items in a part's box (`floe part guide`, *The CSS layers and @scope*).
- `page.css` reaches the display, the padding and the items of an element that takes the look `rows`, `table`, `cards` or `tags`. It does not reach them in a list that is a part's box.
- Do not write `display`, `gap` or `grid-template` on the items of `numbered` or `marked`, nor on the box of a `note` or of a part of yours whose `part.yaml` has `marks: box`, however it is placed (the box is a row with a mark itself: `.caution { display: flex; gap: 0.5rem; }` puts a note's title and text in one row; the rows of `links`, `timeline` and `checklist` stand in the part's box, which `page.css` does not reach) (`page.css` is over `floe.marks`, so such a rule wins over the layout of the marks: `grid-template-columns: 1fr` on a `numbered` item puts the ring and the line on the text, and `floe template check` passes). Size the marks with `--floe-mark` and `--floe-mark-gap` on the list (`.steps { --floe-mark: 3rem; --floe-mark-gap: 2rem; }`).
- Do not pad the rows of a box that takes the look `rows` (`rows` pads its rows by `--floe-row-pad` above and below, and `--floe-pad-inline` at the sides: a padding of yours replaces the look's).
- Write `overflow-x: auto` and `contain: inline-size` in one rule under `@media screen`, in a box of yours that scrolls sideways, and fold what it holds on paper (without `contain: inline-size`, the box tells its content's width outside, and a grid or flex item around it widens to its longest line, and the page with it).
- Keep such a box out of a place whose width its content sets (an inline-block, a float, an absolute or other shrink-to-fit box, a width of `min-content`, `max-content` or `fit-content`, a table's cell of auto width, `justify-self: start`, an item of a flex row on `flex-basis: auto`), where it shrinks to its border and padding (`floe part guide`, *Scrolling boxes*).
- `chart`, `keys` and `timeline`, which lay out by their own width, shrink so in such a place too.

### The column

- floe starts every page with no margin on the body. An element of the class `floe-column` (`<main class="floe-column">`) is centred in a column `--column-width` wide, within `--page-pad` each side. Both stop at the parts' boxes, like `page.css`, and `page.css` wins over them even with selectors of less specificity.
- Set `--column-width` in the template's `page.css`, to any length (`:scope { --column-width: 40rem; }`) or to the palette's `--text-width`, which `adr` and `runbook` do for a page of one column (`:scope { --column-width: var(--text-width); }`). A theme's palette that sets it widens the column of a template that sets none (`report`, `design`, `handoff` and `postmortem`), but not of a `report` with contents, whose `page.css` sets `--column-width: 100%` on screen (`@media screen { main:has(> .body) { --column-width: 100%; } }`).
- floe sets no font size, so the page's text takes the size its reader's browser sets.

These lines of `report`'s `page.css` lay out its column (`<main class="floe-column">`):

```css
main { display: grid; gap: 2.5rem; }
section > h2 { border-top: var(--rule); padding-top: 1.3rem; }
.lead { color: var(--muted); font-size: 1.05rem; }
```

### Variables a template reads or sets

A template reads or sets these. `floe part guide` lists `--floe-room`, `--floe-pad-block`, `--floe-pad-inline` and `--floe-row-pad` with their values (*Variables floe sets*), and `--floe-pill-wrap`, `--face` and `--on-face` (*Variables a part sets*); `--column-width` is the template's own (*The column*), and the palette's variables are in *The palette*.

| Variable | Set in | What it changes |
|---|---|---|
| `--floe-room` | The page's root | The space between parts that stand one after another |
| `--floe-pad-block`, `--floe-pad-inline` | The page's root | A box's padding above and below, and at its sides, and the padding at the sides of a row of the look `rows` (the look `panel` pads nothing itself) |
| `--floe-row-pad` | The page's root | The padding above and below a row of the look `rows` |
| `--floe-pill-wrap` | An element around pills, in `page.css` | `normal` lets a long pill wrap in the element. floe's base resets it on what a part's box holds |
| `--column-width` | `page.css`, on `:scope` or on the column's element | The width of the column of `.floe-column` (*The column*) |
| `--face`, `--on-face` | An element that fills itself, in `page.css` | The background and the text colour of the fill (*The palette*) |

## Parts in a template

A part draws an item of the data: `floe part list --all` lists every part, look and shared schema a `$ref` can name, with its shape, and `floe part show <part>` lists on its `files:` line the files one reads, which `floe part show <part> schema.json` prints: what one takes is a part's `schema.json` or a shared schema's `<name>.json`, and a look has none. The data file writes the part in its own shape (the part `text` is a plain string, `text: "…"`). The schema names it with `$ref`, under `properties`, `additionalProperties` (not the root's) or `items`:

```json
"fields": { "$ref": "floe:///parts/fields.json", "description": "The keys of the setting" }
```

`page.html` puts the item where it goes, `{{.Data.fields}}`: floe checks it against the part's schema and checks, its mistakes at the data file's lines and columns, and draws it as HTML. A part is drawn from an item of the data alone, in its own shape (a `checklist` takes objects, not plain texts). `page.html` has no function that draws a part: to show a computed value or plain texts as a part, write the part's box in `page.html` (`cell` and `note`, below) or borrow a look.

The part `text` is Markdown that stands among other parts, where `code` and `terminal` can stand beside it: floe warns of a fenced block of code in it, to be written in `code`. floe draws its Markdown in a box of no part (`data-floe-text`), with no background or tone of its own, so that it takes the face and tone around it; `page.css` and the theme's `base.css` stop at it, and the theme's `markdown.css` draws it as Markdown anywhere. Its `h1` is 1.2em and its `h2` 1.1em, as in a template's own Markdown. A field of plain Markdown, where no code part stands, is `{"type": "string", "format": "markdown"}` instead.

A part of any kind, one key naming its kind, refers to `any.json`; a list of them, as report's sections do, refers to it as its items. Your own parts join it (`floe part guide`). In a template's root schema:

```json
"part":  { "$ref": "floe:///parts/any.json" },
"parts": { "type": "array", "items": { "$ref": "floe:///parts/any.json" } }
```

Such a value comes to `page.html` as a map of every kind: the kind written holds the part drawn, the others nil. Range over it (`{{.Data.part}}` writes the map as text, `map[asks:<nil> bars:<nil> …]`), which does nothing for an item left out:

```html
{{range .Data.part}}{{.}}{{end}}
{{range .Data.parts}}{{range .}}{{.}}{{end}}{{end}}
{{with .Data.part}}{{with .note}}<aside>{{.}}</aside>{{end}}{{end}}
```

The last draws the part only when it is a note. Test the item before its kind, as here: `{{.Data.part.note}}` fails when the item is left out.

### Looks to borrow

An element of the template borrows a look with `data-floe-look`, drawn alike in every theme (`floe part guide`, *Borrowing a look*, tells the rules). `floe part list --all` lists them.

| Look | Put it on | What it draws |
|---|---|---|
| `panel` | A box (`<section data-floe-look="panel">`) | The box most parts are drawn in. It pads nothing itself |
| `rows` | A list | The rows of `checklist` in a panel, a rule between each |
| `table` | A `<table>` | The look of `table` |
| `cards` | The element that holds the cards | Cards side by side, each a panel, numbered by their place when the element has `class="numbered"` |
| `band` | A box | A band filled with the accent, its text, links, code and focus ring in the colour on a fill: the look paints them, and pads nothing itself |
| `numbered` | An `<ol>` | Items numbered in rings joined by a line |
| `toc` | A `<nav>` of the page's contents | A quiet list of links, the one being read in the accent |
| `tags` | An element of short values | The values spaced and wrapping, in small quiet text |
| `marked` | A `<ul>` | A letter before each item as a row's mark |

- `cards`: write each card as a child of `class="card"`, a panel by `data-floe-look="panel"`, with its title in an element of `class="title"` (`<h3 class="title">` for a heading). The cards stand side by side in a grid: write `grid-template-columns: 1fr` in `page.css`, on a class of the element, for one on a line. `class="numbered"` on the element numbers them by their place.
- Write each item's text of a `numbered` list in an element (`<li><p>Restart <code>api</code> now</p></li>`): text written straight in an item breaks, at each run of text, onto a row of its own.
- Start a `numbered` item with text (`<li><p><span>09:10</span> Restart</p></li>` does): a ring stands at the bottom of a first child with no text (an image, an empty block).
- A ring of `numbered` takes its colour from the tone (`data-floe-tone`: `accent`, `good`, `warn`, `bad` or `muted`, the item's own or around it) by `data-floe-mark`: `ring` rings it in the tone, `solid` fills it and the line from it (`<li data-floe-mark="solid" data-floe-tone="good">`); without the attribute the ring is the line's colour. Colour the rings so, not over `::before`, so that a theme that fills tones in its own colours draws them too.
- `toc` goes on `<nav data-floe-look="toc"><ul><li><a href="#one">One</a></li></ul></nav>{{contents}}`, with the script just after it that marks the link to the part being read. It shows on screen alone, not on paper. Link to the template's fixed ids.
- Where `toc` stands and when it sticks are the template's `page.css`: `floe template show report page.css` and `page.html` show the contents beside the sections on a wide screen and above them on a narrow one.
- `tags`: write each value in a `<span>` of its own (a line of Markdown, or a pill) in the element. A long pill wraps in it. A part's `looks:` takes it for the part's box: take it so for a part whose box holds its values alone.
- `marked`: `page.css` writes the letter and its colour, for the list or for an item (`.done > li::before { content: "✓"; content: "✓" / ""; color: var(--good) }`, or `.no::before { content: "×"; content: "×" / ""; … }` on `<li class="no">`), the form after the `/` telling a screen reader to read no word.
- Write each item's text of a `marked` list in one element (`<li><span>…</span></li>`).

A status the template words itself (`{{word …}}`: a state, a severity, a kind) and a caution are drawn as parts, not borrowed as looks: write a part's box in `page.html`, a pill as `data-floe-part="cell"` and a caution as `data-floe-part="note"`, and their colour by `data-floe-tone` (`<p class="status" data-floe-part="cell" data-floe-tone="good">`), as adr's, postmortem's, handoff's and runbook's are. They then take the colours of a band, a face or a tone around them as parts do, and are drawn as the theme draws its pills and notes; `page.css` places the box and does not reach in it. Write a note's text in an element (`<div data-floe-part="note"><p>{{markdownLine .}}</p></div>`, `markdownLine` for text that is Markdown), its title as `<strong class="title">`. A note written so has none of the hidden words the part writes (`floe part show note part.html`): write one as `<span class="floe-hidden">…</span>` where the tone alone would not tell a screen reader what the note is. Do not draw text a writer writes freely (a line of Markdown, a name) as a pill: it may run long, and a link in it may not read on a theme's filled pill. A pill keeps to one line, in a table's cell too; set `--floe-pill-wrap: normal` on an element around pills to let long ones wrap.

## The palette

The page's colours and faces are CSS variables that the theme's `palette.css` sets, and that the parts, the diagrams, floe's base and the template draw with. floe's `default` palette defines them all and goes into every page; the theme a data file picks sets them over it. `floe theme show default palette.css` prints them with their values.

| Variable | What it changes | Default |
|---|---|---|
| `--bg`, `--surface` | The page's background, a part's background | `#f4f3f0`, `#fff` |
| `--fg`, `--muted` | Text, faint text | `#22211f`, `#615f5a` |
| `--muted-soft` | The pale background of the tone `muted`, mixed from `--fg` and `--surface` by default; with `--muted`, the colours of that tone | `color-mix(in srgb, var(--fg) 8%, var(--surface))` |
| `--quiet` | A quiet background within a box of the surface: a table's head, a bar's track, a keycap. Mixed from `--line` and `--surface` by default | `color-mix(in srgb, var(--line) 45%, var(--surface))` |
| `--line` | Lines and borders | `#dad7d0` |
| `--accent`, `--accent-soft` | The colour that stands out, and its pale background; the colours of the tone `accent` | `#1d6670`, `#dcebec` |
| `--good`, `--warn`, `--bad`, `--good-soft`, `--warn-soft`, `--bad-soft` | Good, warning and bad, and their pale backgrounds: the colours of those tones | `#1d7542`, `#8a5a10`, `#b3261e`, `#dcefe3`, `#f6e8d2`, `#fbe5e2` |
| `--on-fill` | Text on a face filled with `--accent` or a tone's colour, such as a pill | `var(--surface)` |
| `--accent-fill`, `--good-fill`, `--warn-fill`, `--bad-fill`, `--muted-fill` | A theme's own fill for a tone (`--good-fill`), which `--tone-fill` takes; left out, the tone's `-soft` | not set |
| `--tone`, `--tone-soft`, `--tone-fill` | The colours of the tone of the element they are read in (`data-floe-tone`, `floe part guide` (*Tones*)), from those above; accent's outside a tone. A template marks a tone so too | built from the palette |
| `--shadow` | A colour for shadows; a box casts one only when its theme sets `--floe-shadow-color` on it (`floe theme guide`) | `rgb(34 33 31 / 0.12)` |
| `--floe-shadow-x`, `--floe-shadow-y`, `--floe-shadow-blur`, `--floe-shadow-spread` | The theme's one shadow, in px or rem (none by default) | not set |
| `--radius`, `--radius-small`, `--radius-pill` | How round corners are: a box's (a panel, a note, code, a quote), a small box's within one (a key, a bar, an image, code in a line), and a pill's (a cell, a tab's title; a cell that wraps rounds no more than floe's `--floe-pill-end`). `--radius-small` and `--radius-pill` MUST each be one item, a length or one function (`var()`, `calc()` and `min()` pass; two lengths or a `/` void a bar's `rx` and a pill's `min()`); give `--radius` one length too (`floe theme guide`, *Corners*) | `20px`, `6px`, `999px` |
| `--font`, `--display`, `--mono` | The text's typeface, the headings', and the fixed-width one. Name a face before `monospace` in `--mono` (a browser sizes text in `monospace` alone from 13px) | `"Hiragino Sans", "Noto Sans JP", "Noto Sans CJK JP", "Yu Gothic UI", Meiryo, system-ui, -apple-system, "Segoe UI", sans-serif`, `var(--font)`, `ui-monospace, "SFMono-Regular", Menlo, Consolas, "DejaVu Sans Mono", "Liberation Mono", monospace, "Hiragino Sans", "Noto Sans JP", "Noto Sans CJK JP"` |
| `--code-bg`, `--code-fg`, `--code-prompt` | The background of code and a terminal, always dark; their text; a terminal's prompt | `#1b2120`, `#e2e9e7`, `#b5e05a` |
| `--code-add`, `--code-del` | A diff's lines added and removed | `#6fcf97`, `#f2b8b5` |
| `--chart-1`, `--chart-2`, `--chart-3`, `--chart-4` | The colours of a chart's series, in order, at 3:1 at the least on a part's background | `#008fa3`, `#c2660a`, `#6157d9`, `#c8366f` |
| `--page-bg` | The whole page's background, a pattern such as squares too (`--bg` by default) | `var(--bg)` |
| `--rule` | The `border` of a rule across the page, such as between sections | `1px solid var(--line)` |
| `--text-width` | The width of a page of one column, which a template sets its column to by `--column-width: var(--text-width)` (as `adr` and `runbook` do), and of a note. Text itself takes the width of the column or the box it is in | `56rem` |
| `--page-width`, `--page-pad` | The column a page is centred in (`.floe-column`): its width, unless the template sets `--column-width`, and the pad each side of it, inside the column's box | `96rem`, `16px` |

floe's base draws the page's background, text colour and font on its root with them (`:root { background: var(--page-bg); color: var(--fg); font-family: var(--font); }`), which `page.css` may set otherwise on `:scope`. A theme of your own may set them too (`floe theme guide`). In print, every background is kept as the screen draws it.

- Set `--face` to the background and `--on-face` to the text colour of an element that fills itself with a colour and holds text, as a band or a filled note, and paint it with them (`background: var(--face); color: var(--on-face)`): floe's base then draws the links, a line's code (framed, on no background of its own) and the focus ring in it in `--on-face`, where the accent would sink into the fill.
- Start afresh what an element that paints a light background of its own holds, and set its text colour with the background, as a quote on a band (`.quote { background: var(--surface); color: var(--fg); }` and `.quote > * { --face: initial; --on-face: initial; }`). What a part holds starts afresh on the part's own background, and the look `panel` sets its text's colour with its background.
- Write a colour set on text of no background of its own as `var(--on-face, …)`, so that it reads in a face.

## schema.json

A JSON Schema (2020-12) of the data file's items.

- `$schema` MUST be `https://json-schema.org/draft/2020-12/schema` or the 2019-09 one, or be left out. The file MUST be one JSON value, not empty, with no key twice in an object.
- `lang`, `template`, `theme` and `title` are read by floe: the schema MUST NOT declare them under `properties` or require them. The `additionalProperties` of the root MUST NOT hold a part or an item of a format of floe's (it would draw those keys too): declare the items of the page under `properties`.
- floe reaches a part, or an item of a format of floe's (`markdown`, `html`, `image`), only by walking from the root through `properties`, `additionalProperties` and `items`, and through a `$ref` to `#/$defs/…`. The schema MUST place each one so. floe refuses one under any other keyword (`if`, `then`, `oneOf`, `allOf` and the rest), and a format of floe's beside a `$ref` to a part (a `description` beside it is fine).
- A `$ref` MUST name a part or a schema the parts share, whole (`floe:///parts/table.json`), or a place in the schema's own file (`#/$defs/row`). One out of floe (`file://`, `https://`), into a part (`floe:///parts/table.json#/properties/rows`) or to a part there is none of is refused. The root of the schema MUST NOT take an `$id`.
- Write `"additionalProperties": false` so that a misspelt key is refused: the `schema.json` that `floe template new memo` makes has none.
- To refuse a key only under a condition, list under `then` the keys the condition takes, with `"additionalProperties": false`: the refusal names the key, where `false` for the key says only `false schema`. The list copies `properties`: add a key to both.

An item of `"format": "markdown"`:

- It is Markdown text: `{{.Data.text}}` draws it as HTML, in the box of a template's own Markdown (`<div data-floe-markdown>`), as `{{markdown}}` does.
- floe's base spaces the box's blocks `--floe-room` apart, as a text's, and sizes its headings as a text's.
- `page.css` and the theme's `base.css` reach in the box, unlike a text's: name your own headings outside it (`header h1`, `section > h2`), not by the element alone (`h2 { … }`), which would reach the author's headings too.
- Placed in the box of a part that `page.html` writes (`<div data-floe-part="note">{{.Data.text}}</div>`), it is spaced `--floe-room` apart still, and `page.css` does not reach it, as it stops at the part's box.

An item of `"format": "html"`:

- It goes in as it is, a template's or a part's alike.
- It MUST NOT leave an element open, and MUST NOT close an element that the page around it opened (floe reads the page drawn the way a browser does): write each element whole. A `</div>` where the page has no `<div>` open closes nothing, and is not refused.
- `page.html`, or the `part.html` of a part holding it, puts it where elements go, not in a text or an attribute (`<title>`, `title="…"`), where HTML does not work: floe refuses it at the item.

An item of `"format": "image"` is the path of an image, relative to the data file's directory and under it (*Images*).

## words.yaml

The words the template draws itself, such as the name of a status, by language:

```yaml
en:
  status_doing: 'In progress'
ja:
  status_doing: '進行中'
```

Each key is a language tag (`ja` and `JA`, or `en-GB` and `en_gb`, are one tag: MUST NOT be written twice), each word text. `{{word "status_doing"}}` draws the word in the page's `lang:` (`ja-JP` taking `ja`), and in `en` for a language the file has no words of. A word the page's language does not have, in a file that has words of it, stops the drawing, naming `words.yaml`: give every language each word. `floe template check` draws `example.yaml` in its `lang:` alone.

A word is text alone, and the spaces of a quoted word are kept: build a sentence around a value from two words (`issue_before: 'Issue '` and `issue_after: ''`, `'第 '` and `' 号'`), not by a branch on `.Lang`.

## checks.yaml

Relations between items that a schema cannot say, each checked on every item of a list, or once on the whole data file:

```yaml
# a path of keys to a list (sections: every section)
- for: sections
  # the item's key to point at when it fails (optional)
  field: heading
  expr: 'item.heading != nil || len(item.parts ?? []) < 2'
  message: 'give a section of more than one part a heading'
```

- A list on the way of `for` is gone through item by item: `sections.parts` checks the parts of every section, and a failure is reported at its item.
- Without `for`, a check runs once, on the whole data file as `item` (`doc` too), and its failure has no line or column unless its `field` names a key the data writes.
- `expr` is an expr expression that must hold. It sees the list's item as `item` and the data file's items as `doc`. Each item it fails for is reported at its `field`, with `message` (at the item itself when `field` is left out or the item lacks it).
- The path of `for` MUST be one the schema declares; `field` is not checked against it.
- A check runs only on data that passed `schema.json`: to refuse a missing key with a message of your own, leave it out of `required` and write the check here.
- An expression with a ` #` in it (expr's `#` in `filter(xs, # > 1)`) MUST be in quotes: floe refuses a `#` that begins a YAML comment where it may have cut a value short.

## computed.yaml

Values computed from the data, for `page.html` to show as `.Calc.<name>`:

```yaml
item_bytes: 'sum(attributes, .avg_bytes)'
storage_gb: 'item_bytes * records.in_1_year / 1e9'
```

- An expression sees the data file's items that the schema declares, and the other computed values, by name, whatever order they are written in. It is an expr expression, as in `checks.yaml` (`len(teams)`, `sum(teams, .shipped)`, `shipped > 0 ? incidents * 100 / shipped : 0`), and MUST NOT use `$env`: use the names.
- A name MUST be one that expr and `page.html` read as one name, MUST NOT be an item of the data, a word or function of expr, or start with `_`, and values computed from each other are refused.
- A value the data cannot give (a division by zero, an item left out that the expression needs), or one that is not a finite number, is reported at its expression, naming the data file. Give the value for an item left out with `??` (`count(prs ?? [], .state == "merged")`), and read into one with `?.` (`records?.in_1_year ?? 0`).

## example.yaml

A data file of the template, to start one from: `floe template show <template> example.yaml` prints it, and `floe template check` draws it as `floe check` draws a data file. In a template of your own, write every item the template takes, so that the check reaches every part of `page.html`. floe's own examples are kept short for agents to read: report's leaves out `toc`, and draws no subsection of a section without a heading in its contents.

## Images

An image is one of the data (an item of `"format": "image"`) or one of `assets/`.

- An image MUST be a png, jpeg, gif, webp or svg file, by its name, and at most 1 MiB.
- An image of the data MUST be named by its path, relative to the data file's directory and under it (`floe part show image schema.json`).
- An image of `assets/` MUST be named by its path in `assets/`, with one `/` between names. The path MUST NOT hold `./` or `..`, and MUST NOT go under a name starting with a dot (floe does not read one: `assets/ has no ".x/a.png"`).
- `assets/` MUST hold images alone: floe refuses any other file in it.
- `floe template check` reads every image of `assets/`.
- The images `example.yaml` names are read from the template's directory: keep them in `assets/` and name them by that path (`logo: assets/logo.svg`), as an image of another name right in the directory is refused. A data file started from the example needs them beside it.

## What `floe template check` refuses

`floe template check` reads every template of yours and the parts, looks, shared schemas and themes of yours, as `floe part check` does, and draws each template's `example.yaml` as `floe check` draws a data file. It prints every mistake and exits with code 1 when there is one. For a template it refuses, by kind:

- a name, a link, a file or a directory that breaks the rules of *Where templates are* and *Files*, a missing `page.html`, `schema.json` or `example.yaml`, and an `example.yaml` that names another template;
- a `page.html` that does not parse (a syntax mistake, an unknown function), and what html/template's escaping refuses (`ZgotmplZ` in the page drawn, where it did not take a value);
- a `schema.json` that breaks a rule of *schema.json*, and a mistake the JSON Schema library reports;
- a `checks.yaml` that is not a list of checks of `expr` and `message` (and `for` and `field`), a check with an empty or missing part, and a `for` that goes through a key the schema does not declare or leads to no list in the data; a `computed.yaml` that breaks a rule of *computed.yaml*, `$env` in a `computed.yaml` expression, and an `expr` that does not parse (the expr library's words);
- a `words.yaml` that is not language tags each with names and their text, a word that is not text, two language tags that are one, and a word `page.html` names that the example's language lacks;
- an image of `assets/` that is not one floe takes, and an `asset` that names none;
- CSS floe cannot read in `page.css` (*page.css*);
- a `--radius-small` or `--radius-pill` of more than one item in `page.css`, or in a theme or part of yours;
- a file floe cannot read as written, in any file: YAML or JSON that is not UTF-8 or holds a byte order mark inside, JSON that holds a lone surrogate, and YAML that floe would read otherwise than YAML 1.2 does (a `#` that cuts a value short, `{a, b: 1}`, a number read differently); YAML that floe does not take: anchors, aliases, merge keys, tags, and a second document;
- the page drawn from `example.yaml`: what the data file's items break in the schema and the checks, what *page.html* says the page drawn MUST NOT hold, an item of format `markdown`, `html` or `image` that is not text (a number, `true` or `null` unquoted), an item of format `html` that opens or closes an element of the page or stands in a text or an attribute.

A data file is checked in this order, each stage only on data that passed the one before (`floe check` runs them all):

1. The parts, looks, shared schemas, themes and templates of yours are read first.
2. The data file's YAML, and the template and theme it names.
3. The template's `schema.json`.
4. The template's `checks.yaml`, the `checks.yaml` of each part for each item it draws, and the template's `computed.yaml`, together.
5. The page drawn: each part and item of a format (images, diagrams, HTML), then `page.html`, and the page drawn as a whole.

The check tests files, names and syntax, and of values those two corners alone, not places. A page without `<!doctype html>`, a `page.css` rule that restyles nothing or breaks a layout, a word missing in a language the example does not use, and a `schema.json` without `"additionalProperties": false` pass with exit code 0.
