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

package checker

import (
	"errors"
	"time"
)

const maxConsecutiveErrors = 3

var errConditionTimeout = errors.New("condition timeout")

func waitForCondition(timeout, interval time.Duration, condition func() (bool, error)) error {
	if interval <= 0 {
		interval = time.Second
	}

	consecutiveErrors := 0
	var lastErr error

	deadline := time.Now().Add(timeout)
	for {
		done, err := condition()
		if err != nil {
			consecutiveErrors++
			lastErr = err
			if consecutiveErrors > maxConsecutiveErrors {
				return lastErr
			}
		} else {
			consecutiveErrors = 0
			if done {
				return nil
			}
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return errConditionTimeout
		}
		time.Sleep(interval)
	}
}

func pollingInterval(timeout time.Duration) time.Duration {
	interval := timeout / 10
	if interval < time.Second {
		return time.Second
	}
	if interval > 5*time.Second {
		return 5 * time.Second
	}
	return interval
}
