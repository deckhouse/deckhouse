/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package retry

import (
	"context"
	"fmt"
	"time"
)

type RetryFn func() error
type BreakFn func(lastErr error) bool
type BeforeFn func(interval time.Duration, attempts, attempt uint, lastErr error)

type Retry struct {
	Attempts uint
	Interval time.Duration
	Before   BeforeFn
	Break    BreakFn
}

func Default() *Retry {
	return &Retry{
		Attempts: 10,
		Interval: 5 * time.Second,
	}
}

func (r *Retry) WithBreak(fn BreakFn) *Retry {
	r.Break = fn
	return r
}

func (r *Retry) WithBefore(fn BeforeFn) *Retry {
	r.Before = fn
	return r
}

// Do calls fn up to attempts times, waiting interval between retries.
// The delay occurs AFTER a failed attempt, not before the first one.
// If Before is not nil, it is called before each retry wait.
// If Break returns true for an error, retries stop immediately.
// Returns nil on first success, or the last error after all attempts are exhausted.
func (r *Retry) Do(ctx context.Context, fn RetryFn) error {
	if r == nil {
		return fmt.Errorf("retry is nil")
	}

	var lastErr error
	for i := uint(0); i < r.Attempts; i++ {
		if i != 0 {
			if r.Before != nil {
				r.Before(r.Interval, r.Attempts, i, lastErr)
			}

			select {
			case <-time.After(r.Interval):
			case <-ctx.Done():
				if lastErr != nil {
					return fmt.Errorf("retry cancelled, last error: %w", lastErr)
				}
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			}
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if r.Break != nil && r.Break(lastErr) {
			return fmt.Errorf("retry broken at attempt %d: %w", i, lastErr)
		}
	}
	return fmt.Errorf("all %d attempts failed, last error: %w", r.Attempts, lastErr)
}
