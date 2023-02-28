/*
Copyright 2023 Flant JSC

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

package check

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// Runner is a glue between the scheduler and a check.
type Runner struct {
	// defined by check
	ref       *ProbeRef
	checkName string
	checker   Checker
	period    time.Duration

	// defined by lifecycle
	state *state

	logger *log.Entry
}

func NewRunner(groupName, probeName, checkName string, period time.Duration, checker Checker, logger *log.Entry) *Runner {
	ref := &ProbeRef{
		Group: groupName,
		Probe: probeName,
	}
	return &Runner{
		ref:       ref,
		checkName: checkName,
		checker:   checker,
		period:    period,
		state:     &state{},
		logger:    logger,
	}
}

func (r *Runner) ProbeRef() ProbeRef {
	return *r.ref
}

func (r *Runner) Run(start time.Time) Result {
	r.state.Start(start)
	defer r.state.Done()

	status := Up
	err := r.checker.Check()
	if err != nil {
		status = err.Status()
		r.logger.WithField("status", status).Errorf(err.Error())
	}

	return NewResult(*r.ref, r.checkName, status)
}

func (r *Runner) Period() time.Duration {
	return r.period
}

func (r *Runner) ShouldRun(when time.Time) bool {
	return r.state.shouldRun(when, r.period)
}

type state struct {
	lastStart time.Time
	running   bool
}

// ShouldRun checks that the probe can be run. Returns true if the probe is not
// running and its period passed
func (s *state) shouldRun(when time.Time, period time.Duration) bool {
	if s.running {
		return false
	}
	nextStart := s.lastStart.Add(period)
	return when.Equal(nextStart) || when.After(nextStart)
}

func (s *state) Start(t time.Time) {
	s.running = true
	s.lastStart = t
}

func (s *state) Done() {
	s.running = false
}
