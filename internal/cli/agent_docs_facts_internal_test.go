package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"floe/internal/files"
	"floe/internal/i18n"
	"floe/internal/link"
	"floe/internal/store"
)

// The docs for agents but the guide and the doc of tasks (guidePath, taskGuidePath).
const (
	partDocPath     = "../../docs/agents/parts.md"
	templateDocPath = "../../docs/agents/templates.md"
	themeDocPath    = "../../docs/agents/themes.md"
	diagramDocPath  = "../../docs/agents/diagrams.md"
)

// shellWord is a word of a command line in a doc, as a shell splits it: a run in double quotes, or of no space.
var shellWord = regexp.MustCompile(`"[^"]*"|\S+`)

// shellWords returns the words of line, a command line of a doc without its program's name: split at spaces but in
// double quotes, up to a word that starts with # (a comment) or to &&, each word's double quotes taken off. The comment is cut
// before the quotes come off, so that --task "#130" keeps its value.
// LIMIT: double quotes and # alone are read: single quotes, --flag="a b", a pipe and a line ending in \ are split
// otherwise than a shell does (the docs write none). Read them if a doc comes to.
func shellWords(line string) []string {
	var words []string
	for _, w := range shellWord.FindAllString(line, -1) {
		if strings.HasPrefix(w, "#") || w == "&&" {
			break
		}
		words = append(words, strings.Trim(w, `"`))
	}
	return words
}

// shCommands returns the floe command lines of the sh blocks of doc, each without its floe: the lines that start so.
func shCommands(doc string) []string {
	var out []string
	for _, block := range fences(doc, "sh") {
		for line := range strings.Lines(block) {
			if command, ok := strings.CutPrefix(strings.TrimSpace(line), "floe "); ok {
				out = append(out, command)
			}
		}
	}
	return out
}

// docQuote is a text floe makes that a doc quotes as it is, and how many times the doc holds it.
type docQuote struct {
	doc, text string
	times     int
}

// The docs for agents quote the texts floe makes as floe makes them: each quote below, made from what floe makes it
// of (the catalog, a limit, the places it reads), stands in its doc as many times as said, the doc read alone.
// LIMIT: a quote is held as written in the doc: a row of a table that floe no longer makes, and that no quote below
// names, stays unseen; add its quote here when a doc quotes a new text of floe's.
func TestAgentDocQuotes(t *testing.T) {
	t.Parallel()

	ja := i18n.New(i18n.Tag("ja"))
	unknown, _, _ := strings.Cut(english.Msg("tmpl_unknown", "api-spec", ""), ";")
	usage := func(text string) string { return english.Msg("cli_usage", text) }
	states := make([]string, 0, len(store.States))
	for _, s := range store.States {
		states = append(states, "`"+s.Name+"`")
	}
	quotes := []docQuote{
		{taskGuidePath, english.Msg("store_task_urls_max", "#130", store.MaxURLs+1, store.MaxURLs), 1},
		{taskGuidePath, english.Msg("store_task_url", "ftp://x"), 1},
		{taskGuidePath, usage(english.Msg("cli_task_task_off", "state")), 1},
		{taskGuidePath, english.Msg("cli_task_page_no_task", "design-notes"), 1},
		{taskGuidePath, usage(english.Msg("cli_task_needs_pages", "pin")), 1},
		{taskGuidePath, usage(english.Msg("cli_task_no_pages", "forget")), 1},
		{taskGuidePath, english.Msg("cli_task_no_row", "#130", store.Project("floe")), 1},
		{taskGuidePath, english.Msg("cli_task_no_row", "#99", store.Project(store.ProjectKey(""))), 1},
		{taskGuidePath, english.Msg("store_category_paged", strconv.QuoteToGraphic("designs"), "#130", store.Project("floe"), "design-notes"), 1},
		{taskGuidePath, english.Msg("store_task_pages", "#130", store.Project("floe"), 1, "design-notes"), 1},
		{taskGuidePath, english.Msg("store_parent_none", "#42", store.Project("floe")), 1},
		{taskGuidePath, english.Msg("store_parent_nested", "#130", "#120"), 1},
		{taskGuidePath, english.Msg("store_parent_kids", "#42", "#41"), 1},
		{taskGuidePath, usage(english.Msg("cli_task_parent_task")), 1},
		{taskGuidePath, usage(english.Msg("cli_task_no_task")), 1},
		{taskGuidePath, english.Msg("store_no_page", "design-notes"), 1},
		{taskGuidePath, english.Msg("cli_task_none_open", store.Project("floe")), 1},
		{taskGuidePath, refused(t, "task", "design-notes"), 1},
		{taskGuidePath, strings.Join(states[:len(states)-1], ", ") + " or " + states[len(states)-1], 2},
		{taskGuidePath, "`http` or `https` URLs, " + strconv.Itoa(store.MaxURLs) + " at most", 1},
		{guidePath, english.Msg("ui_edit") + ", open " + english.Msg("ui_edit_more") + ", which is folded, type the project's name in " +
			english.Msg("ui_project") + ", and " + english.Msg("ui_save") + ".", 1},
		{guidePath, "in Japanese these are " + ja.Msg("ui_edit") + ", " + ja.Msg("ui_edit_more") + ", " + ja.Msg("ui_project") + " and " +
			ja.Msg("ui_save") + ".", 1},
		{repositoryAgentDocPath, unknown, 1},
		{guidePath, "cut to " + strconv.Itoa(files.MaxSlugLen) + " characters", 1},
		{guidePath, "MUST be 1 to " + strconv.Itoa(files.MaxSlugLen) + " lower case", 1},
	}
	docHolds(t, quotes)
	t.Run("the places", func(t *testing.T) {
		t.Parallel()
		docHolds(t, []docQuote{
			{diagramDocPath, iconsNotKept(t), 1},
			{partDocPath, ownPlaceQuote(t, "parts"), 1},
			{templateDocPath, ownPlaceQuote(t, "templates"), 1},
			{themeDocPath, ownPlaceQuote(t, "themes"), 1},
		})
	})
}

