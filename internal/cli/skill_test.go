package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// skillBody is the start of floe's skill, which every write puts.
const skillBody = "---\nname: floe\n"

func evalTemp(t *testing.T) string {
	t.Helper()
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return tmp
}

func mustDo(t *testing.T, errs ...error) {
	t.Helper()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// floe skill install writes floe's skill where the agent named reads skills: made when none is there, left when
// this floe's is, kept but for --force when another is; links are followed to a directory named floe and a file
// named SKILL.md alone; only the directory a setting names is told for the setting.
func TestSkillInstall(t *testing.T) {
	t.Parallel()

	type check struct {
		code int
		has  []string // parts the output (stdout, then stderr) holds
		not  []string // parts it does not
	}
	for _, c := range []struct {
		name  string
		agent string
		force bool
		setup func(t *testing.T, home string) map[string]string // the environment past HOME
		want  check
		after func(t *testing.T, home string)
	}{
		{name: "none", agent: "claude", want: check{0, []string{"made ", ".claude/skills/floe/SKILL.md"}, nil},
			after: func(t *testing.T, home string) {
				t.Helper()
				if !strings.HasPrefix(readFile(t, home+"/.claude/skills/floe/SKILL.md"), skillBody) {
					t.Error("not floe's skill")
				}
			}},
		{name: "none codex", agent: "codex", want: check{0, []string{"made ", ".agents/skills/floe/SKILL.md"}, nil}},
		{name: "same", agent: "claude", setup: installed(".claude"),
			want: check{0, []string{"is this floe's skill already"}, nil}},
		{name: "same force", agent: "gemini", force: true, setup: installed(".agents"),
			want: check{0, []string{"is this floe's skill already"}, nil}},
		{name: "other", agent: "claude", setup: written(".claude/skills/floe/SKILL.md", "mine\n"),
			want:  check{1, []string{".claude/skills/floe/SKILL.md: ", "--force"}, nil},
			after: content(".claude/skills/floe/SKILL.md", "mine\n")},
		{name: "other force", agent: "codex", force: true, setup: written(".agents/skills/floe/SKILL.md", "mine\n"),
			want: check{0, []string{"replaced ", ".agents/skills/floe/SKILL.md"}, nil}},
		{name: "big", agent: "claude", setup: written(".claude/skills/floe/SKILL.md", strings.Repeat("x", 1<<20)),
			want: check{1, []string{"--force"}, nil}},
		{name: "big force", agent: "claude", force: true, setup: written(".claude/skills/floe/SKILL.md", strings.Repeat("x", 1<<20)),
			want: check{0, []string{"replaced "}, nil}},
		{name: "other files kept", agent: "claude", setup: written(".claude/skills/floe/notes.txt", "n\n"),
			want: check{0, []string{"made "}, nil}, after: content(".claude/skills/floe/notes.txt", "n\n")},
		{name: "floe links to a floe", agent: "claude", setup: linked("dot/floe", true, ".claude/skills/floe"),
			want: check{0, []string{"made ", "/dot/floe/SKILL.md"}, nil}, after: isLink(".claude/skills/floe")},
		{name: "floe links to another name", agent: "claude", setup: linked("dot/floe-skill", true, ".claude/skills/floe"),
			want: check{1, []string{".claude/skills/floe: ", "dot/floe-skill", "not named floe"}, nil}},
		{name: "floe links nowhere", agent: "codex", setup: linked("gone", false, ".agents/skills/floe"),
			want: check{1, []string{".agents/skills/floe: cannot be written"}, nil}},
		{name: "floe links to another floe", agent: "claude", setup: linked("work/floe", true, ".claude/skills/floe"),
			want: check{0, []string{"made ", "/work/floe/SKILL.md"}, nil}},
		{name: "SKILL.md links outside floe", agent: "claude", force: true, setup: linkedFile("dot/x/SKILL.md", ".claude/skills/floe/SKILL.md"),
			want: check{0, []string{"replaced ", "/dot/x/SKILL.md"}, nil}, after: isLink(".claude/skills/floe/SKILL.md")},
		{name: "SKILL.md links to .bashrc", agent: "claude", force: true, setup: linkedFile(".bashrc", ".claude/skills/floe/SKILL.md"),
			want: check{1, []string{"floe/SKILL.md: ", "/.bashrc", "not named SKILL.md"}, nil}, after: content(".bashrc", "mine\n")},
		{name: "SKILL.md links to another skill", agent: "claude", setup: linkedFile("other/SKILL.md", ".claude/skills/floe/SKILL.md"),
			want: check{1, []string{"/other/SKILL.md: ", "--force"}, nil}},
		{name: "SKILL.md links nowhere", agent: "claude", setup: linked("gone/SKILL.md", false, ".claude/skills/floe/SKILL.md"),
			want: check{1, []string{"floe/SKILL.md: cannot be written"}, nil}},
		{name: "SKILL.md is a directory", agent: "claude", setup: dirAt(".claude/skills/floe/SKILL.md"),
			want: check{1, []string{"floe/SKILL.md: is a directory"}, nil}},
		{name: "SKILL.md is a FIFO", agent: "claude", setup: fifoAt(".claude/skills/floe/SKILL.md"),
			want: check{1, []string{"floe/SKILL.md: ", "FIFO"}, nil}},
		{name: "floe is a file", agent: "claude", setup: written(".claude/skills/floe", "x"),
			want: check{1, []string{"floe/SKILL.md: not a directory"}, nil}},
		{name: ".claude is a file", agent: "claude", setup: written(".claude", "x"),
			want: check{1, []string{"/.claude: cannot be written (not a directory)"}, nil}},
		{name: ".agents is a file", agent: "codex", setup: written(".agents", "x"),
			want: check{1, []string{"/.agents: cannot be written (not a directory)"}, nil}},
		{name: "skills is a file", agent: "cursor", setup: written(".agents/skills", "x"),
			want: check{1, []string{"/.agents/skills: cannot be written (not a directory)"}, nil}},
		{name: ".agents links nowhere", agent: "codex", setup: linked("gone", false, ".agents"),
			want: check{1, []string{"/.agents: cannot be written (file exists)"}, nil}},
		{name: ".agents links to itself", agent: "copilot", setup: linked(".agents", false, ".agents"),
			want: check{1, []string{"/.agents: cannot be written"}, nil}},
		{name: "skills links nowhere", agent: "claude", setup: linked("gone", false, ".claude/skills"),
			want: check{1, []string{"/.claude/skills: cannot be written (file exists)"}, nil}},
		{name: ".claude links, no skills yet", agent: "claude", setup: linked("dot/claude", true, ".claude"),
			want: check{0, []string{"made ", "/dot/claude/skills/floe/SKILL.md"}, nil}},
		{name: "CLAUDE_CONFIG_DIR is a file", agent: "claude", setup: config("cfg", "file"),
			want: check{1, []string{"floe: CLAUDE_CONFIG_DIR is ", "not a directory"}, nil}},
		{name: "skills under CLAUDE_CONFIG_DIR is a file", agent: "claude", setup: config("cfg", "skills file"),
			want: check{1, []string{"/cfg/skills: cannot be written (not a directory)"}, []string{"CLAUDE_CONFIG_DIR is"}}},
		{name: "skills under CLAUDE_CONFIG_DIR links nowhere", agent: "claude", setup: config("cfg", "skills gone"),
			want: check{1, []string{"/cfg/skills: cannot be written (file exists)"}, []string{"CLAUDE_CONFIG_DIR is"}}},
		{name: "SKILL.md under CLAUDE_CONFIG_DIR is a FIFO", agent: "claude", setup: config("cfg", "fifo"),
			want: check{1, []string{"/cfg/skills/floe/SKILL.md: ", "FIFO"}, []string{"CLAUDE_CONFIG_DIR is"}}},
		{name: "CLAUDE_CONFIG_DIR", agent: "claude", setup: config("cfg", ""),
			want: check{0, []string{"made ", "/cfg/skills/floe/SKILL.md"}, nil}},
		{name: "CLAUDE_CONFIG_DIR relative", agent: "claude",
			setup: func(*testing.T, string) map[string]string { return map[string]string{"CLAUDE_CONFIG_DIR": "x"} },
			want:  check{1, []string{"CLAUDE_CONFIG_DIR is x, a relative path"}, nil}},
		{name: "no HOME", agent: "claude",
			setup: func(*testing.T, string) map[string]string { return map[string]string{"HOME": ""} },
			want:  check{1, []string{"set HOME to an absolute path"}, []string{"CLAUDE_CONFIG_DIR"}}},
		{name: "no HOME codex", agent: "codex",
			setup: func(*testing.T, string) map[string]string { return map[string]string{"HOME": ""} },
			want:  check{1, []string{"set HOME to an absolute path"}, nil}},
		{name: "HOME is a file", agent: "claude",
			setup: func(*testing.T, string) map[string]string { return map[string]string{"HOME": "/dev/null"} },
			want:  check{1, []string{"/dev/null/.claude: cannot be written (not a directory)"}, nil}},
		{name: "HOME is a file codex", agent: "codex",
			setup: func(*testing.T, string) map[string]string { return map[string]string{"HOME": "/dev/null"} },
			want:  check{1, []string{"/dev/null/.agents: cannot be written (not a directory)"}, nil}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			home := evalTemp(t)
			env := map[string]string{"HOME": home}
			if c.setup != nil {
				for k, v := range c.setup(t, home) {
					env[k] = v
				}
			}
			args := []string{"skill", "install", c.agent}
			if c.force {
				args = append(args, "--force")
			}
			code, out, errs := runEnv(home, env, args...)
			said := out + errs
			if code != c.want.code {
				t.Errorf("exit %d, want %d: %q", code, c.want.code, said)
			}
			for _, part := range c.want.has {
				if !strings.Contains(said, part) {
					t.Errorf("%q lacks %q", said, part)
				}
			}
			for _, part := range c.want.not {
				if strings.Contains(said, part) {
					t.Errorf("%q holds %q", said, part)
				}
			}
			if c.after != nil {
				c.after(t, home)
			}
		})
	}
}

