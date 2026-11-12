# Parts

How to make a part, a look or a shared schema of your own. A part draws an item of a data file. To use parts in a template, see `floe template guide` (*Parts in a template*); to write them in a data file, see `floe guide`.

## Where parts are

- Parts, looks and shared schemas of your own are in `FLOE_PARTS_DIR` (default `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`): a part or a look is a directory, a shared schema a file `<name>.json` beside them. Another file right there is not read, and nothing says so: put a part's files in its directory.
- A name is lower case letters, digits, `-` and `_`, starting with a letter or a digit. A name floe's parts, looks or shared schemas have, or a key a section of floe's templates takes (`html`), is refused: yours sit beside floe's, and never take their place. A part or look is not named `base` or `palette`, the names of a theme's files for every page.
- A directory in them may not be a symlink; a file may.

```sh
floe part new quote     # makes the smallest part, quote, to fill in
floe part check         # checks every part, look, shared schema and theme of your own
floe part show quote    # lists its looks and the files it reads
```

A mistake of one stops every page, as `floe template guide` (*Where templates are*) tells: `floe part check` lists them. While you write one, make it in another directory and move it in once it is checked, as `floe template guide` (*Make one in another directory*) tells: set `FLOE_PARTS_DIR` to it and run `floe part check`.

## A part's files

```text
quote/
├── schema.json    # what a data file may write of it (JSON Schema)
├── part.html      # how it is drawn, a Go html/template
├── part.css       # its style (may be empty)
├── part.yaml      # the looks it takes, its rows with a mark (optional)
├── checks.yaml    # relations between its items (optional), as a template's
└── words.yaml     # its words, by language (optional), as a template's
```

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption>{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

- A schema names it as floe's are named, `"$ref": "floe:///parts/quote.json"`; its own schema names floe's parts and the shared schemas so too.
- Your own parts join `any.json`, so a section of floe's templates (report's `parts`) takes them as it takes floe's, and so does a schema of your own that names `any.json`. They do not join `stack`, what `details`, `tabs`, `columns`, `options`, `entries` and `checks` hold: a part of your own may hold a form or HTML, which those would break on. `floe part list` lists them with floe's; `floe part show any any.json` prints floe's `any.json` as it is written.
- Its element is its box, which names it: `data-floe-part="quote"`.
- It draws no heading (`<h1>` to `<h6>`): a part knows not the level it is put at, in a template's section, in `details` or in another part. A title is an element of `class="title"` (`<div class="title">`), which the look `cards` draws as a card's title; the headings are the template's and those its Markdown draws. floe does not check it.
- `part.html` sees the item as `.`, and the functions of a template's `page.html` but `asset` (`floe template guide`, *page.html*). Its `markdown` draws the Markdown in a text's box (`<div data-floe-text>`), as an item of format `markdown` in the part's value is drawn: floe spaces the box's blocks `--floe-room` apart, and the part spaces the box alone, writing no room between the blocks (`:scope p` reaches into the box, `:scope > p` does not; in the quote above, the blockquote holds the box). Empty Markdown draws no box. An item its schema declares but the value leaves out is nil, as in `page.html`'s `.Data`. A mistake of running `part.html` (a key its schema does not declare, a value it cannot range over) is told at its line and column in `part.html`, not at the value that brought it: give the values their shape in the part's schema, so that a value of another shape is told at it.
- `part.html` has the functions floe's parts draw with too ([Functions of part.html](#functions-of-parthtml)).
- Its schema takes floe's `format`s as a template's does (`html`, `markdown`, `image`), declared under `properties`, `additionalProperties` or `items`, where floe follows, or as the root, and a shared schema that holds them. Such an item comes to `part.html` drawn: `html` as written, `markdown` as HTML, `image` as a data URI (`<img src="{{.file}}" alt="{{.alt}}">`). Write it as `{{.h}}`: do not pass it to a function (`markdown`, `markdownLine`, `printf`, `slice`, `eq`, …) nor put it in a `<script>`, as it is HTML, not the text written, and does not work there. An item of `html` is checked in the page drawn as a template's is (`floe template guide`). A plain string drawn as Markdown is `{{markdown .text}}`. A value of `any.json` does not come drawn as one item but as a map of every kind ([Looks and shared schemas](#looks-and-shared-schemas)).
- A part of rows one under another takes the look `rows` (`looks: [rows]`) and writes no padding of its rows, which `rows` gives (`--floe-row-pad` above and below, `--floe-pad-inline` at the sides); rows that cannot take it read `var(--floe-row-pad)`. [Borrowing a look](#borrowing-a-look) tells more.
- `floe part show <name>` lists on a `files:` line the files one reads, floe's too; `floe part show <name> <file>` prints one of them: copy from floe's parts what fits.

## Functions of part.html

It has the functions floe's parts draw with too. floe's parts show them in their `part.html`, which `floe part show <name> part.html` prints (`floe part show code part.html`).

### `copyButton`

`{{copyButton}}` draws the copy button, in the page's language. Put it right under the part's box (`<figure data-floe-part="quote">…{{copyButton}}</figure>`): what it copies is found from the button up, by the nearest box of a part, so that in the box of a part inside yours it copies that part's, not yours. It copies the text of each element of the part marked `data-floe-copy`, each ending in a line break (code marks its `<code>`; a terminal, each command). Its look is floe's; the part's CSS places it and keeps text clear of it, as code's does: `isolation: isolate` on the part's box, so that the button stays under the page outside; `margin: var(--floe-copy-gap)` around it; and, where text lies under it, `padding-inline-end: var(--floe-copy-room)` after the longest line, under `:scope:has(> .floe-copy)`, so that a line scrolled to its end ends left of the button. floe sets `--floe-copy-size` (the button's size), `--floe-copy-gap` and `--floe-copy-room` on the page's root, and applies none of them to a part.

