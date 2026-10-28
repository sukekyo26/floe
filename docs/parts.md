# Parts and themes

How to make a part, a look or a theme of your own. A part draws an item of a data file; a theme sets a page's colours, faces and the shape of the parts. To use parts in a template, see [templates.md](templates.md#parts-in-a-template); to write them in a data file, see [pages.md](pages.md#parts).

## Where parts and themes are

- Parts, looks and shared schemas of your own are in `FLOE_PARTS_DIR` (default `$XDG_CONFIG_HOME/floe/parts`, else `~/.config/floe/parts`): a part or a look is a directory, a shared schema a file `<name>.json` beside them.
- Themes of your own are in `FLOE_THEMES_DIR` (default `$XDG_CONFIG_HOME/floe/themes`, else `~/.config/floe/themes`), a directory each.
- A name is lower case letters, digits, `-` and `_`, starting with a letter or a digit. A name floe's parts, looks, shared schemas or themes have is refused: yours sit beside floe's, and never take their place. A part or look is not named `base` or `palette`, the names of a theme's files for every page.
- A directory in them may not be a symlink; a file may.

```sh
floe part new quote     # makes the smallest part, quote, to fill in
floe part check         # checks every part, look, shared schema and theme of your own
floe part show quote    # prints its looks, schema and checks
```

A mistake of a part, theme or template of your own stops every page drawn (`floe check`, `floe render`, `floe publish`), whether it uses that one or not, so that none goes unseen: `floe part check` and `floe template check` list them all.

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
- `part.html` sees the item as `.`, and the functions of a template's `page.html` but `asset` ([templates.md](templates.md#pagehtml)).
- Of floe's `format`s its schema takes `image` alone, declared under `properties` or `items`, where floe follows, and comes to `part.html` as a data URI (`<img src="{{.file}}" alt="{{.alt}}">`). It takes no `markdown` or `html`, nor a shared schema that holds a format (`any.json`): its `part.html` draws its items, Markdown with `{{markdown .text}}`.
- `floe part show <name>` prints what one takes, floe's too: copy from floe's parts what fits.

## Looks and shared schemas

- A look alone is a directory of a `part.css` (and a `part.yaml` with its `description:` and the looks it takes), without a schema: it draws nothing, and lends its style to parts and elements that take it. floe's looks alone are `panel` and `rows`.
- A part takes the looks of others by its `part.yaml`, `looks: [panel]`, and the looks those take in turn, each before it, which it adjusts.
- A shared schema is a `<name>.json` the parts' schemas name (`floe:///parts/<name>.json`). floe's are `tone.json` (`accent`, `good`, `warn`, `bad`) and `any.json` (a part of any kind).

## The CSS layers and @scope

floe puts a page's CSS into `<style>`s before its `</head>`, in five cascade layers, each later one winning over those before it whatever the selectors (`floe.parts` the strongest):

1. `floe.palette`: the palettes, floe's `default` and then the theme's.
2. `floe.base`: floe's base, the page's ground, colour and face on its root, and text, lists, links, code and quotes.
3. `floe.looks`: the looks of elements that take one without being a part's box (`data-floe-look`).
4. `floe.page`: the template's `page.css`.
5. `floe.parts`: the parts' style, each part after the looks it takes, then the theme's `base.css` and its files of the parts.

A part's `part.css`, and a theme's file of it, are scoped to the elements that have its look, from their own on, short of what the parts in them hold. Write the element itself as `:scope`. A part's style reaches the box of a part it holds, but not what that part holds.

As in `page.css`, a `}` that closes no block and an `@import` are refused at their line.

## Borrowing a look

- An element that is no part's box takes a look by `data-floe-look`: `<section data-floe-look="panel">` is a panel in every theme. It has it in `floe.looks`, under the parts and the template, so that the part or template it is in adjusts it.
- floe's parts take looks so: `compare`, `fields`, `findings`, `matrix` and `risks` take the table's, `stats`, `options`, `facts`, `links` and `decision` the cards', `diff` and `terminal` code's, `checklist`, `faq`, `glossary` and `keys` the look `rows`, and the other parts drawn in a panel `panel`'s. A theme's rules for a look draw every part that takes it.

## Themes

A theme of your own is a directory of these files, each left out if not needed:

| File | What it holds |
|---|---|
| `palette.css` | The variables of [the palette](templates.md#the-palette), and those of names of its own, on `:root`. It goes over floe's `default` palette, so it writes only what it changes. |
| `base.css` | Rules for every page, such as Markdown's quotes. |
| `<name>.css` | The style of the part or look `<name>`, scoped as its `part.css` is: `panel.css` reaches every panel, `code.css` code, a diff and a terminal. |

- A file of another name is refused.
- Write a colour once, in `palette.css`, and take it with `var()` (`var(--accent)`) in the other files, so that the template's colours on `:scope` and the palette agree.
- A data file picks the theme with `theme:`, as one of floe's.

## Checking

`floe part check` reads every part, look, shared schema and theme of your own over floe's own, and prints every mistake of them (exit code 1): of their files, their names, the looks they take, a part that takes its own look, a path of a `checks.yaml` that its schema does not declare, and their CSS. A part is not drawn, as it has no data: `floe template check` draws the example of a template that uses it.
