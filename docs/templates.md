[English](templates.md) | [日本語](templates.ja.md)

# Templates

A template draws a page from a data file (`floe publish <data.yaml>`). floe has one built in, `report`; a template of your own is a directory, and it takes the place of a built-in one of its name.

## Where templates are

- A template of your own is a directory `$FLOE_TEMPLATES_DIR/<name>/`, named after the template. `FLOE_TEMPLATES_DIR` defaults to `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`.
- The directory may be a symlink, to a clone of a repository for example. A file in it that leads out of it, through a symlink, is refused.
- A template's name is lower case letters, digits, `-` and `_`, starting with a letter or a digit. A file, or a directory of another name, is not a template.
- A broken template of your own stops the publish with an error: floe does not fall back to the built-in one of its name.

## Files

```text
my-report/
├── template.yaml    # the floe it needs (optional)
├── page.html        # the page, a Go html/template
├── page.css         # the page's CSS, under the parts' (optional)
├── schema.json      # the items a data file may hold (JSON Schema)
├── example.yaml     # a data file to start from
├── checks.yaml      # relations between items (optional)
├── computed.yaml    # values computed from the data (optional)
├── assets/          # images page.html shows with asset (optional)
└── themes/
    ├── default.css  # the template's default theme, for the page (required)
    └── print.css    # more themes (optional)
```

