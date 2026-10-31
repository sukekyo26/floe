# Parts and themes

How to make a part, a look or a theme of your own. A part draws an item of a data file; a theme sets a page's colours, faces and the shape of the parts. To use parts in a template, see [templates.md](templates.md#parts-in-a-template); to write them in a data file, see [pages.md](pages.md#parts).

## Where parts and themes are

- Parts, looks and shared schemas of your own are in `FLOE_PARTS_DIR` (default `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`): a part or a look is a directory, a shared schema a file `<name>.json` beside them. Another file right there is not read, and nothing says so: put a part's files in its directory.
- Themes of your own are in `FLOE_THEMES_DIR` (default `$XDG_CONFIG_HOME/floe/themes`, else `~/.config/floe/themes`), a directory each. A file right there is not read, and nothing says so: put a theme's files in its directory.
- A name is lower case letters, digits, `-` and `_`, starting with a letter or a digit. A name floe's parts, looks, shared schemas or themes have is refused: yours sit beside floe's, and never take their place. A part or look is not named `base` or `palette`, the names of a theme's files for every page.
- A directory in them may not be a symlink; a file may.

```sh
floe part new quote     # makes the smallest part, quote, to fill in
floe part check         # checks every part, look, shared schema and theme of your own
floe part show quote    # prints its looks and every file it reads
```

