package drawproc

import "time"

// Stopper is stopper, for the tests outside the package to give a Child their own timer.
type Stopper = stopper

// SetAfter has c time each page with after, as time.AfterFunc.
func (c *Child) SetAfter(after func(d time.Duration, f func()) Stopper) {
	c.after = after
}