func installed(base string) func(t *testing.T, home string) map[string]string {
	return func(t *testing.T, home string) map[string]string {
		t.Helper()
		code, _, errs := runEnv(home, map[string]string{"HOME": home}, "skill", "install", map[string]string{".claude": "claude", ".agents": "codex"}[base])
		if code != 0 {
			t.Fatal(errs)
		}
		return nil
	}
}

func written(rel, text string) func(t *testing.T, home string) map[string]string {
	return func(t *testing.T, home string) map[string]string {
		t.Helper()
		path := filepath.Join(home, rel)
		mustDo(t, os.MkdirAll(filepath.Dir(path), 0o700), os.WriteFile(path, []byte(text), 0o600))
		return nil
	}
}

func content(rel, want string) func(t *testing.T, home string) {
	return func(t *testing.T, home string) {
		t.Helper()
		if got := readFile(t, filepath.Join(home, rel)); got != want {
			t.Errorf("%s is %q, want %q", rel, got, want)
		}
	}
}

func isLink(rel string) func(t *testing.T, home string) {
	return func(t *testing.T, home string) {
		t.Helper()
		if info, err := os.Lstat(filepath.Join(home, rel)); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is no longer a link: %v", rel, err)
		}
	}
}

// linked makes a link at rel to to, made a directory first when made.
func linked(to string, made bool, rel string) func(t *testing.T, home string) map[string]string {
	return func(t *testing.T, home string) map[string]string {
		t.Helper()
		at := filepath.Join(home, rel)
		mustDo(t, os.MkdirAll(filepath.Dir(at), 0o700))
		if made {
			mustDo(t, os.MkdirAll(filepath.Join(home, to), 0o700))
		}
		mustDo(t, os.Symlink(filepath.Join(home, to), at))
		return nil
	}
}

