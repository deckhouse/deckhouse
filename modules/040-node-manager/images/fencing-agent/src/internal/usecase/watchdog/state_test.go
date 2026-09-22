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

const testNodeGroup = "worker"

func newState() *SelfState {
	return NewSelfState("uid-worker-1", testNodeGroup, log.NewNop())
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
		NodeGroup:          testNodeGroup,
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
	state.Observe(domain.NodeSignals{UID: "uid-worker-1", NodeGroup: testNodeGroup})

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
		NodeGroup:      testNodeGroup,
		PlannedRemoval: true,
		RemovalReason:  domain.RemovalReasonAutoscaler,
	})

	// A later event without the taint (a stale watch update, a reverted taint).
	state.Observe(domain.NodeSignals{UID: "uid-worker-1", NodeGroup: testNodeGroup})

	snapshot := state.Snapshot()
	if !snapshot.PlannedRemoval || snapshot.RemovalReason != domain.RemovalReasonAutoscaler {
		t.Errorf("snapshot is %+v, want the planned removal to stay in effect", snapshot)
	}
}

func TestSelfStateTreatsDeletionAsTerminal(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{UID: "uid-worker-1", NodeGroup: testNodeGroup})
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

	state.Observe(domain.NodeSignals{UID: "uid-worker-1-recreated", NodeGroup: testNodeGroup})

	if !state.Snapshot().UIDMismatch {
		t.Fatal("a different uid on the own Node must be reported")
	}

	state.Observe(domain.NodeSignals{UID: "uid-worker-1", NodeGroup: testNodeGroup})

	if !state.Snapshot().UIDMismatch {
		t.Error("the mismatch must stay: the agent has to restart, not recover in place")
	}
}

func TestSelfStateIgnoresAnEmptyUID(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{NodeGroup: testNodeGroup})

	if state.Snapshot().UIDMismatch {
		t.Error("a missing uid is not a mismatch")
	}
}

// The group label is what scopes fencing to a NodeGroup: once it points
// elsewhere, this group's quorum and SLA profile no longer describe the Node.
func TestSelfStateFlagsANodeMovedToAnotherGroup(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{UID: "uid-worker-1", NodeGroup: "worker-2"})

	snapshot := state.Snapshot()
	if !snapshot.LeftNodeGroup || snapshot.NodeGroup != "worker-2" {
		t.Fatalf("snapshot is %+v, want the Node reported outside the group this agent serves", snapshot)
	}

	// Unlike a removal this is reversible, so it must not be sticky.
	state.Observe(domain.NodeSignals{UID: "uid-worker-1", NodeGroup: testNodeGroup})

	if state.Snapshot().LeftNodeGroup {
		t.Error("a relabel back into the group must put the Node under its fencing policy again")
	}
}

func TestSelfStateFlagsANodeWithoutTheGroupLabel(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{UID: "uid-worker-1"})

	if !state.Snapshot().LeftNodeGroup {
		t.Error("a Node with no group label is not a member of the group either")
	}
}

func TestSelfStateSnapshotDoesNotShareTheReasonSlice(t *testing.T) {
	state := newState()

	state.Observe(domain.NodeSignals{
		UID:                "uid-worker-1",
		NodeGroup:          testNodeGroup,
		Maintenance:        true,
		MaintenanceReasons: []string{domain.FencingDisableAnnotation},
	})

	snapshot := state.Snapshot()
	snapshot.MaintenanceReasons[0] = "mutated by the caller"

	if state.Snapshot().MaintenanceReasons[0] != domain.FencingDisableAnnotation {
		t.Error("a caller must not be able to mutate the stored maintenance reasons")
	}
}
