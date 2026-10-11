package project_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/floe/internal/project"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestName(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	dirs := map[string]string{}
	for _, d := range []string{"alias", "https", "slash", "plain", "outside"} {
		dirs[d] = filepath.Join(tmp, d)
		if err := os.Mkdir(dirs[d], 0o750); err != nil {
			t.Fatal(err)
		}
	}
	origins := map[string]string{
		"alias": "git@github.com:someone/upstream-name.git",
		"https": "https://github.com/someone/over-https",
		"slash": "/srv/repos/by-path.git/",
	}
	for d, url := range origins {
		run(t, dirs[d], "init", "-q")
		run(t, dirs[d], "remote", "add", "origin", url)
	}
	run(t, dirs["plain"], "init", "-q")
	run(t, dirs["plain"], "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
	dirs["wt"] = filepath.Join(tmp, "wt")
	run(t, dirs["plain"], "worktree", "add", "-q", "--detach", dirs["wt"])
	sub := filepath.Join(dirs["plain"], "sub")
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name, dir, want string
	}{
		{"scp-like origin of a clone under another name", dirs["alias"], "upstream-name"},
		{"https origin", dirs["https"], "over-https"},
		{"path origin with trailing slash", dirs["slash"], "by-path"},
		{"no origin", dirs["plain"], "plain"},
		{"subdirectory without origin", sub, "plain"},
		{"worktree without origin", dirs["wt"], "plain"},
		{"outside git", dirs["outside"], "outside"},
		{"root", "/", ""},
	}
	for _, tt := range tests {
		if got := project.Name(t.Context(), tt.dir); got != tt.want {
			t.Errorf("%s: Name(%s) = %q, want %q", tt.name, tt.dir, got, tt.want)
		}
	}
}
