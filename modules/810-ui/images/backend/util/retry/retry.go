// Copyright 2023 Flant JSC
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

package retry

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

const attemptMessage = `Attempt #%d of %d |
	%s failed, next attempt will be in %v"
`

var InTestEnvironment = false

func setupTests(attemptsQuantity *int, wait *time.Duration) {
	if InTestEnvironment {
		*attemptsQuantity = 1
		*wait = 0 * time.Second
	}
}

type BreakPredicate func(err error) bool

func IsErr(err error) BreakPredicate {
	return func(target error) bool {
		return errors.Is(err, target)
	}
}

// Loop retries a task function until it succeeded with number of attempts and delay between runs are adjustable.
type Loop struct {
	name             string
	attemptsQuantity int
	waitTime         time.Duration
	breakPredicate   BreakPredicate
	logger           klog.Logger
}

// NewLoop create Loop with features:
// - it is "verbose" loop — it prints messages through logboek.
// - this loop is interruptable by the signal watcher in tomb package.
func NewLoop(name string, attemptsQuantity int, wait time.Duration) *Loop {
	return &Loop{
		name:             name,
		attemptsQuantity: attemptsQuantity,
		waitTime:         wait,
		logger:           klog.NewKlogr().V(0),
	}
}

func (l *Loop) BreakIf(pred BreakPredicate) *Loop {
	l.breakPredicate = pred
	return l
}

// Run retries a task function until it succeeded or break task retries if break predicate returns true
func (l *Loop) Run(task func() error) error {
	setupTests(&l.attemptsQuantity, &l.waitTime)

	var err error
	for i := 1; i <= l.attemptsQuantity; i++ {
		// Run task and return if everything is ok.
		err = task()
		if err == nil {
			l.logger.V(10).Info("Succeeded!")
			return nil
		}

		if l.breakPredicate != nil && l.breakPredicate(err) {
			l.logger.V(10).Info("Client break loop with %v\n", err)
			return err
		}

		l.logger.Error(err, fmt.Sprintf(attemptMessage, i, l.attemptsQuantity, l.name, l.waitTime))

		// Do not waitTime after the last iteration.
		if i < l.attemptsQuantity {
			time.Sleep(l.waitTime)
		}
	}

	return fmt.Errorf("timeout while %q: last error: %v", l.name, err)
}
