package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"floe/docs"
	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/store"
	"floe/internal/usererr"
)

// skill is the SKILL.md floe skill install writes: it points the agent to floe guide, so that it stays right
// when floe is upgraded.
const skill = "---\nname: floe\ndescription: " +
	"Show the user a page with floe, the local page server - a report, design document or similar page, " +
	"with tables, code and diagrams, drawn from a YAML data file. " +
	"Use when the user asks to show, publish or write up something as a page, or names floe.\n---\n\n" +
	skillLine + "\n"

// skillLine is the body of the skill, the line README gives for AGENTS.md.
const skillLine = "To show the user a page, use floe: run `floe guide` first and follow it. " +
	"It is the guide of the floe installed, so read it each time rather than from memory."

// agent is an agent floe skill install writes the skill for: its name, the setting that moves its directory
// (CLAUDE_CONFIG_DIR, "" for none), the directory under HOME skills go under, and how --help names the place.
type agent struct {
	name, key, home, place string
}

// LIMIT: only claude is run in trials; the others write where their docs say (~/.agents/skills), unchecked by
// running them.
var agents = []agent{
	{
		name: "claude", key: "CLAUDE_CONFIG_DIR", home: ".claude",
		place: "$CLAUDE_CONFIG_DIR/skills if it is set, else ~/.claude/skills",
	},
	{name: "codex", key: "", home: ".agents", place: "~/.agents/skills"},
	{name: "copilot", key: "", home: ".agents", place: "~/.agents/skills"},
	{name: "cursor", key: "", home: ".agents", place: "~/.agents/skills"},
	{name: "gemini", key: "", home: ".agents", place: "~/.agents/skills"},
}

// agentsNamed is the agents and their places, as the mistake of a name tells them: each place after the names
// that share it.
func agentsNamed() string {
	var groups []string
	for i := 0; i < len(agents); {
		j := i
		for j < len(agents) && agents[j].place == agents[i].place {
			j++
		}
		names := make([]string, 0, j-i)
		for _, a := range agents[i:j] {
			names = append(names, a.name)
		}
		group := names[0]
		if len(names) > 1 {
			group = strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
		}
		groups = append(groups, fmt.Sprintf("%s (%s)", group, agents[i].place))
		i = j
	}
	return strings.Join(groups, ", or ")
}

// guideCmd is floe guide: guide.md, without its Tasks section's Ask after publishing unless the task mode is on in the
// management page's settings. With no store to be had (no HOME to put it under), it is off, so that an agent reads the
// guide anyway.
func (a *app) guideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: "Print the guide for agents: how to write, check and publish a page",
		Long: `Print the guide for agents: how to write, check and publish a page. Its Tasks section points to floe task
guide; the part of it that has the agent ask whether to put each new page on a task is printed only when the task mode
is on in the management page's settings, read from the store the other commands send to (FLOE_DIR) without starting
the server.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			set, err := a.settings()
			if e, ok := errors.AsType[*usererr.Error](err); ok && e.Key == "store_no_dir" {
				set, err = store.Settings{Theme: "", TaskMode: false}, nil
			}
			if err != nil {
				return err
			}
			guide := docs.Guide
			if !set.TaskMode {
				if guide, err = kit.WithoutSection(docs.Guide, "Ask after publishing"); err != nil {
					return err //nolint:wrapcheck // floe's failure, which says what failed
				}
			}
			a.write(guide)
			return nil
		},
	}
}

// settings reads settings.json of the store the other commands send to, without starting the server.
func (a *app) settings() (store.Settings, error) {
	root, err := a.storeDir()
	if err != nil {
		return store.Settings{}, err
	}
	// The store a server for these commands holds: daemon refuses one of another (daemon_other_root).
	return store.New(root.dir).ReadSettings() //nolint:wrapcheck // mistakes for the user as they are
}

// docGuideCmd is a command guide that prints doc, a doc of floe's as it is.
func (a *app) docGuideCmd(short string, doc []byte) *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			a.write(doc)
			return nil
		},
	}
}

func (a *app) skillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install a skill that points your agent to floe guide",
		Args:  cobra.NoArgs,
	}
	var force bool
	var table strings.Builder
	for _, ag := range agents {
		fmt.Fprintf(&table, "  %-8s %s\n", ag.name, ag.place)
	}
	install := &cobra.Command{
		Use:   "install <agent>",
		Short: "Write floe's skill where the agent named reads its skills",
		Long: `Write floe's skill, floe/SKILL.md, where the agent named reads the skills of all your projects:

` + table.String() + `
Cursor reads ~/.claude/skills too. If you use Cursor, it reads both places, and shows the skill twice when it
is in both; that does no harm. A skill under ~/.agents/skills is not copied to Cursor's Cloud Agents or remote
SSH, which read ~/.cursor/skills alone.

