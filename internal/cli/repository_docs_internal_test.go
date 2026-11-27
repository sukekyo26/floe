package cli

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"floe/internal/version"
)

// The docs of a repository's documents: for people, in English and Japanese, and for agents.
const (
	repositoryDocPath      = "../../docs/repository.md"
	repositoryJaDocPath    = "../../docs/repository.ja.md"
	repositoryAgentDocPath = "../../docs/agents/repositories.md"
)

// blockWith is the one block of lang in the doc at path that holds text.
func blockWith(t *testing.T, path, lang, text string) string {
	t.Helper()

	var found []string
	for _, b := range fences(readGuide(t, path), lang) {
		if strings.Contains(b, text) {
			found = append(found, b)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s holds %d %s blocks with %q, want 1", path, len(found), lang, text)
	}
	return found[0]
}

// The three docs give a repository the same commands, files and words to copy: each is read alone.
func TestRepositoryDocsAgree(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		lang, text string
		docs       []string
	}{
		{"sh", "floe repository new", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"sh", "floe repository render", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"sh", "floe repository check", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"text", "the setting", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"yaml", "documents:", []string{repositoryJaDocPath}},
		{"text", "linguist-generated", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"markdown", "drawn by floe", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"yaml", "actions/checkout", []string{repositoryJaDocPath}},
		{"yaml", "phases:", []string{repositoryJaDocPath}},
	} {
		want := blockWith(t, repositoryDocPath, c.lang, c.text)
		for _, path := range c.docs {
			if got := blockWith(t, path, c.lang, c.text); got != want {
				t.Errorf("%s: the %s block with %q is\n%s\nand in %s\n%s\nmake them one", path, c.lang, c.text, got, repositoryDocPath, want)
			}
		}
	}
}

// The check of the doc for agents is what the CI examples run: a line taken from one alone fails here.
func TestRepositoryDocsCI(t *testing.T) {
	t.Parallel()

	want := strings.TrimSpace(blockWith(t, repositoryAgentDocPath, "sh", "floe repository check"))
	for _, text := range []string{"actions/checkout", "phases:"} {
		var lines []string
		for line := range strings.Lines(blockWith(t, repositoryDocPath, "yaml", text)) {
			lines = append(lines, strings.TrimSpace(line))
		}
		if !strings.Contains(strings.Join(lines, "\n"), want) {
			t.Errorf("%s: the yaml block with %q does not run\n%s\nas %s does", repositoryDocPath, text, want, repositoryAgentDocPath)
		}
	}
}

// The CI examples of the docs for people install the floe of this release: releasing it bumps them, and one left
// behind fails here. The doc for agents is built into floe, so it holds no URL of its source (check-no-repo-name).
func TestRepositoryDocsVersion(t *testing.T) {
	t.Parallel()

	installs := regexp.MustCompile(`floe/v([0-9.]+)/install\.sh \| FLOE_VERSION=([0-9.]+)`)
	for path, want := range map[string]int{repositoryDocPath: 2, repositoryJaDocPath: 2, repositoryAgentDocPath: 0} {
		found := installs.FindAllStringSubmatch(readGuide(t, path), -1)
		if len(found) != want {
			t.Errorf("%s installs floe %d times, want %d: the test reads them by this form", path, len(found), want)
		}
		for _, m := range found {
			if !slices.Equal(m[1:], []string{version.Version, version.Version}) {
				t.Errorf("%s: %q is not of the release, %s", path, m[0], version.Version)
			}
		}
	}
	// The setting the docs for people show is of the release too.
	for _, path := range []string{repositoryDocPath, repositoryJaDocPath} {
		if got := blockWith(t, path, "yaml", "documents:"); !strings.HasPrefix(got, "version: '"+version.Version+"'\n") {
			t.Errorf("%s: the setting is\n%s\nnot of the release, %s", path, got, version.Version)
		}
	}
}