// linkedFile makes a link at rel to the file to, which holds "mine".
func linkedFile(to, rel string) func(t *testing.T, home string) map[string]string {
	return func(t *testing.T, home string) map[string]string {
		t.Helper()
		written(to, "mine\n")(t, home)
		return linked(to, false, rel)(t, home)
	}
}

func dirAt(rel string) func(t *testing.T, home string) map[string]string {
	return func(t *testing.T, home string) map[string]string {
		t.Helper()
		mustDo(t, os.MkdirAll(filepath.Join(home, rel), 0o700))
		return nil
	}
}

func fifoAt(rel string) func(t *testing.T, home string) map[string]string {
	return func(t *testing.T, home string) map[string]string {
		t.Helper()
		at := filepath.Join(home, rel)
		mustDo(t, os.MkdirAll(filepath.Dir(at), 0o700), syscall.Mkfifo(at, 0o600))
		return nil
	}
}

// config sets CLAUDE_CONFIG_DIR to rel under home, which is: a file, holds a file skills or a link skills to
// nothing, or a FIFO SKILL.md, or is not there ("").
func config(rel, is string) func(t *testing.T, home string) map[string]string {
	return func(t *testing.T, home string) map[string]string {
		t.Helper()
		switch is {
		case "file":
			written(rel, "x")(t, home)
		case "skills file":
			written(rel+"/skills", "x")(t, home)
		case "skills gone":
			linked("gone", false, rel+"/skills")(t, home)
		case "fifo":
			fifoAt(rel+"/skills/floe/SKILL.md")(t, home)
		}
		return map[string]string{"CLAUDE_CONFIG_DIR": filepath.Join(home, rel)}
	}
}

