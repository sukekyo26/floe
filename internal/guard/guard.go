// Package guard watches the memory of a process that draws pages and ends it past a limit: an expression or a
// page.html that repeats a long text for each item would otherwise take gigabytes, which a data file's 1 MiB does
// not bound. Go cannot stop a goroutine from outside, so the process is what stops.
package guard

import (
	"runtime/debug"
	"runtime/metrics"
	"sync"
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
	file   atomic.Pointer[string]
	done   chan struct{}
	once   sync.Once
	target int64 // the collector's target before Start
}

// Start watches the process's memory, and once it holds more than limit, calls stop with the file being drawn (""
// for none), which must end the process. It sets the collector's target at 3/4 of limit, or lower as GOMEMLIMIT
// sets it, so that garbage, even with GOGC=off, is collected before it reaches limit: only memory held stops.
// LIMIT: a right page holding between 3/4 of limit and limit (384 to 512 MiB) keeps the collector running, and is
// drawn slower; a larger GOMEMLIMIT does not lift the target. Raise limit if such pages come.
func Start(limit uint64, stop func(file string)) *Guard {
	target := debug.SetMemoryLimit(-1)
	debug.SetMemoryLimit(min(target, int64(limit/4*3))) //nolint:gosec // G115: 512 MiB fits
	g := &Guard{file: atomic.Pointer[string]{}, done: make(chan struct{}), once: sync.Once{}, target: target}
	go g.watch(limit, stop)
	return g
}

// Stop ends the watch, once the process has drawn, and gives the collector its target back: what the process holds
// after (the pages it writes) is not drawing. A nil g does nothing.
// LIMIT: a stop already called, as the watch read the memory just before, still ends the process.
func (g *Guard) Stop() {
	if g != nil {
		g.once.Do(func() {
			close(g.done)
			debug.SetMemoryLimit(g.target)
		})
	}
}

// Drawing tells g the file the process draws now, "" for none. A nil g does nothing.
func (g *Guard) Drawing(file string) {
	if g != nil {
		g.file.Store(&file)
	}
}

func (g *Guard) watch(limit uint64, stop func(file string)) {
	s := []metrics.Sample{{Name: total, Value: metrics.Value{}}, {Name: released, Value: metrics.Value{}}}
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-g.done:
			return
		case <-tick.C:
		}
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