### `diagram`

`{{diagram}}` takes nothing and draws the part's whole value as a diagram. The value is a spec as the part `diagram` takes it, nothing else: a key beside the spec (a `caption`) is refused by the diagram's schema.

### `zoom`

`{{zoom}}` draws the script that zooms, the one the parts `image` and `diagram` use. Put it in the part's box, before a frame of `class="floe-zoom"`, and take the look `zoom` in `part.yaml` (`looks: [zoom]`; `<figure data-floe-part="shot">{{zoom}}<div class="floe-zoom"><img src="{{.file}}" alt="{{.alt}}"></div></figure>`). The frame's first element is what it zooms, an `svg` or an `img` itself (an element that holds one is not moved): an `svg` by its `viewBox`, an `img` by a `transform`, its size taken from the `viewBox` or the image. The look `zoom` lays an `img` out shrunk to the frame's width (`display: block; max-width: 100%; height: auto`), so the example above, copied as it is, zooms. While zoomed, the image's line is dropped. Only while what it holds is drawn smaller than its actual size, or zoomed, is there a toolbar before the frame and a tab stop on it. The part's CSS sizes the frame. The script goes into each part that uses it, and however many times a page holds it, it sets up each frame once. A frame written with a `title` keeps it, in place of the tooltip that tells how to zoom; a `title` of an element in the frame wins over the frame's while the pointer is over that element. The script puts the line that tells how to zoom as the frame's next element, shown only while the keyboard is on the frame: so a sibling selector such as `.floe-zoom + p` no longer matches, and in a grid or a flex holding the frame directly the line is a new item while it shows. Put the frame in a box of its own there.

### `script`

`{{script .js .data}}` draws an element, and a script that calls `.js`, a function, with it and `.data`, as the part `script` does.

### `diff`

