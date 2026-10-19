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

package current

import (
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
)

// reportSuccess reports a successful run or disable and its settings checkpoint (nil: none; kept
// only by a package that has run, hasRun) in one write: every condition its steps cover is True,
// and Installed follows the kind. Only a success replaces a reported failure.
func (r *Reconciler) reportSuccess(kind operation.Kind, steps []operation.Step, cp *configstate.Accepted) {
	installed := model.TrueCondition()
	if kind == operation.Disable {
		installed = model.FalseCondition(model.ReasonDisabled, "")
	}

	r.reporter.UpdateCurrent(func(cr *model.CurrentReport) {
		*cr = operationConditions(*cr, r.state.version, steps, nil, model.Condition{})
		cr.Installed = installed

		if cp != nil && r.hasRun(cr) {
			cp.Checkpoint(cr)
		}
	})
}

// hasRun reports whether the package has run, so a checkpoint may be written: the checkpoint is
// status.lastAppliedConfiguration, by which the webhooks freeze immutable fields. A disable or an
// accept while disabled writes one only then; one of a package that never ran is never written,
// and such a package restores without settings, waiting for its first Apply as a fresh install.
// A disabled package that has run keeps its checkpoint current: its enabled script reads the
// restored settings. cr carries the checkpoint of an earlier revision that ran.
func (r *Reconciler) hasRun(cr *model.CurrentReport) bool {
	return r.state.ran || cr.AppliedChecksum != ""
}

// reportApplying marks the release apply in flight (see apply). Called from the worker by the
// release service. On a new revision HooksProcessed is the previous revision's, so it is reset;
// Installed never is: the release stays installed across an update. A cancelled run leaves the
// mark until the next operation reports.
func (r *Reconciler) reportApplying() {
	r.reporter.UpdateCurrent(func(cr *model.CurrentReport) {
		if cr.Version != r.state.version {
			cr.HooksProcessed = model.Condition{}
		}

		cr.Version = r.state.version
		cr.ManifestsApplied = model.FalseCondition(model.ReasonApplyingManifests, "")
	})
}

// reasoned is a failure that names its own reason (the release service: RenderFailed, ...).
type reasoned interface {
	Reason() string
}

func failureReason(step *operation.Step, err error) string {
	var r reasoned
	if errors.As(err, &r) && r.Reason() != "" {
		return r.Reason()
	}

	return step.Reason
}

// reportFailure reports a failed operation over its steps (operationConditions); Installed is
// left as it is: a failure never uninstalls.
func (r *Reconciler) reportFailure(res operation.Result, steps []operation.Step) {
	failure := model.FalseCondition(failureReason(res.Failed, res.Err), fmt.Sprintf("%s: %v", res.Failed.Name, res.Err))

	r.reporter.UpdateCurrent(func(cr *model.CurrentReport) {
		*cr = operationConditions(*cr, r.state.version, steps, res.Failed, failure)
	})
}

// operationConditions writes the result of an operation of revision over steps whole, so no
// condition keeps a value another operation wrote that this one contradicts. Per condition, as the
// steps of one condition interleave (a run's hooks come before and after its apply): the failed
// step's condition is failure; a condition with a step completed before the failure, and none
// failed, is True; a condition none of whose steps was reached keeps its value within the
// revision and is Unknown on a new one (it is the previous revision's). failed is nil on success.
func operationConditions(prev model.CurrentReport, revision string, steps []operation.Step, failed *operation.Step, failure model.Condition) model.CurrentReport {
	completed := make(map[operation.Condition]bool, 2)

	for i := range steps {
		if failed != nil && steps[i].Name == failed.Name {
			break
		}

		completed[steps[i].Condition] = true
	}

	next := prev
	next.Version = revision

	decide := func(cond operation.Condition, prevValue model.Condition) model.Condition {
		switch {
		case failed != nil && failed.Condition == cond:
			return failure
		case completed[cond]:
			return model.TrueCondition()
		case prev.Version == revision:
			return prevValue
		default:
			return model.Condition{}
		}
	}

	next.HooksProcessed = decide(operation.HooksProcessed, prev.HooksProcessed)
	next.ManifestsApplied = decide(operation.ManifestsApplied, prev.ManifestsApplied)

	return next
}
