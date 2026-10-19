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

// What Global tells the outside: its node and facts to the scheduler, and its Report.

package global

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
)

// Node returns the global node: always enabled, order 0, no subscriptions, whatever the global
// module declares. revision is the version Global publishes its facts with. The barrier the other
// nodes wait behind is enforced by the scheduler by name, not by the order.
func Node(version *semver.Version, revision string) schedule.Node {
	return schedule.Node{
		Name:     schedule.GlobalNode,
		Version:  version,
		Revision: revision,
		Constraints: schedule.Constraints{
			Order: 0,
			Floor: rule.Static(rule.Enable),
		},
	}
}

// ownInputs is Facts.OwnInputs: the hash of version, accepted settings and values generation.
func (g *Global) ownInputs(in input) string {
	sum := ""
	if a := g.state.settings.Accepted; a != nil {
		sum = a.Checksum
	}

	h := sha256.Sum256(fmt.Appendf(nil, "version=%s\nsettings=%s\nvalues=%d\n", g.version, sum, in.valuesGen))

	return hex.EncodeToString(h[:])
}

func (g *Global) publishFacts(ownInputs string) {
	g.deps.Scheduler.Publish(schedule.GlobalNode, schedule.Facts{
		Version:   g.version,
		OwnInputs: ownInputs,
		Applied:   g.state.applied,
		Output:    g.output,
	})
}

// reportSuccess reports a successful run and its settings checkpoint; only a success replaces a
// reported failure.
func (g *Global) reportSuccess(a *configstate.Accepted) {
	g.reporter.UpdateCurrent(func(r *model.CurrentReport) {
		r.HooksProcessed = model.TrueCondition()
		r.ManifestsApplied = model.TrueCondition()

		if a != nil {
			a.Checkpoint(r)
		}
	})
}

// reportFailure reports a failed operation on the condition its step names, with the step's
// reason.
func (g *Global) reportFailure(res operation.Result) {
	cond := model.FalseCondition(res.Failed.Reason, fmt.Sprintf("%s: %v", res.Failed.Name, res.Err))

	g.reporter.UpdateCurrent(func(r *model.CurrentReport) {
		switch res.Failed.Condition {
		case operation.HooksProcessed:
			r.HooksProcessed = cond
		case operation.ManifestsApplied:
			r.ManifestsApplied = cond
		}
	})
}