`{{$d := diff .text}}` reads a diff as `git diff` writes it: `$d.Lines`, each with `.Kind` and `.Text`; and `$d.Added` and `$d.Removed`, how many `add` and `del` lines it has. `.Kind` is `hunk` for a line starting with `@@`; `meta` for one starting with `\` (in a hunk too), for one starting with `--- ` or `+++ ` outside a hunk, and for any other line not starting with `+`, `-` or a space (`diff --git`, `index`), which also ends a hunk; `add` and `del` for the other lines starting with `+` and `-`, outside a hunk too; `""` for a line starting with a space, or empty.

### `code`

`{{$c := code .text .start .mark}}` cuts text into lines, as the part `code` does: `$c.Lines`, each with `.Text` (its line break kept) and `.Mark`, whether `.mark`, a list of line numbers or nothing, names it. `.start`, a line number or nothing, is the number of the first line: written, `$c.Numbered` is true, and `$c.Before` is the count before the first line and `$c.Digits` the digits of the last line's number. A line number is a whole number, and anything else is refused; its range is for your schema to set, as the part `code`'s does (from 1 to 1,000,000,000). A number no line has marks nothing.

### `bars`

`{{range bars .}}` reads a list of items, each written as keys and values: `label`, `value`, `of`, `unit` and `tone`. Each comes as `.Label`, `.Tone`, `.Unit` (empty when not written), `.Value` and `.Of` as written (`.Of` empty without one), `.Now` and `.Max`, its value and of in plain digits, without grouping or an exponent (for `aria-valuenow` and `aria-valuemax`; `num` writes them grouped), and `.Width`: for an item with `of`, its value as a percentage of its `of`; for the others, as a percentage of the largest value among them, which the items with `of` are left out of. `bars` reads an `of` of null as one not written; a part's schema that declares `of` a number refuses null before. An item not written as keys and values is a mistake, told with which item it is; so is a `value` that is not a number or is below 0, told with the value, one not written or null, told as missing, an `of` that is not a number or not above 0, a `value` over its `of`, and a `unit` that is not text, each told with its bar's number in the list and its label. The mistakes of every item are told together, at the part's value. To have a mistake of a value's shape told at the item or the value itself, give the list in the part's schema the items `bars` takes, as the part `bars` does (`minimum` alone lets a value that is not a number through to `bars`):

```json
"items": {
  "type": "object",
  "properties": {
    "label": {"type": "string", "minLength": 1},
    "value": {"type": "number", "minimum": 0},
    "of": {"type": "number", "exclusiveMinimum": 0},
    "unit": {"type": "string", "minLength": 1},
    "tone": {"$ref": "floe:///parts/tone.json"}
  },
  "required": ["label", "value"],
  "additionalProperties": false
}
```

Put it in the schema of the list `part.html` gives to `bars`: for `bars .items`, under `properties.items`, as `properties.items.items`; at the root it checks nothing of the list.

### `stats`

`{{range stats .}}` reads a list of items, each written as keys and values: `label`, `value`, `unit`, `note`, `tone`, `from` and `better`. Each comes as `.Label`, `.Value` as written, `.Unit`, `.Note`, `.Tone` and `.From`; `.Way`, `better`, `worse` or `same`, and `.WayTone`, `good`, `bad` or `muted`, empty without `from`; and `.Change`, `value` less `from`, signed and exact, to the decimals of the longer of the two, empty when they are the same. With `from`, `from` and `value` are text of digits alone (`^-?[0-9]+([.][0-9]+)?$`) and `better` is `lower` or `higher`: the function refuses others at the part's value, naming the item by its number and label. The other keys are for the part's schema to say.

### `chart`

`{{$c := chart .}}` counts the places of the part `chart` from the part's whole value, for its `part.html` to write the SVG. It is for that part: its fields, and the height, text and class they assume, are not promised. It refuses a value it cannot place: `kind` other than `line` or `bar`, an empty `x` or `series`, a number of `values` other than that of `x`, or a value or `min` not a number. A number of `values` and a value or `min` not a number are told where they are written in the part's value, the others at the part's value.

### `written`

`{{written}}` returns the part's value as written, for `part.html` to compare it, or to draw a word of it as text, as a pill's (not as Markdown or HTML: draw those items from `.`): the parts and the formats in it are not drawn, so it may go to `eq` and `index`, and a key the value leaves out is not there, not nil (`{{(written).note}}` is a mistake when `note` is left out; take it with `index` or `with`). floe's `table` matches the cells of a column's `pills` so.

### `tree`

`{{range tree .}}` reads a list of items of `path` (a folder's ending in `/`), `note` and `change`, and nests them into the folders the paths name, a folder not listed of its own too: each comes as `.Name`, `.Note`, `.Change`, `.Dir` (a folder) and `.Children`.

## Looks and shared schemas

- A look alone is a directory of a `part.css` (and a `part.yaml` with its `description:`, the looks it takes and its `marks:`), without a schema: it draws nothing, and lends its style to parts and elements that take it. floe's looks alone are `panel`, `rows`, `cards`, `band`, `numbered`, `marked`, `tags` and `zoom` (`floe template guide` (*Looks to borrow*) tells what each but `zoom` draws; `zoom` is the frame that zooms and its toolbar, which a part takes with the function [`zoom`](#zoom)).
- A part takes the looks of others by its `part.yaml`, `looks: [panel]`, and the looks those take in turn, each before it, which it adjusts.
- A part's or look's `part.yaml` tells by `marks:` which of its elements are rows with a mark at their left, which floe lays out: `box` (a part's box; a look's element that takes it, and the box of a part that takes it), `items` (each `li` child of that, alone: a row of `div`s takes no mark) or `joined` (`items`, a line running from each mark to the next). One of a part or look and the looks it takes, theirs too, writes `marks:`; `floe part check` refuses a second. A look that takes one (`looks: [numbered]`) makes rows of the elements that take it. Do not give one element two of them in HTML (`data-floe-look="numbered marked"`, or a box of `links` that takes `marked`): both rules reach its rows, and that is not refused.
- A shared schema is a `<name>.json` a template's or a part's schema names (`floe:///parts/<name>.json`), always at its root. floe's are `tone.json` (`accent`, `good`, `warn`, `bad`, `muted`) and `any.json` (a part of any kind). Its schemas are checked as a template's are, from its own root: a part or a format of floe's it holds is drawn where `properties` and `items` reach it from there, in a template's page or a part's alike.
- A value of `any.json` comes to `part.html` as it does to a template's `page.html`, a map of every kind: the kind written holds the part drawn, the others nil. `{{.x}}` writes the map as text; draw it with `{{range .x}}{{.}}{{end}}`, or `{{range .}}{{.}}{{end}}` when the schema's root is `any.json` (`floe template guide` (*Parts in a template*) tells how to draw one kind alone).

