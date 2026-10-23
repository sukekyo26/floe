package cli

import (
	"fmt"

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

// use is how a command uses a setting: it reads what is there, or it writes there or cannot do without it.
type use string

const (
	reads  use = "read"
	writes use = "write"
)

// declared marks a command that declares the settings it uses, those it does not included.
const declared = "settings"

// uses declares, as the Annotations of a command, how it uses each setting it uses; one it does not name it
// does not use, and is told nothing of.
func uses(each map[setting]use) map[string]string {
	marks := map[string]string{declared: ""}
	for s, u := range each {
		marks[string(s)] = string(u)
	}
	return marks
}

// settle stops cmd, telling every reason at once, on a setting it uses that is written wrong, and on one it
// writes or cannot do without that has no default; one it only reads that has no default it goes on
// without, its value none. It is the one place that tells the mistakes of the settings.
func (a *app) settle(cmd *cobra.Command) error {
	var found usererr.List
	for _, s := range settings {
		u := use(cmd.Annotations[string(s)])
		switch {
		case u == "":
		case a.cfg.wrong[s] != nil:
			found = append(found, a.cfg.wrong[s])
		case u == writes && a.cfg.absent[s] != nil:
			found = append(found, a.cfg.absent[s])
		}
	}
	return found.Err()
}

// tellWarnings prints warned to stderr, as every mistake is laid out, for the command to go on.
func (a *app) tellWarnings(warned usererr.List) {
	if warned != nil {
		//nolint:errcheck // nowhere left to report
		_, _ = fmt.Fprintln(a.env.Stderr, usererr.Tell(english, warned).Render(named))
	}
}
