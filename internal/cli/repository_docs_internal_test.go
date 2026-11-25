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
		{"sh", "floe template new", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"sh", "  floe render docs/specs", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"sh", "floe template check &&", []string{repositoryJaDocPath, repositoryAgentDocPath}},
		{"sh", "no YAML for", []string{repositoryJaDocPath, repositoryAgentDocPath}},
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

// The CI examples install the floe of this release: releasing it bumps them, and one left behind fails here.
func TestRepositoryDocsVersion(t *testing.T) {
	t.Parallel()

	installs := regexp.MustCompile(`floe/v([0-9.]+)/install\.sh \| FLOE_VERSION=([0-9.]+)`)
	for path, want := range map[string]int{repositoryDocPath: 2, repositoryJaDocPath: 2, repositoryAgentDocPath: 1} {
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
}