## The CSS layers and @scope

floe puts a page's CSS into `<style>`s before its `</head>`, in six cascade layers, each later one winning over those before it whatever the selectors (`floe.parts` the strongest):

1. `floe.palette`: the palettes, floe's `default` and then the theme's.
2. `floe.base`: floe's base, the page's ground, colour and face on its root, and text, lists, links, code and quotes.
3. `floe.theme`: the theme's `base.css`, scoped as the template's `page.css` is: it reaches the page and the boxes of the parts, not what they hold nor a text's Markdown (its box, `data-floe-text`). Its root is `:scope`; `:root` and `html` are outside the scope, and a rule for what a part holds (`table th`) goes in the theme's file of that part. After it, the theme's `markdown.css`, which reaches the whole page, what the parts hold too: the look of Markdown wherever it stands.
4. `floe.looks`: the looks of elements that take one without being a part's box (`data-floe-look`). A layer of their own, so that the template or part that borrows a look adjusts it whatever the selectors and however near its scope, and the theme's `base.css` wins over none of it, unless with `!important` (which turns the order of the layers round, and which floe does not refuse).
5. `floe.page`: the template's `page.css`, over floe's start of every page in a layer under it: the body's margin of 0 and the column `.floe-column`, scoped as `page.css` is.
6. `floe.parts`: the parts' style, each part after the looks it takes, then the theme's files of the parts. A theme's file wins over its part's `part.css`, its `@media print` too: a state print leaves out, such as the tab picked, a part paints in `@media screen`, from variables a theme sets (below).

A part's `part.css`, and a theme's file of it, are scoped to the elements that have its look, from their own on, short of what the parts in them hold. Write the element itself as `:scope`. A part's style reaches the box of a part it holds, but not what that part holds.

A list (`ul`, `ol` or `menu`) that is a part's box or takes a look has no marks, padding or room between its items, and is laid out in a grid: floe sets so in the lowest layer of `floe.parts` and of `floe.looks` (`floe.parts.boxes`, `floe.looks.boxes`), under every part's and look's own rules. So a part writes none of that to leave a list's look out, and a template's `page.css` or a theme's `base.css` that writes a rule for every list changes none of these four of a part's box (its margin and a rule for the items, `li` or `::marker`, still reach it); an element that takes a look keeps them from a theme's such rule, and its template's `page.css` adjusts them. A part whose box is a list it wants marked sets `list-style` and the padding in its `part.css`, or holds the list in its box; for a list without marks to be read out as a list, as Safari does not, it writes `role="list"` on it.

A part takes its colours from the palette's variables as they are (`var(--accent)`), with no value of its own to fall back on: every page of floe holds the palette. So the CSS a part or look reads (on the `files:` line of `floe part show <name>`, printed by naming it after) draws without colours in a page without a palette; for diagrams in such a page, take the assets of `floe diagram render`'s fragment, which carry `default`'s palette (`floe diagram render --help`). Nor does that CSS leave a list's marks and padding out of a part whose box is a list (checklist, links, steps, timeline): floe does so as it draws the page, as above; a page drawn without floe writes it.

