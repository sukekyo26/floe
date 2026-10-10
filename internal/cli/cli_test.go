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
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
