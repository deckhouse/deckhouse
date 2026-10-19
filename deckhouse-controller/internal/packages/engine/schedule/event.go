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

import (
	"maps"
	"slices"
)

// Wake signals that events are pending; any number of computations leaves one signal. Stop
// closes it, which ends the consumer's loop.
func (s *Scheduler) Wake() <-chan struct{} {
	return s.wake
}

// Take returns the latest unread event of every node, sorted by name, and empties the slots.
func (s *Scheduler) Take() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	batch := make([]Event, 0, len(s.pending))
	for _, name := range slices.Sorted(maps.Keys(s.pending)) {
		batch = append(batch, s.pending[name])
	}

	clear(s.pending)

	return batch
}

// Stop ends the scheduler: no more computations, and Wake is closed. Idempotent.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}

	s.stopped = true
	close(s.wake)
}