// What cannot be read or written is told at its path, not as the setting's; the same skill is not written again.
func TestSkillInstallPermissions(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads and writes whatever its permissions")
	}

	home := evalTemp(t)
	env := map[string]string{"HOME": home}
	skills := filepath.Join(home, ".claude", "skills")
	mustDo(t, os.MkdirAll(skills, 0o700), os.Chmod(skills, 0o500))
	t.Cleanup(func() { _ = os.Chmod(skills, 0o700) })
	if code, out, errs := runEnv(home, env, "skill", "install", "claude"); code != 1 ||
		!strings.Contains(errs, "/.claude/skills/floe: cannot be written (permission denied)") {
		t.Errorf("unwritable skills: %d %q %q", code, out, errs)
	}

	cfg := filepath.Join(home, "ro", "cfg")
	mustDo(t, os.MkdirAll(filepath.Dir(cfg), 0o500))
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(cfg), 0o700) })
	env["CLAUDE_CONFIG_DIR"] = cfg
	if code, out, errs := runEnv(home, env, "skill", "install", "claude"); code != 1 ||
		!strings.Contains(errs, "floe: CLAUDE_CONFIG_DIR is "+cfg) || !strings.Contains(errs, "permission denied") {
		t.Errorf("CLAUDE_CONFIG_DIR not to be made: %d %q %q", code, out, errs)
	}
	if _, err := os.Stat(cfg); err == nil {
		t.Error("left a directory")
	}

	home = evalTemp(t)
	env = map[string]string{"HOME": home}
	skill := filepath.Join(home, ".claude", "skills", "floe", "SKILL.md")
	mustDo(t, os.MkdirAll(filepath.Dir(skill), 0o700), os.WriteFile(skill, []byte("x"), 0o200))
	if code, out, errs := runEnv(home, env, "skill", "install", "claude"); code != 1 ||
		!strings.Contains(errs, "floe/SKILL.md: permission denied") {
		t.Errorf("unreadable SKILL.md: %d %q %q", code, out, errs)
	}

	home = evalTemp(t)
	env = map[string]string{"HOME": home}
	if code, _, errs := runEnv(home, env, "skill", "install", "claude"); code != 0 {
		t.Fatal(errs)
	}
	skill = filepath.Join(home, ".claude", "skills", "floe", "SKILL.md")
	old := time.Unix(1_000_000_000, 0)
	mustDo(t, os.Chtimes(skill, old, old))
	if code, _, errs := runEnv(home, env, "skill", "install", "claude", "--force"); code != 0 {
		t.Fatal(errs)
	}
	if info, err := os.Stat(skill); err != nil || !info.ModTime().Equal(old) {
		t.Errorf("the same skill was written again: %v", err)
	}
}

// A name that is none, two or unknown is a usage error that lists the agents and their places.
func TestSkillInstallNames(t *testing.T) {
	t.Parallel()

	home := evalTemp(t)
	for _, args := range [][]string{{}, {"claude", "codex"}, {"claud"}} {
		code, out, errs := runEnv(home, map[string]string{"HOME": home}, append([]string{"skill", "install"}, args...)...)
		if code != 2 || out != "" {
			t.Errorf("%v: %d %q", args, code, out)
		}
		for _, part := range []string{"claude", "~/.claude/skills", "codex", "copilot", "cursor", "gemini", "~/.agents/skills"} {
			if !strings.Contains(errs, part) {
				t.Errorf("%v: %q lacks %q", args, errs, part)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); err == nil {
		t.Error("a name refused made a directory")
	}
}
