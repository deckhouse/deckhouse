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

package health

import (
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/health/monitor"
)

// Health is the reduced workload-health snapshot for one package.
// State is the machine-readable enum; consumers map it into a K8s
// condition's reason field (the enum values are already valid reason
// strings). Message carries the dominant-workload detail
// ("Kind/Name: cause") and flows into the condition's message field.
// StateScaled is the all-healthy case and carries no Message.
type Health struct {
	State   State
	Message string
}

type State string

// State values returned to callbacks. The zero value of State is the
// empty string, not StateUnknown — code that switches on State should
// rely on the named constants below, not on default-value behavior.
const (
	// StateUnknown means the package has no known workloads.
	StateUnknown State = "Unknown"
	// StateReconciling means at least one workload is rolling out
	// (generation drift, replicas catching up, revision swap) and none
	// have failed. The controller is still doing work; ask again later.
	StateReconciling State = "Reconciling"
	// StateScaled means every workload is at the desired replica count
	// and the controller has nothing left to reconcile.
	StateScaled State = "Scaled"
	// StateDegraded means at least one workload is Failed, or has been
	// observed Terminating while still in cache.
	StateDegraded State = "Degraded"
)

// Event is the payload delivered to Callback on every package-health
// transition. It carries only what the consumer needs: the new State
// and a short human-readable Message naming the dominant workload (for
// example "Deployment/api: ProgressDeadlineExceeded"). The package name
// is the first argument of Callback and is intentionally not duplicated
// here. Because Callback is invoked only on transitions, State always
// differs from the value reported in the previous Event for the same
// package.
type Event struct {
	Health Health
}

// reducePackage collapses per-workload statuses into a single State and
// produces a Message that names the dominant workload.
//
// State mapping (in precedence order, first match wins):
//
//	any Failed or Terminating workload → StateDegraded
//	any InProgress workload            → StateReconciling
//	all Current workloads              → StateScaled
//	empty input                        → StateUnknown
//
// The Message picks the dominant workload at the workload level:
// Failed > Terminating > InProgress. StateScaled carries no Message —
// "everything is fine" has no dominant workload worth naming.
func reducePackage(workloads []monitor.WorkloadStatus) Health {
	if len(workloads) == 0 {
		return Health{State: StateUnknown}
	}

	var (
		failed, terminating, progress *monitor.WorkloadStatus
	)
	for i := range workloads {
		w := &workloads[i]
		switch w.Health {
		case monitor.Failed:
			if failed == nil {
				failed = w
			}
		case monitor.Terminating:
			if terminating == nil {
				terminating = w
			}
		case monitor.InProgress:
			if progress == nil {
				progress = w
			}
		}
	}

	switch {
	case failed != nil:
		return Health{State: StateDegraded, Message: formatMessage(failed)}
	case terminating != nil:
		return Health{State: StateDegraded, Message: formatMessage(terminating)}
	case progress != nil:
		return Health{State: StateReconciling, Message: formatMessage(progress)}
	default:
		return Health{State: StateScaled}
	}
}

// formatMessage builds a "Kind/Name: cause" or "Kind/Name" string from a
// single workload status, suitable for a K8s condition's message field.
// Returns the empty string for a nil input so callers in the all-Current
// branch don't need to handle "no dominant workload" specially.
func formatMessage(w *monitor.WorkloadStatus) string {
	if w == nil {
		return ""
	}
	if w.Cause == "" {
		return fmt.Sprintf("%s/%s", w.Kind, w.Name)
	}
	return fmt.Sprintf("%s/%s: %s", w.Kind, w.Name, w.Cause)
}
