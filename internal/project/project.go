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
// directory has no name and gives "".
func Name(ctx context.Context, dir string) string {
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
