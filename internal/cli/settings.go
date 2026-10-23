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

const (
	// declared marks a command that declares the settings it uses, those it does not included.
	declared = "settings"
	needs    = "need"
	reads    = "read"
)

// uses declares, as the Annotations of a command, the settings it needs, whose mistakes stop it, and those
// it reads when they are right, whose mistakes it is warned of and goes on without. A setting it declares
// neither way it does not use, and its mistakes are not told.
func uses(need, read []setting) map[string]string {
	marks := map[string]string{declared: ""}
	for _, s := range need {
		marks[string(s)] = needs
	}
	for _, s := range read {
		marks[string(s)] = reads
	}
	return marks
}

// settle stops cmd on the mistakes of the settings it needs, all at once, and warns of those of the
// settings it reads, which it then goes on without: the one place that tells the mistakes of the settings.
func (a *app) settle(cmd *cobra.Command) error {
	var found, warned usererr.List
	for _, s := range settings {
		switch cmd.Annotations[string(s)] {
		case needs:
			found = append(found, a.cfg.wrong[s]...)
		case reads:
			for _, w := range a.cfg.wrong[s] {
				warned = append(warned, usererr.New("cli_setting_left_out", w, cmd.CommandPath()))
			}
			if dir := a.cfg.dir(s); dir != nil && a.cfg.wrong[s] != nil {
				*dir = ""
			}
		}
	}
	if err := found.Err(); err != nil {
		return err
	}
	a.tellWarnings(warned)
	return nil
}

// tellWarnings prints warned to stderr, as every mistake is laid out, for the command to go on.
func (a *app) tellWarnings(warned usererr.List) {
	if warned != nil {
		//nolint:errcheck // nowhere left to report
		_, _ = fmt.Fprintln(a.env.Stderr, usererr.Tell(english, warned).Render(named))
	}
}
