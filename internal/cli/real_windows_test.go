package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

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
	env := map[string]string{
		"HOME":            filepath.Join(dir, "home"),
		"FLOE_PARTS_DIR":  filepath.Join(x, "j", "new"),
		"FLOE_THEMES_DIR": filepath.Join(y, "new"),
	}
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
	env := map[string]string{
		"HOME":            filepath.Join(dir, "home"),
		"FLOE_PARTS_DIR":  filepath.Join(dir, "pp", "new"),
		"FLOE_THEMES_DIR": drive + `\pp\new`,
	}
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
	env := map[string]string{"HOME": home}
	for range 2 {
		code, out, errs := runEnv(home, env, "skill", "install", "claude")
		if code != 0 || errs != "" || strings.Contains(out, `\\?\`) || !strings.Contains(out, "SKILL.md") {
			t.Errorf("floe skill install = %d, %q, %q, want SKILL.md named without \\\\?\\", code, out, errs)
		}
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
