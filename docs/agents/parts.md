# Parts

A part draws an item of a data file. It is a directory that says what the item may hold, how it is drawn and how it looks. A look is a directory of CSS alone, which parts and elements borrow. A shared schema is a JSON Schema that parts and templates name. This doc says what each file holds, what `part.html` can call, what a part's CSS reaches and how a change takes effect, and what `floe part check` refuses. A data file writes a part as one key naming it (`- quote: { text: Hello }` among a section's `parts`). A part is used in a template as `floe template guide` tells (*Parts in a template*), and in a data file as `floe guide` tells.

## Where parts are

- Parts, looks and shared schemas are in `FLOE_PARTS_DIR` (default `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`): a part or a look is a directory, a shared schema a file `<name>.json` beside them. `floe part new quote` makes the smallest part, `quote`: a `schema.json`, a `part.html` holding its box and an empty `part.css`. A file directly in `FLOE_PARTS_DIR` other than `<name>.json` is ignored without a message: put a part's files in its directory.
- The name MUST be lower case letters, digits, `-` and `_`, starting with a letter or a digit. It MUST NOT be the name of one of floe's parts, looks or shared schemas, nor a key a section of floe's templates takes (`html`, the one key of `any.json` that no part has): yours sit beside floe's and never take their place.
- A part's or look's name MUST NOT be `base`, `palette` or `markdown` (the names of a theme's files for every page), and MUST NOT be the name of a shared schema of yours too (a directory and a `<name>.json` of one name).
- A directory in `FLOE_PARTS_DIR`, and anything in a part's or look's directory, MUST NOT be a link to a directory (a link to a file is read), and a file MUST be a regular file (a FIFO or a device is refused unread).
- `FLOE_PARTS_DIR` MUST NOT be the directory of your themes or templates (`FLOE_THEMES_DIR`, `FLOE_TEMPLATES_DIR`), contain it, or be in it: floe would read the files of one as those of the other (`floe part check` compares it with the themes' directory; the commands that draw a page compare all three).
- A mistake in any part of yours stops every page, whether or not the page uses that part, until it is fixed (`floe template guide`, *Where templates are*). A new part made in another directory and moved in once `floe part check` is clean stops none: set `FLOE_PARTS_DIR` to that directory for `floe part new quote` and `floe part check` (`floe template guide`, *Make one in another directory*).

## Files

`floe part show <part>` lists the files a part or look reads (floe's too) on a `files:` line, and `floe part show <part> part.css` prints one. A directory with a `schema.json` is a part; one without is a look.

| File | In a part | In a look | What it holds |
|---|---|---|---|
| `schema.json` | required | none | What a data file may write of the part, as JSON Schema (*schema.json*) |
| `part.html` | required | MUST NOT | How it is drawn, a Go html/template (*part.html*) |
| `part.css` | required, may be empty | required | Its style (*The CSS layers and @scope*) |
| `part.yaml` | optional | optional | The looks it takes, its rows with a mark, a look's description (*part.yaml*) |
| `checks.yaml` | optional | MUST NOT | Relations between its items, written as a template's (`floe template guide`, *checks.yaml*) |
| `words.yaml` | optional | MUST NOT | Its words by language, written as a template's (`floe template guide`, *words.yaml*) |

A file or directory of any other name MUST NOT be in a part or a look: floe refuses it. A name that begins with `.` (`.git/`, `.DS_Store`) is skipped, neither read nor refused.

### part.yaml

| Key | Takes | What it does |
|---|---|---|
| `looks:` | A list of names of parts or looks, each text (`looks: [panel]`; in quotes for a name YAML reads as a number, `true`, `false` or `null`) | The part or look takes those looks, and the looks they take in turn, each before it, which it adjusts. A name MUST be a part or look floe or you hold, and a part or look MUST NOT take itself, directly or through others |
| `marks:` | `box`, `items` or `joined` | Which of its elements are rows with a mark at their left, which floe lays out (*Rows with a mark*). A part or look and the looks it takes MUST NOT write it more than once in all |
| `description:` | Text | A look's, what it is (`floe part list --all` and `floe part show <look>` print it). A part's description is its `schema.json`'s, and MUST NOT be in its `part.yaml` |

A key other than these MUST NOT be in `part.yaml`, and the file MUST be keys and values.

## schema.json

- A schema names a part, floe's or yours, or a shared schema as `"$ref": "floe:///parts/<name>.json"`, in a template's schema and in a part's alike. It refers to no other file or URL: `$ref` MUST NOT name one, and `$id`, if written, MUST be an absolute URL of your own (not a `floe:` one), and MUST NOT be on the root, as floe gives the root its URL. `$schema` MUST name draft 2020-12 or 2019-09, or be left out.
- floe draws an item where `properties`, `additionalProperties` and `items` reach it from the root, through a `$ref` when nothing beside it draws. A part, or an item of a format of floe's, written where they do not reach (a `$dynamicRef`, a `patternProperties`, a `$ref` that points into a part of floe's or a shared schema rather than at the whole of it) MUST NOT be: floe refuses it, as the item would be checked and not drawn.
- A mistake of running `part.html` (a key its schema does not declare, a value it cannot range over) is told at its line and column in `part.html`, not at the value that brought it: give the values their shape in the schema, so that a value of another shape is told at it.
- A part of yours joins `any.json`: a section of floe's templates (report's `parts`) takes it as it takes floe's, and so does a schema of yours that names `any.json`. It does not join `stack`, which the parts that hold parts take (`floe part show stack` says which): a part of yours may hold a form or HTML, which those would break on. `floe part list` lists your parts with floe's, and `floe part show any any.json` prints floe's `any.json` as it is written.
- A shared schema is a `<name>.json` that a template's or a part's schema names (`floe:///parts/<name>.json`), always at its root. floe's are `tone.json` (`accent`, `good`, `warn`, `bad`, `muted`) and `any.json` (a part of any kind). It is checked from its own root: a part or a format of floe's it holds is drawn where `properties`, `additionalProperties` and `items` reach it from there, in a template's page or a part's alike.

### Formats

A schema takes floe's `format`s as a template's does, declared under `properties`, `additionalProperties` or `items`, or as the root, or in a shared schema that holds them. Such an item comes to `part.html` drawn:

| Format | Comes to `part.html` as | Written in `part.html` |
|---|---|---|
| `html` | The HTML as written. It MUST close each element and comment it opens and close none it did not open, and be put where elements go (refused when a page is drawn, as for a template's) | `{{.h}}` |
| `markdown` | The Markdown as HTML | `{{.m}}` |
| `image` | A data URI | `<img src="{{.file}}" alt="{{.alt}}">` (the format is on the item that holds the path, `file` here; `alt` is a plain item beside it) |

- Do not pass such an item to `markdown`, `markdownLine` or `markdownLabel` (refused at the line of `part.html`: wrong type for value), nor to `printf`, `slice`, `eq` or `len` (they take the HTML or the data URI, not the text written), nor put it in a `<script>` (an `html` item is refused at the data file's `template:`; a `markdown` or `image` item becomes a string of its HTML or data URI). A plain string drawn as Markdown is `{{markdown .text}}`; an item of format `markdown` is written `{{.text}}`.
- A value of `any.json` does not come as one item drawn but as a map of every kind: the kind written holds the part drawn, the others nil. `{{.x}}` writes the map as text: draw it with `{{range .x}}{{.}}{{end}}`, or `{{range .}}{{.}}{{end}}` when the schema's root is `any.json` (`floe template guide`, *Parts in a template*).

## part.html

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption>{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

- The part's element is its box, which names it: `data-floe-part="<name>"`. Write it on the one element every other element of the part is in.
- Do not draw `<h1>`–`<h6>` in `part.html` (floe gives every heading of the page an id from its text, in page order, so a part's heading takes the id a writer's link names, and the writer's own heading with that text gets the next one: `summary-1`). Write a title as an element of `class="title"`, which the look `cards` styles in a `.card` and `code`'s look in its heading; style it yourself elsewhere.
- `.` is the item (the quote above: its schema declares `text` a plain string, and its `words.yaml` the word `by`). The functions of a template's `page.html` but `asset` are there, and the functions of the next section.
- `markdown` draws the Markdown in a box of no part (`<div data-floe-text>`), as an item of format `markdown` is drawn. floe spaces the box's blocks `--floe-room` apart, and the part spaces the box alone: it adds no space between the blocks (`:scope p` reaches into the box, `:scope > p` does not; in the quote above the blockquote holds the box). Empty Markdown draws no box. The box of an item of format `markdown` is a child of the element it is written in, so that `:scope > *` reaches it.
- An item its schema declares but the value leaves out is nil, as in `page.html`'s `.Data`.
- A `{{word "x"}}` of a word `words.yaml` has not is refused when a page is drawn with the part, at its line and column in `part.html`, once however many times the part is used. A page's language takes the first of its tag (`ja-JP`), the tag without its last subtag (`ja`) and `en` that `words.yaml` has, and a word that language lacks is refused.
- `data-floe-part` MUST be the name of a part, and `data-floe-look` the name of a part or look: a page drawn with the part refuses a `data-floe-part` that is empty, names a look or names no part, and a `data-floe-look` that names neither. A `part.html` without `data-floe-part`, and `data-floe-look` on a part's own box, draw.
- `floe part check` parses `part.html` and runs html/template's escaping on it, without data: it refuses an unknown function, a syntax mistake, and what html/template refuses in a context. It does not draw the part: `floe template check` draws the example of a template that uses it. A page drawn with the part holds no `<style>`, `<base>`, `<noscript>` or `<meta>` but for its charset and viewport, and no element of a `<select>` that is read as text (`floe template guide`, *page.html*): refused at the data file's `template:`, not by `floe part check`.

## Functions of part.html

The functions floe adds to html/template's own (`printf`, `index`, `eq` and the rest, which `part.html` can call too). The first six are `page.html`'s without `asset` (`floe template guide`, *page.html*); `markdown` is `page.html`'s too, but draws the box described in *part.html*. `floe part show <part> part.html` prints a floe part's (`floe part show code part.html`).

| Function | Call | What it gives |
|---|---|---|
| `markdownLine` | `{{markdownLine .text}}` | A line of Markdown as HTML, without a paragraph around it |
| `markdownLabel` | `{{markdownLabel .text}}` | A line of Markdown as `markdownLine` writes it, its links as their words |
| `num` | `{{num .n}}` | A number written with grouping; `{{num 3 .n}}` to 3 significant digits |
| `bytes` | `{{bytes .n}}` | A count of bytes in the binary unit that keeps it under 1024 (1536 is `1.5 KiB`) |
| `contents` | `{{contents}}` | The script that marks, in the contents just before it, the place the page is read at |
| `word` | `{{word "by"}}` | A word of the part's `words.yaml`, in the page's language |
| `markdown` | `{{markdown .text}}` | A Markdown text drawn in a box of no part (*part.html*) |
| `copyButton` | `{{copyButton}}` | The copy button, in the page's language |
| `diagram` | `{{diagram}}` | The part's whole value drawn as a diagram |
| `zoom` | `{{zoom}}` | The script that zooms a frame |
| `script` | `{{script .js .data}}` | An element and a script that calls `.js` with it and `.data` |
| `diff` | `{{$d := diff .text}}` | A `git diff` read into lines |
| `code` | `{{$c := code .text .start .mark}}` | Text cut into lines |
| `bars` | `{{range bars .}}` | A list of bars read |
| `stats` | `{{range stats .}}` | A list of stats read |
| `chart` | `{{$c := chart .}}` | The places of the part `chart` |
| `written` | `{{written}}` | The part's value as written |
| `tree` | `{{range tree .}}` | A list of paths nested |

### copyButton

It copies the text of each element marked `data-floe-copy`, each ending in a line break (`code` marks each line's `<span>`, `diff` its `<code class="lines">`, `terminal` each `.run` span). It copies every marked element in the nearest box of a part above the button, the boxes of parts inside that one too: the button of a part inside yours copies that part's, and yours copies both. Its look is floe's.

- Put `{{copyButton}}` right under the part's box (`<figure data-floe-part="quote">…{{copyButton}}</figure>`).
- Write `isolation: isolate` on the part's box, unless the part takes `code`'s look (`looks: [code]`), which has it from code's CSS (the button has `z-index: 1`: without `isolation` on the box the button is drawn over elements outside the part that lie above the part, such as a fixed header).
- Place the button and keep text clear of it in the part's CSS, unless the part takes `code`'s look, which does it too: `margin: var(--floe-copy-gap)` around it, and where text lies under it, `padding-inline-end: var(--floe-copy-room)` after the longest line, under `:scope:has(> .floe-copy)` (a line scrolled to its end then ends left of the button).

`--floe-copy-size`, `--floe-copy-gap` and `--floe-copy-room` are set on the page's root (*Variables floe sets*); floe applies none of them to a part.

### diagram

It takes nothing and draws the part's whole value as a diagram. The value MUST be a spec as the part `diagram` takes it, and a key beside the spec (a `caption`) MUST NOT be there (refused by the diagram's schema when a page is drawn).

### zoom

It draws the script that zooms, the one the parts `image` and `diagram` use, once for each use of the part in a page: each frame is set up once however many copies of the script the page holds.

- Put `{{zoom}}` in the part's box before a frame of `class="floe-zoom"`, and take the look `zoom` in `part.yaml` (`looks: [zoom]`): `<figure data-floe-part="shot">{{zoom}}<div class="floe-zoom"><img src="{{.file}}" alt="{{.alt}}"></div></figure>`.
- The frame's first element is what it zooms, an `svg` or an `img` itself (an element that holds one is not moved): an `svg` by its `viewBox`, an `img` by a `transform`, its size taken from the `viewBox` or the image. The look `zoom` lays an `img` out shrunk to the frame's width (`display: block; max-width: 100%; height: auto`), so the example above, copied as it is, zooms. While zoomed, the image's line is dropped.
- A toolbar before the frame and a tab stop on it stand only while what it holds is drawn smaller than its actual size, or zoomed. A frame written with a `title` keeps it, in place of the tooltip that tells how to zoom; a `title` of an element in the frame wins over the frame's while the pointer is over that element.
- The part's CSS sizes the frame. Set a width on the part's box rather than on the frame (the toolbar stands before the frame in the box, and takes the box's width).
- Do not select the line that tells how to zoom with a sibling selector such as `.floe-zoom + p`, and put the frame in a box of its own in a grid or a flex that holds it directly (the script puts that line as the frame's next element, shown only while the keyboard is on the frame: it no longer matches, and in a grid or flex it is a new item while it shows).

### script

It draws an element, and a script that calls `.js`, a function, with it and `.data`, as the part `script` does. `js` MUST NOT hold `</script`, or `<!--` before a `<script` (refused when a page is drawn: it would end its `<script>` early); split them in a string, as `'<' + '/script'`.

### diff

`$d.Lines`, each with `.Kind` and `.Text`; `$d.Added` and `$d.Removed`, how many `add` and `del` lines it has.

| `.Kind` | A line that |
|---|---|
| `hunk` | starts with `@@` |
| `meta` | starts with `\` (in a hunk too); starts with `--- ` or `+++ ` outside a hunk; is any other line not starting with `+`, `-` or a space (`diff --git`, `index`), which also ends a hunk |
| `add`, `del` | starts with `+` or `-`, outside a hunk too |
| `""` | starts with a space, or is blank |

### code

It cuts text into lines, as the part `code` does: `$c.Lines`, each with `.Text` (its line break kept) and `.Mark`, whether `.mark`, a list of line numbers or nothing, names it. `.start`, a line number or nothing, is the number of the first line: written, `$c.Numbered` is true, `$c.Before` is the count before the first line and `$c.Digits` the digits of the last line's number. A line number MUST be a whole number (refused when a page is drawn); its range is for your schema to set, as the part `code`'s does (from 1 to 1,000,000,000). A number no line has marks nothing.

### bars

It reads a list of items, each written as keys and values: `label`, `value`, `of`, `unit` and `tone`.

| Field | Is |
|---|---|
| `.Label`, `.Tone`, `.Unit` | As written (`.Unit` empty when not written) |
| `.Value`, `.Of` | As written (`.Of` empty without one) |
| `.Now`, `.Max` | For an item with `of`: `value` and `of` in plain digits, without grouping or an exponent, for `aria-valuenow` and `aria-valuemax` (`num` writes them grouped); empty for the others |
| `.Width` | For an item with `of`, its value as a percentage of its `of`; for the others, as a percentage of the largest value among them, which the items with `of` are left out of |

`bars` reads an `of` of null as one not written; a part's schema that declares `of` a number refuses null before. Each item MUST be written as keys and values, `value` MUST be a number of 0 or more, `of` if written MUST be a number above 0, `value` MUST NOT be over its `of`, and `unit` if written MUST be text. `bars` refuses the others when a page is drawn, all together at the part's value, each told with its bar's number in the list and its label (an item not as keys and values with which item it is; a `value` that is not a number or is below 0 with the value; one not written or null as missing).

To have a mistake of a value's shape told at the item or the value itself, give the list in the part's schema the items `bars` takes, as the part `bars` does (`minimum` alone lets a value that is not a number through to `bars`):

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

Put it as the `items` of the schema of the list `part.html` gives to `bars`: `properties.items.items` for `bars .items`, the root's `items` for `bars .` (a part whose value is the list). An `items` at the root of an object schema checks nothing of a list under `properties`.

### stats

It reads a list of items, each written as keys and values: `label`, `value`, `unit`, `note`, `tone`, `from` and `better`.

| Field | Is |
|---|---|
| `.Label`, `.Value`, `.Unit`, `.Note`, `.Tone`, `.From` | As written |
| `.Way` | `better`, `worse` or `same`; empty without `from` |
| `.WayTone` | `good`, `bad` or `muted`; empty without `from` |
| `.Change` | `value` less `from`, signed and exact, to the decimals of the longer of the two; empty when they are the same |

With `from`, `from` and `value` MUST be text of digits alone (`^-?[0-9]+([.][0-9]+)?$`) and `better` MUST be `lower` or `higher`: the function refuses others at the part's value, naming the item by its number and label. The other keys are for the part's schema to say.

### chart

It counts the places of the part `chart` from the part's whole value, for its `part.html` to write the SVG. Its fields, what it assumes of the height, text and class, and what it refuses are not promised to other parts.

### written

It returns the part's value as written, for `part.html` to compare it, or to draw a word of it as text, as a pill's. The parts and the formats in it are not drawn (do not use it for an item that is Markdown or HTML: draw those from `.`). It may go to `eq` and `index`. A key the value leaves out is not there, not nil: `{{(written).note}}` is a mistake when `note` is left out; take it with `index` or `with`. floe's `table` matches the cells of a column's `pills` so.

### tree

It reads a list of items of `path` (a folder's ending in `/`), `note` and `change`, and nests them into the folders the paths name, a folder not listed of its own too: each comes as `.Name` (a folder's keeps its trailing `/`), `.Note`, `.Change`, `.Dir` (a folder) and `.Children`. An item that is not written as keys and values is skipped, and a path listed again overwrites the note and the change of the first.

## Borrowing a look

A look draws nothing and lends its style to parts and elements that take it. A look takes looks as a part does (`rows` takes `panel`: `floe part show <look> part.yaml` prints what one takes), and a part's `looks:` may name a part, which lends its CSS (`diff` takes `code`). `floe part list --all` lists the looks, floe's and yours. `floe template guide` says what each but `zoom` draws (*Looks to borrow*); `zoom` is the frame that zooms and its toolbar, which a part takes with the function `zoom`.

| Way to take a look | Where | What it makes |
|---|---|---|
| `looks: [panel]` in `part.yaml` | A part or look | Its box has the look; floe's parts take looks so (`floe part show <part>` prints a `looks:` line) |
| `data-floe-look="panel"` | An element that is no part's box | The element has the look in every theme. Several: `data-floe-look="panel cards"` |

A theme's rules for a look draw every part that takes it. A card in an element that takes `cards` may take `panel` itself (`<div data-floe-look="cards"><div class="card" data-floe-look="panel">`): the cards, the outer, draw it as they draw the cards of a part.

- Do not take two looks that write `marks:` on one element in HTML (`data-floe-look="numbered marked"`, or a box of `links` that takes `marked`): both rules reach its rows, and floe does not refuse it.

### Names floe gives

| Name | Where | What it is |
|---|---|---|
| `data-floe-part` | Box of a part | Names the part; its CSS is scoped to it |
| `data-floe-look` | An element that takes a look | Names the looks it takes |
| `data-floe-tone` | Any element | The tone it draws in (*Tones*) |
| `data-floe-text` | The box of drawn Markdown (a box of no part) | Its blocks are spaced `--floe-room` apart |
| `data-floe-copy` | An element of a part | Its text is what the copy button copies |
| `.title` | An element in a `.card` | A card's title, which the look `cards` draws |
| `.floe-hidden` | A word | Shown to no one, read to a screen reader (*Rows with a mark*) |
| `.floe-zoom` | A frame | What the function `zoom` zooms |
| `.floe-copy` | The copy button | The button `copyButton` draws |

## The CSS layers and @scope

floe puts a page's CSS into `<style>`s before its `</head>`, in cascade layers, each later one winning over those before it whatever the selectors:

| Layer | What is in it |
|---|---|
| `floe.palette` | The palettes: floe's `default`, then the theme's, then the tones |
| `floe.base` | floe's base: the page's background and text colour on its root, and text, lists, links, code and quotes |
| `floe.theme` | The theme's `base.css`, scoped as the template's `page.css` is: it reaches the page and the boxes of the parts, not what they hold, nor a text's Markdown. Then the theme's `markdown.css`, which reaches the whole page |
| `floe.looks` | The looks of elements that take one without being a part's box (`data-floe-look`) |
| `floe.marks` | The layout of the rows with a mark (*Rows with a mark*) |
| `floe.page` | The template's `page.css`, over floe's start of every page in a sublayer under it: the body's margin of 0 and the column `.floe-column` |
| `floe.parts` | The parts' style, each part after the looks it takes, then the theme's files of the parts |

The layers have sublayers, the names DevTools shows: `floe.looks.boxes` and `floe.parts.boxes` (the list layout below), `floe.looks.depth<N>` and `floe.parts.depth<N>` (a layer for each depth of nesting: the outer wins), and `floe.page.defaults` (the body's margin and the column).

- A part's `part.css`, and a theme's file of it, are scoped to the elements that have its look, from their own on, short of what the parts in them hold. Write the element itself as `:scope`. A part's style reaches the box of a part it holds, but not what that part holds.
- A theme's file of a part wins over its `part.css` (its `@media print` too). floe's themes, `default` among them, hold such files for floe's parts and looks (`floe theme show default`), which draw a part that takes one of those looks too.
- A part's `}` that closes no block, and an `@import`, are refused (*What `floe part check` refuses*).
- Nested, the outer wins. `floe.looks` and `floe.parts` are each split into layers by the depth of the element a style is scoped to (the element that takes the look or is the part's box): how many parts' boxes and elements that take looks stand around it, the outer the stronger. So every rule of a look or a part wins over the rules of a look or a part scoped to an element it holds, whatever the selectors and however near the scope: a look's rule for an element in it that takes another look, a part's for a part's box in it, and a rule for what such an element holds too (e.g. `.card :is(p, ul, ol)` over the text of a card that takes `panel`, or `:scope :is(th, td)` over the cells of an element in it that takes `table` too). Between a look and a part the order of the layers above stands: a look's style loses to the box of a part it holds. On one element, a theme's file of a look wins over the `part.css` of a part that takes it.
- `!important` turns the layers' order round (an earlier layer's wins), and floe does not refuse it.

A list (`ul`, `ol` or `menu`) that is a part's box or takes a look has no marks, padding or gap between its items, and is laid out in a grid: floe sets so in the lowest layer of `floe.parts` and of `floe.looks`, under every part's and look's own rules.

- Do not write rules in a part to leave a list's marks, padding or gap out (floe does so, and a template's `page.css` or a theme's `base.css` that writes a rule for every list changes none of the four of a part's box: its margin is still reached; its items, `li` and `::marker`, are not, as they stand in the part's box, but for a theme's `markdown.css`, which reaches the whole page and so the items too). An element that takes a look keeps the four against a theme's such rule, and its template's `page.css` adjusts them.
- Set `list-style` and the padding in the part's `part.css`, or hold the list in its box, for a part whose box is a list it wants marked.
- The CSS of a part or look, read from `floe part show <part> part.css`, draws without colours or the list layout in a page drawn without floe: that page has no palette, and floe leaves a list's marks out as it draws the page. For diagrams in such a page, take the assets of the fragment a diagram is drawn as, which carry `default`'s palette (`floe diagram render --help`: `--no-assets` leaves them out).

### Variables floe sets

floe sets these, or registers them with their default, where the Set in column says; a part or a look reads them. The palette's variables are set on the page too; read them as they are (`var(--accent)`, `var(--radius)`, `var(--mono)`): `floe theme show default palette.css` prints every one with its value, and `floe theme guide` says what each changes (*Palette*).

| Variable | Set in | What it is | Default |
|---|---|---|---|
| `--floe-room` | The page's root | The space between parts that stand one after another | `0.75rem` |
| `--floe-pad-block` | The page's root | A box's padding above and below its text, from its frame (a card, what takes `note`; the look `panel` pads nothing itself) | `1.1rem` |
| `--floe-pad-inline` | The page's root | A box's padding at its sides, and the padding at the sides of a row of the look `rows` | `1.3rem` |
| `--floe-row-pad` | The page's root | The padding above and below a row, which the look `rows` gives each of its rows | `0.4rem` |
| `--floe-text-small` | The page's root | The small text of parts: a pill, a chip, a ring's number, a key, a table's heading, a term, a date, a count, a note beside a value | `0.8rem` |
| `--floe-text-aside` | The page's root | The slightly larger small text: a card's or a step's words, a note's title, a small title over a list, a tab's name, a value's unit | `0.9rem` |
| `--floe-pill-end` | The page's root | The most a pill's corners round (a pill that wraps is a label) | `0.75lh` |
| `--floe-copy-size` | The page's root | The copy button's size | `30px` |
| `--floe-copy-gap` | The page's root | The gap around the copy button | `0.4rem` |
| `--floe-copy-room` | The page's root | The padding a text keeps at its right for the copy button (`0px` on paper, where the button is left out) | `calc(var(--floe-copy-size) + 2 * var(--floe-copy-gap))` |
| `--floe-shadow-color` | The part's or look's file, on the box or element that casts (registered by `@property`) | The colour of the shadow a box casts (*Shadows*) | `transparent` |
| `--floe-inset` | The same | A shadow drawn within the box (`initial` clears it; `none` voids the box's shadow too) | `0 0 #0000` |
| `--tone` | An element with `data-floe-tone`; the page's root and a part's box to accent's | The colour of the tone of the element it is read on (*Tones*) | — |
| `--tone-soft` | The same | The pale background of that tone | — |
| `--tone-fill` | The same | A theme's own fill of that tone, else the pale background (*Tones*) | — |

The text of code (its lines, a tree's names) follows code's size, not the two small sizes. Both are in rem: small text in large text is no larger.

### Variables a part sets

A part or look sets these where the Set in column says, and a part or look that takes it reads them.

| Variable | Set in | What it is | Default |
|---|---|---|---|
| `--floe-pill-wrap` | An element of the part that holds pills (`tags` and `asks` set it on theirs) | `normal` lets a long pill wrap in it. A pill (`cell`) otherwise keeps to one line wherever it stands, in a table's or a grid's column too. floe's base resets it on what a part's box holds, so a value on a part's root does not reach the parts in it | not set |
| `--face` | The root of the part and its children (`:scope, :scope > *`), for a part that fills a box with colours of its own | The box's background (*Colours*) | not set |
| `--on-face` | The same | The text colour on it, which links, a line's code and the focus ring in the box are drawn in (*Colours*) | not set |

## How a part's CSS takes effect

Each item gives the action, then the reason in parentheses.

### Page and layers

- Name `@keyframes`, `@counter-style` and `@font-face` with your part's name, `mypart-fade` (the names are the page's, not held by the scope).
- Expect, of two files that make one such name, the one of the higher layer to win wherever it is used: a part's over the template's, the outer part's over the inner's. `@font-face` replaces only a face whose descriptors (`font-weight`, `font-style`, `font-stretch`, `unicode-range` and the rest) are all the same; one that differs joins the same family.
- Paint a state that paper leaves out, such as the picked tab, under `@media screen` (on paper every tab shows under its title).

### Space

- Do not write a margin on a part's box (what places the part spaces it `--floe-room` from what stands next to it; a margin written there adds to that space in a section, a stack or columns).
- floe's base gives the blocks a part's box may be (`p`, `ul`, `ol`, `dl`, `figure`, `form`, `menu`, `fieldset` and the like) no margin. The part sets the space in the flow of its own Markdown, between its paragraphs and before and after a part's box in it, such as a Markdown table.
- Do not write a padding on the rows of a part that takes the look `rows`, or at the sides of an element in a row (a padding on a row replaces the look's; one on an element in it, such as a `summary` or a `div`, sits twice as far in).
- The look `rows` gives each row `--floe-row-pad` above and below and `--floe-pad-inline` at the sides, its box's `::after` too; a line indented within its row writes the indent alone. A part whose rows do not take `rows` reads `var(--floe-row-pad)` (`padding-block: var(--floe-row-pad)`).
- An element that opens a row across its width reaches over the row's padding at the sides and pads itself as much (`margin-inline: calc(-1 * var(--floe-pad-inline)); padding-inline: var(--floe-pad-inline)`).
- Keep what a box draws outside itself (a shadow, a pseudo-element standing out, an outline) within `--floe-room` on every side, and in a size that does not follow the box's (nothing makes space for it, so that a larger one meets the next box or the page's edge; a tilt or a scale reaches out further the bigger the box).

### Colours

- Set `--face` and `--on-face` on `:scope, :scope > *`, and paint with `background: var(--face); color: var(--on-face)`, to fill the root of a part with colours of its own (floe's base draws the links and the focus ring in `--on-face`, and a line's code on `--face` within a 1px inner ring of `--on-face`, and sets both to `initial` on what a part's box holds: without the two on the box and its children these sink into the fill). A part that fills a box that is not its root and holds links, a line's code or a focus ring in it sets them on that box.
- Write a colour set on text of no background of its own as `var(--on-face, …)` (`color: var(--on-face, var(--good))`), so that it reads in a face around it, a theme's filled card among them.
- Set `color` with the background in a look or part that paints a background of its own (the look `panel`, `note`), `color: var(--fg)`, and take no face from around it (so that it reads in a band). A part of no background of its own takes the colour around it, a band's or a template's; a part on a background of its own does not take a template's text colour. A theme that takes that background away gives both back, with `:scope { color: inherit; --face: inherit; --on-face: inherit; }` and `:scope > * { --face: inherit; --on-face: inherit; }`.

### Tones

A tone (one of `tone.json`'s) is marked by `data-floe-tone` on the element drawn in it, not by a class: `<span data-floe-tone="good" data-floe-part="cell">`. floe gives that element, and what it holds short of a part's box, the tone's colours as `--tone`, `--tone-soft` and `--tone-fill` (from the palette's `--good`, `--good-soft`, and `--good-fill` or else `--good-soft`); a part's box, and the page, take accent's, so that a part takes no tone from one around it (a text's box, which is no part's, takes the tone around it), wherever the mark stands (a template's or a look's element too). A `data-floe-tone` of no tone is as if not written: a part's box draws in accent's colours, another element in the tone around it.

- Draw in the tone with `background: var(--tone-soft); color: var(--tone)`, and write a rule of a tone only for what is no colour of it (a mark, a box's text).
- Set `--tone`, `--tone-soft` and `--tone-fill` on the part's root only for a part whose box takes no `data-floe-tone` itself, and whose things without a tone take other colours (e.g. `stats`: `--muted` and `--muted-soft`). Do not set any of them on a `:scope` that can take the mark, as `<span data-floe-tone="good" data-floe-part="cell">` does (the part's rule is in `floe.parts`, over `floe.palette` where floe builds the tone's colours: it wins, and the box's tone is lost). For a part that draws them in each shape's own colours (e.g. `diagram`), set `--tone`, `--tone-soft` and `--tone-fill` to `initial` on the root and write each shape's colour after `var()` (`var(--tone, var(--line))`).

### Shadows

- Do not write `box-shadow` on a part's box, a look's element or a quote (floe's base writes it from the shadow variables, and one written over it drops them). Set `--floe-shadow-color` and, for a shadow within, `--floe-inset` on the box that casts (`.card { --floe-shadow-color: var(--fg); }`, on an element that is a part's box or takes a look; the colour reaches no box inside).
- The theme's `palette.css` gives the shadow's shape, `--floe-shadow-x`, `--floe-shadow-y`, `--floe-shadow-blur` and `--floe-shadow-spread`: floe's `default` theme gives none, so no shadow shows there.

### Scrolling boxes

- Write `overflow-x: auto` and `contain: inline-size` in one rule under `@media screen`, in a box of yours that scrolls sideways, and fold what it holds on paper, where what scrolls is not read (floe's `pre` folds its lines there). Without `contain: inline-size`, the box tells its content's width outside, and a grid or flex item around it (a list's item, a card) widens to its longest line, and the page with it.
- A `pre` has both from floe's base, which also gives it a background, a border, a radius and padding: a part that draws it otherwise resets them.
- Keep such a box out of a place whose width its content sets: an inline-block, a float, an absolute or other shrink-to-fit box, a width of `min-content`, `max-content` or `fit-content`, a table's cell of auto width, `justify-self: start`, an item of a flex row on `flex-basis: auto` (it shrinks to its border and padding there).
- Make an element in it that paints a background reach to the end of what it holds on screen (`width: fit-content; min-width: 100%` for what folds, `max-content` for lines that never fold): else, scrolled, its background stops at the box's width and what it holds runs out of it.

## Rows with a mark

`marks:` in `part.yaml` says which of the part's elements are rows with a mark at their left, which floe lays out in the layer `floe.marks`.

| Value | The elements that are rows |
|---|---|
| `box` | The part's box; a look's element that takes the look, and the box of a part that takes it |
| `items` | Each `li` child of that, alone: a row of `div`s takes no mark |
| `joined` | `items`, with a line running from each mark to the next |

- A row is a grid with the columns `mark` and `text`. Its `::before` is the mark in `mark`, the mark's box `--floe-mark` across and `--floe-mark-gap` from the text, standing, as what the row holds does, on the baseline of the first line. In a row of `joined` its `::after` is the line to the next mark, from 0.35rem below the mark to 0.35rem above the row's end.
- floe writes where they stand and how big, no colour, corner or content: a part writes its mark's look (`content`, background, frame) and the line's colour, and spaces what the row holds by `> * + * { margin-top }`.
- A part or look that takes a look that writes `marks:` (`looks: [numbered]`) has its rows laid out by it, and MUST NOT write `marks:` of its own.
- Write each row's text in elements (`<li><span>…</span></li>`): text written straight in a row becomes a grid item for each run of text, each placed in the next free cell, so that the row breaks onto rows of its own (`Restart <code>api</code> now` takes three rows).
- A mark of letters before each row of a list is a row's mark too, by the look `marked` (`<ul data-floe-look="marked">`, or `looks: [marked]`). The part writes its letter and colour (`li::before { content: "×" }`) and each item's text in one element, so that a line of text and code flows as one.
- A sign before one title, word or cell (a prompt, an →), and a list's own marker (`list-style`, `::marker`), are no row's mark.
- Do not write `display`, `gap` or `grid-template` on a row in a part's `part.css`, a theme's file of the part, or a look file the part takes (they are in `floe.parts`, over floe's layout in `floe.marks`: the rule wins and the marks lose their layout). A template's `page.css` wins too, but it reaches the part's box and not what the box holds: rows of `box`, not the `li` of `items` and `joined`.
- Expect a rule for those on an element that is no part's box (an element that takes a look, such as the `<ul>` of a template) to change nothing when it is in a look's file or a theme's file of that look (they are in `floe.looks`, under `floe.marks`).
- Expect a theme's `base.css` to change nothing there or anywhere (it is in `floe.theme`).
- In a row of `box` or `items`, write `position: absolute; position-anchor: --floe-mark; top: anchor(top); left: anchor(left)` on an `::after` that is to lie over the mark (`position-anchor` alone places nothing).
- An `::after` with no `position` is an item of the grid: it adds a row in the column `mark` under what the row holds, and one wider than the mark moves the text too.
- Write the words that tell what a mark alone means as hidden text, `<span class="floe-hidden">{{word "done"}}</span>`, with its `words.yaml`, outside what the copy button copies (`data-floe-copy`).
- Put the hidden text within a row's text (`<li><span><span class="floe-hidden">…</span>…</span></li>`), not as the row's child (it would shift the space between what the row holds).
- A row that holds no element of text, such as a Markdown box, takes it as its first child and counts its gaps from after it: `:scope > :not(.floe-hidden) + *`.
- Do not write a word in a `content` string (CSS text does not pass `words.yaml`, so a word stays in one language whatever the page's).
- Set `--floe-mark` and `--floe-mark-gap` on the part's or look's own root (a value around it, a card's number's, would reach its marks).

| Variable | Set in | What it changes |
|---|---|---|
| `--floe-mark` | The root of the part or look that draws the rows | The size of a row's mark: the box it is drawn in |
| `--floe-mark-gap` | The same | The gap between a row's marker and its text |

floe's parts that draw rows set their own values on their roots: `floe part show numbered part.css` prints one.

## Code's look

A part that takes code's look (`looks: [code]`, as `diff` and `terminal` do) reads these variables, which `code` sets on its root; they are floe's, not a theme's to set.

| Variable | Set in | What it is | Default |
|---|---|---|---|
| `--floe-code-pad` | `code`'s root | The padding at a line's sides | `1.1rem` |
| `--floe-code-room` | `code`'s root | The padding after a line, with the copy button's when the part draws it | `var(--floe-code-pad)` |
| `--floe-code-faint` | `code`'s root | A line's faint text | `color-mix(in srgb, currentColor 70%, transparent)` |
| `--floe-code-head-faint` | `code`'s root | The heading's faint text | `color-mix(in srgb, var(--code-head-fg, var(--code-fg)) 85%, transparent)` |

The padding is on each line, an element `pre` holds, not on `pre`.

- Make each line an element: one element a line in one `<code class="lines">`, as code and a diff do; one `<code>` around the whole; or one element a line in `pre`, as a terminal does (text `pre` holds outside an element has no padding, and one inline among it has padding around it).
- Do not write padding on a line (it drops code's padding).
- Lines in a `<code class="lines">` have code's look paint their background to the end of the line and move the padding onto each (e.g. `diff`). A part whose lines differ further moves it onto them by the variables: `:scope pre > code { padding-inline: 0; } :scope code > span { padding-inline: var(--floe-code-pad) var(--floe-code-room); }`.
- The look styles the part's `figcaption` as a heading row, in it `.names` (holding `.title` and `.file`) and `.lang`, and in the lines `.lines`, `.mark` and `.numbered`: a part that takes the look and uses those names is styled by it.
- Code's look makes a block of a `<code>` that `pre` holds. A `<code>` anywhere inside a `pre` has none of the inline look (its size, background, shadow and padding: floe's base); a `<code>` elsewhere in such a part, in its heading, keeps it.

## What `floe part check` refuses

`floe part check` reads every part, look and shared schema of yours and every theme (whose files need the parts), over floe's own, and prints every mistake (exit code 1; nothing when they are right). With parts of yours it compiles floe's templates with them too. It refuses, by kind:

- a name, a link, a file or a directory that breaks the rules of *Where parts are* and *Files*;
- a part without a `part.html` or `part.css`, a look without a `part.css`, and a look with a file a look does not read;
- a `part.yaml` that is not keys and values, a key it does not take, a `description:` in a part's `part.yaml`, a look's `description:` that is not text, a `looks:` that is not a list of names, a name that is no part or look, a part or look that takes itself, directly or through the looks it takes, a `marks:` that is not `box`, `items` or `joined`, and `marks:` written twice in a part or look and the looks it takes;
- a `schema.json` that is not JSON (a key twice in an object, more than one value, an empty file), a mistake the JSON Schema library reports, and what *schema.json* says MUST NOT be;
- a `checks.yaml` that is not a list of checks of `expr` and `message` (and `for` and `field`), a check with an empty or missing part, and a `for` that goes through a key the schema forbids (`additionalProperties: false`);
- a `words.yaml` that is not language tags each with names and their text, and a word that is not text;
- a file floe cannot read as written, in any file: YAML or JSON that is not UTF-8 or holds a byte order mark inside, JSON that holds a lone surrogate, and YAML that floe would read otherwise than YAML 1.2 does (a `#` that cuts a value short, `{a, b: 1}`, a number read differently), and two language tags of `words.yaml` that are one (`en-GB` and `en_gb`);
- YAML that floe does not take: anchors, aliases, merge keys, tags, and a second document;
- a `part.html` that does not parse (a syntax mistake, an unknown function), and what html/template's escaping refuses;
- CSS floe cannot read: a block, string, comment or `url()` left open at the end of the file, a `}` that closes no block, a string that runs past its line, an `@import`, an escape (`\`) outside a string, a `url()` or a comment, or a `url()` that is not one address or one quoted string alone.

The check tests files, names and syntax, not values or places. A part's CSS that breaks a *How a part's CSS takes effect* item (a `box-shadow` on its box, a margin on its box, a padding on a look's rows, a variable read where nothing sets it), and a `part.html` that leaves out `data-floe-part`, pass with exit code 0.
