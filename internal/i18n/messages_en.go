package i18n

func init() {
	register(English, messagesEN)
}

// messagesEN is the reference catalog: every other language has exactly its keys.
var messagesEN = map[string]string{
	// link: which files may be linked
	"link_not_real":        "%s is not its absolute real path; pass it through filepath.Abs and filepath.EvalSymlinks (symlinks resolved, path cleaned) first",
	"link_not_html":        "%s is not a regular .html or .htm file; only HTML files can be linked",
	"link_home_unresolved": `cannot resolve the home directory "%s"; set HOME to an existing absolute directory`,
	"link_hidden_home":     "%s is under a hidden directory of your home (settings and keys live there); move it out to link it",
	"link_exposes_dir":     "%s would expose all of %s; move it into a directory of its own and link it there",
}
