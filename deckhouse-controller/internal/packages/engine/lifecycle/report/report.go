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

// Package report stores the Report of one package or of Global.
package report

import (
	"reflect"
	"sync"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// Reporter stores one Report and calls notify after every change, outside the lock. Each section
// has one writer: Current or Global writes the Current section (UpdateCurrent), the package
// Reconciler the rest. The writers reach it through their own narrow interfaces, so the compiler
// keeps them out of each other's sections. The reader is Engine.Report (Snapshot). Safe for
// concurrent use.
type Reporter struct {
	mu     sync.Mutex
	report model.Report
	notify func() // notify must not block.
}

// New returns an empty Reporter; notify nil notifies nobody.
func New(notify func()) *Reporter {
	return &Reporter{notify: notify}
}

// UpdateCurrent applies f to the Current section under the lock, so a loop and its worker (the
// ApplyingManifests mark) never lose each other's write.
func (r *Reporter) UpdateCurrent(f func(*model.CurrentReport)) {
	r.write(func(rep *model.Report) bool {
		next := rep.Current
		f(&next)

		changed := !reflect.DeepEqual(next, rep.Current)
		rep.Current = next

		return changed
	})
}

// SetCandidate replaces the Candidate section; a repeated blocker or failure changes nothing.
func (r *Reporter) SetCandidate(ur model.CandidateReport) {
	r.write(func(rep *model.Report) bool {
		changed := rep.Candidate != ur
		rep.Candidate = ur

		return changed
	})
}

// SetActivated records the activated version and clears the Candidate section in one write, so no
// reader sees the activated version as a pending candidate.
func (r *Reporter) SetActivated(version string) {
	r.write(func(rep *model.Report) bool {
		changed := rep.CurrentVersion != version || rep.Candidate != model.CandidateReport{}
		rep.CurrentVersion = version
		rep.Candidate = model.CandidateReport{}

		return changed
	})
}

// SetRemoved marks the package removed.
func (r *Reporter) SetRemoved() {
	r.write(func(rep *model.Report) bool {
		changed := !rep.Removed
		rep.Removed = true

		return changed
	})
}

// SetRemoveError replaces RemoveError; "" clears it.
func (r *Reporter) SetRemoveError(msg string) {
	r.write(func(rep *model.Report) bool {
		changed := rep.RemoveError != msg
		rep.RemoveError = msg

		return changed
	})
}

// Snapshot returns a copy of the Report.
func (r *Reporter) Snapshot() model.Report {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.report
}

// write applies f under the lock and notifies outside it when f reports a change.
func (r *Reporter) write(f func(*model.Report) bool) {
	r.mu.Lock()
	changed := f(&r.report)
	r.mu.Unlock()

	if changed && r.notify != nil {
		r.notify()
	}
}
