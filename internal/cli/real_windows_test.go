package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/cli"
	"floe/internal/testfs"
)

// Two places of one's own not there yet, one through a junction to the other's directory, overlap: their real
// paths are made from the real path of what is there (EvalSymlinks would keep the junction's name).
func TestOwnDirsJunction(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good})
	x, y := filepath.Join(dir, "x"), filepath.Join(dir, "y")
	mkdirs(t, x, y)
	testfs.Junction(t, filepath.Join(x, "j"), y)
	env := cli.HomeEnv(filepath.Join(dir, "home"), map[string]string{
		"FLOE_PARTS_DIR":  filepath.Join(x, "j", "new"),
		"FLOE_THEMES_DIR": filepath.Join(y, "new"),
	})
	code, _, stderr := runEnv(dir, env, "part", "check")
	if code != 1 || !strings.Contains(stderr, "are one directory, or one is in the other") ||
		!strings.Contains(stderr, "FLOE_PARTS_DIR") || !strings.Contains(stderr, "FLOE_THEMES_DIR") {
		t.Errorf("places overlapping through a junction = %d, %q, want them told to overlap", code, stderr)
	}
}

// A place of one's own not there yet on a subst drive and the same place by its own drive overlap (one there
// would be found by os.SameFile all the same).
func TestOwnDirsSubst(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good})
	mkdirs(t, filepath.Join(dir, "pp"))
	drive := testfs.Subst(t, dir)
	env := cli.HomeEnv(filepath.Join(dir, "home"), map[string]string{
		"FLOE_PARTS_DIR":  filepath.Join(dir, "pp", "new"),
		"FLOE_THEMES_DIR": drive + `\pp\new`,
	})
	code, _, stderr := runEnv(dir, env, "part", "check")
	if code != 1 || !strings.Contains(stderr, "are one directory, or one is in the other") {
		t.Errorf("a place and its subst drive = %d, %q, want them told to overlap", code, stderr)
	}
}

// Two pages not written yet, one through a junction to the other's directory, are one page: the real path of the
// directory they go to is made through the junction.
func TestRenderSameThroughJunction(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"y/p.yaml": good, "y/p.yml": good})
	testfs.Junction(t, filepath.Join(dir, "j"), filepath.Join(dir, "y"))
	code, _, stderr := run(dir, "render", filepath.Join("y", "p.yaml"), filepath.Join("j", "p.yml"))
	if code != 1 || !strings.Contains(stderr, "would both write") {
		t.Errorf("render of one page through a junction = %d, %q, want cli_render_same_file", code, stderr)
	}
}

// The skill's lines name its file as people write it, without \\?\.
func TestSkillInstallShown(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	env := cli.HomeVars(home)
	for range 2 {
		code, out, errs := runEnv(home, env, "skill", "install", "claude")
		if code != 0 || errs != "" || strings.Contains(out, `\\?\`) || !strings.Contains(out, "SKILL.md") {
			t.Errorf("floe skill install = %d, %q, %q, want SKILL.md named without \\\\?\\", code, out, errs)
		}
	}
}

// A skill's directory there in another case (skills\Floe) is floe's own on Windows, which names files in any case:
// the skill is written in it.
func TestSkillInstallCase(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills", "Floe"), 0o700); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runEnv(home, cli.HomeVars(home), "skill", "install", "claude")
	if code != 0 || errs != "" || !strings.Contains(out, "SKILL.md") {
		t.Errorf("floe skill install over skills\\Floe = %d, %q, %q, want it written", code, out, errs)
	}
}

// A skill's directory that is a junction to a directory of another name is a link as a symlink is: refused.
func TestSkillInstallJunction(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	skills := filepath.Join(home, ".claude", "skills")
	other := filepath.Join(home, "other")
	for _, d := range []string{skills, other} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	testfs.Junction(t, filepath.Join(skills, "floe"), other)
	code, _, errs := runEnv(home, cli.HomeVars(home), "skill", "install", "claude")
	if code != 1 || !strings.Contains(errs, "this is a link to") || !strings.Contains(errs, "not named floe") {
		t.Errorf("floe skill install over a junction to other = %d, %q, want cli_skill_link", code, errs)
	}
	if _, err := os.Stat(filepath.Join(other, "SKILL.md")); err == nil {
		t.Error("the skill was written where the junction leads")
	}
}

// A path relative to a drive's own working directory is refused, telling how to write it.
func TestDriveRelative(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"b.yaml": good})
	code, _, stderr := run(dir, "check", "C:b.yaml")
	if code != 1 || !strings.Contains(stderr, "C:b.yaml is relative to the working directory of drive C:") ||
		!strings.Contains(stderr, "write the whole path") {
		t.Errorf("floe check C:b.yaml = %d, %q, want path_drive_relative", code, stderr)
	}
}