A tone (one of `tone.json`'s) is marked by `data-floe-tone` on the element drawn in it, not by a class: `<span data-floe-tone="good" data-floe-part="cell">`. floe gives that element, and what it holds short of a part's box, the tone's colours as `--tone` and `--tone-soft` (from the palette's `--good` and `--good-soft`); a part's box, and the page, take accent's, so that a part takes no tone from one around it (a text's box, which is no part's, takes the tone around it), wherever the mark stands (a template's or a look's element too). A part draws in the two with one rule (`background: var(--tone-soft); color: var(--tone)`), and writes a rule of a tone only for what is no colour of it (note's marks, a diagram's bad box's text). A part whose things without a tone take other colours sets the two on its root (stats: `--muted` and `--muted-soft`); one that draws them in each shape's own colours, as the diagram does, sets both to `initial` on its root and writes each shape's colour after `var()` (`var(--tone, var(--line))`). A `data-floe-tone` of no tone is as if not written: a part's box draws in accent's colours, another element in the tone around it.

A look or part that paints a ground of its own (the look `panel`, `note`) sets its text's colour with it (`color: var(--fg)`) and takes no face from around it, so that it reads in a band; a part of no ground of its own takes the colour around it, a band's or a template's. A theme that takes that ground away gives both back on its `:scope` and on what it holds, as swiss's `panel.css` and `note.css` do (`:scope { color: inherit; --face: inherit; --on-face: inherit; }` and `:scope > * { --face: inherit; --on-face: inherit; }`); floe's rule is as specific as a theme's `:scope`, which comes after it. A part on a ground of its own does not take a template's text colour. A pill (`cell`) keeps to one line wherever it stands, in a table's or a grid's column too; what lets long pills wrap sets `--floe-pill-wrap: normal` on an element of its own, as entries' tags do, and a part's box stops it for the parts it holds. A pill that wraps is a label of round corners: its corners round no more than `--floe-pill-end` (base.css), past half a pill of one line.

A part that fills a box with a colour of its own and holds links, a line's code or a focus ring in it makes the box a face: it sets `--face` to the ground and `--on-face` to the text's colour and paints with them (`background: var(--face); color: var(--on-face)`), and floe's base draws those in `--on-face`. A part whose box itself is the face sets both on `:scope, :scope > *`, as `code` does, since floe's base starts afresh what a part's box holds. A colour a part sets on text of no ground of its own is written `var(--on-face, …)` (`color: var(--on-face, var(--good))`, as options' signs are), so that it reads in a face around it, a theme's filled card among them.

The names `@keyframes`, `@counter-style` and `@font-face` make are the page's, not held by the scope: name yours with your part's name (`mypart-fade`). Of two files that make one name, the one of the higher layer wins wherever it is used (a part's over the template's, the outer part's over the inner's), and `@font-face` replaces only a face whose descriptors (`font-weight`, `font-style`, `font-stretch`, `unicode-range` and the rest) are all the same; one that differs joins the same family.

