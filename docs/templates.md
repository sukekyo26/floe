# Templates

How to make a template of your own: the page a data file is drawn into. A data file names its template with `template:` ([pages.md](pages.md) tells how to write one); floe's template is `report`. To make a part or a theme of your own, see [parts.md](parts.md).

## Where templates are

A template of your own is a directory in `FLOE_TEMPLATES_DIR` (default `$XDG_CONFIG_HOME/floe/templates`, else `~/.config/floe/templates`), named after the template. One of the name of floe's template hides it.

```sh
floe template new memo     # makes the smallest template, memo, to fill in
floe template check        # checks every template of your own, drawing its example.yaml
floe template show memo    # prints its themes, schema, checks and example
```

A mistake of a template of your own stops every page, whether it is drawn with that template or not (a page of `report` too), until it is fixed; `floe template check` lists them all. A published page is drawn with the server's templates ([commands.md](commands.md#the-server)); after you change one, `floe rebuild` draws the stored pages again with it.

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

`page.html` is a Go [html/template](https://pkg.go.dev/html/template), which escapes what it inserts. `floe template new` writes the smallest one:

```html
<!doctype html>
<html lang="{{.Lang}}">
<head>
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
| `.Data` | The data file's items, without `lang`, `template`, `theme` and `title`, which floe reads itself. An item the schema declares but the data leaves out is nil, so `{{with .Data.lead}}` works. A part comes drawn (see [Parts in a template](#parts-in-a-template)), as does an item of `"format": "markdown"` (its HTML) or `"format": "image"` (a data URI) |
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

`page.css` lays the page out around the parts: their places, the page's width, its header. It goes into a layer under the parts' styles and the theme's, so it does not restyle a part.

- floe puts it in `@scope (:root) to ([data-floe-part] > *)`: it reaches a part's box, so that it places the part, but not what the part holds.
- Its root is `:scope`. Write the page's variables on `:scope`, not on `:root` or `html`, which are outside the scope.
- A `}` that closes no block, and an `@import`, are refused at their line: load a stylesheet, such as Google Fonts, with a `<link>` in `page.html`.
- Draw with the palette's variables (`var(--muted)`), so that the page follows the theme the data file picks.

`report`'s `page.css`, for one:

```css
:scope { --page-width: 96rem; }
main { max-width: var(--page-width); margin-inline: auto; display: grid; gap: 2.5rem; }
section > h2 { border-top: var(--rule); padding-top: 1.3rem; }
.lead { color: var(--muted); font-size: 1.05rem; }
```

## Parts in a template

A part draws an item of the data: `floe part list` lists them, and `floe part show <name>` prints what one takes. The schema names it with `$ref`, under `properties` or `items`:

```json
"risks": { "$ref": "floe:///parts/risks.json", "description": "What may go wrong" }
```

`page.html` puts the item where it goes, `{{.Data.risks}}`: floe checks it against the part's schema and checks, its mistakes at the data file's lines and columns, and draws it as HTML.

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

`<section data-floe-look="panel">` draws an element of the template as a panel, the box most parts are drawn in, in every theme; `rows`, `cards` or `table` borrow those looks. [parts.md](parts.md#borrowing-a-look) tells the rules.

## The palette

The page's colours and faces are CSS variables that the theme's `palette.css` sets, and that the parts, the diagrams, floe's base and the template draw with. floe's `default` palette defines them all and goes into every page; the theme a data file picks sets them over it.

| Variable | What it is |
| --- | --- |
| `--bg`, `--surface` | The page's ground, a part's face |
| `--fg`, `--muted` | Text, faint text |
| `--line` | Lines and borders |
| `--accent`, `--accent-soft` | The colour that stands out, and its pale ground |
| `--good`, `--warn`, `--bad` and their `-soft` | Good, warning and bad, and their pale grounds |
| `--on-fill` | Text on a face filled with `--accent` or a tone's colour, such as a pill |
| `--shadow` | The colour of shadows |
| `--font`, `--display`, `--mono` | The text's typeface, the headings', and the fixed-width one |
| `--code-bg`, `--code-fg`, `--code-prompt` | The ground of code and a terminal, always dark; their text; a terminal's prompt |
| `--code-add`, `--code-del` | A diff's lines added and removed |
| `--page-bg` | The whole page's ground, a pattern such as squares too (`--bg` by default) |
| `--rule` | The `border` of a rule across the page, such as between sections |
| `--text-width` | How wide prose grows, which a template sets for its page on `:scope` |

floe's base draws the page's ground, colour and face on its root with them (`:root { background: var(--page-bg); color: var(--fg); font-family: var(--font); }`), which `page.css` may set otherwise on `:scope`. A theme of your own may set them too ([parts.md](parts.md#themes)).

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

An image, of the data or of `assets/`, is a png, jpeg, gif, webp or svg file, by its name, up to 1 MiB. A path is relative, with one `/` between names, without `./`, `..` or names starting with a dot.

- `floe template check` reads every image of `assets/`.
- The images `example.yaml` names are read from the template's directory: a data file started from the example needs them beside it.

## The order of the checks

A data file is checked in this order, and each stage runs only on data that passed the one before. `floe check` runs them all.

The parts, looks, shared schemas, themes and templates of your own are read first: a mistake of one stops every page ([parts.md](parts.md#where-parts-and-themes-are)).

1. Its YAML, and the template and theme it names
2. The template's `schema.json`
3. The template's `checks.yaml`, the `checks.yaml` of each part for each item it draws, and the template's `computed.yaml`, together
4. The page drawn: each part and item of a format (images, diagrams, HTML), then `page.html`, and the page drawn as a whole
