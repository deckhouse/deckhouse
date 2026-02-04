// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflightnew

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

type CheckName string

func (n CheckName) String() string {
	return string(n)
}

type Check struct {
	Name        CheckName
	Description string
	Phase       Phase
	Run         func(ctx context.Context) error
	Retry       RetryPolicy
	Disabled    bool
}

func (c *Check) Disable() {
	c.Disabled = true
}

type RetryPolicy struct {
	Attempts int
	Options  []backoff.ExponentialBackOffOpts
}

var DefaultRetryPolicy = RetryPolicy{
	Attempts: 5,
	Options: []backoff.ExponentialBackOffOpts{
		backoff.WithInitialInterval(time.Second),
		backoff.WithMultiplier(2),
		backoff.WithMaxElapsedTime(0),
	},
}

type CheckInfo struct {
	Name        CheckName
	Description string
	Phase       Phase
	Disabled    bool
}
