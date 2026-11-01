# Templates

How to make a template of your own: the page a data file is drawn into. A data file names its template with `template:` ([pages.md](pages.md) tells how to write one); floe's templates are `report`, `design`, `handoff`, `adr`, `postmortem`, `runbook` and `slides` (`floe template list` lists them). To make a part or a theme of your own, see [parts.md](parts.md).

## Where templates are

A template of your own is a directory in `FLOE_TEMPLATES_DIR` (default `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`), named after the template. One of the name of floe's template hides it. A file right in `FLOE_TEMPLATES_DIR` is not read, and nothing says so: put a template's files in its directory.

```sh
floe template new memo     # makes a template, memo, to fill in
floe template check        # checks every template of your own, drawing its example.yaml
floe template show memo    # prints its themes and every file it reads, its schema and example first
```

A mistake of a template, part, look, shared schema or theme of your own stops every page, whether it is drawn with that one or not (a page of `report` too), until it is fixed: `floe check`, `floe render`, and the server's publishes and linked pages alike. `floe part check` and `floe template check` list them all. A published page is drawn with the server's templates ([commands.md](commands.md#the-server)); after you change one, `floe rebuild` draws the stored pages again with it.

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

- `floe template check` and `floe render` draw without the server. Do not `floe publish` while the variable is set: a server running refuses it, as it draws with the directories it was started with, and with none running, the publish starts one on the other directory.
- Make `~/.config/floe/templates` before the `mv`: with none there, `mv` would rename `memo` to `templates` itself, its files right in the directory of templates, where floe reads none and says nothing until a page names `memo`.
- Keep the other directory on the file system of your templates, so that `mv` puts the template in at once rather than a copy the server reads half made.
- To change a template you have, copy it to the other directory, change and check it there, and move it back in place of the old one.
- Parts are made the same way, with `FLOE_PARTS_DIR`, `floe part new` and `floe part check`. A theme has no `new`: make its directory in `FLOE_THEMES_DIR` by hand ([parts.md](parts.md#themes)) and check it with `floe part check`.

## Files

```text
memo/
├── page.html        # the page, a Go html/template
├── schema.json      # the items a data file may hold (JSON Schema)
├── example.yaml     # a data file to start from, which floe template check draws
├── page.css         # the page's CSS, around the parts (optional)
├── checks.yaml      # relations between items (optional)
├── computed.yaml    # values computed from the data (optional)
├── words.yaml       # the template's words, by language (optional)
└── assets/          # images page.html shows with asset (optional)
```

A file of another name is refused. `example.yaml` names the template of its directory (`template: memo`).

## page.html

`page.html` is a Go [html/template](https://pkg.go.dev/html/template), which escapes what it inserts. `floe template new` writes this one, the smallest floe takes with a viewport `<meta>`: without the viewport, a phone draws a page opened by itself 980px wide and shrinks it. You need not write a charset: a page opened without floe's server (`floe render`, a download) gets one from floe, first in its head, and the server sends it in its header.

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

It sees:

| Name | What it is |
|---|---|
| `.Title` | The page's title: the data file's `title:`, else the slug |
| `.Lang` | The page's language: the data file's `lang:` |
| `.Data` | The data file's items, without `lang`, `template`, `theme` and `title`, which floe reads itself. An item the schema declares but the data leaves out is nil: `{{with .Data.lead}}`, `{{range}}` and `{{if}}` read it as empty, but `len`, `index` and `slice` refuse it (`len of nil pointer`), so write them inside `with`: `{{with .Data.done}}{{len .}}{{else}}0{{end}}`. A part comes drawn (see [Parts in a template](#parts-in-a-template)), as does an item of `"format": "markdown"` (its HTML) or `"format": "image"` (a data URI) |
| `.Calc` | The values of `computed.yaml`, by name |
| `markdown` | Markdown text as HTML (`{{markdown .text}}`), raw HTML in it left out |
| `markdownLine` | A line of Markdown, such as a table cell, as HTML without a paragraph around it |
| `num` | A number with its digits grouped (`{{.Calc.total \| num}}` is `1,234,567`), or rounded to significant digits (`{{.Calc.rate \| num 3}}` is `0.00648`) |
| `bytes` | A count of bytes in a binary unit (`{{.Calc.size \| bytes}}` is `1.5 KiB`) |
| `asset` | An image of the template's `assets/` as a data URI (`<img src="{{asset "logo.svg"}}">`) |
| `word` | A word of the template's [`words.yaml`](#wordsyaml) in the page's language (`{{word "status_doing"}}`) |

- The page starts with `<!doctype html>` and holds a `</head>`: floe puts the page's CSS before it.
- The page drawn holds no `<style>` (write CSS in `page.css`), no `<base>`, no `<noscript>`, and no `<meta>` but for its charset and viewport, whoever writes them: `page.html`, a part or an item of the data. They are refused at the data file's `template:`, as the page is drawn.
- Each heading of the page drawn (`<h1>` to `<h6>`) without an `id` gets one made of its text, for links to it (`#1-what-happens`).
- A page drawn from a data file holds all it shows, and its scripts cannot send what it holds out: its CSP lets it load only images, fonts and media of data URIs, inline styles and scripts, Google Fonts, and libraries from cdnjs, jsDelivr and unpkg. `fetch`, XHR, an image from elsewhere and a form are blocked. CSP guards against a slip, not a hostile template: use templates you trust.

## page.css

`page.css` lays the page out around the parts: their places, the page's width, its header. It goes into a layer under the parts' styles and the theme's files of them, so it does not restyle a part, and over the theme's `base.css`.

- floe puts it in `@scope (:root) to ([data-floe-part] > *)`: it reaches a part's box, so that it places the part, but not what the part holds.
- Its root is `:scope`. Write the page's variables on `:scope`, not on `:root` or `html`, which are outside the scope.
- A `}` that closes no block, and an `@import`, are refused at their line: load a stylesheet, such as Google Fonts, with a `<link>` in `page.html`.
- Draw with the palette's variables (`var(--muted)`), so that the page follows the theme the data file picks.
- Leave `var(--floe-room)` (0.75rem) between parts that stand one after another (in a section, in an item), as every place of floe's does, the most a theme's shadow reaches outside a box. The paragraphs and headings of a part text stand there as parts too.
- A rule for an element that is a part's box too (`ul { margin-block: 1rem }`, `figure { … }`) reaches the part, as a part writes no margin of its own on its box (checklist and links are `ul`, steps and timeline `ol`, facts, keys, entries and stats `dl`, code, image, diff and terminal `figure`, tabs `form`). It does not change a list's display, marks, padding or room between items, which floe sets for a list that is a part's box ([parts.md](parts.md#the-css-layers-and-scope)); a list that takes a look takes those from the template's `page.css` too, so that the template adjusts it.

`report`'s `page.css`, for one, centres its page in the column every floe template uses:

```css
main {
  max-width: calc(var(--page-width) + 2 * var(--page-pad)); margin-inline: auto; padding-inline: var(--page-pad);
  display: grid; gap: 2.5rem;
}
section > h2 { border-top: var(--rule); padding-top: 1.3rem; }
.lead { color: var(--muted); font-size: 1.05rem; }
```

## Parts in a template

A part draws an item of the data: `floe part list` lists them, and `floe part show <name>` prints what one takes. The schema names it with `$ref`, under `properties` or `items`:

```json
"findings": { "$ref": "floe:///parts/findings.json", "description": "What the review found" }
```

`page.html` puts the item where it goes, `{{.Data.findings}}`: floe checks it against the part's schema and checks, its mistakes at the data file's lines and columns, and draws it as HTML.

The part `text` is Markdown that stands among other parts, where `code` and `terminal` can stand beside it: floe warns of a fenced block of code in it, to be written in `code`. A field of plain Markdown, where no code part stands, is `{"type": "string", "format": "markdown"}` instead.

A part of any kind, one key naming its kind, refers to `any.json`; a list of them, as report's sections do, refers to it as its items. In a template's root schema:

```json
"part":  { "$ref": "floe:///parts/any.json" },
"parts": { "type": "array", "items": { "$ref": "floe:///parts/any.json" } }
```

Such a value comes to `page.html` as a map of every kind: the kind written holds the part drawn, the others nil. `{{.Data.part}}` writes the map as text (`map[bars:<nil> …]`): range over it instead, which does nothing for an item left out:

```html
{{range .Data.part}}{{.}}{{end}}
{{range .Data.parts}}{{range .}}{{.}}{{end}}{{end}}
{{with .Data.part}}{{with .note}}<aside>{{.}}</aside>{{end}}{{end}}
```

The last draws the part only when it is a note. Test the item before its kind, as here: `{{.Data.part.note}}` fails when the item is left out.

### Looks to borrow

An element of the template borrows a look by `data-floe-look`, drawn alike in every theme ([parts.md](parts.md#borrowing-a-look) tells the rules):

- `panel`: the box most parts are drawn in (`<section data-floe-look="panel">`).
- `rows`, `table`: the looks of `checklist` and of `table`.
- `cards`: its children of `class="card"` side by side, each a panel by `data-floe-look="panel"` with its title in an element of `class="title"` (`<h3 class="title">` for a heading); numbered by their place with `class="numbered"`, as `options` is.
- `band`: a band filled with the accent, its text, links, code and focus ring in the colour on a fill, as design's conclusion and handoff's head are.
- `numbered`: on an `<ol>`, its items numbered in rings joined by a line, as the part `steps` and runbook's steps are. Write each item's text in an element (`<li><p>Restart <code>api</code> now</p></li>`), as text written straight in an item breaks into a grid item a run, and start the item with text, as a ring stands at the bottom of a first child with no text (an image, an empty block).

A status the template words itself (`{{word …}}`: a state, a severity, a kind) and a caution are drawn as parts, not borrowed as looks: the template writes a part's box in `page.html`, a pill by `data-floe-part="cell"` and a caution by `data-floe-part="note"`, their colour by `data-floe-tone` (`<p class="status" data-floe-part="cell" data-floe-tone="good">`), as adr's, postmortem's, handoff's and runbook's are. They then stand in a band, a face and a tone as parts do, drawn as the theme draws its pills and notes; `page.css` places the box and does not reach in it. Write a note's text in an element (`<div data-floe-part="note"><p>…</p></div>`), its title as `<strong class="title">`. Text a writer writes freely (a line of Markdown, a name) is no pill: it may run long, and a link in it may not read on a theme's filled pill.

## The palette

The page's colours and faces are CSS variables that the theme's `palette.css` sets, and that the parts, the diagrams, floe's base and the template draw with. floe's `default` palette defines them all and goes into every page; the theme a data file picks sets them over it.

| Variable | What it is |
| --- | --- |
| `--bg`, `--surface` | The page's ground, a part's face |
| `--fg`, `--muted` | Text, faint text |
| `--muted-soft` | The pale ground of the tone `muted`, mixed from `--fg` and `--surface` by default; with `--muted`, the colours of that tone |
| `--quiet` | A quiet ground within a box of the surface: a table's head, a bar's track, progress's, a keycap. Mixed from `--line` and `--surface` by default |
| `--line` | Lines and borders |
| `--accent`, `--accent-soft` | The colour that stands out, and its pale ground; the colours of the tone `accent` |
| `--good`, `--warn`, `--bad` and their `-soft` | Good, warning and bad, and their pale grounds: the colours of those tones |
| `--on-fill` | Text on a face filled with `--accent` or a tone's colour, such as a pill |
| `--<tone>-fill` | A theme's own fill for a tone (`--good-fill`), which `--tone-fill` takes; left out, the tone's `-soft` |
| `--tone`, `--tone-soft`, `--tone-fill` | The colours of the tone of the element they are read in (`data-floe-tone`, [parts.md](parts.md#the-css-layers-and-scope)), from those above; accent's outside a tone. A template marks a tone so too |
| `--shadow` | A colour for shadows; a box casts one only when its theme sets `--floe-shadow-color` on it ([parts.md](parts.md#themes)) |
| `--floe-shadow-x`, `-y`, `-blur`, `-spread` | The theme's one shadow, in px or rem (none by default) |
| `--radius`, `--radius-small`, `--radius-pill` | How round corners are, one length each: a box's (a panel, a note, code, a quote), a small box's within one (a key, a bar, an image, code in a line), and a pill's (a cell, a tab's title). A box along the edge of another, a gap `p` within it, rounds by `calc(var(--radius) - p)` |
| `--font`, `--display`, `--mono` | The text's typeface, the headings', and the fixed-width one |
| `--code-bg`, `--code-fg`, `--code-prompt` | The ground of code and a terminal, always dark; their text; a terminal's prompt |
| `--code-add`, `--code-del` | A diff's lines added and removed |
| `--page-bg` | The whole page's ground, a pattern such as squares too (`--bg` by default) |
| `--rule` | The `border` of a rule across the page, such as between sections |
| `--text-width` | The width of a page of one column, from which a template sets its column (as `adr` and `runbook` do), and of a note. Text itself takes the width of the column or the box it is in |
| `--page-width`・`--page-pad` | The column a page is centred in: its width, and the pad each side of it. As the pad is inside the box, the column is `max-width: calc(var(--page-width) + 2 * var(--page-pad)); margin-inline: auto; padding-inline: var(--page-pad)` |

floe's base draws the page's ground, colour and face on its root with them (`:root { background: var(--page-bg); color: var(--fg); font-family: var(--font); }`), which `page.css` may set otherwise on `:scope`. A theme of your own may set them too ([parts.md](parts.md#themes)). In print, every ground is kept as the screen draws it.

An element that fills itself with a colour and holds text, as a band or a filled note, sets `--face` to its ground and `--on-face` to its text's colour, and paints itself with them (`background: var(--face); color: var(--on-face)`): floe's base then draws the links, a line's code (framed, on no ground of its own) and the focus ring in it in `--on-face`, where the accent would sink into the fill. What a part holds starts afresh, on the part's own ground. An element that paints a light ground of its own, as a quote on a panel, starts afresh what it holds too (`blockquote > * { --face: initial; --on-face: initial; }`), and a colour set on no ground of its own is written `var(--on-face, …)`, so as to read in a face.

## schema.json

A [JSON Schema](https://json-schema.org/) (2020-12) of the data file's items. Write `"additionalProperties": false` so that a misspelt key is refused: the `schema.json` that `floe template new` makes has none, so add it yourself.

- `lang`, `template`, `theme` and `title` are read by floe and are not declared, nor required.
- A `format` that tells floe how to draw a text: `markdown`, `html` or `image`. Declare it under `properties` or `items`, where floe follows.
- A `$ref` names a part or a schema the parts share, whole (`floe:///parts/table.json`), under `properties` or `items`, or a place in the schema's own file (`#/$defs/row`). One out of floe (`file://`, `https://`) or into a part is refused. The root of the schema takes no `$id`.

An item of `"format": "markdown"` is Markdown text: `{{.Data.text}}` draws it as HTML.

An item of `"format": "html"` goes in as it is, a template's or a part's alike. It leaves no element open nor closes one of the page around it, as a browser reads the page drawn, refused at the item: write each element whole. `page.html`, or the `part.html` of a part holding it, puts it where elements go, not in a text or an attribute (`<title>`, `title="…"`), where HTML does not work.

An item of `"format": "image"` is the path of an image, relative to the data file's directory and under it ([Images](#images)).

## words.yaml

The words the template draws itself, such as the name of a status, by language:

```yaml
en:
  status_doing: In progress
ja:
  status_doing: 進行中
```

Each key is a language tag. `{{word "status_doing"}}` draws the word in the page's `lang:` (`ja-JP` taking `ja`), and in `en` for a language the file has no words of. A word the language does not have stops the drawing, naming `words.yaml`.

## checks.yaml

Relations between items that a schema cannot say, each checked on every item of a list, or once on the whole data file:

```yaml
# a path of keys to a list (sections: every section)
- for: sections
  # the item's key to point at when it fails (optional)
  field: heading
  expr: item.heading != nil || len(item.parts ?? []) < 2
  message: give a section of more than one part a heading
```

- A list on the way of `for` is gone through item by item: `sections.parts` checks the parts of every section, and a failure is reported at its item.
- Without `for`, a check runs once, on the whole data file as `item`.
- `expr` is an [expr](https://expr-lang.org/) expression that must hold. It sees the list's item as `item` and the data file's items as `doc`. Each item it fails for is reported at its `field`, with `message`.
- The path of `for` must be one the schema declares; `field` is not checked against it.
- An expression with a ` #` in it, as expr's `#` in `filter(xs, # > 1)`, goes in quotes: YAML would read a comment from there.

## computed.yaml

Values computed from the data, for `page.html` to show as `.Calc.<name>`:

```yaml
item_bytes: sum(attributes, .avg_bytes)
storage_gb: item_bytes * records.in_1_year / 1e9
```

- An expression sees the data file's items that the schema declares, and the other computed values, by name, whatever order they are written in.
- A name is one expr and `page.html` read as one name, and is neither one of the data's items nor a word or function of expr. Values computed from each other are refused.
- A value the data cannot give (a division by zero, an item left out that the expression needs), or one that is not a finite number, is reported at its expression, naming the data file. Give the value for an item left out with `??` (`count(prs ?? [], .state == "merged")`), and read into one with `?.` (`records?.in_1_year ?? 0`).

## example.yaml

A data file of the template, to start one from: `floe template show` prints it, and `floe template check` draws it as `floe check` draws a data file. Write every item the template takes, so that the check reaches every part of `page.html`.

## Images

An image, of the data or of `assets/`, is a png, jpeg, gif, webp or svg file, by its name, up to 1 MiB. An image of the data is named as [pages.md](pages.md#images) tells; one of `assets/` by its path in `assets/`, with one `/` between names, without `./` or `..`, and never under a name starting with a dot, which floe does not read (`assets/ has no ".x/a.png"`).

- `floe template check` reads every image of `assets/`.
- The images `example.yaml` names are read from the template's directory: a data file started from the example needs them beside it.

## The order of the checks

A data file is checked in this order, and each stage runs only on data that passed the one before. `floe check` runs them all.

The parts, looks, shared schemas, themes and templates of your own are read first ([Where templates are](#where-templates-are)).

1. Its YAML, and the template and theme it names
2. The template's `schema.json`
3. The template's `checks.yaml`, the `checks.yaml` of each part for each item it draws, and the template's `computed.yaml`, together
4. The page drawn: each part and item of a format (images, diagrams, HTML), then `page.html`, and the page drawn as a whole
