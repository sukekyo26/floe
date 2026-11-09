package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// floe publish keys a data file by its repository and its path from the repository's root, else by its real path,
// wherever the command runs, in the repository or outside it: the same file overwrites its page from another directory
// or project, and another file of the same name is refused.
func TestPublishPlace(t *testing.T) {
	t.Parallel()

	env, home := served(t)
	repo := filepath.Join(filepath.Dir(home), "r")
	for _, f := range []string{"docs/plan.yaml", "notes/plan.yaml"} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(f)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, f), []byte("lang: en\ntemplate: pic\ntitle: Plan\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := exec.CommandContext(t.Context(), "git", "init", "-q")
	git.Dir = repo
	git.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	outside, other := filepath.Join(filepath.Dir(home), "a"), filepath.Join(filepath.Dir(home), "b")
	for _, dir := range []string{outside, other} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	notes := filepath.Join(outside, "notes.yaml")
	if err := os.WriteFile(notes, []byte("lang: en\ntemplate: pic\ntitle: Notes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	url := "http://127.0.0.1:" + env["FLOE_PORT"] + "/a/plan/\n"
	notesURL := "http://127.0.0.1:" + env["FLOE_PORT"] + "/a/notes/\n"
	for _, step := range []struct {
		dir, file string
		code      int
		out, errs string
	}{
		{repo, "docs/plan.yaml", 0, url, ""},
		{filepath.Join(repo, "docs"), "plan.yaml", 0, url, ""},
		{repo, "notes/plan.yaml", 1, "", `floe: slug "plan" was published from docs/plan.yaml in repository r: choose another slug` + "\n"},
		{outside, filepath.Join(repo, "docs", "plan.yaml"), 0, url, ""},
		{outside, "notes.yaml", 0, notesURL, ""},
		{other, notes, 0, notesURL, ""},
	} {
		code, out, errs := runIn(step.dir, env, "publish", step.file)
		if code != step.code || out != step.out || (step.errs != "" && errs != step.errs) {
			t.Errorf("floe publish %s in %s = %d, %q, %q; want %d, %q, %q", step.file, step.dir, code, out, errs, step.code, step.out, step.errs)
		}
	}
}
