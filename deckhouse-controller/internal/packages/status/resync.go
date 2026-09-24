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

package status

import "time"

// defaultResyncPeriod is how often every registered package is re-enqueued.
const defaultResyncPeriod = 10 * time.Minute

// StartResync launches the periodic re-enqueue of every registered package: the
// level-triggered backstop that republishes a status whose notification was
// lost. Called once, paired with Shutdown.
func (s *Service) StartResync() {
	go s.runResync()
}

// runResync re-enqueues every registered package on every tick until stopResync
// closes resyncStop. One goroutine serves every package; the workqueues
// coalesce the keys, so a tick costs at most one item per package.
func (s *Service) runResync() {
	defer close(s.resyncDone)

	ticker := time.NewTicker(defaultResyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-s.resyncStop:
			return
		case <-ticker.C:
			s.enqueueAll()
		}
	}
}

// stopResync ends the resync goroutine and waits for it to exit, so no key can
// reach a queue after the caller shuts it down.
func (s *Service) stopResync() {
	close(s.resyncStop)
	<-s.resyncDone
}

// enqueueAll adds every registered package to the queue that owns it. The key
// snapshot is taken under s.mu and the lock is dropped before the first Add, so
// a queue can never be reached while a status mutator is blocked on the lock.
// Packages buffered in pendingHealth are skipped: they carry no status yet, and
// NewStatus enqueues them when it drains the buffer.
func (s *Service) enqueueAll() {
	s.mu.Lock()
	names := make([]string, 0, len(s.statuses))
	for name := range s.statuses {
		names = append(names, name)
	}
	s.mu.Unlock()

	for _, name := range names {
		s.queueFor(name).Add(name)
	}
}