The skill only tells the agent to run floe guide, so it stays right when floe is upgraded. A SKILL.md there
already is kept unless it is this floe's (said so) or --force is given; links to a directory named floe and a
file named SKILL.md are followed, and the path written to is printed. To remove the skill, remove the floe
directory.`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 || !slices.ContainsFunc(agents, func(ag agent) bool { return ag.name == args[0] }) {
				return usageOf(usererr.New("cli_skill_agent", agentsNamed()))
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			i := slices.IndexFunc(agents, func(ag agent) bool { return ag.name == args[0] })
			return a.installSkill(agents[i], force)
		},
	}
	install.Flags().BoolVar(&force, "force", false, "write this floe's skill over a SKILL.md that is not")
	cmd.AddCommand(install)
	return cmd
}

// installSkill writes the skill for ag: made when none is there, nothing when this floe's is, and over another
// only with force.
func (a *app) installSkill(ag agent, force bool) error {
	p, dir, err := a.skillDir(ag)
	if err != nil {
		return err
	}
	floeAt := filepath.Join(dir, "floe")
	realFloe, err := followNamed(floeAt, "floe")
	if err != nil {
		return err
	}
	skillAt := filepath.Join(realFloe, "SKILL.md")
	realSkill, err := followNamed(skillAt, "SKILL.md")
	if err != nil {
		return err
	}
	there, same, info, err := readSkill(realSkill)
	switch {
	case err != nil:
		return usererr.CannotRead(skillAt, err)
	case there && same:
		a.print("%s", english.Msg("cli_skill_same", realSkill))
		return nil
	case there && !force:
		return usererr.NewAt(realSkill, 0, 0, "cli_skill_kept")
	}
	if err = os.MkdirAll(p.dir, 0o755); err != nil { //nolint:gosec // G301: the agent's own directory, as it makes it
		return usererr.SettingDir(p.setting(), p.dir, err, usererr.CannotWrite(p.dir, err))
	}
	for _, d := range []string{dir, realFloe} {
		if err = os.MkdirAll(d, 0o755); err != nil { //nolint:gosec // G301: as above
			return usererr.CannotWrite(d, err)
		}
	}
	if err = files.WriteAtomic(realSkill, []byte(skill), 0o644, info); err != nil {
		return usererr.CannotWrite(skillAt, err)
	}
	// Followed again: a directory on the way made just now may be a link's, which realPath could not follow
	// before it was there.
	if written, werr := filepath.EvalSymlinks(skillAt); werr == nil {
		realSkill = written
	}
	key := "cli_made"
	if there {
		key = "cli_skill_replaced"
	}
	a.print("%s", english.Msg(key, realSkill))
	return nil
}

// skillDir is the place of ag (p) and the directory of skills in it, which neither is made yet. Only the
// directory a setting names is told for the setting; what is under it is told at its own path.
func (a *app) skillDir(ag agent) (dataPlace, string, error) {
	p, err := a.dataDir(ag.key, "", ag.home, "")
	switch {
	case err != nil:
		return p, "", err
	case p.unusable != nil:
		return p, "", p.unusable
	case p.dir == "":
		return p, "", usererr.New("cli_new_no_dir", "HOME")
	}
	dir := filepath.Join(p.dir, "skills")
	if err = statDir(p.dir); err != nil {
		return p, "", usererr.SettingDir(p.setting(), p.dir, err, usererr.CannotWrite(p.dir, err))
	}
	if err = statDir(dir); err != nil {
		return p, "", usererr.CannotWrite(dir, err)
	}
	return p, dir, nil
}

// followNamed is at with its links followed, by the rule render follows: a link leads only to one of the name
// at has (a directory floe, a file SKILL.md), or nothing is written. A link to the same name elsewhere is
// followed. LIMIT: a link named floe to another floe (~/work/floe) is written in, but a SKILL.md there is kept
// without --force, and the path printed says where.
func followNamed(at, name string) (string, error) {
	followed, err := realPath(at, at)
	if err != nil {
		return "", err
	}
	if filepath.Base(followed) != name {
		return "", usererr.NewAt(at, 0, 0, "cli_skill_link", followed, name)
	}
	return followed, nil
}

// statDir is why path cannot be a directory to write in: nil when it is one or is not there (it is made then),
// else the system's error, a file there as making a directory over it would say.
func statDir(path string) error {
	info, err := os.Stat(path)
	switch {
	case err == nil && !info.IsDir():
		return &fs.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return err //nolint:wrapcheck // the system's, told by the caller
	}
	return nil
}

// readSkill reads the SKILL.md at path: whether one is there, whether it is this floe's, and its information,
// for the one written over it to keep its permissions.
func readSkill(path string) (there, same bool, info fs.FileInfo, err error) {
	f, info, err := files.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, false, nil, nil
	}
	if err != nil {
		return false, false, nil, err //nolint:wrapcheck // told by the caller
	}
	defer f.Close() //nolint:errcheck // read only
	same, err = sameSkill(f)
	if err != nil {
		return false, false, nil, err
	}
	return true, same, info, nil
}

// sameSkill reports whether r holds this floe's skill, reading at most one byte past it: a SKILL.md of any size,
// a link's too, is read no further.
func sameSkill(r io.Reader) (bool, error) {
	got, err := io.ReadAll(io.LimitReader(r, int64(len(skill))+1))
	if err != nil {
		return false, err //nolint:wrapcheck // told by the caller
	}
	return bytes.Equal(got, []byte(skill)), nil
}
