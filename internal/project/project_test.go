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

func TestWithin(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	deep := filepath.Join(repo, "docs", "v1")
	outside := filepath.Join(tmp, "outside")
	for _, d := range []string{deep, outside} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	run(t, repo, "init", "-q")
	for dir, want := range map[string]string{repo: "", filepath.Join(repo, "docs"): "docs", deep: "docs/v1", outside: "", "": ""} {
		if got := project.Within(t.Context(), dir); got != want {
			t.Errorf("Within(%q) = %q, want %q", dir, got, want)
		}
	}
}

// No directory names no project: git is not run in the process's working directory, here this repository.
func TestNameOfNoDir(t *testing.T) {
	t.Parallel()

	if got := project.Name(t.Context(), ""); got != "" {
		t.Errorf(`Name(ctx, "") = %q, want ""`, got)
	}
}

// A file's key is its repository's name and its path from the repository's root, a worktree's file keyed as its main
// checkout's, through a symlink too; outside a repository, "" and its actual path. No directory it is published from
// takes part.
func TestPlace(t *testing.T) {
	t.Parallel()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, b, out := filepath.Join(tmp, "a"), filepath.Join(tmp, "b"), filepath.Join(tmp, "out")
	for _, f := range []string{"a/docs/plan.yaml", "a/plan.yaml", "b/docs/plan.yaml"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmp, f)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, repo := range []string{a, b} {
		run(t, repo, "init", "-q")
		run(t, repo, "add", ".")
		run(t, repo, "-c", "user.email=a@b", "-c", "user.name=a", "commit", "-qm", "i")
	}
	wt := filepath.Join(a, ".claude", "worktrees", "w")
	run(t, a, "worktree", "add", "-q", wt)
	if err := os.MkdirAll(out, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(a, "docs"), filepath.Join(out, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "notes.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ file, repo, place string }{
		{"a/docs/plan.yaml", "a", "docs/plan.yaml"},
		{"a/plan.yaml", "a", "plan.yaml"},
		{"a/.claude/worktrees/w/plan.yaml", "a", "plan.yaml"},
		{"a/.claude/worktrees/w/docs/plan.yaml", "a", "docs/plan.yaml"},
		{"b/docs/plan.yaml", "b", "docs/plan.yaml"},
		{"out/linked/plan.yaml", "a", "docs/plan.yaml"},
		{"out/notes.yaml", "", filepath.Join(out, "notes.yaml")},
	} {
		repo, place := project.Place(t.Context(), filepath.Join(tmp, tt.file))
		if repo != tt.repo || place != tt.place {
			t.Errorf("Place(%s) = %q, %q, want %q, %q", tt.file, repo, place, tt.repo, tt.place)
		}
	}
}
