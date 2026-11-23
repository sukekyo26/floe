package guard_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"floe/internal/guard"
)

// child makes the test binary a process that the tests watch: "hold" keeps what it makes, "garbage" drops it, and
// "drawn" keeps it after the watch is stopped.
const child = "FLOE_GUARD_TEST_CHILD"

// limit is the memory a child may hold; each child makes 512 MiB at most, so that one a watch fails to end ends.
const (
	limit = 64 << 20
	made  = 512 << 20
	chunk = 1 << 20
)

// stopped is the exit code of a child its watch ended.
const stopped = 3

func TestMain(m *testing.M) {
	if mode := os.Getenv(child); mode != "" {
		g := guard.Start(limit, func(file string) {
			fmt.Print(file)
			os.Exit(stopped)
		})
		g.Drawing("page.yaml")
		if mode == "drawn" {
			g.Stop()
		}
		var kept [][]byte
		for range made / chunk {
			b := make([]byte, chunk)
			b[0] = 1
			if mode != "garbage" {
				kept = append(kept, b)
			}
			// Slow enough for the watch to read between: 512 steps of 2 ms.
			time.Sleep(2 * time.Millisecond)
		}
		_ = kept
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// run runs a child of mode with the environment vars, and returns its exit code and output.
func run(t *testing.T, mode string, vars ...string) (int, string) {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, self)
	cmd.Env = append([]string{child + "=" + mode}, vars...)
	out, err := cmd.Output()
	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		return ee.ExitCode(), string(out)
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, string(out)
}

// A process holding more than the limit is stopped, told the file it draws; one that makes eight times the limit as
// garbage is not, though GOMEMLIMIT or GOGC=off would let the collector wait past the limit, nor one that holds it
// once its watch is stopped.
func TestStart(t *testing.T) {
	t.Parallel()

	for _, vars := range [][]string{nil, {"GOMEMLIMIT=2GiB"}, {"GOGC=off"}, {"GOGC=off", "GOMEMLIMIT=2GiB"}} {
		if code, out := run(t, "hold", vars...); code != stopped || out != "page.yaml" {
			t.Errorf("a child holding %d MiB with %v = %d, %q; want %d, page.yaml", made>>20, vars, code, out, stopped)
		}
		if code, out := run(t, "garbage", vars...); code != 0 {
			t.Errorf("a child dropping %d MiB with %v = %d, %q; want 0", made>>20, vars, code, out)
		}
		if code, out := run(t, "drawn", vars...); code != 0 {
			t.Errorf("a child holding %d MiB after its watch with %v = %d, %q; want 0", made>>20, vars, code, out)
		}
	}
}
