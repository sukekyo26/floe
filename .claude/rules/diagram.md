---
paths:
  - "internal/diagram/**"
  - "internal/kit/floe/parts/diagram/**"
  - "docs/agents/diagrams.md"
---

# 図（線の動きの作りと数は、コードとその LIMIT が持つ）

- **図の形は schema、配置は Go**：図の spec の形（鍵・型・値の一覧・要るもの）は部品 diagram の schema.json だけが決め、jsonschema が確かめる。Go は確かめた値を読むだけで、Go が持つのは次だけ：名前の解決、線の辺と道を決めること（`route.go`・`shares.go`・`detour.go`）、配置の検査（図という機能そのもの。箱の形は `outlineOf` 1 つから取り、描くのも測るのも同じ）、raw の要素（x/net/html で読む）、tree と graph を箱と線に開くこと（`tree.go`・`graph.go`。決定的で、同点は書いた順。read が作った型の構造体を読むだけなので「値を歩くのは Walk だけ」に当たらない）、省いた `width`・`height` を描いた端から決めること（`reach`）。入口は `diagram.Drawer` 1 つで、渡された値をいつも schema で確かめてから読む。配置と形の誤りは spec の中の JSON ポインタで返し、行と列は呼ぶ側が `yamlpos.Place` で付ける。配置の誤りを置くのは `tell` 1 つ
- **graph のアイコンは見えない長方形**：graph のアイコンは、配置の間は見えない長方形（`iconCell`）で、線の道を決めるコードはアイコンを知らない。線の端を置くのは `icons.go` の `settleEnds` 1 か所。graph の `x`・`y` は描いたものの左上（`unseen`）。枠の見出しの場所は `headParts`・`headArea`
- **1 つの辺の線は等分**：1 つの辺に来る n 本の線は辺を n+1 等分した点に置き、点を決めるのは `sidePoint` 1 か所。手で書いた線は `shareEnds`、graph の線は `settleEnds`。dot・行・bar は真ん中のまま（ユーザーが決めた）。この数は docs に書かない（docs/agents/diagrams.md は、書き方・断り・直し方だけを言う）
- **図の字は描く class で測る**：図の字の大きさ・太字・等幅を知るのは `faceOf(class)` 1 つで、part.css と 1 対 1 に書く（part.css の字を変えたら `faceOf` も直す）。描く所も測る所も同じ class の値を読み、呼ぶ側は太字かを選ばない。`TestRenderedDiagramFaces`
- **図は自分の地を塗る**：部品 diagram の枠は地を `--bg`、字を `--fg` で塗り、下を隠すものも `--bg` で塗る。図の外の地は読まない。`TestRenderedDiagramGround`
- **ランダムのテストの個数**：race の時間を占めるランダムのテストの個数は `tries` で読む（`only` で図を名指すものは減らさない）。race のときだけ 1/5 になり、全部の個数は race なしのジョブが走らせる。全部の個数で測った下限と割合は全部の個数のときだけ見て、上限は 1/5 でも見る
- **raw の色は default の palette.css の色の変数**：raw の SVG の色は、`floe theme show default palette.css` が定義する色の変数（docs/agents/diagrams.md の raw の節の表が挙げる）と、raw の中の `data-floe-tone` の下の `var(--tone)`・`--tone-soft`・`--tone-fill` で書かせる。部品の class・`data-floe-look`・`data-floe-part`・テーマだけの変数（`--*-fill`）は勧めない。`TestRenderedFragment`（raw の色を測る）
- **raw の色の表は default の palette の変数と照らす** [raw-in-docs]：docs/agents/diagrams.md の raw の節の「Variable」の表は、default の palette.css の変数の全部から、raw に取らせないもの（字・大きさ・角・影・chart の系列・code の盤・ページの背景の模様。規則の `Allow` が名指し、理由を書く）を除いたものを、名前を字のまま書く（raw が頼る約束なので）。palette に変数が増えたら、表に足すか `Allow` に足すかを、テストが落ちて決めさせる。`TestLintRule/raw-in-docs`
