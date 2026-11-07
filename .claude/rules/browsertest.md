---
paths:
  - "internal/{browsertest,servetest}/**"
  - ".github/**"
  - "justfile"
---

# ブラウザのテストと CI

- **ブラウザのテストは `internal/browsertest` に置く**：サーバーは `internal/servetest` で本当のポートに起こし、publish は CLI（`cli.Run`）で行う。CI は justfile の `browser_looks` に並べた重いテストと、その残りの 2 つのジョブで走らせる。テストを足したら、2 つの時間の釣り合いを手で直す（`LIMIT:`）
- **偽の一覧はページより前に**：管理ページの一覧を偽のものにするのは `(*browser).stub` 1 つで、ページの script より前に走る script で `fetch` を替える。最初の `stub` は空のタブで、偽にするページを開く前に呼ぶ（開いた後に替えると、ページがもう出した読みが偽を上書きする）。本物の一覧に戻すのは `unstub`
- **動くスタイルは止まってから読む**：ブラウザのテストで、transition・animation のかかる性質の値を、期待を知らずに読んで記録するときは `settled` で読み、動きが止まるのを待つだけのときは `still` で待つ（どちらも、ページ全体の終わりのある動きを待つ）。期待の値まで待つ読みは `until` のまま。決まった時間の sleep で動きを待たない
- **操作の終わりを待つ**：操作の後に値を読むときは、操作の途中で変わる印（ボタンの字、URL）でなく、操作の最後に変わるもの（操作中を表す変数、読む値そのもの）を `until` で待つ
- **Chrome は版を決めて入れる**：CI の Chrome の版は go-ci.yml の `CHROME_VERSION` 1 か所で、上げるのは手で
