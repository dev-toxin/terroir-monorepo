package domain

import "time"

// Clock abstracts "now" so that drinking-window logic is testable.
type Clock interface {
	Now() time.Time
}

// SystemClock returns real UTC time.
type SystemClock struct{}

// Now implements Clock.
func (SystemClock) Now() time.Time { return time.Now().UTC() }

// FixedClock always returns the same instant. Useful in tests and demos.
type FixedClock struct{ T time.Time }

// Now implements Clock.
func (c FixedClock) Now() time.Time { return c.T }
