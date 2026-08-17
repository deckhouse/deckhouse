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
	"slices"
	"strings"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

// Snapshot is the watchdog-relevant state of the agent's own Node at one moment.
type Snapshot struct {
	// Observed is false until the own-Node cache has delivered the Node once.
	Observed bool
	// UIDMismatch means the Node was recreated under the same name: the agent
	// identity and the profile it started with no longer describe this machine.
	UIDMismatch bool
	// Maintenance is true while any maintenance annotation is present.
	Maintenance        bool
	MaintenanceReasons []string
	// PlannedRemoval is sticky: a removal is never revoked.
	PlannedRemoval bool
	RemovalReason  string
}

// SelfState keeps the last observed state of the agent's own Node. It is written
// by the Node informer and read by the feed loop, so every access is guarded.
//
// Two properties matter for safety: the state only ever changes on an actual
// event (a frozen cache therefore keeps maintenance in effect instead of
// silently clearing it), and a planned removal is sticky, because re-arming the
// watchdog on a Node that is being deleted would panic it mid-removal.
type SelfState struct {
	expectedUID string
	logger      *log.Logger

	mu    sync.Mutex
	state Snapshot
}

func NewSelfState(expectedUID string, logger *log.Logger) *SelfState {
	return &SelfState{expectedUID: expectedUID, logger: logger}
}

func (s *SelfState) Observe(signals domain.NodeSignals) {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := s.state

	s.state.Observed = true
	s.state.Maintenance = signals.Maintenance
	s.state.MaintenanceReasons = slices.Clone(signals.MaintenanceReasons)

	if signals.UID != "" && signals.UID != s.expectedUID {
		s.state.UIDMismatch = true
	}

	if signals.PlannedRemoval {
		s.state.PlannedRemoval = true
		s.state.RemovalReason = signals.RemovalReason
	}

	s.logTransitions(previous)
}

// Deleted is the terminal signal: the Node object is gone, so this machine is
// being removed from the cluster and must not be fenced any more.
func (s *SelfState) Deleted() {
	s.mu.Lock()
	defer s.mu.Unlock()

	previous := s.state

	s.state.Observed = true
	s.state.PlannedRemoval = true
	s.state.RemovalReason = domain.RemovalReasonDeleted

	s.logTransitions(previous)
}

func (s *SelfState) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.state
	state.MaintenanceReasons = slices.Clone(s.state.MaintenanceReasons)

	return state
}

// logTransitions reports changes only, so a per-tick read never floods the log.
func (s *SelfState) logTransitions(previous Snapshot) {
	if !previous.Observed {
		s.logger.Info("own node observed",
			"maintenance", s.state.Maintenance,
			"planned_removal", s.state.PlannedRemoval,
		)
	}

	if s.state.Maintenance != previous.Maintenance ||
		strings.Join(s.state.MaintenanceReasons, ",") != strings.Join(previous.MaintenanceReasons, ",") {
		s.logger.Info("own node maintenance state changed",
			"maintenance", s.state.Maintenance,
			"annotations", strings.Join(s.state.MaintenanceReasons, ","),
		)
	}

	if s.state.PlannedRemoval && !previous.PlannedRemoval {
		s.logger.Info("own node is being removed from the cluster", "reason", s.state.RemovalReason)
	}

	if s.state.UIDMismatch && !previous.UIDMismatch {
		s.logger.Error("own node was recreated with a different uid", "expected_uid", s.expectedUID)
	}
}
