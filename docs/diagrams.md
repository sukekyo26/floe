# Diagrams

A `diagram` part, in a section or in `details` and `tabs`, is a spec written in place: a canvas and the parts placed on it at absolute coordinates (the top left is 0, 0), each within a million either way (a size within a million, a canvas within ten million). It is checked before the page is drawn, its mistakes at the data file's lines and columns, and drawn as inline SVG that zooms. `floe diagram render` draws a spec of a file of its own the same way ([commands.md](commands.md#floe-diagram-render-specyaml)).

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
- A diagram paints its own ground, the page's `--bg`, in its frame, and what hides the lines under it (a label's box, a hollow end or dot) is painted that ground: on a panel or a theme's patterned page, the diagram stands as a box of the page's ground.
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
| `label` | `text`, `x` `y`, `anchor` (`middle`; `start`, `end`), `plain` (no background: the default for a label with `in`; `false` gives it one), `strong`, `in` (the names of the circles of a Venn diagram it sits inside, outside the others; without `x` and `y`, floe places it). |
| `line` | `x1` `y1` `x2` `y2`, `kind` (`line`; `lifeline`, `divider`). It may cross other parts, but not leave the canvas. |
| `tree` | `x` `y` (the root's centre and top), `root`: a node of `rows`, `name`, `tone`, `shape` as a `box` has them, and `children`, nodes too. floe places each node under its parent, the children side by side and each depth in one row, and draws an edge down from each node to each child. A node's `name` can be the end of an edge written by hand; written without a side, the edge keeps off the sides the tree's edges end at (a node's top below the root, and its bottom when it has children), as at a box of a graph. |
| `graph` | `x` `y` (the top left of all of it), `nodes`, `edges`, `direction` (`down`; `right`), and `lanes` in place of `nodes`. A node is `box: { name, rows, tone, shape }`, as a `box` has them, its `name` needed, or `group: { name, label, accent, nodes }`, boxes and groups framed together; an edge is `{ from, to }`, two names of its boxes or groups, with `label`, `dashed`, `bold`, `tone`, `start` and `end` (`arrow` or `none`). floe lays the boxes out in ranks and draws each edge between them, as below. |
| `raw` | `svg`: SVG that no part can draw, of SVG's elements alone (no script or `foreignObject`), not laid out or checked. |

A `graph` holds only how its boxes are joined; floe places them:

```yaml
- graph:
    x: 20
    y: 20
    nodes:
      - box: { name: write, rows: [Write the data] }
      - box: { name: check, shape: hex, rows: ["Checks pass?"] }
      - box: { name: show, rows: [Show the page] }
    edges:
      - { from: write, to: check }
      - { from: check, to: show, label: "yes" }
      - { from: check, to: write, label: "no" }
```

