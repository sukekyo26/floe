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

// A place of one's own set on a share, on a network drive, or as a link to a share, is refused for its setting
// (path_not_local), and compared with no other place: no overlap is told of it.
func TestOwnPlaceNotLocal(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good, "parts/.keep": ""})
	shared, mapped := testfs.MapShare(t, dir)
	linked := filepath.Join(t.TempDir(), "linked")
	link(t, filepath.Join(shared, "parts"), linked)
	for _, at := range []string{filepath.Join(shared, "parts"), filepath.Join(mapped, "parts"), linked} {
		env := map[string]string{"FLOE_PARTS_DIR": at, "FLOE_THEMES_DIR": filepath.Join(dir, "parts")}
		for _, args := range [][]string{{"part", "check"}, {"check", "d.yaml"}} {
			code, _, stderr := runEnv(dir, env, args...)
			if code != 1 || !strings.Contains(stderr, "FLOE_PARTS_DIR") || !strings.Contains(stderr, "not on a local drive") ||
				strings.Contains(stderr, "one is in the other") {
				t.Errorf("FLOE_PARTS_DIR=%s floe %v = %d, %q; want path_not_local alone", at, args, code, stderr)
			}
		}
	}
}

// A device, a share or a link to one is refused before it is opened (path_not_local), where Unix tells a device by its
// kind: a part's file that is a link to a device, a data file on a share, a page to write that is a link to a
// device, and pages to write on a share or a network drive (told to put them on a local drive, not to look at
// their directory).
func TestNotLocalWindows(t *testing.T) {
	t.Parallel()

	parts := write(t, map[string]string{"mine/schema.json": ownSchema, "mine/part.html": ownPage})
	css := filepath.Join(parts, "mine", "part.css")
	link(t, `\\.\NUL`, css)
	code, _, stderr := runEnv(t.TempDir(), map[string]string{"FLOE_PARTS_DIR": parts}, "part", "check")
	if code != 1 || !strings.Contains(stderr, css+": this is not on a local drive under an ordinary name") {
		t.Errorf("floe part check of a link to NUL = %d, %q, want path_not_local", code, stderr)
	}

	dir := write(t, map[string]string{"a.yaml": good, "e.yaml": good})
	shared, mapped := testfs.MapShare(t, dir)
	on := filepath.Join(shared, "a.yaml")
	if code, _, stderr := run(dir, "check", on); code != 1 || !strings.Contains(stderr, "this is not on a local drive") {
		t.Errorf("floe check %s = %d, %q, want path_not_local", on, code, stderr)
	}

	const lookAtDir = "check that its directory exists"
	link(t, `\\.\NUL`, filepath.Join(dir, "e.html"))
	if code, _, stderr := run(dir, "render", "e.yaml"); code != 1 || !strings.Contains(stderr, "not on a local drive") ||
		!strings.Contains(stderr, "e.html") || strings.Contains(stderr, lookAtDir) {
		t.Errorf("floe render of a page linked to NUL = %d, %q, want it not written, told", code, stderr)
	}

	// MkdirAll refuses the share first: nothing is made under dir.
	for _, to := range []string{shared, mapped} {
		code, _, stderr := run(dir, "render", "--out-dir", filepath.Join(to, "pages"), "a.yaml")
		if code != 1 || !strings.Contains(stderr, "not on a local drive") || !strings.Contains(stderr, "put it there") ||
			strings.Contains(stderr, lookAtDir) {
			t.Errorf("floe render --out-dir under %s = %d, %q, want it told as not local", to, code, stderr)
		}
	}
}
