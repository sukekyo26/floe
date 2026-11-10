# Diagrams

A `diagram` part, in a section or in `details`, `tabs`, `columns` and the `parts` of an item of `entries`, `checks` or `options`, is a spec written in place: a canvas and the parts drawn on it. Parts written by hand use absolute coordinates (the top left is 0, 0), each within a million either way (a size within a million, a canvas within ten million); a graph is placed by floe. It is checked before the page is drawn, its mistakes at the data file's lines and columns, and drawn as inline SVG, which zooms where it is drawn smaller than its actual size. `floe diagram render` draws a spec of a file of its own the same way (`floe diagram render --help`).

```yaml
- diagram:
    width: 640
    height: 200
    # read aloud by screen readers
    title: How the API is laid out
    parts:
      - frame: { x: 16, y: 16, w: 608, h: 168, label: VPC }
      - box: { name: alb, x: 40, y: 72, tone: accent, rows: [[ALB, main code], public entry] }
      - box: { name: api, x: 400, y: 72, rows: [API server, ECS] }
      - edge: { from: alb, to: api, label: HTTPS }
```

- `width` and `height` may each be left out: the canvas then reaches as far right (or down) as what is drawn, and 20 more. Its top left stays 0, 0, so the coordinates are as written. `raw` SVG is not measured: write `width` and `height` with it, or it may be cut off.
- `example: <kind>` in place of the rest draws a built-in example (`floe diagram example` lists them). The page shows the diagram, not its spec: to read or copy an example's spec, print it with `floe diagram example <kind>`.
- Text (a `title`, a row, a label) is text, or its words in each language, English among them: `{ en: Store, ja: 保存先 }`. A diagram's `title` is empty in no language, as it is what a reader who cannot see the diagram is told. Nor is a label's text (every `label` key, a graph's edges and groups among them, and the `text` of the part `label`): for no label, leave the key out. The page draws the words of its `lang:`, and the spec is laid out and checked once for every language it is written in.
- A diagram paints its own ground, the page's `--bg`, in its frame, and what hides the lines under it (a label's box, a hollow end or dot) is painted that ground: on a panel or a theme's patterned page, the diagram stands as a box of the page's ground.
- Each part is one key naming its kind. `floe part show diagram schema.json` prints every key; `floe diagram example <kind>` prints a spec of each kind, its comments explaining its grid.

## Two ways to write a diagram

Pick by what the diagram is for:

- **By hand** for diagrams read as pictures, such as architecture or layout diagrams, where spatial arrangement matters. You decide where each part goes, so you can keep flows straight, align related parts, and place each label on its edge. The example above is written this way.
- **A `graph`** for a flow, a state machine, or a diagram that changes often or joins many things. You write what joins what, and adding to it takes no coordinates. floe decides where everything stands.

