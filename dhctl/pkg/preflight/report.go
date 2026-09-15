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
)

// PhaseError is what a phase with at least one failed check returns. Its message is the report
// the reader acts on; Unwrap keeps every underlying cause reachable with errors.Is and
// errors.As, so a caller that wants to single out one of them still can.
type PhaseError struct {
	Title   string
	Results []Result
	// Tally is what became of every check of the phase, including the ones that never ran. It is
	// in the report rather than on a line of its own because the process box prints the phase as
	// failed by itself: a second line naming the same phase said the same thing twice, and the
	// reader is already looking here.
	Tally string
}

func newPhaseError(title string, results []Result, notRun int) error {
	var failed []Result
	for _, r := range results {
		if r.Status == StatusFailed {
			failed = append(failed, r)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return &PhaseError{Title: title, Results: failed, Tally: summarize(results, notRun)}
}

func (e *PhaseError) Unwrap() []error {
	errs := make([]error, 0, len(e.Results))
	for _, r := range e.Results {
		if r.Err != nil {
			errs = append(errs, r.Err)
		}
	}
	return errs
}

func (e *PhaseError) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%d preflight check", len(e.Results))
	if len(e.Results) > 1 {
		b.WriteString("s")
	}
	if e.Tally != "" {
		fmt.Fprintf(&b, " failed (%s)", e.Tally)
		b.WriteString(":\n")
		e.writeResults(&b)
		return b.String()
	}
	b.WriteString(" failed:\n")
	e.writeResults(&b)
	return b.String()
}

func (e *PhaseError) writeResults(b *strings.Builder) {
	for i, r := range e.Results {
		fmt.Fprintf(b, "\n[%d] %s — %s\n", i+1, r.Name, r.Description)
		writeCause(b, r.Err)
		writeSkipLine(b, r)
		writeField(b, "docs", docsFor(r.Err))
	}

	// Both ways forward, named. The reader who can fix the configuration re-runs; the reader who
	// cannot — a registry that is down for the afternoon, a check that is wrong about their
	// setup — needs to know the flags exist and are printed above.
	b.WriteString("\nRe-run the same command after fixing, or add the skip flags above to proceed anyway.")
}

// writeCause renders what the check reported: the five fields when it returned a *Failure, and
// the error text under `reason:` when it returned anything else.
func writeCause(b *strings.Builder, err error) {
	var failure *Failure
	if errors.As(err, &failure) {
		failure.writeFields(b)
		return
	}
	if err != nil {
		writeField(b, "reason", err.Error())
	}
}

// writeSkipLine names the flag that turns this check off, or says there is none. The flag used
// to be discoverable only from --help, and the check names there have changed over releases, so
// the reader could not derive it from the failure they were looking at.
func writeSkipLine(b *strings.Builder, r Result) {
	if r.CannotBeSkipped {
		writeField(b, "skip", "this check cannot be skipped")
		return
	}
	writeField(b, "skip", fmt.Sprintf("--preflight-skip-check=%s", r.Name))
}

func docsFor(err error) string {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.docs()
	}
	return DocsURL
}
