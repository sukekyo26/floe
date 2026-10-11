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

	// store: publishing and editing pages
	"store_invalid_lang":  `invalid language "%s": give --lang a BCP 47 tag such as en, ja or pt-BR`,
	"store_cannot_read":   "cannot read %s (%s); check the path and that the file exists",
	"store_invalid_slug":  `invalid slug "%s": use only lowercase letters, digits and hyphens`,
	"store_slug_is_page":  `slug "%[1]s" is used by a stored page; choose another slug, or remove that page first (floe rm %[1]s)`,
	"store_slug_is_link":  `slug "%[1]s" is used by a linked file; choose another slug, or remove that link first (floe rm %[1]s)`,
	"store_slug_linked":   `slug "%[1]s" is linked to %[2]s; choose another slug, or replace that link with floe publish --link --force`,
	"store_slug_owned":    `slug "%[1]s" was published from project %[2]s; use another --slug, or add --force to replace that page`,
	"store_slug_taken":    `slug "%s" is already in use; choose another slug`,
	"store_project_slash": `a project name cannot be just "/"`,
	"store_no_page":       `no page "%s" (see floe list)`,
}
