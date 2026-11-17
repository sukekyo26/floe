---
paths:
  - "**/*_unix.go"
  - "**/*_windows.go"
  - "**/*_test.go"
  - "internal/{files,daemon,testfs}/**"
---

# OS ごとのコード

- **OS ごとのファイルは対にする**：Unix と Windows で違う所は `<名>_unix.go`（`//go:build unix`）と `<名>_windows.go` に、同じ名前と署名で置く。置くのは OS の API を呼ぶ最小の関数と、OS で値の違う定数だけ。判定・誤りの言葉・分岐は共通のファイルに置く。Unix と Windows を `runtime.GOOS` で分けない
- **OS で違うテストの準備は `internal/testfs`**：FIFO・待たないロック・symlink・junction は testfs の道具で作る（作れない OS や権限ではスキップする）。テストに `syscall`・`x/sys` を直に書かない。Windows でコンパイルできないテストは `_unix_test.go` に置く
- **Windows で走らせないテストには印を付ける**：期待そのものが Unix のもの（umask・権限のビット・root・FIFO）だけを `testfs.UnixOnly` で飛ばす。Windows で落ちる本体の不具合は、テストを飛ばさずに直す。準備だけが Unix のものなら、その所をサブテストに分けて印を付け、残りは Windows で走らせる。パッケージごと飛ばさない（ブラウザのテスト `internal/browsertest` だけは Windows のジョブが外す）
- **改行は LF**：`.gitattributes` が決める。わざと CRLF・BOM・UTF-8 でないバイト列を持つ testdata には `-text` を付ける
