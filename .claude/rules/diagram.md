---
paths:
  - "internal/diagram/**"
  - "internal/kit/floe/parts/diagram/**"
---

# 図（線の動きは docs/diagrams.md）

- **図の形は schema、配置は Go**：図の spec の形（鍵・型・値の一覧・要るもの）は部品 diagram の schema.json だけが決め、jsonschema が確かめる。Go は確かめた値を読むだけで、Go が持つのは次だけ：名前の解決、線の辺と道を決めること（`route.go`・`shares.go`）、配置の検査（図という機能そのもの。箱の形は `outlineOf` 1 つから取り、描くのも測るのも同じ）、raw の要素（x/net/html で読む）、tree と graph を箱と線に開くこと（`tree.go`・`graph.go`。決定的で、同点は書いた順。read が作った型の構造体を読むだけなので「値を歩くのは Walk だけ」に当たらない）、省いた `width`・`height` を描いた端から決めること（`reach`）。入口は `diagram.Drawer` 1 つで、渡された値をいつも schema で確かめてから読む。配置と形の誤りは spec の中の JSON ポインタで返し、行と列は呼ぶ側が `yamlpos.Place` で付ける。配置の誤りを置くのは `tell` 1 つ
- **graph のアイコンは見えない長方形**：graph のアイコンは、配置の間は見えない長方形（`iconCell`）で、線の道を決めるコードはアイコンを知らない。線の端を置くのは `icons.go` の `settleEnds` 1 か所。graph の `x`・`y` は描いたものの左上（`unseen`）。枠の見出しの場所は `headParts`・`headArea`
- **1 つの辺の線は等分**：1 つの辺に来る n 本の線は辺を n+1 等分した点に置き、点を決めるのは `sidePoint` 1 か所。手で書いた線は `shareEnds`、graph の線は `settleEnds`。dot・行・bar は真ん中のまま（ユーザーが決めた）
- **図の字は描く class で測る**：図の字の大きさ・太字・等幅を知るのは `faceOf(class)` 1 つで、part.css と 1 対 1 に書く（part.css の字を変えたら `faceOf` も直す）。描く所も測る所も同じ class の値を読み、呼ぶ側は太字かを選ばない。`TestRenderedDiagramFaces`
- **図は自分の地を塗る**：部品 diagram の枠は地を `--bg`、字を `--fg` で塗り、下を隠すものも `--bg` で塗る。図の外の地は読まない。`TestRenderedDiagramGround`
- **ランダムのテストの個数**：race の時間を占めるランダムのテストの個数は `tries` で読む（`only` で図を名指すものは減らさない）。race のときだけ 1/5 になり、全部の個数は race なしのジョブが走らせる。全部の個数で測った下限と割合は全部の個数のときだけ見て、上限は 1/5 でも見る
- **raw の色はパレットの表の変数**：raw の SVG の色は、docs のパレットの表の変数と、raw の中の `data-floe-tone` の下の `var(--tone)` で書かせる。部品の class・`data-floe-look`・`data-floe-part`・テーマだけの変数は勧めない。`TestLookRuleFragmentRoot`（色だけを測る）
