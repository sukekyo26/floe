---
paths:
  - "internal/{store,server,link,daemon,files}/**"
  - "internal/cli/client.go"
  - "internal/cli/server.go"
---

# 保存先とサーバー

- **保存したページは描いた HTML を返す**：publish（か `store.Rebuild`）の時に `tmpl.Draw` が描いた HTML を写しに保存し、開くときはそれを返す。開くときに描くのは link したデータファイルだけ。CSP とサンドボックスは HTML に焼き込まない。版を増やすのは、publish で中身（データファイルと読んだ画像。theme は入れない）が変わったときだけで、rebuild・開くときの描き直し・edit・link は版も `updatedAt` も動かさず、前の版は描き直さない。番号は rm の後に戻りうるので、タブの印は番号と時刻で作る。縮め方を知るのは store だけで、縮めるのは lock の外
- **slug と写しのパスは 1 か所で作る**：slug をパスにするのは `store` の `dir`、meta.json の `source` を写しのパスにするのは `copyDir`、版の番号を版のパスにするのは `versionFile` だけで、そこで確かめ、呼ぶ側で別に確かめない。URL の `v` を版の番号として読むのは `versionParam`（保存したページの rest の無い URL だけ）、`diff` の `from`・`to` と差分の枠の `?raw` の `from`（`diffParam`）は `versionNumber`。2 つの版とその key を読むのは `store.VersionPair` 1 つ
- **URL 名を省いた publish は `ownSlug` 1 つが決める**：データもリンクも、そのファイルの項目（ページは `publishedFrom`・`publishedFile`、リンクは `path`）を探す。`Prepare` で決め、`commit` が lock の中で確かめ直す。CLI・handler は URL 名を決めない
- **publish は読んだ画像だけを書く**：publish が保存する画像は、描くときに `kit.ReadImage` が読んだ名前のものだけ。送られても読まなかった名前は書かない（信頼境界）。rebuild は前の写しの `images/` を全部引き継ぐ。版の中身の印も、読んだ画像だけから作る。版は画像を別に持たない（描いた HTML に埋め込まれている）
- **サーバーを止めるのは `/api/stop` 1 つ**：`daemon.Stop` が `POST /api/stop` で、止めたいサーバーの pid を本文に書いて頼み、違えば 409。floe のコマンドはシグナルを送らない（名乗る pid は別のプロセスでありうる）。要求の ctx は `Serve` の ctx で切らない。本文は変えない（入れ替えの相手の古いビルドが `decodeStrict` で読むため）
- **サーバーの記録は `server.log` 1 つ**：書くのは `floe serve` だけ（`Store.Begin` の後に `server.OpenLog` で開き、`Config.Log` で渡す）。CLI と daemon は書かず、サーバーを止めたことは `Ensure` の戻り値で CLI が 1 行言う。要求ごとの記録は書かない
- **アーカイブは meta.json の `archived` 1 つ**：一覧から外すのはこれだけで持ち、立てる・外すのは `Store.Edit` と publish し直し（`newMeta`）だけ。`GET /api/pages` は全部を返し、一覧から外すのは見せる側でする
- **フォルダの中は開いた所だけ読む**：フォルダは meta.json の `folder` で見分け、名前で種類を決めない（データファイルかは `Meta.IsData`）。中は `link.Dir` が開いたディレクトリ 1 つを `link.within` を通して返すだけで、全体を歩かず、中の HTML を読まず、HTML ごとの状態を持たない
- **CSP とサンドボックスは返すときに付ける**：ページとその下のファイルはヘッダーで。ヘッダーの無い出口（render・ダウンロード）は、`draw` の `withCutOff` が描いたページの木に CSP の meta を足し、ページの charset の meta を全部外して、floe の charset 1 つを頭の先頭に置く（サーバーが返すページは、page.html の charset が残ってもヘッダーの utf-8 が勝つ）。render は確かめた木のまま書き（`tmpl.Options.CutOff`）、読み直すのは保存したページから作るダウンロードだけ（`CutOffPage`）
- **描くものはまとまり 1 つ**：サーバーは `Config.Drawing` で読んだ Kit とアイコン集を 1 つ持ち、自分の 3 つの根とアイコン集を 1 つのタイマーで毎秒見張り、変われば丸ごと読み直す。見張りが後で直せない描き（ストアに書くものと download）は、描く前に同じ見張りをその場で 1 回走らせる（`latest`）。読めなければ `store.Drawing.Err` で描く出力を全部止める
- **API の誤りは `refuse` 1 つ**：usererr の Report を返し、409 は `store.Conflict` で見分ける。プロトコルの断り（forbidden host・origin・not found、`/api/stop` が名指す pid が自分でないときの 409 など。floe のコマンドが読み、ユーザーに見せない）は英語の固定の文
- **書き込みの本文は `decodeStrict` 1 つで読む**：API の書き込みの本文は、送る形の型に `decodeStrict` が encoding/json/v2 だけで読み（v1 で読み直さない）、誤りは v2 の言葉のまま 400 で断る。受ける鍵は型の欄にだけ書き、map で読んで鍵を拾わない。floe のコマンドの要求は `daemon.Request` 1 つで組み、本文は v2 の `Marshal` で作り、CLI はユーザーの値を `encode` で確かめる。送る型のポインタの欄は `omitempty` でなく `omitzero`
- **タグは足す・外すで書く**：残すページのタグを変えるのは `Edit` の `tags` の `add`・`remove` だけ。送るのはユーザーが変えた名前だけで、タグ全部も、読んだまま触っていない名前も送らない（ほかの書き手が後から書いた分を消さないため）
- **前に読んだ版は `ReadBefore` 1 つ**：差分の起点と知らせの条件を決めるのは store の `ReadBefore` だけで、記録の `n` が最新の版でなければその版、最新の版なら `before`。どちらも今の版に無ければ 0