// docHolds checks that each doc holds its quote as many times as said.
func docHolds(t *testing.T, quotes []docQuote) {
	t.Helper()
	for _, q := range quotes {
		switch got := strings.Count(readGuide(t, q.doc), q.text); {
		case got < q.times:
			t.Errorf("%s holds %q %d times, floe makes it %d: write it as floe says it", q.doc, q.text, got, q.times)
		case got > q.times:
			t.Errorf("%s holds %q %d times, the test %d: write it as floe says it, or, if the doc now says it more often, change its count in the test", q.doc, q.text, got, q.times)
		}
	}
}

// refused returns what floe prints for args, a command it refuses, without its head (floe: ) and its line break.
func refused(t *testing.T, args ...string) string {
	t.Helper()
	code, _, errs := runIn(guideEnv(t, t.TempDir()), args...)
	if code == 0 {
		t.Fatalf("floe %v = 0, want a refusal", args)
	}
	return strings.TrimSuffix(strings.TrimPrefix(errs, "floe: "), "\n")
}

// iconsNotKept returns what floe prints for a diagram of an icon of a set not kept, as diagrams.md quotes it: the
// icons' directory as <dir>, cut after the command that keeps the set.
func iconsNotKept(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.yaml")
	if err := os.WriteFile(spec, []byte("title: T\nparts:\n  - icon: { x: 10, y: 10, icon: aws/amazon-ec2 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := guideEnv(t, dir)
	_, _, errs := runIn(env, "diagram", "inspect", spec)
	text, _, ok := strings.Cut(strings.ReplaceAll(errs, env.Getenv("FLOE_ICONS_DIR"), "<dir>"), `fetch aws"`)
	if !ok {
		t.Fatalf("floe diagram inspect of an AWS icon not kept printed %q", errs)
	}
	// The docs write the place with / on every system; the message holds no other \.
	return filepath.ToSlash(text) + `fetch aws"…`
}

// ownPlaceQuote returns how the docs say where one's own of kind are: its setting and its defaults, from the places
// ownPlace makes with the XDG directory alone set, and with the home alone, written with / as the docs write them.
func ownPlaceQuote(t *testing.T, kind string) string {
	t.Helper()
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	x, h := filepath.Join(root, "X"), filepath.Join(root, "H")
	xdg, err := with(map[string]string{"XDG_CONFIG_HOME": x}).ownPlace(kind)
	if err != nil {
		t.Fatal(err)
	}
	home, err := with(map[string]string{link.HomeKey: h}).ownPlace(kind)
	if err != nil {
		t.Fatal(err)
	}
	return "`" + xdg.key + "` (default `" + filepath.ToSlash(strings.Replace(xdg.dir, x, "$XDG_CONFIG_HOME", 1)) +
		"`, else `" + filepath.ToSlash(strings.Replace(home.dir, h, "~", 1)) + "`)"
}

// Every flag floe task takes is named in the doc of tasks, as `--name` or `--name …`: a flag a PR adds is told there.
// LIMIT: one way alone, as no table is read here: a flag the doc's tables still name after floe dropped it passes,
// unless a command of the doc gives it (TestGuideCommands); and a row of a flag's table taken out passes while the doc
// names the flag elsewhere (each flag is named twice or more). Read the tables if a flag comes to be left in a table
// alone, or out of one.
func TestTaskGuideFlags(t *testing.T) {
	t.Parallel()

	doc := readGuide(t, taskGuidePath)
	root, _ := newRoot(guideEnv(t, t.TempDir()), nil)
	task, _, err := root.Find([]string{"task"})
	if err != nil {
		t.Fatal(err)
	}
	task.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return // cobra's
		}
		if name := "`--" + f.Name; !strings.Contains(doc, name+"`") && !strings.Contains(doc, name+" ") {
			t.Errorf("%s names no --%s, a flag of floe task: name it in the doc", taskGuidePath, f.Name)
		}
	})
}
