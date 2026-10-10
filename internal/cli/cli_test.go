package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/cli"
	"github.com/sukekyo26/floe/internal/version"
)

func TestRun(t *testing.T) {
	t.Parallel()

	want := "floe " + version.Version + "\n"
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "version subcommand", args: []string{"version"}, wantCode: 0, wantStdout: want},
		{name: "version flag", args: []string{"--version"}, wantCode: 0, wantStdout: want},
		{name: "no args prints help", args: nil, wantCode: 0, wantStdout: "Usage:"},
		{name: "unknown command", args: []string{"nope"}, wantCode: 1, wantStderr: `floe: unknown command "nope"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			if got := cli.Run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %q)", got, tt.wantCode, stderr.String())
			}
			checkOutput(t, "stdout", stdout.String(), tt.wantStdout)
			checkOutput(t, "stderr", stderr.String(), tt.wantStderr)
		})
	}
}

// checkOutput requires an empty stream when want is empty; strings.Contains(got, "")
// would pass for any output.
func checkOutput(t *testing.T, stream, got, want string) {
	t.Helper()

	if want == "" {
		if got != "" {
			t.Errorf("%s = %q, want it empty", stream, got)
		}
		return
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s = %q, want it to contain %q", stream, got, want)
	}
}
