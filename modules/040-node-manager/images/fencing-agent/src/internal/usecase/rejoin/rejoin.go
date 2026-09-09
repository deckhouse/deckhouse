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

package rejoin

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	idleTick = time.Second

	jitterFraction = 0.2
)

type Params struct {
	Interval    time.Duration
	MaxInterval time.Duration
}

type Deps struct {
	Attempt   func(ctx context.Context) error
	HasQuorum func() bool
	NotMember    func(err error) bool
	APIReachable func() bool
	Changed      <-chan struct{}
	Sleep        func(ctx context.Context, d time.Duration) bool
}

type Loop struct {
	params Params
	deps   Deps
	logger *log.Logger
	reportedNotMember bool
}

func New(params Params, deps Deps, logger *log.Logger) *Loop {
	if deps.Sleep == nil {
		deps.Sleep = sleep
	}

	return &Loop{params: params, deps: deps, logger: logger}
}

func (l *Loop) Run(ctx context.Context) error {
	ticker := time.NewTicker(idleTick)
	defer ticker.Stop()

	delay := l.params.Interval
	attempts := 0

	for ctx.Err() == nil {
		if l.deps.HasQuorum() {
			if attempts > 0 {
				l.logger.Info("gossip quorum restored", "rejoin_attempts", attempts)
			}

			attempts, delay = 0, l.params.Interval
			l.reportedNotMember = false

			select {
			case <-ctx.Done():
			case <-l.deps.Changed:
			case <-ticker.C:
			}

			continue
		}

		if !l.deps.APIReachable() {
			l.logger.Debug("no gossip quorum and no Kubernetes API, rejoin is not attempted")

			if !l.deps.Sleep(ctx, jitter(delay)) {
				return nil
			}

			continue
		}

		attempts++

		if err := l.deps.Attempt(ctx); err != nil {
			l.reportAttemptError(err, attempts, delay)
		} else if l.deps.HasQuorum() {
			continue
		} else {
			l.logger.Info("rejoin attempt did not restore quorum", "attempt", attempts, "next_in", delay.String())
		}

		if !l.deps.Sleep(ctx, jitter(delay)) {
			return nil
		}

		delay = min(delay*2, l.params.MaxInterval)
	}

	return nil
}

func (l *Loop) reportAttemptError(err error, attempt int, delay time.Duration) {
	if l.deps.NotMember != nil && l.deps.NotMember(err) {
		if l.reportedNotMember {
			return
		}

		l.reportedNotMember = true

		l.logger.Warn("this node is not a member of its NodeGroup, rejoin is not attempted until that changes", "error", err)

		return
	}

	l.reportedNotMember = false

	l.logger.Warn("rejoin attempt failed", "error", err, "attempt", attempt, "next_in", delay.String())
}

func jitter(delay time.Duration) time.Duration {
	spread := float64(delay) * jitterFraction

	return delay + time.Duration((rand.Float64()*2-1)*spread)
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
