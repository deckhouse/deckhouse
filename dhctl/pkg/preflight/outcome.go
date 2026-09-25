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
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status is how a check ended. Every outcome that is not a pass has a glyph of its own: the
// runner used to print ✓ for a pass, for a cached result, for a check the user skipped and for a
// check that found nothing to look at, which made "all green" mean nothing.
type Status string

const (
	// StatusPassed — the check ran now and passed.
	StatusPassed Status = "passed"
	// StatusCached — a previous run of this check passed and nothing it depends on has changed.
	StatusCached Status = "cached"
	// StatusSkipped — the user turned it off with --preflight-skip-check/--preflight-skip-all-checks.
	StatusSkipped Status = "skipped"
	// StatusNotApplicable — the check had nothing to look at (no bastion configured, no
	// <Provider>ClusterConfiguration, an installer built without an edition). Not a pass: it
	// asserts nothing, so it is neither cached nor counted as one.
	StatusNotApplicable Status = "not applicable"
	// StatusBlocked — a check this one depends on failed, so running it would only produce a
	// second copy of the same error (every SSH check after static-ssh-credential).
	StatusBlocked Status = "blocked"
	// StatusWarned — it ran, it did not like what it found, and it let the run continue.
	//
	// For the findings that are about how the cluster was sized rather than about whether this
	// bootstrap can finish. node-disk-space is the one: the documentation asks a master for a
	// 50 GB disk, and every platform we run e2e on gives it less — those clusters bootstrap and
	// work. A check that refuses them refuses the product, and nearly every run reaches dhctl
	// through Commander, where there is no command line to put a skip flag on. Saying it and
	// going on is the honest answer; failing is not.
	StatusWarned Status = "warned"
	// StatusFailed — it ran and failed.
	StatusFailed Status = "failed"
)

// PhaseStoppedError is the sentence the summary carries when a check on the critical path failed
// and the rest of the phase was not attempted.
const phaseStoppedSuffix = "not run after the phase stopped"

func (s Status) glyph() string {
	switch s {
	case StatusPassed, StatusCached:
		return "✓"
	case StatusSkipped:
		return "–"
	case StatusNotApplicable:
		return "↷"
	case StatusBlocked:
		return "⊘"
	case StatusWarned:
		return "⚠"
	default:
		return "✗"
	}
}

// Result is one line of the phase report.
type Result struct {
	Name        CheckName
	Description string
	Status      Status
	// Detail is what the check itself reported: the value-bearing success line, the reason it
	// did not apply, the flag that skipped it, the check that blocked it. Empty for a pass that
	// has nothing to add, in which case Description carries the line.
	Detail string
	// Err is set for StatusFailed only.
	Err error
	// Attempts is how many times the body ran; 0 for outcomes decided without running it.
	Attempts int
	Elapsed  time.Duration
	// Permanent records that the body asked not to be retried, so the report can say why a
	// check with a 3-attempt policy stopped after one.
	Permanent bool
	// CannotBeSkipped is carried over from the check so the report can say that there is no
	// flag for this one rather than print a flag that will be refused.
	CannotBeSkipped bool
	// CannotBeSkippedReason is carried over with it.
	CannotBeSkippedReason string
	// Age is how long ago the remembered pass behind StatusCached was recorded. Zero when the
	// entry predates the timestamped format and carries no time.
	Age time.Duration
}

// line renders the record the terminal shows while the phase runs.
func (r Result) line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s %s", r.Status.glyph(), r.Name, r.text())
	if suffix := r.suffix(); suffix != "" {
		fmt.Fprintf(&b, " (%s)", suffix)
	}
	return b.String()
}

// text is the body of the record line: what the check reported, or — when it reported nothing —
// the assertion that holds when it passes.
func (r Result) text() string {
	if r.Detail != "" {
		switch r.Status {
		case StatusSkipped:
			return fmt.Sprintf("skipped: %s", r.Detail)
		case StatusNotApplicable:
			return fmt.Sprintf("not applicable: %s", r.Detail)
		case StatusBlocked:
			return fmt.Sprintf("blocked: %s", r.Detail)
		case StatusWarned:
			return fmt.Sprintf("warning: %s", r.Detail)
		default:
			return r.Detail
		}
	}
	if r.Status == StatusSkipped {
		return "skipped"
	}
	return r.Description
}

// suffix is the parenthesised tail: how the result was reached rather than what it was.
func (r Result) suffix() string {
	switch r.Status {
	case StatusCached:
		if r.Age > 0 {
			return fmt.Sprintf("cached, %s ago", roundDuration(r.Age))
		}
		return "cached"
	case StatusFailed:
		switch {
		case r.Permanent:
			return "permanent failure, not retried"
		case r.Attempts > 1:
			return fmt.Sprintf("failed after %d attempts, %s", r.Attempts, roundDuration(r.Elapsed))
		}
	}
	return ""
}

// roundDuration drops the precision nobody reads. A check that took 3.41 seconds is worth two
// decimals; an answer remembered twelve minutes ago is not worth six hundred milliseconds of them.
func roundDuration(d time.Duration) time.Duration {
	switch {
	case d < time.Second:
		return d.Round(time.Millisecond)
	case d < time.Minute:
		return d.Round(10 * time.Millisecond)
	case d < time.Hour:
		return d.Round(time.Second)
	default:
		return d.Round(time.Minute)
	}
}

// ErrNotApplicable is what errors.Is matches a not-applicable outcome against.
var ErrNotApplicable = errors.New("not applicable")

// ErrWarning is what errors.Is matches a warning against.
var ErrWarning = errors.New("warning")

type warningError struct {
	reason string
}

func (e *warningError) Error() string { return e.reason }

func (e *warningError) Is(target error) bool { return target == ErrWarning }

// Warning ends a check with something worth saying that is not worth stopping for. The phase
// goes on, nothing downstream is blocked, and the record line carries the reason.
func Warning(format string, args ...any) error {
	return &warningError{reason: fmt.Sprintf(format, args...)}
}

func warningReason(err error) string {
	var w *warningError
	if errors.As(err, &w) {
		return w.reason
	}
	return ""
}

type notApplicableError struct {
	reason string
}

// NotApplicable ends a check with the outcome that says it found nothing to look at. It is not a
// pass: the check asserts nothing, so it is not cached and does not count towards the passed
// tally. The reason is printed as-is, so write it for the reader — "no --ssh-bastion-host", not
// "bastionHost == \"\"".
func NotApplicable(format string, args ...any) error {
	return &notApplicableError{reason: fmt.Sprintf(format, args...)}
}

func (e *notApplicableError) Error() string { return "not applicable: " + e.reason }

func (e *notApplicableError) Is(target error) bool { return target == ErrNotApplicable }

// notApplicableReason returns the reason carried by a not-applicable error.
func notApplicableReason(err error) string {
	var na *notApplicableError
	if errors.As(err, &na) {
		return na.reason
	}
	return ""
}
