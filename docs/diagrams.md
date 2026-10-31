# Diagrams

A `diagram` part, in a section or in `details` and `tabs`, is a spec written in place: a canvas and the parts placed on it at absolute coordinates (the top left is 0, 0). It is checked before the page is drawn, its mistakes at the data file's lines and columns, and drawn as inline SVG that zooms. `floe diagram render` draws a spec of a file of its own the same way ([commands.md](commands.md#floe-diagram-render-specyaml)).

```yaml
- diagram:
    width: 640
    height: 200
    # read aloud by screen readers
    title: How the API is laid out
    parts:
      - frame: { x: 16, y: 16, w: 608, h: 168, label: VPC }
      - box: { name: alb, x: 40, y: 72, w: 200, h: 56, tone: accent, rows: [[ALB, main code], public entry] }
      - box: { name: api, x: 400, y: 72, w: 200, h: 56, rows: [API server, ECS] }
      - edge: { from: alb, to: api, label: HTTPS }
```

- `width` and `height` may each be left out: the canvas then reaches as far right (or down) as what is drawn, and 20 more. Its top left stays 0, 0, so the coordinates are as written. `raw` SVG is not measured: write `width` and `height` with it, or it may be cut off.
- `example: <kind>` in place of the rest draws a built-in example (`floe diagram example` lists them). The page shows the diagram, not its spec: to read or copy an example's spec, print it with `floe diagram example <kind>`.
- Text (a `title`, a row, a label) is text, or its words in each language, English among them: `{ en: Store, ja: 保存先 }`. The page draws the words of its `lang:`, and the spec is laid out and checked once for every language it is written in.
- Each part is one key naming its kind. `floe part show diagram schema.json` prints every key; `floe diagram example <kind>` prints a spec of each kind, its comments explaining its grid.

| Part | Keys |
|---|---|
| `frame` | `x` `y` `w` `h`, `label`, `name`, `accent`, `square` (no rounded corners), `icon` (an icon such as `aws/group/region`: the AWS group style, its icon in the corner). Its label sits inside the top left corner, so place its contents 32 or more below its top (40 with an icon). |
| `box` | `x` `y`, `rows`, `w` `h` (as large as the rows need when left out), `name`, `origin` (`left`; `centre` for an `x` at the middle), `tone`, `shape` (`rect`; `pill`, `hex` for a decision, `state`). A row is text, or `[text, style]` with the style `main`, `sub`, `main code` or `sub code`; without a style the first row is `main` and the rest `sub`. |
| `table` | `x` `y`, `title`, `rows`, `w`, `name`, `origin`, `tone`. A row is `[name]`, `[name, type]`, `---` for a separator, or `{ name, cells: [name, type] }` for a row an edge can end at. |
| `icon` | `x` `y` (the top left), `icon`, `label` (below the icon), `size` (48), `name`. `icon` is the icon's set and its name in the set, like `aws/amazon-ec2` (see `floe icons search`). |
| `edge` | `from` and `to`, each the `name` of a part, a named row or a node of a `tree`, or a point `[x, y]`; `fromSide` and `toSide` (`top`, `right`, `bottom`, `left`), `via` (points to turn at), `straight` (point to point, not right-angled), `label` with `at: [x, y]`, `dashed`, `bold`, `tone`, `start` (`none`), `end` (`arrow`), `crossing`. The ends are `arrow`, `triangle`, `diamond`, `odiamond`, `one`, `many`, `zero-one`, `zero-many` or `none`. `crossing: true` lets this edge cross others. |
| `dot` | `cx` `cy`, `r` (7), `name`, `tone`, `hollow`. Edges may reach its centre. |
| `bar` | `x1` `x2` `y`, `name`, `tone`, `label` (right of the bar). |
| `axis` | `x1` `x2` `y`, `ticks: [[x, text], ...]`, `grid` (the top of the grid lines). |
| `circle` | `cx` `cy` `r`, `name`, `tone`. Circles may overlap. |
| `label` | `text`, `x` `y`, `anchor` (`middle`; `start`, `end`), `plain` (no background), `strong`, `in` (the names of the circles of a Venn diagram it sits inside, outside the others; without `x` and `y`, floe places it). |
| `line` | `x1` `y1` `x2` `y2`, `kind` (`line`; `lifeline`, `divider`). It may cross other parts, but not leave the canvas. |
| `tree` | `x` `y` (the root's centre and top), `root`: a node of `rows`, `name`, `tone`, `shape` as a `box` has them, and `children`, nodes too. floe places each node under its parent, the children side by side and each depth in one row, and draws an edge down from each node to each child. A node's `name` can be the end of an edge written by hand. |
| `raw` | `svg`: SVG that no part can draw, of SVG's elements alone (no script or `foreignObject`), not laid out or checked. |

An edge without `via` and sides runs right-angled from the side of one end facing the other, with two turns at most; give `via` or a side to take another way.

The checks:

- Text too wide or too tall for its box (a frame's label for the frame's width).
- Parts (edges, lines and axes too) sticking out of the canvas: left of or above 0, 0, or past a `width` or `height` written.
- Boxes, marks, labels and frame labels overlapping each other, and boxes and marks straddling the edge of a frame. Frames are not checked against one another, so they may cross.
- Edges running through boxes, labels or frame labels, and edges crossing like an X (unless one has `crossing: true`). A box is the shape drawn: an edge may pass a hex's points and a rounded corner, and run along its outline or within 1 of it.
- An edge ending at a name no part has, or at a side its end does not have.
