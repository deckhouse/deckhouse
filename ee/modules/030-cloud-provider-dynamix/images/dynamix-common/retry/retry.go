/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package retry

import (
	"context"
	"time"
)

type Retryer struct {
	attempts int
	delay    time.Duration
}

func NewRetryer() Retryer {
	return Retryer{
		attempts: 3,
		delay:    5 * time.Second,
	}
}

func (r Retryer) WithAttempts(attempts int) Retryer {
	r.attempts = attempts

	return r
}

func (r Retryer) WithDelay(delay time.Duration) Retryer {
	r.delay = delay

	return r
}

func (r Retryer) Do(ctx context.Context, fn func() (bool, error)) error {
	var err error

	for i := 0; i < r.attempts; i++ {
		var stop bool

		stop, err = fn()
		if stop {
			return err
		}
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(r.delay):
		}
	}

	return err
}
