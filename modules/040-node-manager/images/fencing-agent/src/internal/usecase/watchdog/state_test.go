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

package watchdog

import (
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

func newState() *SelfState {
	return NewSelfState("uid-worker-1", log.NewNop())
}

func TestSelfStateStartsUnobserved(t *testing.T) {
	state := newState().Snapshot()

	if state.Observed {
		t.Error("state must not claim the own Node is known before the first event")
	}
}

func TestSelfStateReportsMaintenance(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{
		UID:                "uid-worker-1",
		Maintenance:        true,
		MaintenanceReasons: []string{domain.DisruptionApprovedAnnotation},
	})

	snapshot := state.Snapshot()
	if !snapshot.Observed || !snapshot.Maintenance {
		t.Fatalf("snapshot is %+v, want an observed Node in maintenance", snapshot)
	}

	if len(snapshot.MaintenanceReasons) != 1 || snapshot.MaintenanceReasons[0] != domain.DisruptionApprovedAnnotation {
		t.Errorf("maintenance reasons are %v, want the annotation that caused it", snapshot.MaintenanceReasons)
	}

	// Removing the annotation must bring fencing back.
	state.Observe(domain.NodeSignals{UID: "uid-worker-1"})

	if state.Snapshot().Maintenance {
		t.Error("maintenance must clear once the annotations are gone")
	}
}

// A removal is never revoked: re-arming a Node that is being deleted would panic
// it mid-removal.
func TestSelfStateKeepsPlannedRemovalSticky(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{
		UID:            "uid-worker-1",
		PlannedRemoval: true,
		RemovalReason:  domain.RemovalReasonAutoscaler,
	})

	// A later event without the taint (a stale watch update, a reverted taint).
	state.Observe(domain.NodeSignals{UID: "uid-worker-1"})

	snapshot := state.Snapshot()
	if !snapshot.PlannedRemoval || snapshot.RemovalReason != domain.RemovalReasonAutoscaler {
		t.Errorf("snapshot is %+v, want the planned removal to stay in effect", snapshot)
	}
}

func TestSelfStateTreatsDeletionAsTerminal(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{UID: "uid-worker-1"})
	state.Deleted()

	snapshot := state.Snapshot()
	if !snapshot.PlannedRemoval || snapshot.RemovalReason != domain.RemovalReasonDeleted {
		t.Errorf("snapshot is %+v, want a terminal removal after the Node object is gone", snapshot)
	}
}

// A Node recreated under the same name makes the identity and profile stale, and
// only a restart refreshes them.
func TestSelfStateDetectsAndKeepsUIDMismatch(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{UID: "uid-worker-1-recreated"})

	if !state.Snapshot().UIDMismatch {
		t.Fatal("a different uid on the own Node must be reported")
	}

	state.Observe(domain.NodeSignals{UID: "uid-worker-1"})

	if !state.Snapshot().UIDMismatch {
		t.Error("the mismatch must stay: the agent has to restart, not recover in place")
	}
}

func TestSelfStateIgnoresAnEmptyUID(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{})

	if state.Snapshot().UIDMismatch {
		t.Error("a missing uid is not a mismatch")
	}
}

func TestSelfStateSnapshotDoesNotShareTheReasonSlice(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{
		UID:                "uid-worker-1",
		Maintenance:        true,
		MaintenanceReasons: []string{domain.FencingDisableAnnotation},
	})

	snapshot := state.Snapshot()
	snapshot.MaintenanceReasons[0] = "mutated by the caller"

	if state.Snapshot().MaintenanceReasons[0] != domain.FencingDisableAnnotation {
		t.Error("a caller must not be able to mutate the stored maintenance reasons")
	}
}
