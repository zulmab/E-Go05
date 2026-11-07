package clock

import "time"

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

type ManualClock struct {
	current time.Time
}

func NewManualClock(start time.Time) *ManualClock {
	return &ManualClock{current: start}
}

func (c *ManualClock) Now() time.Time {
	return c.current
}

func (c *ManualClock) Set(next time.Time) {
	c.current = next
}
