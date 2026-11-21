---
paths:
  - "internal/{browsertest,servetest}/**"
  - ".github/**"
  - "justfile"
---

# ブラウザのテストと CI

- **ブラウザのテストは `internal/browsertest` に置く**：サーバーは `internal/servetest` で本当のポートに起こし、publish は CLI（`cli.Run`）で行う。CI の組は `just test-browser` が機械で分ける（テストを足しても組は選ばない）。組が 8 分を超えた注記が出たら、justfile の `browser_rest_jobs` を増やす（`LIMIT:`）
- **テストの名前の頭は 3 つ**：決まりの行が挙げる見た目の決まりは `looks` の 1 行（`TestLookRule/<名前>`）、管理ページを開くテストは `TestUI…`、描いたページのそのほかは `TestRendered…`。ファイルの名前も同じ頭にする。`TestNames`
- **偽の一覧はページより前に**：管理ページの一覧を偽のものにするのは `(*browser).stub` 1 つで、ページの script より前に走る script で `fetch` を替える。最初の `stub` は空のタブで、偽にするページを開く前に呼ぶ（開いた後に替えると、ページがもう出した読みが偽を上書きする）。本物の一覧に戻すのは `unstub`。開いた後に偽の答えを替えるときは、偽の一覧を替えて `reread` する（ページを読み込み直さない）。開いた後に本物の答えを止めるのは `holdReads`、本物の答えを古い形に替えて返すのは `staleReads` だけ（読み込みの順を確かめる `TestUILoadOrder` は自前の `loadOrderJS`）
- **動くスタイルは止まってから読む**：ブラウザのテストで、transition・animation のかかる性質の値を、期待を知らずに読んで記録するときは `settled` で読み、動きが止まるのを待つだけのときは `still` で待つ（どちらも、ページ全体の終わりのある動きを待つ）。期待の値まで待つ読みは `until` のまま。決まった時間の sleep で動きを待たない
- **操作の終わりを待つ**：操作の後は、結果の DOM（行・字・属性・フォーカス）を `until` で待つ。変わらないことと、書き込みの後の読みそのものを確かめるときだけ、`acted` と `untilRead` で読みの数を待ち、書き込みの記録が増えたのも一緒に待つ。ページ全体が移る操作の後は URL を待てばよい（`until` は読み込み中のページでは答えない）が、後から読み込む一覧などはそれを待つ
- **画面だけで確かめる**：管理ページのテストは、ページの関数・変数を読まず呼ばず、クラスで選ばず、状態は `aria-*`・`data-*` で読む。`TestScreenOnly`
- **入力は `fill`**：欄に字を入れるのは `fill`。`.value =` と作った `input` イベントは React に届かない
- **Chrome は版を決めて入れる**：CI の Chrome の版は go-ci.yml の `CHROME_VERSION` 1 か所で、上げるのは手で
