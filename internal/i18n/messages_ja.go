package i18n

func init() {
	register("ja", messagesJA)
}

var messagesJA = map[string]string{
	// link
	"link_not_real":        "%s は実パス（symlink を解決して正規化したパス）ではないので、登録できるかを確かめられません",
	"link_not_html":        "%s は通常の HTML ファイル（.html / .htm）ではないので、登録できません",
	"link_home_unresolved": "ホームディレクトリ「%s」を解決できないので、登録できるかを確かめられません。HOME に存在するディレクトリの絶対パスを設定してください",
	"link_hidden_home":     "%s はホーム直下の隠しディレクトリ（設定や鍵の置き場）の中にあるので、登録できません",
	"link_exposes_dir":     "%[1]s を登録すると %[2]s 全体が公開されます。専用のディレクトリに置いてから登録してください",
}
