/*
Copyright 2025 Flant JSC

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
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

var DefaultKubeAPIRetryBackoff = wait.Backoff{
	Duration: 5 * time.Second,
	Factor:   1.5,
	Jitter:   0.1,
	Cap:      1 * time.Minute,
}

func DoWithRetry(ctx context.Context, name string, backoff wait.Backoff, fn func(ctx context.Context) (bool, error)) error {
	if backoff.Duration <= 0 {
		backoff.Duration = time.Second
	}
	if backoff.Cap <= 0 {
		backoff.Cap = backoff.Duration
	}

	delay := backoff.Duration
	for {
		syncCtx, cancel := context.WithTimeout(ctx, backoff.Cap)
		done, err := fn(syncCtx)
		cancel()

		if done {
			return nil
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("%s stopped: %w", name, ctx.Err())
		}
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			klog.Warningf("%s attempt failed: %v", name, err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("%s stopped: %w", name, ctx.Err())
		case <-time.After(delay):
		}

		nextDelay := time.Duration(float64(delay) * backoff.Factor)
		if nextDelay > backoff.Cap {
			nextDelay = backoff.Cap
		}
		delay = nextDelay
	}
}

func RetryWithBackoff(ctx context.Context, name string, backoff wait.Backoff, fn func(ctx context.Context) (bool, error)) error {
	return DoWithRetry(ctx, name, backoff, fn)
}
