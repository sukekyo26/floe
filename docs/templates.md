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
├── schema.json      # the items a data file may hold (JSON Schema)
├── example.yaml     # a data file to start from
├── checks.yaml      # relations between items (optional)
├── computed.yaml    # values computed from the data (optional)
├── assets/          # images page.html shows with asset (optional)
└── themes/
    ├── default.css  # the default theme (required)
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
| `.Data` | The data file's items, without `template`, `theme` and `title`. An item the schema declares but the data leaves out is nil, so `{{with .Data.lead}}` works; a name the schema does not declare stops the drawing. An item of `"format": "html"` goes in as HTML, one of `"format": "diagram"` as its SVG, and one of `"format": "script"` as its element and the call of its function |
| `.Calc` | The values of `computed.yaml`, by name |
| `.Theme` | The theme's CSS, for `<style>{{.Theme}}</style>` |
| `markdown` | Markdown text as HTML (`{{markdown .text}}`), raw HTML in it left out, a table wrapped in `<div class="table-wrap">` |
| `markdownLine` | A line of Markdown, such as a table cell, as HTML without a paragraph around it (`<td>{{markdownLine .}}</td>`): code, emphasis and links, while a line like `-`, `1.` or `# x` stays text. Lines, indented or apart, are joined as one. Raw HTML in it is left out |
| `num` | A number with its digits grouped (`{{.Calc.total \| num}}` is `1,234,567`), or rounded to significant digits (`{{.Calc.rate \| num 3}}` is `0.00648`) |
| `bytes` | A count of bytes in a binary unit (`{{.Calc.size \| bytes}}` is `1.5 KiB`) |
| `asset` | An image of the template's `assets/` as a data URI (`<img src="{{asset "logo.svg"}}">`) |

A page drawn from a data file holds all it shows, and its scripts cannot send what it holds out: its Content-Security-Policy lets it load only images, fonts and media of data URIs (and `blob:`), inline styles and scripts, Google Fonts, and libraries from cdnjs, jsDelivr and unpkg (their scripts, styles and fonts; not `eval` or WebAssembly). `fetch`, XHR, an image from elsewhere and a form are blocked, and so is the management page's API. Its download keeps the same policy in a `<meta>`. The page still asks Google Fonts and those CDNs for what it loads from them. CSP cannot stop a script from moving the page, opening a window, WebRTC or DNS prefetching, so this guards against a slip, not a hostile template: use templates you trust.

## schema.json

A [JSON Schema](https://json-schema.org/) (2020-12) of the data file's items. Write `"additionalProperties": false` so that a misspelt key is refused. `template`, `theme` and `title` are read by floe and are not declared.

A `format` is one of JSON Schema's that floe checks (`date`, `email`, `uri`…), `html`, `image`, `diagram` or `script`; `floe template check` refuses another, which this floe would let any value pass. `html`, `image`, `diagram` and `script` go under `properties` and `items` alone, where floe looks for them: under `anyOf`, `$defs` and the like, the item would be drawn as text. Declare `"type": "string"` alone for `html` and `image`, and `"type": "object"` alone for `diagram` and `script`: such an item is drawn whole, and a format inside it is not drawn, which `floe template check` refuses too.

An item of `"format": "image"` is the path of an image, relative to the data file's directory and under it, without `..` or names starting with a dot. The image goes into the page as a data URI, for `<img src="{{.Data.chart}}">`, so that the page holds all it shows.

An item of `"format": "diagram"` is a diagram spec, written in place as [`floe diagram render`](commands.md#floe-diagram-render-specyaml) takes it. It is checked as `floe diagram render` checks a spec, its mistakes reported at the data file's lines and columns, and the ids of a page's diagrams must differ. `{{.Data.plan}}` draws it as inline SVG, and the page gets the diagrams' style and zoom once, at its end. Diagrams take their colours and fonts from the theme's CSS variables, as `report`'s default theme defines them (`--fg`, `--bg`, `--accent`…): A theme defines them, or `page.html` does for every theme in a `<style>` of its own; `floe template check` warns of a theme that leaves one out, whose diagrams keep light colours in dark mode.

An item of `"format": "script"` is something that moves, written as `js`, a function, and `data`, any value: `{{.Data.calculator}}` draws an empty element and calls the function with it and the data, `function (el, data) { … }`. The data goes in as JSON, the keys of a mapping in the order of their names: write a list where the order matters. floe checks the shape alone (`js` is a function's text, and nothing but `js` and `data`) and refuses a `js` holding `</script` or `<!--`, where the page would end its script; what the function does shows in the browser. It runs where it stands, as the page is read: the elements after it are not there yet, nor the web fonts, so wait for `load` (`addEventListener('load', …)`) for those. It runs in the page's sandbox under its CSP (see [page.html](#pagehtml)).

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
2. Its template, the floe the template needs, its theme, the template's `schema.json`, its images and its diagrams
3. The template's `checks.yaml`
4. The template's `computed.yaml`
5. Drawing `page.html`
