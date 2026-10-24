package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/usererr"
)

// setting is one of floe's settings a command may use.
type setting string

const (
	portSetting      setting = "port"
	storeSetting     setting = "store"
	iconsSetting     setting = "icons"
	templatesSetting setting = "templates"
	partsSetting     setting = "parts"
	themesSetting    setting = "themes"
)

// settings are floe's settings, in the order their mistakes are told.
var settings = []setting{portSetting, storeSetting, iconsSetting, templatesSetting, partsSetting, themesSetting}

// use is how a command uses a setting: it reads what is there, or it needs it, to write there or as it
// cannot do without it.
type use string

const (
	reads use = "read"
	needs use = "need"
)

// serverUses is how the server uses the settings, and the commands that may start it: alike, so that a server
// they start is not handed none where they read a setting written wrong.
var serverUses = map[setting]use{
	portSetting: needs, storeSetting: needs, iconsSetting: reads, templatesSetting: reads, partsSetting: reads,
	themesSetting: reads,
}

// drawUses is how the commands that draw a page as floe publish does use the settings.
var drawUses = map[setting]use{iconsSetting: reads, templatesSetting: reads, partsSetting: reads, themesSetting: reads}

// declared marks a command that declares the settings it uses, those it does not included.
const declared = "settings"

// uses declares, as the Annotations of a command, how it uses each setting it uses; one it does not name it
// does not use, and is told nothing of.
// LIMIT: declared by hand: a setting a command reads but does not declare is none to it, unchecked; make the
// settings tell their mistakes where read if a declaration comes to be missed.
func uses(each map[setting]use) map[string]string {
	marks := map[string]string{declared: ""}
	for s, u := range each {
		marks[string(s)] = string(u)
	}
	return marks
}

// settle stops cmd, telling every reason at once, on a setting it uses that is written wrong, and on one it
// needs that has no default; one it only reads that has no default it goes on without, its value none, and
// is warned of when a file is in its way, once for each such file. It is the one place that tells the
// mistakes of the settings.
// LIMIT: a command needs one directory at most, so a default a file holds up is told once; tell those one file
// holds up in one line if a command comes to need two (TestEveryCommandDeclaresItsSettings guards it).
func (a *app) settle(cmd *cobra.Command) error {
	var found usererr.List
	var heldBy []string // the files in the way of defaults cmd reads, in the order first met
	held := map[string][]string{}
	for _, s := range settings {
		u := use(cmd.Annotations[string(s)])
		switch at := a.cfg.heldBy[s]; {
		case u == "":
		case a.cfg.wrong[s] != nil:
			found = append(found, a.cfg.wrong[s])
		case u == needs && a.cfg.absent[s] != nil:
			found = append(found, a.cfg.absent[s])
		case at != "":
			if held[at] == nil {
				heldBy = append(heldBy, at)
			}
			held[at] = append(held[at], dirKeys[s])
		}
	}
	if err := found.Err(); err != nil {
		return err
	}
	var warned usererr.List
	for _, at := range heldBy {
		warned = append(warned, usererr.New("cli_defaults_held_up_read", at, strings.Join(held[at], ", "), cmd.CommandPath()))
	}
	a.tellWarnings(warned)
	return nil
}

// withWarnings returns the mistakes of err with warned, for the command's exit to tell them in one list, or
// tells warned alone when err holds none, returning it.
func (a *app) withWarnings(warned usererr.List, err error) error {
	all := slices.Clone(warned)
	if failed := all.Add(err); failed != nil || err == nil {
		a.tellWarnings(warned)
		return err
	}
	return all.Err()
}

// tellWarnings prints warned to stderr, as every mistake is laid out, for the command to go on.
func (a *app) tellWarnings(warned usererr.List) {
	if warned != nil {
		//nolint:errcheck // nowhere left to report
		_, _ = fmt.Fprintln(a.env.Stderr, usererr.Tell(english, warned).Render(named))
	}
}