A mistake of one stops every page, as [templates.md](templates.md#where-templates-are) tells: `floe part check` lists them. While you write one, make it in another directory and move it in once it is checked, as [templates.md](templates.md#make-one-in-another-directory) tells: set `FLOE_PARTS_DIR` or `FLOE_THEMES_DIR` to it and run `floe part check`.

## A part's files

```text
quote/
├── schema.json    # what a data file may write of it (JSON Schema)
├── part.html      # how it is drawn, a Go html/template
├── part.css       # its style (may be empty)
├── part.yaml      # the looks it takes (optional)
├── checks.yaml    # relations between its items (optional), as a template's
└── words.yaml     # its words, by language (optional), as a template's
```

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption>{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

- A schema names it as floe's are named, `"$ref": "floe:///parts/quote.json"`; its own schema names floe's parts and the shared schemas so too.
- Its element is its box, which names it: `data-floe-part="quote"`.
- It draws no heading (`<h1>` to `<h6>`): a part knows not the level it is put at, in a template's section, in `details` or in another part. A title is an element of `class="title"` (`<div class="title">`), which the look `cards` draws as a card's title; the headings are the template's and those its Markdown draws. floe does not check it.
- `part.html` sees the item as `.`, and the functions of a template's `page.html` but `asset` ([templates.md](templates.md#pagehtml)). An item its schema declares but the value leaves out is nil, as in `page.html`'s `.Data`. A mistake of running `part.html` (a key its schema does not declare, a value it cannot range over) is told at its line and column in `part.html`, not at the value that brought it: give the values their shape in the part's schema, so that a value of another shape is told at it.
- `part.html` has the functions floe's parts draw with too ([Functions of part.html](#functions-of-parthtml)).
- Its schema takes floe's `format`s as a template's does (`html`, `markdown`, `image`), declared under `properties` or `items`, where floe follows, or as the root, and a shared schema that holds them. Such an item comes to `part.html` drawn: `html` as written, `markdown` as HTML, `image` as a data URI (`<img src="{{.file}}" alt="{{.alt}}">`). Write it as `{{.h}}`: do not pass it to a function (`markdown`, `markdownLine`, `printf`, `slice`, `eq`, …) nor put it in a `<script>`, as it is HTML, not the text written, and does not work there. An item of `html` is checked in the page drawn as a template's is ([templates.md](templates.md)). A plain string drawn as Markdown is `{{markdown .text}}`. A value of `any.json` does not come drawn as one item but as a map of every kind ([Looks and shared schemas](#looks-and-shared-schemas)).
- A part of rows one under another takes the look `rows` (`looks: [rows]`) and writes no padding above and below its rows, which `rows` gives; rows that cannot take it read `var(--floe-row-pad)`. A theme leaves that variable as it is. [Borrowing a look](#borrowing-a-look) tells more.
- `floe part show <name>` prints every file one reads, floe's too, under a `files:` line naming them; `floe part show <name> <file>` prints one of them alone: copy from floe's parts what fits.

## Functions of part.html

It has the functions floe's parts draw with too. floe's parts show them in their `part.html`, which `floe part show <name> part.html` prints (`floe part show code part.html`).

### `copyButton`

`{{copyButton}}` draws the copy button, in the page's language. Put it right under the part's box (`<figure data-floe-part="quote">…{{copyButton}}</figure>`): what it copies is found from the button up, by the nearest box of a part, so that in the box of a part inside yours it copies that part's, not yours. It copies the text of each element of the part marked `data-floe-copy`, each ending in a line break (code marks its `<code>`; a terminal, each command). Its look is floe's; the part's CSS places it and keeps text clear of it, as code's does: `isolation: isolate` on the part's box, so that the button stays under the page outside; `margin: var(--floe-copy-gap)` around it; and, where text lies under it, `padding-inline-end: var(--floe-copy-room)` after the longest line, under `:scope:has(> .floe-copy)`, so that a line scrolled to its end ends left of the button. floe sets `--floe-copy-size` (the button's size), `--floe-copy-gap` and `--floe-copy-room` on the page's root, and applies none of them to a part.

### `diagram`

`{{diagram}}` takes nothing and draws the part's whole value as a diagram. The value is a spec as the part `diagram` takes it, nothing else: a key beside the spec (a `caption`) is refused by the diagram's schema.

### `script`

`{{script .js .data}}` draws an element, and a script that calls `.js`, a function, with it and `.data`, as the part `script` does.

### `diff`

`{{$d := diff .text}}` reads a diff as `git diff` writes it: `$d.Lines`, each with `.Kind` and `.Text`; and `$d.Added` and `$d.Removed`, how many `add` and `del` lines it has. `.Kind` is `hunk` for a line starting with `@@`; `meta` for one starting with `\` (in a hunk too), for one starting with `--- ` or `+++ ` outside a hunk, and for any other line not starting with `+`, `-` or a space (`diff --git`, `index`), which also ends a hunk; `add` and `del` for the other lines starting with `+` and `-`, outside a hunk too; `""` for a line starting with a space, or empty.

### `bars`

`{{range bars .}}` reads a list of items, each written as keys and values: `label`, `value` and `tone`. Each comes as `.Label`, `.Tone`, `.Value` as written and `.Width`, its value as a percentage of the largest. An item not written as keys and values is a mistake, told at the part's value with which item it is; so is a `value` that is not a number or is below 0, told with the value, and one not written or null, told as missing. To have such a mistake told at the item or the value itself, give the list in the part's schema the items `bars` takes, as the part `bars` does (`minimum` alone lets a value that is not a number through to `bars`):

```json
"items": {
  "type": "object",
  "properties": {
    "label": {"type": "string", "minLength": 1},
    "value": {"type": "number", "minimum": 0},
    "tone": {"$ref": "floe:///parts/tone.json"}
  },
  "required": ["label", "value"],
  "additionalProperties": false
}
```

Put it in the schema of the list `part.html` gives to `bars`: for `bars .items`, under `properties.items`, as `properties.items.items`; at the root it checks nothing of the list.

### `written`

`{{written}}` returns the part's value as written, for `part.html` to compare it, or to draw a word of it as text, as a pill's (not as Markdown or HTML: draw those items from `.`): the parts and the formats in it are not drawn, so it may go to `eq` and `index`, and a key the value leaves out is not there, not nil (`{{(written).note}}` is a mistake when `note` is left out; take it with `index` or `with`). floe's `table` matches the cells of a column's `pills` so.

### `tree`

`{{range tree .}}` reads a list of items of `path` (a folder's ending in `/`), `note` and `change`, and nests them into the folders the paths name, a folder not listed of its own too: each comes as `.Name`, `.Note`, `.Change`, `.Dir` (a folder) and `.Children`.

## Looks and shared schemas

- A look alone is a directory of a `part.css` (and a `part.yaml` with its `description:` and the looks it takes), without a schema: it draws nothing, and lends its style to parts and elements that take it. floe's looks alone are `panel`, `rows`, `cards`, `band` and `numbered` ([templates.md](templates.md#looks-to-borrow) tells what each draws).
- A part takes the looks of others by its `part.yaml`, `looks: [panel]`, and the looks those take in turn, each before it, which it adjusts.
- A shared schema is a `<name>.json` a template's or a part's schema names (`floe:///parts/<name>.json`), always at its root. floe's are `tone.json` (`accent`, `good`, `warn`, `bad`) and `any.json` (a part of any kind). Its schemas are checked as a template's are, from its own root: a part or a format of floe's it holds is drawn where `properties` and `items` reach it from there, in a template's page or a part's alike.
- A value of `any.json` comes to `part.html` as it does to a template's `page.html`, a map of every kind: the kind written holds the part drawn, the others nil. `{{.x}}` writes the map as text; draw it with `{{range .x}}{{.}}{{end}}`, or `{{range .}}{{.}}{{end}}` when the schema's root is `any.json` ([templates.md](templates.md#parts-in-a-template) tells how to draw one kind alone).

## The CSS layers and @scope

floe puts a page's CSS into `<style>`s before its `</head>`, in six cascade layers, each later one winning over those before it whatever the selectors (`floe.parts` the strongest):

1. `floe.palette`: the palettes, floe's `default` and then the theme's.
2. `floe.base`: floe's base, the page's ground, colour and face on its root, and text, lists, links, code and quotes.
3. `floe.theme`: the theme's `base.css`, scoped as the template's `page.css` is: it reaches the page and the boxes of the parts, not what they hold, which a part draws from floe's base. Its root is `:scope`; `:root` and `html` are outside the scope, and a rule for what a part holds (`table th`) goes in the theme's file of that part.
4. `floe.looks`: the looks of elements that take one without being a part's box (`data-floe-look`). A layer of their own, so that the template or part that borrows a look adjusts it whatever the selectors and however near its scope, and the theme's `base.css` wins over none of it, unless with `!important` (which turns the order of the layers round, and which floe does not refuse).
5. `floe.page`: the template's `page.css`.
6. `floe.parts`: the parts' style, each part after the looks it takes, then the theme's files of the parts. A theme's file wins over its part's `part.css`, its `@media print` too: a theme that fills a state print leaves out, such as the tab picked, writes it in `@media screen`.

A part's `part.css`, and a theme's file of it, are scoped to the elements that have its look, from their own on, short of what the parts in them hold. Write the element itself as `:scope`. A part's style reaches the box of a part it holds, but not what that part holds.

A part writes no margin on its box: floe's base gives the blocks a part's box may be (`p`, `ul`, `ol`, `dl`, `figure`, `form`, `menu`, `fieldset` and the like) none, and what places the part spaces it (a template's sections, a stack, a note), at least `--floe-room` from what stands next to it. A margin written there wins over the placing one.

Nested, the outer wins. `floe.looks` and `floe.parts` are each split into layers by the depth of the element a style is scoped to, the element that takes the look or is the part's box: how many parts' boxes and elements that take looks stand around it, the outer the stronger. So every rule of a look or a part wins over the rules of a look or a part scoped to an element it holds, whatever the selectors and however near the scope: a look's rule for an element in it that takes another look, a part's for a part's box in it, and a rule for what such an element holds too (cards' `.card :is(p, ul, ol)` over the text of a card that takes `panel`; table's `:scope :is(th, td)` over the cells of an element in it that takes `table` too). A part's rules still stop at the boxes of the parts it holds, short of what they hold. Between a look and a part the order of the layers above stands: a look's style loses to the box of a part it holds. On one element, a theme's file of a look wins over the `part.css` of a part that takes it, as above.

As in `page.css`, a `}` that closes no block and an `@import` are refused at their line.

## Borrowing a look

- An element that is no part's box takes a look by `data-floe-look`: `<section data-floe-look="panel">` is a panel in every theme. It has it in `floe.looks`, under the parts and the template, so that the part or template it is in adjusts it. A card in an element that takes `cards` may take `panel` itself (`<div data-floe-look="cards"><div class="card" data-floe-look="panel">`): the cards, the outer, draw it as they draw the cards of a part.
- floe's parts take looks so: `compare`, `fields` and `findings` take the table's, `stats`, `options`, `facts`, `links` and `decision` the cards', `diff` and `terminal` code's, `checklist`, `entries`, `faq` and `keys` the look `rows`, `steps` `numbered` beside `panel`, and the other parts drawn in a panel `panel`'s. A theme's rules for a look draw every part that takes it.
- The look `rows` leaves out a list's marks and padding, and pads each of its rows by `--floe-row-pad` (floe's base, 0.4rem) above and below; its box's `::after` is a row too, padded alike, as checklist's count: a part or a template that takes it writes its rows' padding at the sides alone. A part whose rows do not take `rows` takes the variable, as the table's cells and facts' tiles do (`padding-block: var(--floe-row-pad)`), so that its rows are as tall as the others.
- A row with a mark that is no letter (numbered's ring, timeline's dot, links' box, note's mark) is a grid with the columns `mark` and `text` and the row `mark`: the mark is its `::before` in `grid-area: mark`, `--floe-mark` big with its frame (`box-sizing: border-box`) and `--floe-mark-gap` from the text (its `margin-inline-end`), and stands, as what the row holds does (`grid-column: text`), on the baseline of the first line (`align-self: first baseline`), the row growing when the mark is the taller. What the row holds is spaced by `> * + * { margin-top }`, not the grid's gap. The line to the next mark is the row's `::after`, from the mark's bottom by anchor positioning (`anchor-name: --floe-mark` on the mark, `anchor-scope: --floe-mark` on the row, `top: calc(anchor(--floe-mark bottom) + 0.35rem)`, `left: calc(anchor(--floe-mark center) - 1px)`) to 0.35rem above the row's end. A part of one's own whose rows have such a mark draws them so; a mark of letters (decision's ×) stands in the text's flow and needs none of it.

## Themes

A theme of your own is a directory of these files, each left out if not needed. `floe theme list` lists the themes, and `floe theme show <name>` prints the files one reads, floe's too (`floe theme show ink palette.css`):

| File | What it holds |
|---|---|
| `palette.css` | The variables of [the palette](templates.md#the-palette), and those of names of its own, on `:root`. It goes over floe's `default` palette, so it writes only what it changes. |
| `base.css` | Rules for every page, such as Markdown's quotes. They reach the page, not what a part holds, under the template's `page.css` and the looks. Write the root as `:scope`, not `:root` or `html`, which are outside the scope, and what a part holds in the file of that part. |
| `<name>.css` | The style of the part or look `<name>`, scoped as its `part.css` is: `panel.css` reaches every panel, `code.css` code, a diff and a terminal. |

- A file of another name is refused.
- Write a colour once, in `palette.css`, and take it with `var()` (`var(--accent)`) in the other files, so that the template's colours on `:scope` and the palette agree.
- Leave `--floe-row-pad`, the padding above and below a row, as floe's base sets it: the rows of every part are as tall in every theme.
- A row's mark (note's, numbered's ring, timeline's dot, links' box) is sized, its frame within, by `--floe-mark` and kept from the text by `--floe-mark-gap`; a theme sets those, not the mark's `top`, `left` or the row's padding, and hides the mark by `content: none`, its column then taking no room.
- A shadow is the theme's one, given once in `palette.css` as `--floe-shadow-x`, `-y`, `-blur` and `-spread` (in px or rem: one in em grows with a box's font, and a percentage leaves no shadow), and cast by a box on which the part's or look's file sets its colour, `--floe-shadow-color` (`.card { --floe-shadow-color: var(--fg); }`); the colour reaches no box inside. A shadow drawn within the box goes in `--floe-inset` (`initial` clears it; `none` voids the box's shadow too). Write no `box-shadow` on a part's box, a look's element or a quote: floe's base writes it from these, and one written over it drops them.
- Nothing makes room for what a box draws outside itself (its shadow, a tilt, an outline): keep it within `--floe-room` (0.75rem) on every side, the least room floe leaves between parts, so that it never meets another box or the page's edge. A tilted box takes a width at most, half of which times the tilt's sine, with its shadow, stays within `--floe-room`, as notebook's sticky notes do. Narrow no gap between parts below `--floe-room`.
- A data file picks the theme with `theme:`, as one of floe's.

## Checking

`floe part check` reads every part, look, shared schema and theme of your own over floe's own, and prints every mistake of them (exit code 1): of their files, their names, the looks they take, a part that takes its own look, a path of a `checks.yaml` that its schema does not declare, the schemas of parts and shared schemas, and their CSS. A part is not drawn, as it has no data: `floe template check` draws the example of a template that uses it.
