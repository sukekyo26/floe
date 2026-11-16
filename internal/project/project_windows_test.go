package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"floe/internal/files"
	"floe/internal/project"
)

// A file in a repository named by a path without \\?\ is keyed by its repository and its path from the root: the
// root git gives (C:/…) is made real as the file is, so the two compare.
func TestPlaceWindows(t *testing.T) {
	t.Parallel()

	repo := filepath.Join(t.TempDir(), "repo")
	file := filepath.Join(repo, "docs", "a.yaml")
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "init", "-q")
	if repoName, place := project.Place(t.Context(), file); repoName != "repo" || place != "docs/a.yaml" {
		t.Errorf("Place(%q) = %q, %q, want repo, docs/a.yaml", file, repoName, place)
	}
}

// Where a directory is in its repository is told from its real path (\\?\…) too, as a link's folder is registered.
func TestWithinWindows(t *testing.T) {
	t.Parallel()

	repo := filepath.Join(t.TempDir(), "repo")
	docs := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docs, 0o750); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "init", "-q")
	at, err := files.Real(docs)
	if err != nil {
		t.Fatal(err)
	}
	if got := project.Within(t.Context(), at); got != "docs" {
		t.Errorf("Within(%q) = %q, want docs", at, got)
	}
}