A hand-placed diagram takes more work to write. Moving a part means moving what copies where it stands: the frame round it, and the `via` or `at` of its edges. Writing only the turns you choose avoids copying them in an edge (see [Edges written by hand](#edges-written-by-hand)); a frame is moved by hand.

A graph usually takes more space than a hand-placed diagram. A diagram wider than the page shrinks to fit, until it is zoomed. Try both `direction`s: one often comes out far better than the other for the same spec. See [Graph](#graph) for an example.

### Writing by hand so that floe does not refuse it

Box sizes:

- Give each box an `x` and `y`.
- Leave `w` and `h` out to let floe size the box to its text; specify them when you need to align boxes.
- A `w` that is too small is refused with the one it needs: `box "a": "A long title here" needs w: 174 or more`.
- `floe diagram inspect` prints the size floe gave each box (`box "alb" 240,90 144x29`). Place the next box by it.

Labels on edges:

- Between the boxes of a horizontal edge with a label, leave at least the label's width, else `box "a" overlaps label "HTTPS"`.
- The `label` row of `floe diagram inspect` gives the width (`label "HTTPS" 80,5.5 46x18`: 46).
- Between the boxes of a vertical edge, leave at least the label's height (18).
- A label stands across a vertical edge, centred on it, so a long one reaches both sides of the line.
- Keep half the label's width between the line and the left of the canvas, else `label "HTTPS-LONG-LABEL" sticks out of the 142.6x169 canvas` (for a line 60 from the left).
- To fix it, move the line to the right or shorten the label. A `width` only widens the canvas to the right.

Frames:

- Put a box whole inside a frame or whole outside it, else `box "a" straddles the edge of frame "F" at x 100, spanning 80 to 160`.
- Put a box below the bottom edge of the frame's heading, else `box "a" overlaps the heading of frame "Frame label"`.
- Put it below the frame's icon too, if the frame has one, else `the icon of frame "Group" overlaps box "a"`.

Edges:

- Write an edge as `{ from, to }` and let floe route it.
- Use `via`, `fromSide` or `toSide` when you need to control its path; see [Edges written by hand](#edges-written-by-hand).
- An edge with a box between its ends is refused: `edge "a"→"c" runs through box "b"; move one of them, or go round it by via`.
- To fix it, move a box or write `via`.

Checking:

- Run `floe diagram inspect` on the data file after every few parts.
- It prints where each part went, and each refusal at the line to change.

### An AWS architecture: draw the services as icons

In an AWS architecture, represent each service with an AWS icon, not a box. Do not draw a box around the icon. floe ships no icons. The one set is AWS's, kept once on each machine:

1. `floe icons fetch aws` keeps the set (about 14MB, from aws.amazon.com). Without it, `floe check`, `floe diagram render` and `floe diagram inspect` refuse a diagram that holds an icon (`inspect` prints where the parts went first). The refusal reads `floe: AWS icons are not in <dir>/aws; run "floe icons fetch aws" (downloads about 14MB from aws.amazon.com) and try again`.
2. `floe icons search <word>` lists the names of icons that hold the word, each with its group. `floe icons search alb` lists `aws/res/elastic-load-balancing-application-load-balancer`, and `floe icons search rds` lists `aws/amazon-rds`. A short name such as `alb` works. Take the name as it is printed.
3. Write the icon as an `icon` part with `x`, `y` and its name in `icon`. Join icons by edges naming their `name`s. Or write it as an `icon` node of a `graph`, which floe places (`floe diagram example aws-graph` is one). A frame of the AWS group style takes an `icon` of its own (`aws/group/region`).

```yaml
- diagram:
    title: A web app on AWS
    parts:
      - icon: { name: alb, x: 40, y: 40, icon: aws/res/elastic-load-balancing-application-load-balancer, label: ALB }
      - icon: { name: api, x: 240, y: 40, icon: aws/amazon-elastic-container-service, label: ECS }
      - icon: { name: db, x: 440, y: 40, icon: aws/amazon-rds, label: RDS }
      - edge: { from: alb, to: api, label: HTTPS }
      - edge: { from: api, to: db, label: SQL }
```

## The parts

| Part | Keys |
|---|---|
| `frame` | `x` `y` `w` `h`, `label`, `name`, `accent`, `square` (no rounded corners), `icon` (an icon such as `aws/group/region`: the AWS group style, its icon in the corner). Its label sits inside the top left corner, so place its contents at or below the bottom edge of its heading and of its icon: the `the heading of frame` and `the icon of frame` rows of `floe diagram inspect` give where they stand (a heading of one line stands 9 below the frame's top and is 18 tall, so 27; with an `icon`, the icon is 31 square in the corner and the heading stands 6 down, so 31). |
| `box` | `x` `y`, `rows`, `w` `h` (as large as the rows need when left out), `name`, `origin` (`left`; `centre` for an `x` at the middle), `tone`, `shape` (`rect`; `pill`, `hex` for a decision, `state`). A row is text, or `[text, style]` with the style `main`, `sub`, `main code` or `sub code`; without a style the first row is `main` and the rest `sub`. |
| `table` | `x` `y`, `title`, `rows`, `w`, `name`, `origin`, `tone`. A row is `[name]`, `[name, type]`, `---` for a separator, or `{ name, cells: [name, type] }` for a row an edge can end at. |
| `icon` | `x` `y` (the top left), `icon`, `label` (centred below the icon, 3 under it and 18 tall; an edge at the icon's bottom side ends under the label), `size` (48), `name`. `icon` is the icon's set and its name in the set, like `aws/amazon-ec2` (see `floe icons search`). |
| `edge` | `from` and `to`, each the `name` of a part, a named row or a node of a `tree`, or a point `[x, y]`; `fromSide` and `toSide` (`top`, `right`, `bottom`, `left`), `via` (points to turn at: `[x, y]`, or one axis, `{ x: 710 }` or `{ y: 218 }`), `straight` (point to point, not right-angled), `label` with `at: [x, y]` (the middle of the label; without it, the label stands in the middle of the edge's longest run), `dashed`, `bold`, `tone`, `start` (`none`), `end` (`arrow`), `crossing`. The ends are `arrow`, `triangle`, `diamond`, `odiamond`, `one`, `many`, `zero-one`, `zero-many` or `none`. `crossing: true` lets this edge cross others. |
| `dot` | `cx` `cy`, `r` (7), `name`, `tone`, `hollow`. An edge naming it is drawn to its outline, towards the edge's first point off its centre, the tip of its arrow (or other marker) on the outline; it is drawn to the centre when that run is too short for the marker (or no longer than the radius, with none), between two dots that touch or overlap, and when the end is written as a point at the centre (name the dot). The edge is checked, and its label placed, as one to the centre. |
| `bar` | `x1` `x2` `y`, `name`, `tone`, `label` (right of the bar). |
| `axis` | `x1` `x2` `y`, `ticks: [[x, text], ...]`, `grid` (the top of the grid lines), `labels` (`at`, a tick's text at it; `between`, halfway between it and the next tick on its right, or `x2` past the last). A tick of empty text (`""`) is a mark alone, not a label. |
| `circle` | `cx` `cy` `r`, `name`, `tone`. Circles may overlap. |
| `label` | `text`, `x` `y`, `anchor` (`middle`; `start`, `end`), `plain` (no background: the default for a label with `in`; `false` gives it one), `strong`, `in` (the names of the circles of a Venn diagram it sits inside, outside the others; without `x` and `y`, floe places it). |
| `line` | `x1` `y1` `x2` `y2`, `kind` (`line`; `lifeline`, `divider`). It may cross other parts, but not leave the canvas. |
| `tree` | `x` `y` (the root's centre and top), `root`: a node of `rows`, `name`, `tone`, `shape` as a `box` has them, and `children`, nodes too. floe places each node under its parent, the children side by side and each depth in one row, and draws an edge down from each node to each child. A node's `name` can be the end of an edge written by hand; written without a side, the edge keeps off the sides the tree's edges end at (a node's top below the root, and its bottom when it has children), as at a box of a graph. |
| `graph` | `x` `y` (the top left of all of it), `nodes`, `edges`, `direction` (`down`; `right`), and `lanes` in place of `nodes`. A node is `box: { name, with, rows, tone, shape }`, as a `box` has them, its `name` needed, `icon: { name, with, icon, label }`, an icon with its label below and no box round it, or `group: { name, label, accent, icon, nodes }`, boxes, icons and groups framed together, `icon` in the frame's corner, the AWS group style; an edge is `{ from, to }`, two names of its boxes, icons or groups, with `label`, `dashed`, `bold`, `tone`, `start` and `end` (`arrow` or `none`). floe lays the boxes out in ranks and draws each edge between them, as below. |
| `raw` | `svg`: SVG that no part can draw, of SVG's elements alone (no script or `foreignObject`), not laid out or checked. Colour it with the variables `floe theme show default palette.css` defines (the CSS of the palette), as attributes too (`fill="var(--accent-soft)" stroke="var(--accent)"`); take the colours: `--bg`, `--surface`, `--fg`, `--muted`, `--muted-soft`, `--quiet`, `--line`, `--accent`, `--accent-soft`, `--good`, `--good-soft`, `--warn`, `--warn-soft`, `--bad`, `--bad-soft` and `--on-fill` (no other colour has a soft one, and a variable that is not there draws black), not the faces, sizes or shadows in it, and no variable a theme alone sets (`--accent-fill` and the other `--*-fill`, which palette.css does not define). For a tone, write `data-floe-tone` on the element or on a `<g>` around it within the raw SVG, and read `var(--tone)`, `var(--tone-soft)` and, for a fill, `var(--tone-fill)`: the diagram's root clears any tone from outside it. Give text a colour and a size, as the parts' text is 12 to 13 (`<text fill="currentColor" font-size="12">`); `currentColor` is `--fg`. A part's classes, `data-floe-look` and `data-floe-part` are floe's own, not for raw. A variable or a tone that is not there, `var(--tone)` with no tone, and text without `fill` draw black (text at the page's size), and are not told. |

## Graph

A `graph` holds only how its boxes are joined; floe places them:

```yaml
- diagram:
    title: How a page is published
    parts:
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

### Ranks

- Each box goes a rank below the boxes its edges come from (an edge back up a cycle is drawn the way it is written), but for the two boxes of an edge written `level` (see below), which stand in one rank, and a box written `with`, below; the boxes of a rank are ordered to cross fewer edges, as written when none cross. The same spec is always laid out the same way.
- `with: <name>` on a box or an icon stands it in the rank of the box or icon named, with no edge drawn, of another group too. Write it for a box that should not go below what reaches it, a gateway below the servers that use it say, or that should not start a group's frame in the first rank. Edges keep the way they mean: `api → nat`, with `with: alb` on `nat`, runs up from `api`.

  ```yaml
  - diagram:
      title: A NAT gateway beside the load balancer, below the servers that use it
      parts:
        - graph:
            x: 20
            y: 20
            nodes:
              - icon: { name: alb, icon: aws/res/elastic-load-balancing-application-load-balancer, label: ALB }
              - icon: { name: api, icon: aws/amazon-elastic-container-service, label: ECS }
              - icon: { name: nat, with: alb, icon: aws/res/amazon-vpc-nat-gateway, label: NAT }
            edges:
              - { from: alb, to: api }
              - { from: api, to: nat }
  ```

- `with` takes only an edge from a box into that box out of the ranks: it runs up or down as the ranks fall, and is refused when they fall in one rank. An edge from a group, into a level edge's other box or out of the box still decides ranks, moving the box named too, and an edge of them back up a cycle is drawn the way it is written, the order written deciding which: write the boxes in another order for another.
- `with` joins boxes no edge joins, which are then laid out together: a box written between them with no edge to either stands past them. Boxes joined by edges keep their order.

### Where edges end

- Edges leave and enter a box at separate points of its side, on its outline, so that no two share a run; a box widens (with `right`, grows taller) for many edges. The n ends on a side stand at the points that part it into n+1 equal parts, in the order of what they reach: one at the middle, two at the thirds, three at the quarters. An edge running straight stays straight, on a point both its sides have room for or where it stands, and the others on its side stand evenly around it; an end that would land on something, cross more edges, run close beside another or take its edge into a frame that holds neither end stays where it is. A box's loops and level edges do not count, and stay where they are. Edges of one graph may cross one another; an edge of a graph crossing another part is checked as any edge.
- An icon stays 48 square however many edges reach it. The n edges at a side of it end at the points that part the side into n+1 equal parts, in the order of what they reach: one at the middle, two at the thirds, three at the quarters. The sides are the icon's own, but for the bottom, where an edge starts under the label, or under the icon past the label's width. A straight edge moves the icon a little across the ranks, once, to run straight through the point; an end alone on its side stands up to 12 off its point when that is not far enough (none on an icon of 24 or less). Where an edge cannot reach its point straight it bends: with no label once between the icons, with a label close to the icon it does not run onto. Close to an icon two edges may share a short stretch, which takes the look of the one drawn last. Many edges at a side stand close, their arrowheads touching at five and overlapping at eight. An icon written `icon` with `x` and `y` is still placed by hand, its edges ending at the middle of a side.

### Labels

- An edge's `label` keeps the ranks and the order of the graph, but not its sizes: once its edges are drawn, the label stands on its edge, on the diagram's ground, cutting the line, and where it lacks room the drawing grows, so that a graph with labels is larger than one without and its boxes stand elsewhere. It stands at the middle of the edge's longest straight run, 12 clear of the run's ends; where something is there it moves along the run, then to the next longest. Where no run has room, floe makes it, lengthening the run by what it lacks, pushing the edges beside it apart, and moving the drawing apart there, widening boxes as it must; none of these bends a run. A label of a loop stands beside the outer loop of its box, with no ground. A label takes the room of its widest language, so that every language lays out alike. The graph's `x` and `y` are the top left of all of it, labels too, to within a column: a label wider than its box, or above it, moves the graph on by whole columns, so that its boxes keep their grid. So a label moves the graph's boxes, down and across, and a part written by hand beside it may come to overlap them, told at the part. A label is not checked against a frame written by hand, and cuts its line where it crosses.

### Groups and lanes

- A `group` keeps its boxes (and its groups') side by side in every rank, with nothing else between, and floe draws a frame around them as a `frame` part, its `label` in the top left corner and `accent` as a frame's. The frame holds the group's boxes and the edges between them, and no other box. An edge between a box inside and one outside crosses the frame's side. An edge with both ends outside a group goes round its frame where the ranks would take it through: a column off the frame, back against the ranks if it must, so it runs longer and may cross more edges. It may still run through a frame, in a small graph too: where floe finds no way round, where many edges end on one side of a box, where floe gives up looking in a crowded graph, and the like. Lanes are not gone round (an edge between two lanes crosses those between), and a level edge stays straight in its rank. A way round minds the graph's own parts only: a part written by hand that it runs into is told at that part. The `label` stands above the group's first rank: boxes an edge reaches from above stand right of it, a box no edge reaches stands under it if it comes before those in the rank (the rank's order is as written, unless floe changes it to cut crossings), and the frame holds it, so a long `label` widens its group only so far (with `right`, it deepens its first rank). A group's `name` names the frame: an edge written by hand may end at it.
- An edge to a group ends at its frame: from a rank above, at the frame's top; else it leaves the frame's bottom (with `right`, the left and right sides), each edge at a point of its own, clear of the heading. An edge between a group and a box or group it holds, or from a group to itself, is refused; so is an edge to a group from a box (or group) that stands between two of its boxes in rank (a box of the group stands no lower than it and another no higher): end the edge at a box of the group.
- A box `with` stands among a group's ranks takes an edge to the group as any box there does: refused, as it can stand neither above nor below the group. Ended at a box of the group as the words say, the edge is then refused for `with`: what helps is dropping `with`.
- `lanes` in place of `nodes` lists the graph's lanes, each written as a group is, `{ name, label, accent, icon, nodes }`, with no `group:` key round it, and each is a lane: a band from end to end of the graph along its edges (with `down`, from top to bottom; with `right`, from left to right), in the order written, 24 apart, as wide as what it holds. Its tops and bottoms line up, headed or not. Lanes and the groups nested in them are as groups, but an edge may not end at a lane, which no edge can reach from outside its ranks: end it at a box or a group in the lane.

### Level edges

- An edge written `level: true` stands its two boxes in one rank and runs straight between their facing sides, or bends at an icon whose point on the side stands off its line, which more than one edge on a side makes it (with `right`, between the bottom of the one above and the top of the other), half a column off the middle of the shorter side, the edges between the same two boxes a column apart (the shorter box grows for them).
- Its two boxes are of one group (or of none), or directly in two lanes. Their left and right (with `right`, top and bottom) follow from the spec, not from how ranks are ordered: the boxes level edges join in one group stand side by side as written, and a box directly in a lane with a level edge to another lane stands at its lane's end towards it (at its start when it has edges both ways).
- So floe refuses, before laying the graph out, an edge written `level` at a group or between other boxes; an edge between two boxes level edges stand in one rank; one whose boxes something stands between in their rank (a box written between them in their row, a box of their row written past the one in a lane, on the side its row faces, a box of a lane between or a group's frame spanning their rank there, another row at the same end of either lane, or, when the first box's row stands at its lane's start, anything else of that lane in the rank); and one taking the side of its first box's loops. Laid out `right`, a lane's heading reaches into the first rank, so a level edge running through one is refused once laid out: shorten the heading.
- A box `with` stands between a level edge's boxes is refused in the words for any box, which say to move it: what helps is dropping `with`.
- A loop written `level` is a loop: its box stands in one rank already. Edges written by hand keep off the sides level edges take, as off a loop's side.

### Loops

- An edge with the same box in `from` and `to` is a loop. Loops do not change how the graph is laid out (but for a rare way round a frame that runs across where a box's loops go, which fails the diagram as floe's own mistake: take the loops or the group off): once its edges are drawn, a box's loops go out of the side the graph's edges between ranks do not use (the right side; with `right`, the bottom; a level edge may not take it), nested in the order written, the first innermost, and their labels stand one after another a column outside the outer loop. A box grows along that side for its loops and their labels, and floe makes room for them as for a label.

### With parts written by hand

- An edge `level` cannot be, between two zones each a group say, or between two boxes `with` stands in one rank, can be written by hand beside the graph. It moves nothing of the graph, so its ends stay in the ranks their other edges give them:

  ```yaml
  - diagram:
      title: A copy between two zones, an edge written by hand beside the graph
      parts:
        - graph:
            x: 20
            y: 20
            nodes:
              - group:
                  name: zone-a
                  label: Zone A
                  nodes:
                    - box: { name: writer, rows: [Writer] }
              - group:
                  name: zone-b
                  label: Zone B
                  nodes:
                    - box: { name: reader, rows: [Reader] }
            edges:
              - { from: zone-a, to: zone-b }
        - edge: { from: writer, to: reader, label: copy, fromSide: right, toSide: right, via: [{ x: 190 }] }
  ```
- A box or an icon of a graph, and a group's or a lane's frame, can be the end of an edge written by hand, which ends on a side as any edge written by hand does (below). Without `fromSide` or `toSide`, with `via` or without, it takes a side the graph's edges leave to it, with its via next to it (or the other end) outside that side: across first, then up or down. The sides left are those no edge of the graph ends at and no loop or level edge takes on a box or an icon, and those no edge of the graph ends at or crosses on a group's or a lane's frame. When no side left has the via outside it, the edge keeps the side it would take and may run along one of the graph's edges: write `fromSide` or `toSide` for a side the graph's edges do not use. A box's side with loops or level edges is theirs: end no edge written by hand there.
- A mistake between a part a graph or a tree laid out and one written by hand (an overlap, an edge through a box, a crossing) is told at the one written by hand, which can be moved. One between parts written by hand is told at the one written later (the later item of `parts`; of one item, the first compared), as the part added or moved last most often made it. One between parts of trees, or between a part of a tree and one of a graph, is told at the part of a tree written later. One that only parts of graphs make (two graphs overlapping or crossing, a graph off the canvas) is told at the graph, to move it by its `x` and `y` (or, off the canvas, to widen the canvas).

### Limits

- A graph holds 200 boxes and groups and 400 edges at most, counted through its groups, and the edges of all the graphs of a diagram may pass 4000 ranks between their ends, all told.

## Edges written by hand

An edge without `via` and sides runs right-angled from the side of one end facing the other, with two turns at most; give `via` or a side to take another way. An icon's side is chosen with its label as part of it. An icon's label stays in the way of an edge of its own: told that an edge runs through the label of the icon it ends at, write `fromSide: right`, or move the other part below the label.

Write in `via` only what you choose of an edge's way, and `at` only when the middle of the edge's longest run, where the label stands without it, will not do; a label without `at` is not checked against a frame's edge, as the edge may cross it. floe adds a turn between points off line and enters the last side square, so the corners it adds need no writing, and a number copied from where a part stands needs rewriting when the part moves. A point may give one axis, `{ x: 710 }` or `{ y: 218 }`: the other is the point before's, and the first point's that of the centre of the edge's start (the point itself, the centre of a dot, the middle of a part or a row). `{ x: 710, y: 218 }` is `[710, 218]`.

```yaml
- diagram:
    title: Edges that turn where they are told
    parts:
      - box: { name: api-a, x: 40, y: 300, rows: [API A] }
      - box: { name: api-c, x: 40, y: 400, rows: [API C] }
      - box: { name: queue, x: 400, y: 300, rows: [Queue] }
      - box: { name: writer, x: 400, y: 340, rows: [Writer] }
      - edge: { from: api-a, to: queue, fromSide: top, toSide: left, via: [{ y: 240 }, { x: 300 }] } # M80,300 V240 H300 V314.5 H400
      - edge: { from: api-c, to: writer, fromSide: right, toSide: left, via: [{ x: 300 }] }          # M120,414.5 H300 V354.5 H400
```

Write the first point on the axis the edge leaves by: `{ y }` from a top or a bottom side, `{ x }` from a left or a right one. `fromSide: top, via: [{ x: 150 }]` takes the height of the box's centre and is refused as running through the box; `via: [{ y: 10 }, { x: 150 }]` leaves upward and then runs across.

Some points cannot be left out: without them the edge runs another way, and nothing tells you, since floe cannot know the way you wanted.

- The first point of an edge with no `fromSide`: it chooses the side the edge starts from. Write `fromSide`, or keep the point on one axis (`{ y: 10 }`).
- The first point of an edge leaving a point or a dot upward or downward: they leave across. Keep the point on one axis (`{ y: 120 }`).
- A turn the other way than the edge came: a turn floe adds goes on the way the edge came first, so after a run down, a corner that goes across first has to be written.
- The only point left in an edge's `via`: with no `via`, floe chooses the way and the edge shares the side with the others. Keep a `via` to stay in the middle of the side.

Edges written by hand, between names and without `via` or `straight`, share a side: the n ends on a side stand at the points that part it into n+1 equal parts (one in the middle, two at thirds, three at quarters), in the order that keeps them from crossing as they leave it. An edge running straight from middle to middle stays straight: it takes a point both sides have room for, or stays where it is, and the others on its side stand evenly around it. An edge that this would make cross another, run through a box, a label or a frame's heading, or come to nothing stays in the middle, so a label written by a side keeps that side's edges together: move the label to part them. Ends with `via`, at a point or `straight` stay where they are written, in the middle of the side, and do not count: to branch from one point, write `via` or use a `tree`. To have edges share a label (see the checks), write `via` on an edge that would otherwise leave the label as the others part. A dot, a row and a bar keep their edges in the middle. An edge bent between facing sides that share a height (12 in from the ends of each) then runs straight in that height, the other ends staying where they are, when the sides stand at least an arrowhead (8) apart and the straight line would touch nothing: no part, label or heading, no line, axis, box or frame edge or dot within 4, no other edge across, touching or within 6. If what comes of that is told as a mistake, a label with no place written landing on something say, all the diagram's edges stay as they were: write its `at` or move what is in its way. An edge that runs through a frame's `label` is told: go round it by `via`.

Where floe places a run itself (the run between facing sides, the run round two sides that face one way, and the two turns halfway into a side), it keeps the run 4 off the lines of `axis` parts (the axis, its `grid` and its ticks, and past the ticks below an axis that has them) and the lines of `line` parts that run across or down, once every part is placed: it moves the run to the nearest place just 4 off such a line, or off a part, frame, dot or edge along it (above or left of two as near), between the ends inside their markers or, for a run round two sides, outward only. Edges whose run lies on one line and that leave or reach one point, or one side of one part, move together. A run stays where it is when every such place would take an edge off the canvas written, through a part or text, or across another edge, would lay what is new of it within 4 along those lines, a part's or frame's edge or another edge's run, or would change the run its label stands on. Edges are not lines to keep off, as edges into one part share their runs on purpose, but a run does not move to lie along another edge, nor onto a dot. Edges of a `graph` or a `tree` are not moved.

An end named without a side on an edge with `via` takes the side towards the via next to it, both its axes known, when that via is outside the end, beside it (left or right) or above or below it (top or bottom), on a `straight` edge too. When the via is off both ways, the end of a right-angled edge takes the side across the way the edge came into the via. Its start, the ends of a `straight` edge with the via off both ways, and an end with the via on it or in it, take the side facing the other end as without `via`. A table's row takes left or right alone, and a dot takes no side. An end written with a side takes that side. The last run of a right-angled edge meets its end's side, written or taken, square when the via is outside that side, and with two turns halfway between when one turn would run back along the edge; when the via is not outside the side written, the edge goes on as it was going and may run along the edge of the end, so write the via outside the side. A right-angled edge turning at a via towards a next point off line goes on the way it came first, and the other way first when that would run back along the edge.

## The checks

- Text too wide or too tall for its box (a frame's label for the frame's width).
- Parts (edges, lines and axes too) sticking out of the canvas: left of or above 0, 0, or past a `width` or `height` written.
- Boxes, marks, labels and frame labels overlapping each other, and boxes, marks and two kinds of label straddling the edge of a frame: the labels written where they stand (a label at an `x` and `y`, an edge's label at `at`), and the labels of a part that may not straddle a frame itself (an icon's, by hand or in a `graph`, and a bar's). A label is checked by its box, 5 wider each side than its text, with no background too, as for overlaps. The other labels are not checked against frames: an edge's with no `at` (in the middle of the edge, which may cross frames), an axis's, a `graph` edge's, and a label in its region (`in` with no `x` and `y`). The mistake is told once for each edge crossed, with where that edge stands and where the part spans on that axis, both in the diagram's numbers: `at x 140, spanning 114.2 to 145.8`. To mend it, change what places the part: `x` and `y`, an edge label's `at`, a dot's `cx` and `cy`, a bar's `x1`, `x2` and `y` (an icon's label goes with its icon, a bar's with its bar), and the `x` and `y` of the whole `graph` or `tree` for what it lays out; or change the frame. Frames are not checked against one another, so they may cross; a label or a box across the edge of one that another crosses is refused all the same.
- Edges running through boxes, labels or frame labels, and edges crossing like an X (unless one has `crossing: true`, or both are edges of one `graph`). A box is the shape drawn: an edge may pass a hex's points and a rounded corner, and run along its outline or within 1 of it. Two edges written by hand going to one name, or coming from one, may share a label: where one runs on exactly the same line as the other, from side to side of the other's label, it is not told, though the label then reads as either's (two edges from one name that meet, share a run and part again pass too). Told still are two labels on what the two share, an edge turning behind the label or ending within 24 of it (the longest marker), a slanting run, and a label over a point of the `via` of its own edge: take that point out or move the label, as a `via` round it will not help. An edge floe moves off its label leaves the other told: write `via` on it to keep it there.
- An edge ending at a name no part has, or at a side its end does not have.

A mistake names a part by its name; with none, by its text in the language that decides its width (the widest, English of two as wide), in quotes; with no text either, by where it stands, its top left and size (`frame 10,10 250x150`). An edge names its two ends as parts are named, a point by where it stands: `edge "a"→"b"`, `edge "a"→[60,120]`, and an edge of a `tree` its two nodes as their boxes are named (`edge "a"→"B"`). A frame's corner icon is `the icon of` its frame (`the icon of frame "F"`). `floe diagram inspect` names parts alike, and writes where each stands after its name, but once for a part named by where it stands; a frame's heading is `the heading of` its frame, and the diagram itself is named by its title, in the widest language.