A part writes no margin on its box: floe's base gives the blocks a part's box may be (`p`, `ul`, `ol`, `dl`, `figure`, `form`, `menu`, `fieldset` and the like) none, and what places the part spaces it (a template's sections, a stack, columns, a note), `--floe-room` from what stands next to it. A margin written there wins over the placing one. In the flow of a part's own Markdown (between its paragraphs, and before and after a part's box in it, such as a Markdown table), the part sets the room.

A part or a template that draws a box among parts, with a ground or a frame of its own (a card, a part or an element that takes the look panel, note), pads it from its frame to its text by `var(--floe-pad-block) var(--floe-pad-inline)` (floe's base, 1.1rem and 1.3rem), as floe's do, and writes no length of its own there. The look `panel` pads nothing itself, as a table, rows and details draw their rows to its frame.

A part writes its text smaller than the page's by floe's base's two steps, never a size of its own: `var(--floe-text-small)` (0.8rem) for a pill, a chip, a ring's number, a key, a table's heading, a term, a date, a count or a note beside a value, and `var(--floe-text-aside)` (0.9rem) for a card's or a step's words, a note's title, a small title over a list, a tab's name or a value's unit. The text of code (its lines, a tree's names) follows code's size instead. Both are in rem: small text in large text is no larger.

A part or look of one's own that reads `--floe-mark` sets it on its own root, as floe's do, so that a value around it (a card's number's) does not reach its marks. A part of your own reads the three too, and writes no length of its own beside a shape (`0`, `50%`, `999px`).

A box casts the theme's shadow (its `palette.css` gives it as `--floe-shadow-x`, `-y`, `-blur` and `-spread`) where its part's or look's file sets its colour, `--floe-shadow-color` (`.card { --floe-shadow-color: var(--fg); }`); the colour reaches no box inside. A shadow drawn within the box goes in `--floe-inset` (`initial` clears it; `none` voids the box's shadow too). Write no `box-shadow` on a part's box, a look's element or a quote: floe's base writes it from these, and one written over it drops them.

Nothing makes room for what a box draws outside itself (its shadow, a pseudo-element standing out, an outline): keep it within `--floe-room` (0.75rem) on every side, the room floe leaves between parts, so that it never meets another box or the page's edge. Make it a size that does not follow the box's: no tilt nor scale, which reach out further the bigger the box.

A box of your own that scrolls sideways writes `overflow-x: auto` and `contain: inline-size` in one rule under `@media screen`, as floe's `pre`, code, table and tree do, and folds what it holds on paper, where what scrolls is not read (floe's `pre` folds its lines there): without it, the box tells its content's width outside, and a grid or flex item around it (a list's item, a card) widens to its longest line, and the page with it. Keep such a box out of a place whose width its content sets (an inline-block, a float, an absolute or other shrink-to-fit box, a width of `min-content`, `max-content` or `fit-content`, a table's cell of auto width, `justify-self: start`, an item of a flex row on `flex-basis: auto`), where it shrinks to its border and padding. An element in it that paints a ground reaches to the end of what it holds on screen (`width: fit-content; min-width: 100%` for what folds, as tree's list does, `max-content` for lines that never fold, as code's lines do): else, scrolled, its ground stops at the box's width and what it holds runs out of it.

Nested, the outer wins. `floe.looks` and `floe.parts` are each split into layers by the depth of the element a style is scoped to, the element that takes the look or is the part's box: how many parts' boxes and elements that take looks stand around it, the outer the stronger. So every rule of a look or a part wins over the rules of a look or a part scoped to an element it holds, whatever the selectors and however near the scope: a look's rule for an element in it that takes another look, a part's for a part's box in it, and a rule for what such an element holds too (cards' `.card :is(p, ul, ol)` over the text of a card that takes `panel`; table's `:scope :is(th, td)` over the cells of an element in it that takes `table` too). A part's rules still stop at the boxes of the parts it holds, short of what they hold. Between a look and a part the order of the layers above stands: a look's style loses to the box of a part it holds. On one element, a theme's file of a look wins over the `part.css` of a part that takes it, as above.

As in `page.css`, a `}` that closes no block and an `@import` are refused at their line.

## Borrowing a look

- An element that is no part's box takes a look by `data-floe-look`: `<section data-floe-look="panel">` is a panel in every theme. It has it in `floe.looks`, under the parts and the template, so that the part or template it is in adjusts it. A card in an element that takes `cards` may take `panel` itself (`<div data-floe-look="cards"><div class="card" data-floe-look="panel">`): the cards, the outer, draw it as they draw the cards of a part.
- floe's parts take looks so: `fields` takes the table's, `stats`, `options`, `asks`, `facts` and `decision` the cards', `diff` and `terminal` code's, `checklist`, `entries`, `keys` and `links` the look `rows`, `steps` `numbered` beside `panel`, `entries` and `steps` `tags` on the element of their tags, and the other parts drawn in a panel `panel`'s. A theme's rules for a look draw every part that takes it.
- A part that takes code's look reads what code sets on its root: `--floe-code-pad` (1.1rem, the room at a line's sides), `--floe-code-room` (the room after a line, with the copy button's when the part draws it), `--floe-code-faint` (a line's faint text) and `--floe-code-head-faint` (the heading's). They are floe's, not a theme's to set. The room is on each line, an element `pre` holds, not on `pre`: make each line an element (one element a line in one `<code class="lines">`, as code and a diff do, one `<code>` around the whole, or one element a line in `pre`, as a terminal does), as text `pre` holds outside an element has no room, and one inline among it has room around it. Write no padding on a line, which drops code's room; lines in a `<code class="lines">` have code's look paint their ground to the end of the line and move the room onto each (a diff is drawn so); a part whose lines differ further moves the room onto them by these variables (`:scope pre > code { padding-inline: 0; } :scope code > span { padding-inline: var(--floe-code-pad) var(--floe-code-room); }`). Code's look makes a block of a `<code>` that `pre` holds, and takes the inline look (its size, ground, shadow and padding) off it, and of no other: a `<code>` elsewhere in such a part (in its heading, in an element `pre` holds) keeps the inline look of floe's base.
- The look `rows` pads each of its rows by `--floe-row-pad` (floe's base, 0.4rem) above and below; its box's `::after` is a row too, padded alike, as checklist's count, and by `--floe-pad-inline` (1.3rem) at the sides, its `::after` too (a decoration drawn there has the sides' padding): a part or a template that takes it writes no padding of its rows, nor of an element in a row at its sides (a `summary` or `div` padded there sits twice as far in), and a line indented within its row writes the indent alone. An element that opens the row across its width reaches over the row's padding at the sides and pads itself as much (`margin-inline: calc(-1 * var(--floe-pad-inline)); padding-inline: var(--floe-pad-inline)`). A part whose rows do not take `rows` takes the variable, as the table's cells and facts' tiles do (`padding-block: var(--floe-row-pad)`), so that its rows are as tall as the others.
- A row with a mark (numbered's ring, timeline's dot, links' sign, note's mark, the letters of `marked`) is laid out by floe, in the layer `floe.marks`, from `marks:`: a grid with the columns `mark` and `text`, its `::before` the mark in `mark`, `--floe-mark` big with its frame and `--floe-mark-gap` from the text, standing, as what the row holds does, on the baseline of the first line; in a row of `joined` its `::after` is the line to the next mark, from the mark's bottom to 0.35rem above the row's end. floe writes where they stand and how big, no colour, corner or content: a part writes its mark's look (`content`, background, frame) and the line's colour, and spaces what the row holds by `> * + * { margin-top }`. In a row of `box` or `items`, an `::after` with no `position` is an item of the grid: it adds a row in the column `mark` under what the row holds, and one wider than the mark moves the text too; to lay it over the mark, write `position: absolute` and `position-anchor: --floe-mark`. A part's, template's or theme's rule that writes a row's `display`, `gap` or `grid-template` wins over floe's layout. A mark of letters before each row of a list is a row's mark too, by the look `marked` (`<ul data-floe-look="marked">`, or `looks: [marked]`): the part writes its letter and colour (`li::before { content: "×" }`) and each item's text in one element (`<li><span>…</span></li>`), so that a line of text and code flows as one. A sign before one title, word or cell (a prompt, an →) stands in the text's flow and is no row, nor is a list's own marker (`list-style`, `::marker`), which the browser places.
- A line the full height of a row that holds parts is no mark: `marks:` places a mark `--floe-mark` big on the first line. Draw it as the row's `::before`, `position: absolute` with `inset-block: 0`, in a row of `position: relative`, not as the row's border: a framed row is a box, which what the parts in it draw outside themselves may not leave.
- What CSS draws with `content` says after a `/` what a screen reader reads: `""` for a mark (`content: "✓"; content: "✓" / "";`, the first for browsers that read no `/`), or all the counters before it, in order, for a number to read (`content: counter(step) / counter(step);`; `""` for one that no word tells from what is around it, as a slide's), and no string, which passes no `words.yaml`. A screen reader names a mark by its shape (a check mark), not by what it means; what a mark alone means, the part tells in the HTML in hidden words, `<span class="floe-hidden">{{word "done"}}</span>` with its `words.yaml`, within a row's text (`<li><span><span class="floe-hidden">…</span>…</span></li>`), not as the row's child, whose room between what it holds it would shift (a row that holds no element of text, as a note's Markdown, takes it as its first child and counts its gaps from after it: `:scope > :not(.floe-hidden) + *`), and outside what the copy button copies (`data-floe-copy`). `.floe-hidden` is floe's base: shown to no one, read to a screen reader.

## Checking

`floe part check` reads every part, look and shared schema of your own, and every theme (whose files need the parts), over floe's own, and prints every mistake of them (exit code 1): of their files, their names, the looks they take, a part that takes its own look, a path of a `checks.yaml` that its schema does not declare, the schemas of parts and shared schemas, and their CSS. A part is not drawn, as it has no data: `floe template check` draws the example of a template that uses it.
