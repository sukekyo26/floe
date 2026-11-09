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
	if url, err := git(ctx, dir, "remote", "get-url", "origin"); err == nil {
		parts := separatorRe.Split(strings.TrimRight(url, "/"), -1)
		if name := strings.TrimSuffix(parts[len(parts)-1], ".git"); name != "" {
			return name
		}
	}
	if common, err := git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		if filepath.Base(common) == ".git" {
			common = filepath.Dir(common)
		}
		return base(common)
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

// Place returns where the file file is, which owns a page published from dir's project with it: its path from its
// repository's root when that repository is dir's (one common .git, any worktree of it), else its real path. A
// root is taken on the file's side, so that a file of a worktree under the main checkout is placed alike from both.
// LIMIT: --path-format came in git 2.31; with an older git there are not two lines, and every file is placed by its
// real path, a worktree's apart from its main checkout's. Ask for git 2.31 or later if that comes to matter.
// LIMIT: places are compared as text, so a file named in another case on a filesystem that ignores case (darwin)
// is another place: a place is kept across machines and worktrees, where no inode can compare it.
func Place(ctx context.Context, dir, file string) string {
	actual, err := filepath.EvalSymlinks(file)
	if err != nil {
		actual = file
	}
	if dir == "" {
		return actual
	}
	at := filepath.Dir(actual)
	got, err := git(ctx, at, "rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir")
	lines := strings.Split(got, "\n")
	if err != nil || len(lines) != 2 {
		return actual
	}
	mine, err := git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || resolved(mine) != resolved(lines[1]) {
		return actual
	}
	rel, err := filepath.Rel(resolved(lines[0]), actual)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return actual
	}
	return filepath.ToSlash(rel)
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
