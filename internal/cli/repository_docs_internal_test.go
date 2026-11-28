package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
		{"text", "the setting", []string{repositoryAgentDocPath}},
		{"yaml", "documents:", []string{repositoryJaDocPath, repositoryAgentDocPath}},
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
// behind fails here. The doc for agents names the version as <version>, which this does not match.
func TestRepositoryDocsVersion(t *testing.T) {
	t.Parallel()

	installs := regexp.MustCompile(`install\.sh \| FLOE_VERSION=([0-9.]+)`)
	for path, want := range map[string]int{repositoryDocPath: 2, repositoryJaDocPath: 2, repositoryAgentDocPath: 0} {
		found := installs.FindAllStringSubmatch(readGuide(t, path), -1)
		if len(found) != want {
			t.Errorf("%s installs floe %d times, want %d: the test reads them by this form", path, len(found), want)
		}
		for _, m := range found {
			if m[1] != version.Version {
				t.Errorf("%s: %q is not of the release, %s", path, m[0], version.Version)
			}
		}
	}
	// The setting the docs show is of the release too.
	for _, path := range []string{repositoryDocPath, repositoryJaDocPath, repositoryAgentDocPath} {
		if got := blockWith(t, path, "yaml", "documents:"); !strings.HasPrefix(got, "version: '"+version.Version+"'\n") {
			t.Errorf("%s: the setting is\n%s\nnot of the release, %s", path, got, version.Version)
		}
	}
}

// The guide's setup, run as it is written in a new repository with a document of its template, ends with a check
// that passes, and the check fails once the document is changed and not drawn again.
func TestRepositoryGuideRuns(t *testing.T) {
	t.Parallel()

	guide := readGuide(t, repositoryAgentDocPath)
	_, setUp, ok := strings.Cut(guide, "## Set up a repository")
	if !ok {
		t.Fatalf("%s has no section Set up a repository", repositoryAgentDocPath)
	}
	setUp, _, _ = strings.Cut(setUp, "\n## ")
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := Env{Stdout: io.Discard, Stderr: io.Discard, Dir: dir, Getenv: nil}
	ran := 0
	for _, block := range fences(setUp, "sh") {
		for line := range strings.Lines(block) {
			words := strings.Fields(line)
			switch {
			case len(words) == 0:
				continue
			case words[0] == "mkdir":
				for _, d := range words[1:] {
					if !strings.HasPrefix(d, "-") {
						if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
							t.Fatal(err)
						}
					}
				}
				continue
			case words[0] != "floe" || len(words) < 3:
				t.Fatalf("%s: the setup runs %q, which this test does not", repositoryAgentDocPath, line)
			case words[2] == "render":
				// The guide's step 2: write the documents, here of the template's example, before they are drawn.
				example, err := os.ReadFile(filepath.Join(dir, ".floe", "templates", "api-spec", "example.yaml"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "docs", "specs", "orders.yaml"), example, 0o600); err != nil { //nolint:gosec // G703: the test's own temporary file
					t.Fatal(err)
				}
			}
			if code := Run(words[1:], env); code != 0 {
				t.Errorf("%s: %s = %d", repositoryAgentDocPath, strings.TrimSpace(line), code)
			}
			ran++
		}
	}
	if ran != 4 {
		t.Fatalf("%s: the setup runs %d floe commands, want 4 (new, template new, render, check)", repositoryAgentDocPath, ran)
	}
	file := filepath.Join(dir, "docs", "specs", "orders.yaml")
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	changed := regexp.MustCompile(`(?m)^title: .*$`).ReplaceAll(src, []byte("title: Orders again"))
	if bytes.Equal(changed, src) {
		t.Fatalf("the example of the template holds no title: %s", src)
	}
	if err := os.WriteFile(file, changed, 0o600); err != nil { //nolint:gosec // G703: the test's own temporary file
		t.Fatal(err)
	}
	var stderr strings.Builder
	env.Stderr = &stderr
	if code := Run([]string{"repository", "check"}, env); code != 1 || !strings.Contains(stderr.String(), "is not what") {
		t.Errorf("floe repository check of a document not drawn again = %d, %q", code, stderr.String())
	}
}
