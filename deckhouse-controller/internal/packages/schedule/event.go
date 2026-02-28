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

package schedule

// EventKind identifies the type of lifecycle event emitted by the Scheduler.
type EventKind int

const (
	// EventSchedule is emitted when a node transitions from idle to scheduled.
	EventSchedule EventKind = iota
	// EventDisable is emitted when a node loses eligibility during a scheduling pass.
	EventDisable
	// EventGlobalDone is emitted when the global sentinel node completes,
	// carrying the list of currently enabled package names.
	EventGlobalDone
)

// Event represents a single lifecycle transition in the scheduling graph.
// Name identifies the affected node; Enabled is populated only for [EventGlobalDone].
type Event struct {
	Name    string
	Kind    EventKind
	Enabled []string
}

// Ch returns a read-only channel that emits [Event] values as the graph
// evolves. The caller must drain this channel to avoid blocking the scheduler.
func (s *Scheduler) Ch() <-chan Event {
	return s.eventCh
}

// Stop closes the event channel. Call this when the scheduler is no longer needed.
func (s *Scheduler) Stop() {
	close(s.eventCh)
}