To start one, copy another with `floe template new my-report --from report`. `floe template show <name>` prints what a data file of a template may hold, and `floe template check <dir>` checks a template before you use or share it: its files, then its `example.yaml` through every stage of a publish ([commands.md](commands.md#floe-template-check-dir)).

## template.yaml

```yaml
floe: 1.2
```

The template needs floe 1.2.0 or later, of major 1. Under major 0, a minor version may change what templates can do, so `floe: 0.2` takes floe 0.2.0 or later of 0.2. With another floe, a publish stops before the data is checked, naming the floe the template needs. A template without `template.yaml` is drawn by any floe, and `floe template check` warns of it.

## page.html

`page.html` is a Go [html/template](https://pkg.go.dev/html/template), which escapes what it inserts. It sees:

| Name | What it is |
|---|---|
| `.Title` | The page's title: `--title`, a title pinned on the management page, the data file's `title:`, else the slug |
| `.Lang` | The page's language: the data file's `lang:` |
| `.Data` | The data file's items, without `lang`, `parts-theme`, `template`, `theme` and `title`, which floe reads itself (`.Lang`, `.Title`; floe puts the themes in by itself); a schema that declares one of them, or lists one as required, is refused wherever the template is used. An item the schema declares but the data leaves out is nil, so `{{with .Data.lead}}` works; a name the schema does not declare stops the drawing. An item of `"format": "html"` goes in as HTML and one of `"format": "markdown"` as its HTML; a diagram goes in as its SVG, and a script as its element and the call of its function |
| `.Calc` | The values of `computed.yaml`, by name |
| `markdown` | Markdown text as HTML (`{{markdown .text}}`), raw HTML in it left out, a table in the look of the table part (`<div data-floe-part="table">`) |
| `markdownLine` | A line of Markdown, such as a table cell, as HTML without a paragraph around it (`<td>{{markdownLine .}}</td>`): code, emphasis and links, while a line like `-`, `1.` or `# x` stays text. Lines, indented or apart, are joined as one. Raw HTML in it is left out |
| `num` | A number with its digits grouped (`{{.Calc.total \| num}}` is `1,234,567`), or rounded to significant digits (`{{.Calc.rate \| num 3}}` is `0.00648`) |
| `bytes` | A count of bytes in a binary unit (`{{.Calc.size \| bytes}}` is `1.5 KiB`) |
| `asset` | An image of the template's `assets/` as a data URI (`<img src="{{asset "logo.svg"}}">`) |

`page.html` holds no `<style>`, which floe refuses at its line: floe puts a page's CSS together (see [Parts](#parts)), from `page.css` and the theme the data file picks. They are scoped to the page, where `:scope` is its root: write the page's variables on `:scope`, as `:root` and `html` name nothing there and are refused at their line. A `<link>` to a stylesheet, such as Google Fonts, stays: it is out of floe's layers and wins over them. Load a stylesheet so, as `page.css` and the themes hold no `@import`, which a browser does not read in a layer and floe refuses at its line, nor a `}` that closes no block, which would close floe's layer around them, nor `</style`, in a comment or a string too, which would close floe's `<style>` and leave what follows to be read as the page's HTML (write it as `<\/style` in a string and as `%3C/style` in a `url()`).

A page drawn from a data file holds all it shows, and its scripts cannot send what it holds out: its Content-Security-Policy lets it load only images, fonts and media of data URIs (and `blob:`), inline styles and scripts, Google Fonts, and libraries from cdnjs, jsDelivr and unpkg (their scripts, styles and fonts; not `eval` or WebAssembly). `fetch`, XHR, an image from elsewhere and a form are blocked, and so is the management page's API. Its download keeps the same policy in a `<meta>`. The page still asks Google Fonts and those CDNs for what it loads from them. CSP cannot stop a script from moving the page, opening a window, WebRTC or DNS prefetching, so this guards against a slip, not a hostile template: use templates you trust.

## schema.json

A [JSON Schema](https://json-schema.org/) (2020-12) of the data file's items. Write `"additionalProperties": false` so that a misspelt key is refused. `template`, `theme`, `parts-theme` and `title` are read by floe and are not declared.

A `format` is one of JSON Schema's that floe checks (`date`, `email`, `uri`…), or one of floe's that tells how to read a text: `html`, `image` or `markdown`; `floe template check` refuses another, which this floe would let any value pass. Floe's go under `properties` and `items` alone, where floe looks for them: under `anyOf`, `$defs` and the like, the item would be drawn as text. Declare `"type": "string"` alone for them. A diagram or a script is a [part](#parts), not a format: `floe template check` refuses `"format": "diagram"` and `"format": "script"` anywhere in a schema, and one under `properties` and `items` stops the template wherever it is used.

An item of `"format": "markdown"` is Markdown text: `{{.Data.text}}` draws it as HTML, as `markdown` does. It is HTML already, so `{{markdown .Data.text}}` stops the drawing.

An item of `"format": "html"` goes in as it is, but holds no `<style>` or `<link>`, wherever it is (a comment's text too, as an item may leave markup open that the next ends), refused at its line and column: floe puts a page's CSS together, so style the HTML with `style` attributes.

An item of `"format": "image"` is the path of an image, relative to the data file's directory and under it, without `..` or names starting with a dot. The image goes into the page as a data URI, for `<img src="{{.Data.chart}}">`, so that the page holds all it shows.

An item the part `diagram` draws (`"$ref": "floe:///parts/diagram.json"`) is a diagram spec, written in place as [`floe diagram render`](commands.md#floe-diagram-render-specyaml) takes it. It is checked as `floe diagram render` checks a spec, its mistakes reported at the data file's lines and columns, and the ids of a page's diagrams must differ. `{{.Data.plan}}` draws it as inline SVG, and the page gets the diagrams' style and zoom once, at its end. Diagrams take their colours and fonts from the theme's CSS variables, as [parts](#parts) do.

An item the part `script` draws (`"$ref": "floe:///parts/script.json"`) is something that moves, written as `js`, a function, and `data`, any value: `{{.Data.calculator}}` draws an empty element and calls the function with it and the data, `function (el, data) { … }`. The data goes in as JSON, the keys of a mapping in the order of their names: write a list where the order matters. floe checks the shape alone (`js` is a function's text, and nothing but `js` and `data`) and refuses a `js` holding `</script` or `<!--`, where the page would end its script; what the function does shows in the browser. It runs where it stands, as the page is read: the elements after it are not there yet, nor the web fonts, so wait for `load` (`addEventListener('load', …)`) for those. It runs in the page's sandbox under its CSP (see [page.html](#pagehtml)).

## Parts

floe has parts that draw items of a template: `table` (with pills in its cells), `cards`, `note`, `code`, `terminal`, `stats`, `progress`, `steps`, `checklist`, `compare`, `options`, `facts`, `diff`, `tree`, `links`, `findings`, `details`, `matrix`, `decision`, `risks`, `fields`, `timeline`, `bars`, `faq`, `glossary`, `keys`, `tabs`, `cell` (a table's cell, a line of Markdown or a pill `{ pill, tone }`, which `table`, `facts`, `findings` and `decision` hold), `stack` (parts one under another, which `details` and `tabs` hold), `diagram` and `script` (both told above). `note` is Markdown `text` with a `tone`, and a `title` above it if one is given. `code` is `text`, code as it is, under its `file` with its `lang` at the right. `terminal` is `steps`, each a command (`run`, its later lines going on with it) and what it printed (`out`), at the prompt of its `shell`, whose name goes above them: `bash` or `sh` (`$`), `zsh` (`%`), `powershell` (`PS>`) or `cmd` (`>`); left out, no name and `$`. Both are dark in light mode too, as an editor and a terminal are. The style draws the prompt, so a copy of the commands leaves it out; a `run` whose first word is a prompt (ending in `$`, `#`, `%`, `>` or `❯`) is refused. `stats` are large numbers, each a `label` and a `value` in quotes with a `unit` and a `note` coloured by its `tone`; `progress` is bars, each a `label` filled as far as `done` goes of `of`. `steps` are numbered, each a `title` over Markdown `text`, `done`, `doing` or `todo` (left out) by its `state`; `checklist` is items of a line of Markdown, ticked when `done`, under the count of those done. `compare` is `rows` of a thing (`what`) `before` and `after`, under the headings `before` and `after`, a row the same both sides drawn faint; `options` are cards of a `title` over Markdown `text`, `pros` and `cons`, the one to take ringed under its `pick`, a word such as Recommended; `facts` are a `name` with a `value`, a line of Markdown or a pill as a table's cell is. `diff` is `text`, a diff as git writes it, in code's look under its `file`, the counts of lines added and removed and its `lang`; `tree` is each a `path` (a folder's ending in `/`) nested into the folders it names, with a `note` and a `change` (`added`, `removed` or `changed`); `links` are cards of a `title` linking to its `url` (http, https, mailto or a path) over a few words (`text`) and the URL itself, under a link's mark. `findings` are what a review found, each its `level` (`must`, `should` or `nit`), where it is (`at`), what it is (`text`), why it was left (`why`) and a cell of what became of it (`status`); `decision` is a `title` large, why in Markdown `text`, the options set aside (`rejected`), under a cell of its `status` and its `date`; `risks` are each `what`, how `likely` and how heavy its `impact` (`low`, `mid` or `high`), coloured by both, and what is done (`fix`). `matrix` is `rows` of a thing (`what`) and `cells`, as many as the `columns`, each `yes` (✓), `part` (△) or `no` (×), or `{ mark, note }` with a few words under it; `fields` are each a `name`, its `type`, whether it is `required`, what it is (`text`) and its `default`. `timeline` is each a `date`, a `title` and Markdown `text`, its dot in its `tone`; `bars` are each a `label` and a `value` no less than 0, the largest the longest, in its `tone`. `faq` is each a question (`q`) opening its Markdown answer (`a`), `open` from the start if so written; `glossary` is each a `term` and Markdown `text`; `keys` are each the `keys` pressed together and what they do (`text`). `details` folds `parts` under a `title`, `open` from the start if so written, and `tabs` shows the `parts` of one `title` at a time, without a script (all of them on paper): each part they hold is one key, `text` (Markdown), `code`, `terminal`, `diff`, `note` or `table`, which floe checks and draws as it does any part. A part draws its few words, such as the headings of `risks`, in the language of the page, English for one floe has no words of. A schema refers to one with `$ref`, under `properties` and `items`, where floe looks for them; `floe template check` refuses one under `anyOf`, `$defs` and the like, which would never be drawn.

```json
"risks": { "$ref": "floe:///parts/table.json", "description": "What may go wrong" }
```

`page.html` puts the item where it goes, `{{.Data.risks}}`: floe checks it against the part's schema and checks, its mistakes at the data file's lines and columns, and draws it as HTML. `floe template show` tells the part's items, after the description written beside the `$ref`. A `$ref` to a part floe does not have is refused, with the parts it has.

A part's element is its box, which names it, `data-floe-part="note"`: one name, as an element is the box of one part. A part takes the look of others by its `part.yaml`, `looks: [table]`, and the looks those take in turn, each before it, which it adjusts. floe has looks alone too, a `part.css` without a schema to draw: `panel`, the box most parts are drawn in, and `rows`, rows in a panel. An element that takes a look without being a part's box names it by `data-floe-look`: a card of `cards` takes the panel's look so, and a template's own element may (`<section data-floe-look="panel">`). A page gets the style of each part and look it names once, a table of Markdown and HTML of the data that names a part included (`<span class="good" data-floe-part="cell">`, a pill in the tone `good`), with the looks they take, followed by the parts theme; `floe template check` refuses a name in `page.html` that is no part's, a `data-floe-part` that is not one part's name as written (`"table compare"`, `" table"`), and one that names a look alone, which has no box; a page whose HTML has one of those two is refused when it is drawn, but for a name in text, a comment or a script's text, which is no attribute and only brings its part's style. A part's style, and the parts theme's for it, reach the elements that have its look and the box of a part they hold, but not what that part holds. They come in the order that the more specific rule wins, else the later: the parts' style, each part after the looks it takes, then the parts theme. An element that takes a look without being a box has it a layer under the parts' and the template's, so that the part it is in, or the template it is the template's own of, adjusts it whatever the selectors; such elements share that layer, so make an element that holds others taking looks a part's box (`data-floe-part`), whose rules for them come over theirs. `compare`, `fields`, `findings`, `matrix` and `risks` take the table's look, `stats`, `options`, `facts`, `links` and `decision` the cards', `diff` and `terminal` code's, and `checklist`, `faq`, `glossary` and `keys` the look `rows`, the other parts drawn in a panel `panel`'s, so that the parts theme's rules for those draw them too; of the cards, `cards` and `options` are numbered.

The parts theme is one of floe's, the same for every template, which the data file picks with `parts-theme:` (else `default`; `floe template show` lists them): it restyles each part or look in a file of its own, `<name>.css`, which reaches every element that has the look (`panel.css` every panel, `code.css` code, a diff and a terminal), and, in files that go into every page, one of no part too, Markdown's quotes (`blockquote`) in `base.css` and the page's colours and faces ([the palette](#the-palette)) in `palette.css`. floe puts the page's CSS into `<style>`s before its `</head>` (at its end without one), one for floe's base, then one for each file: the palettes, each part's style, each file of the parts theme, the template's `page.css` and its theme, which keeps a comment or a block a file leaves open to its own; the CSS is in five cascade layers, each winning over the one under it whatever the selectors: `floe.palette`, the palettes, under `floe.base`, which styles text, lists, links, code and quotes as `report` does, under `floe.looks`, the looks of elements that take one without being a part's box, under `floe.page`, the template's `page.css` then its theme, under `floe.parts`, the parts' style, the diagrams' and then the parts theme. The template's CSS is in `@scope (:root) to ([data-floe-part] > *)`: it reaches a part's element, so that it places the part, but not what the part holds, which takes from it only what is inherited (colours, fonts and the variables) and starts from `floe.base`. A part's element carries the part's classes too (a tone such as `good` or `bad`, `numbered`, a shell such as `sh`), which a rule of the template's for a class of that name reaches: pick a part by its name (`[data-floe-part="note"]`), and name the template's own classes otherwise. So the template does not restyle a part, nor a quote the parts theme styles; it styles the page around them. A parts theme sets the colours of code in its `code.css`, on `:root` in its `base.css` only variables of its own names (`--brutal-ink`), and those of the palette in its `palette.css`: `base.css` is in the top layer, where one would win over the colours a template sets too.

### The palette

The page's colours and faces are CSS variables a parts theme's `palette.css` sets, which the parts, diagrams, floe's base and the template draw with. floe's `default` one defines them all and goes into every page, at the bottom; the picked theme's goes after it and changes those it writes, and a `default` of your own goes over floe's too, not in its place. The others, and every colour of a theme without a `palette.css`, are floe's `default`'s.

`palette.css` sets the variables (and `color-scheme`) on `:root { --name: value; }`, those of dark mode in `@media (prefers-color-scheme: dark) { :root { … } }` after it. Any other rule, which would lose to every other in the bottom layer, a `:root` after the dark block, whose colours would win in dark mode too, and `!important`, which would win over every layer and the template's colours, are refused at their lines. A colour set for light mode alone would stay in dark mode, where the others turn dark, and the page would not read: `floe part check` and `floe template check` warn of one, so set each for both modes. A palette that sets on `:root` every colour `default`'s turns dark, and none for dark mode, one look in both modes (a theme always dark), mixes none and is not warned of; nor is a value that follows the mode itself, such as `light-dark(#fff, #000)`.

| Variable | What it is |
| --- | --- |
| `--bg`, `--surface` | The page's ground, a part's face |
| `--fg`, `--muted` | Text, faint text |
| `--line` | Lines and borders |
| `--accent`, `--accent-soft` | The colour that stands out, and its pale ground |
| `--good`, `--warn`, `--bad` and their `-soft` | Good, warning and bad, and their pale grounds |
| `--font`, `--display`, `--mono` | The text's typeface, the headings', and the fixed-width one |
| `--code-bg`, `--code-fg`, `--code-prompt` | The ground of code and a terminal, dark in light mode too; their text, faded for a command's output; a terminal's prompt and the badge of code's language |
| `--code-add`, `--code-del` | A diff's lines added and removed, on the dark ground |
| `--page-bg` | The whole page's ground, a pattern such as squares too (`--bg` by default) |
| `--rule` | The `border` of a rule across the page, such as between sections (`1px solid` in `--line` by default) |

A template draws the page around the parts with them too (`background: var(--page-bg)`, `border-top: var(--rule)`), so that its ground and rules change with the theme. The palette is the bottom layer: a variable the template sets on `:scope` in `page.css` holds in every theme, for a template to keep to a brand's colours; set those of dark mode in `@media (prefers-color-scheme: dark) { :scope { … } }` too (with more conditions, such as `@media screen and (…)`, in `@layer` or `@supports`, or nested in `:scope`), as `floe template check` warns of a colour that changes with the mode set for light mode alone. How wide text and notes grow, `--text-width`, is `56rem` in `default`'s palette, which a template sets over on `:scope` and a theme in its `palette.css`. A `tone` (of a pill, a note or a part of a diagram) is `accent`, `good`, `warn` or `bad`: the colour of `--accent`, `--good`… and its `-soft`.

## Parts of your own

- Start one as a copy of a part, floe's or your own: `floe part new quote --from note` copies `note` into `$FLOE_PARTS_DIR/quote/`, its box named `quote`, and the file of `note` of each parts theme into `themes/<theme>/quote.css`. `floe part show <name>` tells what a part takes, and `floe part check` checks yours, all of them ([commands.md](commands.md)).
- A part of your own is a directory `$FLOE_PARTS_DIR/<name>/`, named after the part, of the files floe's parts have: `schema.json`, `part.html` and `part.css`, and `part.yaml`, `checks.yaml` and `words.yaml` if it needs them. A look alone of your own is a `part.css`, and a `part.yaml` of the looks it takes if it takes any and of its description, `description:`, which `floe part list` shows (a part's is its `schema.json`'s, which `part.yaml` refuses). `FLOE_PARTS_DIR` defaults to `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`. The directory may be a symlink; a file in it that leads out of it is refused.
- Its name is a template's (lower case letters, digits, `-` and `_`); a directory of another name is told as a mistake, one starting with a dot left alone. `diagram` and `script`, which floe draws itself, `tone`, the tones the parts share, and `base` and `palette`, the names of a parts theme's files for every page, are refused.
- A schema refers to it as to floe's, `"$ref": "floe:///parts/quote.json"`. One of the name of one of floe's takes its place whole, in every template and in the parts drawn in its look (a `table` of your own draws Markdown's tables and `compare` too, which take the looks it takes, not floe's table's); floe's files do not fill in for one it leaves out, and floe's parts themes do not style it, nor the parts drawn in its look, by its file. A look alone cannot take the place of one of floe's that draws.
- Its element is its box, `data-floe-part="quote"`, and its `part.yaml` names the looks it takes: `looks: [panel]` draws it in each parts theme's panel. A `$ref` to a look alone is refused, as it draws nothing. A `$ref` in its schema is read from the schema, as JSON Schema reads it: `cell.json` is `floe:///parts/cell.json`. Its `part.css` is scoped as floe's are: it writes the element as `:scope`, and its classes meet no other part's nor the template's. A `}` that closes no block, an `@import`, `:root` or `html` in a selector, and `</style` anywhere are refused at their line, and a block it leaves open ends with its own `<style>`.
- A part of your own that cannot be used (a mistake in its files, a name it is refused, a part it holds, through `$ref`, holding it in turn, a look it takes taking its look in turn) is refused, with why, to a template that uses it, by its schema or its `page.html`, and to a page that names it, by a table of Markdown too, and floe does not fall back to its own part of the name. A part that takes its look or holds it, floe's too, cannot be used either. `floe template check` warns of each one a template does not use, and `floe part check` refuses them all.

```html
<figure data-floe-part="quote"><blockquote>{{markdown .text}}</blockquote>
{{- with .source}}<figcaption class="source">{{word "by"}} {{.}}</figcaption>{{end}}</figure>
```

A parts theme of your own is a directory `$FLOE_PARTS_DIR/themes/<name>/` of the files floe's have: `base.css` (for every page: its variables, of names of its own, and Markdown's quotes), `palette.css` (for every page: the variables of [the palette](#the-palette)) and `<name>.css` (a part's or a look's, scoped to the elements that have its look), each left out if not needed; a CSS file of another name goes into no page, which `floe template check` warns of, and other files, and those starting with a dot (`.DS_Store`), are left alone. One of the name of one of floe's takes the place of its files of their names, the others staying floe's: `themes/brutal/quote.css` styles a quote of your own in brutal. Put an empty file to leave out one of floe's (but for the palette: floe's `default` one is always at the bottom). Its files are refused at their line as a part's `part.css` is (`:root` and `html` but in `base.css` and `palette.css`), `</style` in a `url()` too: write it as `%3C/style` there. One that cannot be used is refused to a data file that picks it.

A page published, or linked, is drawn with the server's parts, those of the `floe publish` or `floe serve` that started it, and saving a file of a part reloads a linked page.

## Images

An image, of the data or of `assets/`, is a png, jpeg, gif, webp or svg file, by its name and by what it holds, up to 1 MiB. Each name on its path is matched as written, case and all, so that a template made on macOS draws on linux too, and none may be a symlink, wherever it leads. A mistake in an image of the data is reported at its line and column; `floe template check` checks every image of `assets/`, and every one `page.html` names to `asset`, in branches the example does not reach too. The images `example.yaml` names are read from the template's directory: a data file started from the example needs them beside it.

## checks.yaml

Relations between items that a schema cannot say, each checked on every item of a list:

```yaml
- for: access_patterns            # a path of keys to a list (sections.parts: the parts of every section)
  field: key                      # the item's key to point at when it fails (optional)
  expr: item.key in map(doc.attributes, .name)
  message: use an attribute as the key
```

A list on the way of `for` is gone through item by item: `sections.parts` checks the parts of every section, and a failure is reported at `sections[1].parts[0]`. `field` is a path of keys under the item alone.

`expr` is an [expr](https://expr-lang.org/) expression that must hold. It sees the list's item as `item` and the data file's items as `doc`. Each item it fails for is reported at its field, with the field's value:

```text
orders.yaml:13:12: access_patterns[0].key: use an attribute as the key ("user_id")
```

The paths of `for` and `field` must be declared in `schema.json`, through `properties` and `items`: a misspelt one would find nothing in the data to check. `floe template check` follows them through `properties` and `$ref`, and `for` through `items` too, and refuses a path where a schema without its key refuses other keys (`"additionalProperties": false`, `"unevaluatedProperties": false`, or a `type` other than `object`), and a `for` whose types leave `array` out. A path a schema may take otherwise (by `patternProperties`, `allOf`, `anyOf`, or other keys it allows) passes. A mistake in `checks.yaml` itself is reported at its line and column there; like a data file, it takes no anchors, aliases or tags.

## computed.yaml

Values computed from the data, for `page.html` to show as `.Calc.<name>`:

```yaml
item_bytes: sum(attributes, .avg_bytes)
storage_gb: item_bytes * records.in_1_year / 1e9
```

An expression sees the data file's items that the schema declares, and the other computed values, by name, whatever order they are written in. A name is a letter followed by letters, digits and `_`, and is neither one of the data's items nor a word or function of expr. A value the data cannot give (a division by zero, an item left out that the expression needs) is reported, and a value that would be NaN or ±Inf is too. A mistake in `computed.yaml` itself is reported at its line and column there; like a data file, it takes no anchors, aliases or tags.

## The order of the checks

A data file is checked in this order, and each stage runs only on data that passed the one before. `floe check` runs them all without publishing.

1. Its YAML
2. Its template, the floe the template needs, its themes, the template's `schema.json`, its images and its diagrams
3. The template's `checks.yaml`
4. The template's `computed.yaml`
5. Drawing `page.html`
