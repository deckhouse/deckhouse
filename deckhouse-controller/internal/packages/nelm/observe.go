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

// What the release service reports besides its result: the observations it makes while it renders,
// applies and deletes a release, and the reason a failure is reported under.

package nelm

import (
	"encoding/json"
	"slices"
	"sync"

	"github.com/werf/nelm/v2/pkg/legacy/progrep"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
)

// Reasons a release failure is reported under (reasonError); the strings are part of the status
// contract.
const (
	reasonRenderFailed           = "RenderFailed"
	reasonCheckTemplatesFailed   = "CheckTemplatesFailed"
	reasonCreateValuesFileFailed = "CreateValuesFileFailed"
	reasonCheckReleaseFailed     = "CheckReleaseFailed"
	reasonApplyManifestsFailed   = "ApplyManifestsFailed"
)

// observations is the part of the observation store the release service writes, as the only
// writer of the release progress, the endpoints and "no workloads".
type observations interface {
	ResetTracking(name string)
	UpdateTracking(name string, t observed.Tracking)
	UpdateURLs(name string, urls []observed.URL)
	SetNoWorkloads(name string, none bool)
}

// reasonError is a release failure with its reason. The consumer reads it through its own
// Reason() interface, so no type is shared.
type reasonError struct {
	reason string
	err    error
}

// newReasonError wraps err with reason; the message stays err's own.
func newReasonError(reason string, err error) error {
	return &reasonError{reason: reason, err: err}
}

func (e *reasonError) Error() string { return e.err.Error() }

func (e *reasonError) Unwrap() error { return e.err }

// Reason is the CamelCase reason of the failure.
func (e *reasonError) Reason() string { return e.reason }

// The progress reports are converted here, the only place that speaks nelm, so the store and the
// projection stay independent of nelm's types; the report goes to the status as raw JSON.

// isResourceOperation reports whether op acts on a Kubernetes resource. Stage boundaries and
// release record updates carry no resource.
func isResourceOperation(op progrep.Operation) bool {
	return op.Category == progrep.OperationCategoryResource ||
		op.Category == progrep.OperationCategoryTrack
}

// tracking converts a progress report into status.tracking's shape (observed.Tracking). False for
// a report without operations (nelm sends trailing empty ones), which must not replace the
// recorded progress.
func tracking(report progrep.ProgressReport) (observed.Tracking, bool) {
	if len(report.Operations) == 0 {
		return observed.Tracking{}, false
	}

	raw, err := json.Marshal(report)
	if err != nil {
		return observed.Tracking{}, false
	}

	t := observed.Tracking{Report: raw}

	for _, op := range report.Operations {
		if !isResourceOperation(op) {
			continue
		}

		if op.Status == progrep.OperationStatusCompleted {
			t.Completed++
		} else {
			t.Remaining++
		}
	}

	return t, true
}

// trackApply returns the progress callback of one apply: it keeps the operations for the apply's
// own timeout message (progress) and records the progress in the observations.
func (s *Service) trackApply(progress *applyProgress) func(name string, report progrep.ProgressReport) {
	return func(name string, report progrep.ProgressReport) {
		progress.record(report)

		if t, ok := tracking(report); ok {
			s.observations.UpdateTracking(name, t)
		}
	}
}

// trackUninstall is the progress callback of a release delete.
func (s *Service) trackUninstall(name string, report progrep.ProgressReport) {
	if t, ok := tracking(report); ok {
		s.observations.UpdateTracking(name, t)
	}
}

// applyProgress is what one apply in flight has reported, kept by the apply itself so the timeout
// message names exactly what it waited for. Written by nelm's report forwarder, read by the
// deadline timer.
type applyProgress struct {
	mu  sync.Mutex
	ops []progrep.Operation
}

// record keeps the operations of the latest non-empty report.
func (p *applyProgress) record(report progrep.ProgressReport) {
	if len(report.Operations) == 0 {
		return
	}

	ops := slices.Clone(report.Operations)

	p.mu.Lock()
	p.ops = ops
	p.mu.Unlock()
}

// last returns the operations of the latest non-empty report; nil when none came.
func (p *applyProgress) last() []progrep.Operation {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.ops
}
