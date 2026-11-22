// Package guard watches the memory of a process that draws pages and ends it past a limit: an expression or a
// page.html that repeats a long text for each item would otherwise take gigabytes, which a data file's 1 MiB does
// not bound. Go cannot stop a goroutine from outside, so the process is what stops.
package guard

import (
	"runtime/debug"
	"runtime/metrics"
	"sync/atomic"
	"time"
)

// Limit is the memory a process that draws may hold.
const Limit = 512 << 20

// every is how often the memory is read.
// LIMIT: what grows between two reads is held past the limit, a whole allocation at once: a buffer that doubles
// can reach about twice the limit before the process ends.
const every = 10 * time.Millisecond

// The memory read: what the runtime holds from the system, less what it gave back.
const (
	total    = "/memory/classes/total:bytes"
	released = "/memory/classes/heap/released:bytes"
)

// Guard is the watch of a process: the file it draws, which its stop is told.
type Guard struct {
	file atomic.Pointer[string]
}

// Start watches the process's memory, and once it holds more than limit, calls stop with the file being drawn (""
// for none), which must end the process. It sets the collector's target at 3/4 of limit, or lower as GOMEMLIMIT
// sets it, so that garbage, even with GOGC=off, is collected before it reaches limit: only memory held stops.
func Start(limit uint64, stop func(file string)) *Guard {
	debug.SetMemoryLimit(min(debug.SetMemoryLimit(-1), int64(limit/4*3))) //nolint:gosec // G115: 512 MiB fits
	g := &Guard{file: atomic.Pointer[string]{}}
	go g.watch(limit, stop)
	return g
}

// Drawing tells g the file the process draws now, "" for none. A nil g does nothing.
func (g *Guard) Drawing(file string) {
	if g != nil {
		g.file.Store(&file)
	}
}

func (g *Guard) watch(limit uint64, stop func(file string)) {
	s := []metrics.Sample{{Name: total, Value: metrics.Value{}}, {Name: released, Value: metrics.Value{}}}
	for range time.Tick(every) {
		metrics.Read(s)
		if s[0].Value.Uint64()-s[1].Value.Uint64() > limit {
			file := ""
			if f := g.file.Load(); f != nil {
				file = *f
			}
			stop(file)
			return
		}
	}
}
