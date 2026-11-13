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

package cache

import (
	"fmt"
	"sync"
	"time"
)

// passStuckAfter is how long a pass may run past its deadline: a Hugo build
// runs to its end once started. Longer, and a step ignores its context.
const passStuckAfter = 10 * time.Minute

// PassWatch tells the liveness probe of a pass of the pinned versions that
// runs well past its deadline: a step ignores its context, and only a
// restart gets the pinned versions going again. Every Pinner of the pod
// reports to it: the leader's and a follower's fallback may run at once for
// a while.
type PassWatch struct {
	now func() time.Time

	mu sync.Mutex
	// running is the deadline of the pass every Pinner is running.
	running map[*Pinner]time.Time
}

func NewPassWatch() *PassWatch {
	return &PassWatch{now: time.Now, running: make(map[*Pinner]time.Time)}
}

// Check returns an error while a pass is passStuckAfter past its deadline.
func (w *PassWatch) Check() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, deadline := range w.running {
		if over := w.now().Sub(deadline); over >= passStuckAfter {
			return fmt.Errorf("a pass of the pinned versions is %s past its deadline", over.Round(time.Second))
		}
	}

	return nil
}

func (w *PassWatch) start(p *Pinner, timeout time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.running[p] = w.now().Add(timeout)
}

func (w *PassWatch) done(p *Pinner) {
	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.running, p)
}
