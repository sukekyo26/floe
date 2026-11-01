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
├── part.yaml      # the looks it takes, its rows with a mark (optional)
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
- A part of rows one under another takes the look `rows` (`looks: [rows]`) and writes no padding of its rows, which `rows` gives (`--floe-row-pad` above and below, `--floe-pad-inline` at the sides); rows that cannot take it read `var(--floe-row-pad)`. A theme leaves those variables as they are. [Borrowing a look](#borrowing-a-look) tells more.
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

- A look alone is a directory of a `part.css` (and a `part.yaml` with its `description:`, the looks it takes and its `marks:`), without a schema: it draws nothing, and lends its style to parts and elements that take it. floe's looks alone are `panel`, `rows`, `cards`, `band`, `numbered` and `marked` ([templates.md](templates.md#looks-to-borrow) tells what each draws).
- A part takes the looks of others by its `part.yaml`, `looks: [panel]`, and the looks those take in turn, each before it, which it adjusts.
- A part's or look's `part.yaml` tells by `marks:` which of its elements are rows with a mark at their left, which floe lays out: `box` (a part's box; a look's element that takes it, and the box of a part that takes it), `items` (each `li` child of that, alone: a row of `div`s takes no mark) or `joined` (`items`, a line running from each mark to the next). One of a part or look and the looks it takes, theirs too, writes `marks:`; `floe part check` refuses a second. A look that takes one (`looks: [numbered]`) makes rows of the elements that take it. Do not give one element two of them in HTML (`data-floe-look="numbered marked"`, or a box of `links` that takes `marked`): both rules reach its rows, and that is not refused.
- A shared schema is a `<name>.json` a template's or a part's schema names (`floe:///parts/<name>.json`), always at its root. floe's are `tone.json` (`accent`, `good`, `warn`, `bad`, `muted`) and `any.json` (a part of any kind). Its schemas are checked as a template's are, from its own root: a part or a format of floe's it holds is drawn where `properties` and `items` reach it from there, in a template's page or a part's alike.
- A value of `any.json` comes to `part.html` as it does to a template's `page.html`, a map of every kind: the kind written holds the part drawn, the others nil. `{{.x}}` writes the map as text; draw it with `{{range .x}}{{.}}{{end}}`, or `{{range .}}{{.}}{{end}}` when the schema's root is `any.json` ([templates.md](templates.md#parts-in-a-template) tells how to draw one kind alone).

## The CSS layers and @scope

floe puts a page's CSS into `<style>`s before its `</head>`, in six cascade layers, each later one winning over those before it whatever the selectors (`floe.parts` the strongest):

1. `floe.palette`: the palettes, floe's `default` and then the theme's.
2. `floe.base`: floe's base, the page's ground, colour and face on its root, and text, lists, links, code and quotes.
3. `floe.theme`: the theme's `base.css`, scoped as the template's `page.css` is: it reaches the page and the boxes of the parts, not what they hold nor a text's Markdown (its box, `data-floe-text`). Its root is `:scope`; `:root` and `html` are outside the scope, and a rule for what a part holds (`table th`) goes in the theme's file of that part. After it, the theme's `markdown.css`, which reaches the whole page, what the parts hold too: the look of Markdown wherever it stands.
4. `floe.looks`: the looks of elements that take one without being a part's box (`data-floe-look`). A layer of their own, so that the template or part that borrows a look adjusts it whatever the selectors and however near its scope, and the theme's `base.css` wins over none of it, unless with `!important` (which turns the order of the layers round, and which floe does not refuse).
5. `floe.page`: the template's `page.css`.
6. `floe.parts`: the parts' style, each part after the looks it takes, then the theme's files of the parts. A theme's file wins over its part's `part.css`, its `@media print` too: a state print leaves out, such as the tab picked, a part paints in `@media screen`, from variables a theme sets (below).

A part's `part.css`, and a theme's file of it, are scoped to the elements that have its look, from their own on, short of what the parts in them hold. Write the element itself as `:scope`. A part's style reaches the box of a part it holds, but not what that part holds.

A list (`ul`, `ol` or `menu`) that is a part's box or takes a look has no marks, padding or room between its items, and is laid out in a grid: floe sets so in the lowest layer of `floe.parts` and of `floe.looks` (`floe.parts.boxes`, `floe.looks.boxes`), under every part's and look's own rules. So a part writes none of that to leave a list's look out, and a template's `page.css` or a theme's `base.css` that writes a rule for every list changes none of these four of a part's box (its margin and a rule for the items, `li` or `::marker`, still reach it); an element that takes a look keeps them from a theme's such rule, and its template's `page.css` adjusts them. A part whose box is a list it wants marked sets `list-style` and the padding in its `part.css`, or holds the list in its box; for a list without marks to be read out as a list, as Safari does not, it writes `role="list"` on it.

A part takes its colours from the palette's variables as they are (`var(--accent)`), with no value of its own to fall back on: every page of floe holds the palette. So the CSS that `floe part show <name>` prints draws without colours in a page without a palette; for diagrams in such a page, take the assets of `floe diagram render`'s fragment, which carry `default`'s palette ([commands.md](commands.md#floe-diagram-render-specyaml)). Nor does that CSS leave a list's marks and padding out of a part whose box is a list (checklist, links, steps, timeline): floe does so as it draws the page, as above; a page drawn without floe writes it.

A tone (one of `tone.json`'s) is marked by `data-floe-tone` on the element drawn in it, not by a class: `<span data-floe-tone="good" data-floe-part="cell">`. floe gives that element, and what it holds short of a part's box, the tone's colours as `--tone` and `--tone-soft` (from the palette's `--good` and `--good-soft`); a part's box, and the page, take accent's, so that a part takes no tone from one around it (a text's box, which is no part's, takes the tone around it), wherever the mark stands (a template's or a look's element too). A part draws in the two with one rule (`background: var(--tone-soft); color: var(--tone)`), and writes a rule of a tone only for what is no colour of it (note's marks, a diagram's bad box's text). A part whose things without a tone take other colours sets the two on its root (stats: `--muted` and `--muted-soft`); one that draws them in each shape's own colours, as the diagram does, sets both to `initial` on its root and writes each shape's colour after `var()` (`var(--tone, var(--line))`). A `data-floe-tone` of no tone is as if not written: a part's box draws in accent's colours, another element in the tone around it.

A look or part that paints a ground of its own (the look `panel`, `note`) sets its text's colour with it (`color: var(--fg)`) and takes no face from around it, so that it reads in a band; a part of no ground of its own takes the colour around it, a band's or a template's. A theme that takes that ground away gives both back on its `:scope` and on what it holds, as swiss's `panel.css` and `note.css` do (`:scope { color: inherit; --face: inherit; --on-face: inherit; }` and `:scope > * { --face: inherit; --on-face: inherit; }`); floe's rule is as specific as a theme's `:scope`, which comes after it. A part on a ground of its own does not take a template's text colour. A pill (`cell`) keeps to one line wherever it stands, in a table's or a grid's column too; what lets long pills wrap sets `--floe-pill-wrap: normal` on an element of its own, as entries' tags do, and a part's box stops it for the parts it holds. A pill that wraps is a label of round corners: its corners round no more than `--floe-pill-end` (base.css), past half a pill of one line.

A part writes no margin on its box: floe's base gives the blocks a part's box may be (`p`, `ul`, `ol`, `dl`, `figure`, `form`, `menu`, `fieldset` and the like) none, and what places the part spaces it (a template's sections, a stack, a note), `--floe-room` from what stands next to it. A margin written there wins over the placing one. In the flow of a part's own Markdown (between its paragraphs, and before and after a part's box in it, such as a Markdown table), the part sets the room.

A part or a template that draws a box among parts, with a ground or a frame of its own (a card, a part or an element that takes the look panel, note), pads it from its frame to its text by `var(--floe-pad-block) var(--floe-pad-inline)` (floe's base, 1.1rem and 1.3rem), as floe's do, and writes no length of its own there. The look `panel` pads nothing itself, as a table, rows and details draw their rows to its frame.

A box of your own that scrolls sideways (`overflow-x: auto`) writes `contain: inline-size` for its rule under `@media screen`, as floe's `pre`, code, table and tree do: without it, the box tells its content's width outside, and a grid or flex item around it (a list's item, a card) widens to its longest line, and the page with it. Keep such a box out of a place whose width its content sets (an inline-block, a float, an absolute or other shrink-to-fit box, a width of `min-content`, `max-content` or `fit-content`, a table's cell of auto width, `justify-self: start`, an item of a flex row on `flex-basis: auto`), where it shrinks to its border and padding.

Nested, the outer wins. `floe.looks` and `floe.parts` are each split into layers by the depth of the element a style is scoped to, the element that takes the look or is the part's box: how many parts' boxes and elements that take looks stand around it, the outer the stronger. So every rule of a look or a part wins over the rules of a look or a part scoped to an element it holds, whatever the selectors and however near the scope: a look's rule for an element in it that takes another look, a part's for a part's box in it, and a rule for what such an element holds too (cards' `.card :is(p, ul, ol)` over the text of a card that takes `panel`; table's `:scope :is(th, td)` over the cells of an element in it that takes `table` too). A part's rules still stop at the boxes of the parts it holds, short of what they hold. Between a look and a part the order of the layers above stands: a look's style loses to the box of a part it holds. On one element, a theme's file of a look wins over the `part.css` of a part that takes it, as above.

As in `page.css`, a `}` that closes no block and an `@import` are refused at their line.

## Borrowing a look

- An element that is no part's box takes a look by `data-floe-look`: `<section data-floe-look="panel">` is a panel in every theme. It has it in `floe.looks`, under the parts and the template, so that the part or template it is in adjusts it. A card in an element that takes `cards` may take `panel` itself (`<div data-floe-look="cards"><div class="card" data-floe-look="panel">`): the cards, the outer, draw it as they draw the cards of a part.
- floe's parts take looks so: `compare`, `fields` and `findings` take the table's, `stats`, `options`, `facts`, `links` and `decision` the cards', `diff` and `terminal` code's, `checklist`, `entries`, `faq` and `keys` the look `rows`, `steps` `numbered` beside `panel`, and the other parts drawn in a panel `panel`'s. A theme's rules for a look draw every part that takes it.
- The look `rows` pads each of its rows by `--floe-row-pad` (floe's base, 0.4rem) above and below; its box's `::after` is a row too, padded alike, as checklist's count, and by `--floe-pad-inline` (1.3rem) at the sides, its `::after` too (a decoration drawn there has the sides' padding): a part or a template that takes it writes no padding of its rows, nor of an element in a row at its sides (a `summary` or `div` padded there sits twice as far in), and a line indented within its row (faq's answer, past its Q) writes the indent alone. An element that opens the row across its width (faq's question) reaches over the row's padding at the sides and pads itself as much (`margin-inline: calc(-1 * var(--floe-pad-inline)); padding-inline: var(--floe-pad-inline)`). A part whose rows do not take `rows` takes the variable, as the table's cells and facts' tiles do (`padding-block: var(--floe-row-pad)`), so that its rows are as tall as the others.
- A row with a mark (numbered's ring, timeline's dot, links' box, note's mark, the letters of `marked`) is laid out by floe, in the layer `floe.marks`, from `marks:`: a grid with the columns `mark` and `text`, its `::before` the mark in `mark`, `--floe-mark` big with its frame and `--floe-mark-gap` from the text, standing, as what the row holds does, on the baseline of the first line; in a row of `joined` its `::after` is the line to the next mark, from the mark's bottom to 0.35rem above the row's end. floe writes where they stand and how big, no colour, corner or content: a part writes its mark's look (`content`, background, frame) and the line's colour, and spaces what the row holds by `> * + * { margin-top }`. In a row of `box` or `items` the `::after` stands over the mark, the slot of a sign over it (links' sign over its box): a decoration of its own drawn in that `::after` moves onto the mark. A part's, template's or theme's rule that writes a row's `display`, `gap` or `grid-template` wins over floe's layout. A mark of letters before each row of a list is a row's mark too, by the look `marked` (`<ul data-floe-look="marked">`, or `looks: [marked]`): the part writes its letter and colour (`li::before { content: "×" }`) and each item's text in one element (`<li><span>…</span></li>`), so that a line of text and code flows as one. A sign before one title, word or cell (faq's Q, compare's →, a prompt) stands in the text's flow and is no row, nor is a list's own marker (`list-style`, `::marker`), which the browser places.

## Themes

A theme of your own is a directory of these files, each left out if not needed. `floe theme list` lists the themes, and `floe theme show <name>` prints the files one reads, floe's too (`floe theme show ink palette.css`):

| File | What it holds |
|---|---|
| `palette.css` | The variables of [the palette](templates.md#the-palette), and those of names of its own, on `:scope`, which is the page's root (`:root` takes the same element). It goes over floe's `default` palette, so it writes only what it changes. |
| `base.css` | Rules for the page around the parts. They reach the page, not what a part holds nor a text's Markdown, under the template's `page.css` and the looks. Write the root as `:scope`, not `:root` or `html`, which are outside the scope, and what a part holds in the file of that part. Variables of the theme's own may go on its `:scope` here too. |
| `markdown.css` | The look of Markdown (quotes, headings), wherever it stands: on the page, in a text and in what the parts hold. Every part's, look's and template's CSS wins over it. Write rules for what a part draws none of, as a quote or a heading: one for `p` or `ul` reaches a part's own too. |
| `<name>.css` | The style of the part or look `<name>`, scoped as its `part.css` is: `panel.css` reaches every panel, `code.css` code, a diff and a terminal. |

- A file of another name is refused.
- Write a colour once, in `palette.css`, and take it with `var()` (`var(--accent)`) in the other files, so that the template's colours on `:scope` and the palette agree.
- Leave `--floe-row-pad`, the padding above and below a row, and `--floe-pad-block` and `--floe-pad-inline`, a box's padding and a row's at the sides, as floe's base sets them: the rows and boxes of every part are padded alike in every theme. A theme pads no box: a frame it draws around a box (default's grey) is its own, and the element it paints inside reads the two variables.
- A row's mark (note's, numbered's ring, timeline's dot, links' box) is sized, its frame within, by `--floe-mark` and kept from the text by `--floe-mark-gap`; a theme sets those, not the mark's `top`, `left` or the row's padding, and hides the mark by `content: none`, its column then taking no room. The number of a card in cards (`.numbered > .card::before`) keeps the same terms: sized by `--floe-mark`, hidden by `content: none`, and drawn as plain text by `--floe-mark: auto` with no background.
- A card (`.card` in a part that takes cards) takes the panel too: a theme's `panel.css` draws it, and its `cards.css` writes what differs (the ground, the frame, the number), not a card's padding, which is the two variables'. Its first card is padded as the others: a theme lines it up with the column by no padding of 0.
- A theme draws upright and writes no word, as CSS text passes no `words.yaml` and stays in one language whatever the page's. A transform (`transform`, `-webkit-transform`, `-moz-transform`) moves alone, by `translate()`, `translateX()` or `translateY()`, or is `none`; a theme writes no `rotate`, `scale`, `zoom`, `offset-path`, `offset-rotate` or `perspective`, and no variable holding a turn. Its strings are marks alone, with a letter only in a typeface's name (`font-family`, `font`, `--font`, `--display`, `--mono`), written as themselves (no escape); `content` shows no `attr()`; a counter or list style (`counter()`, `counters()`, `list-style`, `list-style-type`) is `decimal`, `decimal-leading-zero` or a shape (`none`, `disc`, `circle`, `square`), by no variable, and a theme writes no `@counter-style`.
- A theme's decoration takes no room: draw it over the padding there is (a ground, `--floe-inset`, a `::before` laid over), not by widening the padding or by a frame of the ground's colour or none, so that it never vies with a part's padding. A frame drawn thicker is the frame's own shape (brutal's, editorial's top rule, washi's double rules), not room for a decoration.
- A theme draws a tone from `var(--tone)` and `var(--tone-soft)` too, with no rule for each tone. A theme that fills the tones in colours of its own (brutal's, notebook's) names them once in its `palette.css`, as `--good-fill` and the like (`--accent-fill` for what has no tone), and draws with `var(--tone-fill)`, which floe sets from `--<tone>-fill`, or else `--<tone>-soft` for a tone it does not name. A theme's file of a part goes after the part's `part.css`, in the layer of the same depth: as specific, it wins, and a more specific selector wins by that.
- A table's head, the tab picked and code's heading are painted by their parts from variables, each read with floe's own value when unset, which a theme sets on the `:scope` of its file of the part (or of its `base.css`, for the whole page): a table's head by `--table-head-bg` and `--table-head-fg`, its rule between rows by `--table-rule` (`none` for none); the tab picked by `--tabs-picked-bg`, `--tabs-picked-fg` and `--tabs-picked-ring` (a `box-shadow`); code's heading (a diff's and a terminal's too) by `--code-head-bg` and `--code-head-fg`, a diff's counts on it by `--code-head-add` and `--code-head-del`, and the copy button's ground by `--code-ground` (code's lines are on the code's own ground: a theme that paints them a ground of its own, on `:scope pre`, sets the same colour in `--code-ground`). A theme writes no rule for a state (`:checked`) or for print, nor colours a part's heading anew for another part: one that lightens code's heading sets the counts' colours with it. A terminal's heading is fainter, at 85% of `--code-head-fg`, whatever colour a theme gives it; a theme that writes the heading's `color` by a rule draws a terminal's at that colour. The picked tab's ring is drawn over a shadow a theme gives every tab title: a theme that wants that shadow on the picked one too sets it as the ring. A heading's rule and its type (face, size, weight) are rules of the theme's, on `:scope thead th` or `:scope figcaption`; a head a theme fills with a ground of its own is a face (`--face`, `--on-face`).
- A quiet ground within a box (a table's head, a bar's track, progress's ground, a keycap) is `var(--quiet)`. A theme that paints a box in a ground near it sets `--quiet` on that box (default sets the surface on its grey panels, and its grey on the faces of the surface within them).
- Round corners are the palette's: a theme sets `--radius`, `--radius-small` and `--radius-pill` in `palette.css`, one length each, and its other files leave the corners to them. Corners of other shapes, such as notebook's drawn by hand, are variables of the theme's own in its `palette.css` or on the `:scope` of its `base.css`, read in its files: one of the three holding four corners or a `/` voids a rule that reads it as one corner or in `calc()` or `min()`. A corner may take the smaller of two variables (`min(var(--radius-pill), var(--floe-pill-end))`, a pill's). A part of your own reads the three too, and writes no length of its own beside a shape (`0`, `50%`, `999px`).
- A shadow is the theme's one, given once in `palette.css` as `--floe-shadow-x`, `-y`, `-blur` and `-spread` (in px or rem: one in em grows with a box's font, and a percentage leaves no shadow), and cast by a box on which the part's or look's file sets its colour, `--floe-shadow-color` (`.card { --floe-shadow-color: var(--fg); }`); the colour reaches no box inside. A shadow drawn within the box goes in `--floe-inset` (`initial` clears it; `none` voids the box's shadow too). Write no `box-shadow` on a part's box, a look's element or a quote: floe's base writes it from these, and one written over it drops them.
- Nothing makes room for what a box draws outside itself (its shadow, a pseudo-element standing out, an outline): keep it within `--floe-room` (0.75rem) on every side, the room floe leaves between parts, so that it never meets another box or the page's edge. Make it a size that does not follow the box's: no tilt nor scale, which reach out further the bigger the box. A theme writes no room between parts: it is `--floe-room` everywhere, set by the parts and templates, and a gap on the root of a theme's `cards.css` or `tabs.css` moves it.
- A data file picks the theme with `theme:`, as one of floe's.

## Checking

`floe part check` reads every part, look, shared schema and theme of your own over floe's own, and prints every mistake of them (exit code 1): of their files, their names, the looks they take, a part that takes its own look, a path of a `checks.yaml` that its schema does not declare, the schemas of parts and shared schemas, and their CSS. A part is not drawn, as it has no data: `floe template check` draws the example of a template that uses it.
