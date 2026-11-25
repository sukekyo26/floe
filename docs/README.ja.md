# floe

[English README](../README.md)

AI エージェントが書く YAML のデータから文書を作ります。テンプレートが HTML に描き、schema に合わないデータは floe が断ります。プロジェクトのテンプレート・データ・ページをリポジトリに置き、CI で確かめられます（[repository.ja.md](repository.ja.md)）。ローカルのサーバーで読み、エージェントが publish したページも、`http://127.0.0.1:7325` で見せます。

> **試作の段階です。** floe はまだ大きく変わります。版が上がると、コマンド・データファイル・テンプレートが、移行の手順なしに変わることがあります。試すのは自由ですが、広く使ってもらう段階ではありません。

## インストール

linux と macOS（amd64 / arm64）:

```sh
curl -fsSL https://raw.githubusercontent.com/sukekyo26/floe/main/install.sh | sh
```

Windows 11（amd64 / arm64）、PowerShell で:

```powershell
irm https://raw.githubusercontent.com/sukekyo26/floe/main/install.ps1 | iex
```

`floe.exe` は `%USERPROFILE%\.local\bin` に入ります。Windows で違うことは [commands.ja.md](commands.ja.md#windows) にあります。

## 使う

```sh
floe skill install <agent>   # 対応するエージェントは floe skill install --help
```

載っていないエージェントには、次の 1 行を指示（AGENTS.md など）に書きます：

```markdown
To show the user a page, or to write or change a document that is a YAML file with `template` and `lang` at its top, use floe: run `floe guide` first and follow it. It is the guide of the floe installed, so read it each time rather than from memory.
```

コマンドの一覧は `floe --help`。コマンドと設定は [commands.ja.md](commands.ja.md) にあります。

## ライセンス

[MIT](../LICENSE)。floe のバイナリには、ほかの人が作った Go の標準ライブラリと Go のモジュールが、それぞれのライセンスのまま入っています。`floe licenses` が floe のライセンスとそれらのライセンスを出します（中身は [internal/licenses/NOTICES.txt](../internal/licenses/NOTICES.txt)）。floe が図のために同梱する [Lucide](https://lucide.dev) のアイコン（`lucide/…`）は ISC ライセンスで、一部は MIT ライセンスも受けます：[internal/icons/lucide/LICENSE](../internal/icons/lucide/LICENSE)。
