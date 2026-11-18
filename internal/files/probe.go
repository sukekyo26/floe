package files

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

var probeMu sync.Mutex

// probe (temporary, S394): one line per call: kind, path, the floe frames above files.
func probe(kind, path string) {
	out := os.Getenv("FLOE_PROBE_LOG")
	if out == "" {
		return
	}
	pc := make([]uintptr, 40)
	n := runtime.Callers(3, pc)
	frames := runtime.CallersFrames(pc[:n])
	var chain []string
	for {
		f, more := frames.Next()
		name := f.Function
		if strings.HasPrefix(name, "floe/") && !strings.HasPrefix(name, "floe/internal/files.") {
			name = strings.TrimPrefix(name, "floe/internal/")
			chain = append(chain, name)
		}
		if !more || len(chain) >= 6 {
			break
		}
	}
	probeMu.Lock()
	defer probeMu.Unlock()
	f, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%d\t%s\t%s\t%s\n", os.Getpid(), kind, path, strings.Join(chain, " < "))
}
