package mocker

import "time"

type options struct {
	MinDelay time.Duration
	MaxDelay time.Duration
}

type option func(*options)

func WithMinDelay(minDelay time.Duration) option {
	return func(o *options) { o.MinDelay = minDelay }
}

func WithMaxDelay(maxDelay time.Duration) option {
	return func(o *options) { o.MaxDelay = maxDelay }
}
