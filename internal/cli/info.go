package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"floe/internal/store"
)

func (a *app) infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <slug>",
		Short: "Print a page's project, task, data file and the rest floe keeps of it, a name and a value a line",
		Long: `Print the page slug names, as the server keeps it (started when it is not running), and write
nothing: a tab-separated name and value a line, the same names in the same order each time, an empty
value for none. Text values are quoted as Go quotes them (title, project, task, category, repository,
source, and each of tags, one after another with a space); slug, url, kind, pinned, archived, version
and updated are not. project is empty for a page of no project and, unquoted, is what --project of
floe task takes. kind is data, link or folder. source is the data file's path from the root of
repository, or its real path when repository is empty, or the linked file's or folder's real path.
version is empty for a link. updated is the time, local, as floe list prints it.`,
		Args: usage(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			// LIMIT: every page is read to tell one, as floe task --slug does. Add GET /api/pages/{slug} if the
			// list grows to where that is slow.
			c, list, err := a.pageList(cmd.Context())
			if err != nil {
				return err
			}
			page, err := listedPage(list, args[0])
			if err != nil {
				return err
			}
			for _, line := range infoLines(page, pageURL(c.base, page.Slug)) {
				a.print("%s", line)
			}
			return nil
		},
	}
}

// infoLines is the lines floe info prints of page, served at url: a name, a tab and a value each, text values quoted.
// LIMIT: a page published before data files were keyed by their files holds its project in PublishedFrom, which
// repository and source then tell wrongly. No compatibility is kept: publish it again to mend it.
func infoLines(page store.Listed, url string) []string {
	kind, source := "data", page.PublishedFile
	if page.IsLink() {
		kind, source = "link", page.Path
	}
	if page.Folder {
		kind = "folder"
	}
	project := ""
	if page.TaskProject != store.ProjectKey("") {
		project = page.TaskProject
	}
	var on store.PageTask
	if page.Task != nil {
		on = *page.Task
	}
	tags := make([]string, len(page.Tags))
	for i, t := range page.Tags {
		tags[i] = strconv.Quote(t)
	}
	version := ""
	if page.Version > 0 {
		version = strconv.Itoa(page.Version)
	}
	fields := [][2]string{
		{"slug", page.Slug},
		{"title", strconv.Quote(page.Title)},
		{"url", url},
		{"kind", kind},
		{"project", quoted(project)},
		{"task", quoted(on.Name)},
		{"category", quoted(on.Category)},
		{"pinned", strconv.FormatBool(on.Pin)},
		{"tags", strings.Join(tags, " ")},
		{"archived", strconv.FormatBool(page.Archived)},
		{"repository", quoted(page.PublishedFrom)},
		{"source", quoted(source)},
		{"version", version},
		{"updated", localTime(page.UpdatedAt)},
	}
	lines := make([]string, len(fields))
	for i, f := range fields {
		lines[i] = f[0] + "\t" + f[1]
	}
	return lines
}

// quoted is s quoted as Go quotes it, or empty for none.
func quoted(s string) string {
	if s == "" {
		return ""
	}
	return strconv.Quote(s)
}
