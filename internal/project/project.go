// Package project names the project a page is published from.
package project

import (
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var separatorRe = regexp.MustCompile(`[/:]`)

// Name returns the project of dir: the repository name in origin's URL (https, scp-like or a path), so
// that a clone under another name agrees; else the directory of the repository, reached through the
// common .git so that a worktree joins its main checkout; else the name of dir itself. The root
// directory has no name and gives "", as does none: "", which git would read as the process's working directory.
func Name(ctx context.Context, dir string) string {
	if dir == "" {
		return ""
	}
	return named(ctx, dir, func() string {
		common, err := git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return ""
		}
		return common
	})
}

// named is Name of dir, common giving its repository's common .git ("" outside one), asked only when origin names
// none.
func named(ctx context.Context, dir string, common func() string) string {
	if url, err := git(ctx, dir, "remote", "get-url", "origin"); err == nil {
		parts := separatorRe.Split(strings.TrimRight(url, "/"), -1)
		if name := strings.TrimSuffix(parts[len(parts)-1], ".git"); name != "" {
			return name
		}
	}
	if c := common(); c != "" {
		if filepath.Base(c) == ".git" {
			c = filepath.Dir(c)
		}
		return base(c)
	}
	return base(dir)
}

// Within returns where dir is in its repository, slash-separated from the repository's root ("docs", "docs/v1"): "" at
// its root, outside one, and for none.
func Within(ctx context.Context, dir string) string {
	if dir == "" {
		return ""
	}
	prefix, err := git(ctx, dir, "rev-parse", "--show-prefix")
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(prefix, "/")
}

// Place returns the key of the data file file, which owns its stored page from whatever directory it is published:
// in a repository, the repository's name (as Name names it, so that a worktree and its main checkout agree) and the
// file's path from the root of its worktree, slash-separated; else "" and its real path. A real path names the file
// alone; a path in a repository names it with the repository.
// LIMIT: repositories are told apart by name, so two of one name (two without origin in directories of one name, or
// two forks named alike) hold one file at each path, the one published later writing over the other's page. Put
// origin's whole URL or the repository's real path in the key if that comes to matter; two clones of one origin
// sharing a page is meant.
// LIMIT: --path-format came in git 2.31; with an older git there are not two lines, and every file is keyed by its
// real path, a worktree's apart from its main checkout's. Ask for git 2.31 or later if that comes to matter.
// LIMIT: paths are compared as text, so a file named in another case on a filesystem that ignores case (darwin)
// is another file: a key is kept across machines and worktrees, where no inode can compare it.
func Place(ctx context.Context, file string) (repo, place string) {
	actual, err := filepath.EvalSymlinks(file)
	if err != nil {
		actual = file
	}
	at := filepath.Dir(actual)
	got, err := git(ctx, at, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	lines := strings.Split(got, "\n")
	if err != nil || len(lines) != 2 {
		return "", actual
	}
	rel, err := filepath.Rel(resolved(lines[0]), actual)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", actual
	}
	return named(ctx, at, func() string { return lines[1] }), filepath.ToSlash(rel)
}

// resolved returns path with its symlinks resolved, path itself when that fails.
func resolved(path string) string {
	if actual, err := filepath.EvalSymlinks(path); err == nil {
		return actual
	}
	return path
}

func base(dir string) string {
	if name := filepath.Base(dir); name != string(filepath.Separator) && name != "." {
		return name
	}
	return ""
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err //nolint:wrapcheck // callers only test for failure
	}
	return strings.TrimSpace(string(out)), nil
}
