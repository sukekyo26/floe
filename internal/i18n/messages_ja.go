package i18n

func init() {
	register("ja", messagesJA)
}

var messagesJA = map[string]string{
	// link
	"link_not_real":        "%s は絶対パスの実パス（symlink を解決して正規化したパス）ではないので、登録できるかを確かめられません",
	"link_not_html":        "%s は通常の HTML ファイル（.html / .htm）ではないので、登録できません",
	"link_home_unresolved": "ホームディレクトリ「%s」を解決できないので、登録できるかを確かめられません。HOME に存在するディレクトリの絶対パスを設定してください",
	"link_hidden_home":     "%s はホーム直下の隠しディレクトリ（設定や鍵の置き場）の中にあるので、登録できません",
	"link_exposes_dir":     "%[1]s を登録すると %[2]s 全体が公開されます。専用のディレクトリに置いてから登録してください",

	// store
	"store_invalid_lang":  "言語「%s」は使えません。--lang には en・ja・pt-BR のような BCP 47 の言語タグを指定してください",
	"store_cannot_read":   "%s を読めません（%s）。パスとファイルがあるかを確かめてください",
	"store_invalid_slug":  "URL 名「%s」は使えません。英小文字・数字・ハイフンで入力してください",
	"store_slug_is_page":  "URL 名「%[1]s」は保存したページで使われています。別の URL 名にするか、先にそれを消してください（floe rm %[1]s）",
	"store_slug_is_link":  "URL 名「%[1]s」はリンクで使われています。別の URL 名にするか、先にそれを消してください（floe rm %[1]s）",
	"store_slug_linked":   "URL 名「%[1]s」は %[2]s のリンクで使われています。別の URL 名にするか、floe publish --link --force でそのリンクを置き換えてください",
	"store_slug_owned":    "URL 名「%[1]s」はプロジェクト %[2]s から公開されています。--slug で別の URL 名にするか、--force を付けてそのページを置き換えてください",
	"store_slug_taken":    "URL 名「%s」は使われています。別の URL 名を入力してください",
	"store_project_slash": "プロジェクト名に / だけは使えません",
	"store_no_page":       "ページ「%s」はありません（floe list で確かめられます）",
}
