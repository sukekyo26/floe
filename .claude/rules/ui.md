---
paths:
  - "internal/ui/**"
  - "internal/pagediff/**"
  - "internal/server/pages.go"
  - "internal/server/api.go"
  - "internal/store/reads.go"
---

# 管理ページと枠

- **未読は reads.json 1 つ**：未読を持つのは publish で保存したページだけ。書くのは store の lock の中だけ。記録の鍵は meta.json の `id` で、slug では持たない（名前の変更は reads.json を書かない）。`id` を作るのは `newMeta` だけで、ほかの meta.json の書き手は読んだ `id` をそのまま書く。ページの `id` を消す操作（今は rm と、別の `id` に置き換える publish）は、同じ lock の中でページより先に `forgetRead` でその記録を消し、reads.json を読めない・書けないときは断る。読んだ印を足すのは `POST /api/pages/{slug}/read` 1 つで、本文はページを返したときにサーバーが作った版の印、slug は枠が自分の持つ値から決める（一覧から取った印は送らない）。戻すのは `DELETE` の同じ道 1 つで、枠の画面の操作だけが呼ぶ。新しい版かを決めるのは store の `news` 1 か所で、画面は比べない。トーストは人が開く操作をしたときだけ出す。`since` は 1 度だけ書き、その後は動かさない
- **ページの中身とタブの印は 1 か所ずつ**：中身は `server` の `content`、タブの印は `version`（ファイルの印は `files.Mark`）で、印は中身の前に取る。返すページはサーバーへのつなぎを持ち続けない（Chrome の 1 つのサーバーへのつなぎは 6 本まで）。ページから枠へ知らせるときは送り先の origin を名指し、枠は自分の iframe の、origin が `null` の知らせで、道が自分の開いた URL と合うものだけを受け、slug は自分の持つ値から決める
- **一覧の列の幅は `list.columns`、つまみは `Grip` 1 つ**：棚・一覧・枠の左の木・差分の目次のつまみは `Grip` 1 つで動かす。幅と畳んだかは見る人の都合としてブラウザ（`localStorage`）にだけ持つ
- **管理ページと枠は `ui.Page` 1 つ**：`/`・`/tasks`・`/a/{slug}/` は、ハンドラが自分の画面（`ui.List`・`ui.Tasks`・`ui.Frame`）を `ui.Page` に渡す。`ui.Page` は React の HTML（`web/dist`）を返す。React の HTML に起動の値を入れるのは `reactPage` だけで、html/template に通さない。ページそのものは `?raw` で、枠の iframe が読む。管理ページと枠には `uiCSP` を付け、枠の中のリンクは `leaveFrame` 1 つが扱う
- **web を触ったら `dist` を作り直してコミットする**：`internal/ui/web/` を変えたら、`just web-build` で作り直した `dist` を同じ PR に入れ、push の前に `just web-check` と `just web-build-check` を通す。`just ci` は Node を使わず、コミットした `dist` を試すので、作り直し忘れに気付かない（気付くのは CI のジョブ「Web」）
- **開いている版は 1 か所**：右のページは `openPage.picked`、全面は `openPage.frameAt()` の `v` で持ち、ページの URL を作るのは `pageHref`・`rawHref`・`diffHref`・`downloadHref` だけ。全面の `?v=` はサーバーの `frame` が読み、JS は読み方を持たない。版を選んで見ている間は最新を既読にしない
- **帯の操作は 1 つの決まり**：帯に見えている操作は「…」に出さず、帯から外れた操作は必ず「…」に出す。「…」はいつも全部の項目を作り、見えるかは CSS で決める。幅で変えるのは 40rem の 1 か所だけ。行は ★ とアーカイブを出し、それ以外を「…」に出す。`TestUIBarActions` が確かめる
- **既にあるタスクはタスクのページで書く**：既にあるタスクのタイトル・URL・カテゴリの並びは「編集」だけ、親は親子のカードだけが変える。ページをカテゴリに付ける書き込み（「タスクを設定」「ページを追加」「タスクから外す」の取り消し）は、`PATCH /api/tasks` 1 回（`taskWrites.putPageOn`）で、名指すタスクの見た版を添えて送る。「タスクを設定」と「ページを追加」は、ほかのタスクに載っているページなら、送る前に `moveAsk` 1 つで聞く（載り先は聞く時の一覧の `task` から読む。取り消しは聞かない）。タスクを作る書き込みは、作るタスクの欄も同じ書き込みで書いてよい。カテゴリは名前を変えず、追加・削除（ページの無いものだけ）・並べ替えだけで、並べ替えのつまみは `Grip` でなく HTML の drag and drop（`TestUITaskEdit`）
- **タグは `openTags` 1 つで変える**：タグを書く画面はタグのモーダル 1 つで、一括の「タグ」・タグの行のボタン・「…」の「タグ」の 3 つが `openTags` を呼ぶ。タグの行（`#read-tags`・`#frame-tags`）は帯の外に置き、帯の決まりに入らない。「…」の「タグ」はいつも出す。`TestUITagLine`
- **管理ページのキーは `appKeys` 1 つ**：キーの表を 1 つ持ち、各画面が `useKeys` に渡す。ページの中から渡すのは、移ることとモーダルを開くことだけのキー（`PAGE_KEYS`）で、枠は自分の iframe の、origin が `null` の知らせだけを、iframe にフォーカスがあるときだけ受ける
- **管理ページの色は明るい 1 組**：色はすべて `index.css` の `@theme` の変数から取る。例外はプロジェクトの色の `TINT_VARS` 1 つで、行ごとの色 `--c` は `@theme` で決まらないので、`@theme` の外で `@theme` の変数と混ぜる（決まりの例外に名指しで置く）。暗い画面・その切り替え・`prefers-color-scheme` と `color-scheme` の決まりは作らない（ユーザーが決めた。複雑になるだけ）。`color-scheme` を付けないので、枠の中のページは OS の設定どおりに `prefers-color-scheme` を読む。
- **字のあるボタンは `Button` 1 つ**：角は `--radius-small`。丸い形はアイコンのボタンと、ボタンでない部品に残す。字は折り返さず、狭いときは並びが折り返す（ユーザーが打った字を持つもの、今は `#tag-new` だけは折り返す）。画面に置く `Button` はアイコンと字、モーダル・選択の帯・トーストの中は字だけ。例外：左の列の行（その下の追加のボタンも）と、設定のモーダルの項目の行と「‹ 設定」は行の形にし、`Button` にしない。帯の画面のリンク（`ScreenSwitch`）とタブも `Button` にしない。`TestUIButtons`
- **管理ページの角と字は `@theme` の目盛り**：角は `--radius-corner`・`--radius-menu`・`--radius-small` か形（`0`・`50%`・`999px`）で、字の大きさは `--text-label`・`--text-small`・`--text-body` で書き、目盛りを置くのは `index.css` の `@theme` だけ。変数の名前はパレットの変数と分け、default から写さない。
- **版の差分は `pagediff` の 1 回の計算から作る**：`pagediff.Diff` 1 つが、字の答え（目次と数）と印のページ（差分の枠）を 1 回の計算から返し、答えは時刻によらず入力だけで決まる（時間切れでなく大きさの予算で切る）。ページの HTML は差分でもサンドボックスの枠（`?raw&v=&from=`）の中だけで見せ、管理ページは字の答えを `textContent` で描く。go-diff を import できるのは `internal/pagediff` だけ（depguard の規則 `go-diff-in-pagediff`）
- **一覧の並びは `arrange` 1 つ、色の順は `projectOrder` 1 つ**：一覧も右のページの `j` `k` も `arrange` の順。全面の `j` `k` は `order` が覚えた順。色の順は全部の項目を数えた `projectOrder` だけが決め、絞り込みで変わらない。タスク一覧の並びは `taskOrder` 1 つで、どの状態も行を最後に変えた新しい順（`at`）
- **行は `Rows` 1 つで描く**：どの画面の行も `Rows` で描く。`moveRow` は描いた行を読み、画面ごとに分岐を足さない。タスク一覧の問い合わせは `TASKS_KEY`、`LIST_KEY` はページ一覧だけ。`order` も描いた行を読む。全面の順と戻り先は `order` 1 つで持ち、一覧の行を選び直すのは `backToRow` 1 か所だけ
- **タスクのページの親と子の並びは `#list` の外**：`Rows` の決まりに入らない
- **タスク一覧は 1 つのプロジェクトだけ**：`/tasks` は `project` を持ち、無ければ `openingProject` 1 か所が選んで URL に書く。あるプロジェクトは `taskProjects` 1 か所が決め、`projectOrder` は色と順にだけ使う。読めない間は選び直さず、覚えも書かない
- **一覧は押している間に作り直さない**：問い合わせと読んだ印の `pages.load` は、`#list` やピンの並びを押してから 2 秒の間（離すまで）は行を作り直さず、次の問い合わせで描く。押下と離す間に行を作り直すと、Chrome は `click` も `dblclick` も出さない。離したことが伝わらない押下があるので、2 秒で止める。操作が呼ぶ `load` は止めない
- **一覧は `pages.load` が 1 本ずつ、書き込みの答えの後に読む**：一覧を読むのは `pages.load` だけで、書き込み（既読の印も）が答えを待つ間は読まない。要求は全部 `lib/api-client.ts` を通す。`TestUILoadOrder`
- **モーダルは `Dialog` 1 つで閉じる**：`close()` を直に呼ばず、close request は `Dialog` の `cancel` が受ける。閉じた後のフォーカスの行き先と答えは、close イベントを待たず同じ task で決める（モーダルごとに決まるものは `afterClose`）。`TestUIDialogClose`
- **書き込みの間、モーダルは `useHold` で止める**：書き込みと、続く一覧の読みを待つ間は `whileHeld` で包み、閉じるのは解いた後にする。送るボタンの `disabled` を関数ごとに出し入れしない。`TestUIDialogHold`
