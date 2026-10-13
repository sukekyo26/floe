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
└── themes/
    ├── default.css  # the default theme (required)
    └── print.css    # more themes (optional)
```

To start one, copy the built-in `report` from [the source](../internal/tmpl/builtin/report) under a new name.

## template.yaml

```yaml
floe: 1.2
```

The template needs floe 1.2.0 or later, of major 1. Under major 0, a minor version may change what templates can do, so `floe: 0.2` takes floe 0.2.0 or later of 0.2. With another floe, a publish stops before the data is checked, naming the floe the template needs. A template without `template.yaml` is drawn by any floe.

## page.html

`page.html` is a Go [html/template](https://pkg.go.dev/html/template), which escapes what it inserts. It sees:

| Name | What it is |
|---|---|
| `.Title` | The page's title: `--title`, a title pinned on the management page, the data file's `title:`, else the slug |
| `.Lang` | The page's language (`--lang`, default `en`) |
| `.Data` | The data file's items, without `template`, `theme` and `title`. An item the schema declares but the data leaves out is nil, so `{{with .Data.lead}}` works; a name the schema does not declare stops the drawing. An item of `"format": "html"` goes in as HTML |
| `.Calc` | The values of `computed.yaml`, by name |
| `.Theme` | The theme's CSS, for `<style>{{.Theme}}</style>` |
| `markdown` | Markdown text as HTML (`{{markdown .text}}`), raw HTML in it left out, a table wrapped in `<div class="table-wrap">` |
| `num` | A number with its digits grouped (`{{.Calc.total \| num}}` is `1,234,567`), or rounded to significant digits (`{{.Calc.rate \| num 3}}` is `0.00648`) |
| `bytes` | A count of bytes in a binary unit (`{{.Calc.size \| bytes}}` is `1.5 KiB`) |

## schema.json

A [JSON Schema](https://json-schema.org/) (2020-12) of the data file's items. Write `"additionalProperties": false` so that a misspelt key is refused. `template`, `theme` and `title` are read by floe and are not declared.

## checks.yaml

Relations between items that a schema cannot say, each checked on every item of a list:

```yaml
- for: access_patterns            # a path of keys to a list
  field: key                      # the item's key to point at when it fails (optional)
  expr: item.key in map(doc.attributes, .name)
  message: use an attribute as the key
```

`expr` is an [expr](https://expr-lang.org/) expression that must hold. It sees the list's item as `item` and the data file's items as `doc`. Each item it fails for is reported at its field, with the field's value:

```text
orders.yaml:13:12: access_patterns[0].key: use an attribute as the key ("user_id")
```

## computed.yaml

Values computed from the data, for `page.html` to show as `.Calc.<name>`:

```yaml
item_bytes: sum(attributes, .avg_bytes)
storage_gb: item_bytes * records.in_1_year / 1e9
```

An expression sees the data file's items that the schema declares, and the other computed values, by name, whatever order they are written in. A name is a letter followed by letters, digits and `_`, and is neither one of the data's items nor a word or function of expr. A value the data cannot give (a division by zero, an item left out that the expression needs) is reported, and a value that would be NaN or ±Inf is too.

## The order of the checks

A data file is checked in this order, and each stage runs only on data that passed the one before. `floe check` runs them all without publishing.

1. Its YAML
2. Its template, the floe the template needs, its theme, and the template's `schema.json`
3. The template's `checks.yaml`
4. The template's `computed.yaml`
5. Drawing `page.html`
