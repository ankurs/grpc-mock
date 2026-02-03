package mocker

import "time"

// options holds configuration for the mock client.
type options struct {
	MinDelay time.Duration
	MaxDelay time.Duration
}

// Option is a functional option for configuring a Mocker.
type Option func(*options)

// WithMinDelay sets the minimum delay before returning a response.
// This is useful for simulating network latency in tests.
func WithMinDelay(minDelay time.Duration) Option {
	return func(o *options) { o.MinDelay = minDelay }
}

// WithMaxDelay sets the maximum delay before returning a response.
// The actual delay will be randomly chosen between MinDelay and MaxDelay.
func WithMaxDelay(maxDelay time.Duration) Option {
	return func(o *options) { o.MaxDelay = maxDelay }
}

// WithDelay sets both minimum and maximum delay to the same value,
// resulting in a fixed delay for all responses.
func WithDelay(delay time.Duration) Option {
	return func(o *options) {
		o.MinDelay = delay
		o.MaxDelay = delay
	}
}

// WithDelayRange sets both minimum and maximum delay values.
// The actual delay will be randomly chosen between min and max.
func WithDelayRange(min, max time.Duration) Option {
	return func(o *options) {
		o.MinDelay = min
		o.MaxDelay = max
	}
}