- Each box goes a rank below the boxes its edges come from (an edge back up a cycle is drawn the way it is written), but for the two boxes of an edge written `level` (see below), which stand in one rank; the boxes of a rank are ordered to cross fewer edges, as written when none cross. The same spec is always laid out the same way.
- Edges leave and enter a box at separate points of its side, on its outline, so that no two share a run; a box widens (with `right`, grows taller) for many edges. Edges of one graph may cross one another; an edge of a graph crossing another part is checked as any edge.
- An edge's `label` does not change how the graph is laid out: once its edges are drawn, the label stands a column beside a straight run of its edge, nearest the box the edge is written from where there is room (right of a run down, else left; below a run across, else above), with no background. Where there is none, floe makes room, moving the drawing apart there and widening boxes as it must, so that edges run as straight as with no label. Where the room below a run (right of a run down) would push the drawing apart across it too, the label stands in the same place above it (left of it) when that is clear, of the parts written by hand too, and moves the graph off none of its `x` and `y`; so the labels of two level edges between the same boxes stand outside them where there is room. A label takes the room of its widest language, so that every language lays out alike. The graph's `x` and `y` are the top left of all of it, labels too, to within a column: a label left of or above its boxes moves the graph on by whole columns, so that its boxes keep their grid.
- A `group` keeps its boxes (and its groups') side by side in every rank, with nothing else between, and floe draws a frame around them as a `frame` part, its `label` in the top left corner and `accent` as a frame's. The frame holds the group's boxes and the edges between them, and no other box. An edge between a box inside and one outside crosses the frame's side, and so may an edge between two boxes outside a group, between two of its ranks. A long `label` widens its group (with `right`, deepens its first rank). A group's `name` names the frame: an edge written by hand may end at it.
- An edge to a group ends at its frame: from a rank above, at the frame's top; else it leaves the frame's bottom (with `right`, the left and right sides), each edge at a point of its own, clear of the heading. An edge between a group and a box or group it holds, or from a group to itself, is refused; so is an edge to a group from a box (or group) that stands between two of its boxes in rank (a box of the group stands no lower than it and another no higher): end the edge at a box of the group.
- `lanes` in place of `nodes` lists the graph's lanes, each written as a group is, `{ name, label, accent, nodes }`, with no `group:` key round it, and each is a lane: a band from end to end of the graph along its edges (with `down`, from top to bottom; with `right`, from left to right), in the order written, 24 apart, as wide as what it holds. Its tops and bottoms line up, headed or not. Lanes and the groups nested in them are as groups, but an edge may not end at a lane, which no edge can reach from outside its ranks: end it at a box or a group in the lane.
- An edge written `level: true` stands its two boxes in one rank and runs straight between their facing sides (with `right`, between the bottom of the one above and the top of the other), half a column off the middle of the shorter side, the edges between the same two boxes a column apart (the shorter box grows for them). Its two boxes are of one group (or of none), or directly in two lanes. Their left and right (with `right`, top and bottom) follow from the spec, not from how ranks are ordered: the boxes level edges join in one group stand side by side as written, and a box directly in a lane with a level edge to another lane stands at its lane's end towards it (at its start when it has edges both ways). So floe refuses, before laying the graph out, an edge written `level` at a group or between other boxes; an edge between two boxes level edges stand in one rank; one whose boxes something stands between in their rank (a box written between them in their row, a box of their row written past the one in a lane, on the side its row faces, a box of a lane between or a group's frame spanning their rank there, another row at the same end of either lane, or, when the first box's row stands at its lane's start, anything else of that lane in the rank); and one taking the side of its first box's loops. Laid out `right`, a lane's heading reaches into the first rank, so a level edge running through one is refused once laid out: shorten the heading. A loop written `level` is a loop: its box stands in one rank already. Edges written by hand keep off the sides level edges take, as off a loop's side.
- A graph holds 200 boxes and groups and 400 edges at most, counted through its groups, and the edges of all the graphs of a diagram may pass 4000 ranks between their ends, all told.
- An edge with the same box in `from` and `to` is a loop. Loops do not change how the graph is laid out: once its edges are drawn, a box's loops go out of the side the graph's edges between ranks do not use (the right side; with `right`, the bottom; a level edge may not take it), nested in the order written, the first innermost, and their labels stand one after another a column outside the outer loop. A box grows along that side for its loops and their labels, and floe makes room for them as for a label.
- A box of a graph, and a group's or a lane's frame, can be the end of an edge written by hand, which ends at the middle of a side. Without `fromSide` or `toSide`, with `via` or without, it takes a side the graph's edges leave to it, with its via next to it (or the other end) outside that side: across first, then up or down. The sides left are those no edge of the graph ends at and no loop or level edge takes on a box, and those no edge of the graph ends at or crosses on a group's or a lane's frame. When no side left has the via outside it, the edge keeps the side it would take and may run along one of the graph's edges: write `fromSide` or `toSide` for a side the graph's edges do not use. A box's side with loops or level edges is theirs: end no edge written by hand there.
- A mistake between a part of a graph and one written by hand (an overlap, an edge through a box, a crossing) is told at the one written by hand, which can be moved. One that only parts of graphs make (two graphs overlapping or crossing, a graph off the canvas) is told at the graph, to move it by its `x` and `y` (or, off the canvas, to widen the canvas).

An edge without `via` and sides runs right-angled from the side of one end facing the other, with two turns at most; give `via` or a side to take another way.

An end named without a side on an edge with `via` takes the side towards the via next to it when that via is outside the end, beside it (left or right) or above or below it (top or bottom), on a `straight` edge too. When the via is off both ways, the end of a right-angled edge takes the side across the way the edge came into the via. Its start, the ends of a `straight` edge with the via off both ways, and an end with the via on it or in it, take the side facing the other end as without `via`. A table's row takes left or right alone, and a dot takes no side. An end written with a side takes that side. The last run of a right-angled edge meets its end's side, written or taken, square when the via is outside that side, and with two turns halfway between when one turn would run back along the edge; when the via is not outside the side written, the edge goes on as it was going and may run along the edge of the end, so write the via outside the side. A right-angled edge turning at a via towards a next point off line goes on the way it came first, and the other way first when that would run back along the edge.

The checks:

- Text too wide or too tall for its box (a frame's label for the frame's width).
- Parts (edges, lines and axes too) sticking out of the canvas: left of or above 0, 0, or past a `width` or `height` written.
- Boxes, marks, labels and frame labels overlapping each other, and boxes, marks and the labels written where they stand (a label at an `x` and `y`, an edge's label at `at`) straddling the edge of a frame. A label is checked by its box, 5 wider each side than its text, with no background too, as for overlaps. The labels floe places are not checked against frames: an edge's with no `at` (in the middle of the edge, which may cross frames), an icon's, a bar's, an axis's, and a label in its region (`in` with no `x` and `y`). Frames are not checked against one another, so they may cross; a label or a box across the edge of one that another crosses is refused all the same.
- Edges running through boxes, labels or frame labels, and edges crossing like an X (unless one has `crossing: true`, or both are edges of one `graph`). A box is the shape drawn: an edge may pass a hex's points and a rounded corner, and run along its outline or within 1 of it.
- An edge ending at a name no part has, or at a side its end does not have.
