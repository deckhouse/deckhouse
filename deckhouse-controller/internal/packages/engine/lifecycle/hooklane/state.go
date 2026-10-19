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

// What the lanes publish for the debug API: read under the lane mutex, never waiting for a hook.

package hooklane

import (
	"slices"
	"strings"
	"time"
)

// dumpItems bounds the waiting items a dump lists; the count and the per-hook breakdown are whole.
const dumpItems = 10

// Dump is the state of the lanes, by package name and queue.
type Dump struct {
	Lanes map[string]map[string]LaneDump `json:"lanes"`
}

// LaneDump is the state of one lane.
type LaneDump struct {
	State    State        `json:"state"`
	InFlight *InFlight    `json:"inFlight,omitempty"`
	Waiting  Waiting      `json:"waiting"`
	Failure  *FailureDump `json:"failure,omitempty"`
	Stats    Stats        `json:"stats"`
}

// InFlight is the item the lane runs now.
type InFlight struct {
	Hook       string   `json:"hook"`
	Kind       Kind     `json:"kind"`
	Bindings   []string `json:"bindings"`
	Batch      int      `json:"batch"` // Batch is the number of binding contexts of the run.
	StartedAt  string   `json:"startedAt"`
	RunningFor string   `json:"runningFor"`
}

// Waiting are the items behind the one in flight (the failed head included while it waits).
type Waiting struct {
	Count     int            `json:"count"`
	ByHook    map[string]int `json:"byHook,omitempty"`
	OldestAge string         `json:"oldestAge,omitempty"`
	Items     []WaitingItem  `json:"items,omitempty"` // Items are the first dumpItems.
}

// WaitingItem is one waiting item.
type WaitingItem struct {
	Hook     string `json:"hook"`
	Kind     Kind   `json:"kind"`
	Contexts int    `json:"contexts"`
	Age      string `json:"age"`
}

// FailureDump is the head's failure while it waits for its retry.
type FailureDump struct {
	Hook      string `json:"hook"`
	Error     string `json:"error"`
	Attempts  int    `json:"attempts"`
	NextRetry string `json:"nextRetry"` // NextRetry is the time until the retry; negative when due.
}

// Stats count since the lane started.
type Stats struct {
	Runs        int       `json:"runs"`
	Failures    int       `json:"failures"`
	LastSuccess time.Time `json:"lastSuccess,omitzero"`
	// AllowedFailures are failures of items that allow them (allowFailure): recorded here, then
	// the item left the lane.
	AllowedFailures    int    `json:"allowedFailures,omitempty"`
	LastAllowedFailure string `json:"lastAllowedFailure,omitempty"`
}

// LaneSummary is one lane in short, for the package dump.
type LaneSummary struct {
	Queue   string `json:"queue"`
	State   State  `json:"state"`
	Waiting int    `json:"waiting"`
	Failure string `json:"failure,omitempty"` // Failure is the failed head's error.
}

// Summary returns name's lanes in short, by queue; nil while its hooks are not started.
func (r *Router) Summary(name string) []LaneSummary {
	d := r.Dump(name).Lanes[name]
	if len(d) == 0 {
		return nil
	}

	res := make([]LaneSummary, 0, len(d))
	for queue, ld := range d {
		s := LaneSummary{Queue: queue, State: ld.State, Waiting: ld.Waiting.Count}
		if ld.Failure != nil {
			s.Failure = ld.Failure.Error
		}

		res = append(res, s)
	}

	slices.SortFunc(res, func(a, b LaneSummary) int { return strings.Compare(a.Queue, b.Queue) })

	return res
}

// dump returns the state of every lane of l, by queue.
func (l *Lanes) dump() map[string]LaneDump {
	res := make(map[string]LaneDump, len(l.lanes))
	for queue, ln := range l.lanes {
		res[queue] = ln.dump()
	}

	return res
}

func (ln *lane) dump() LaneDump {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	now := ln.lanes.router.clock.Now()
	d := LaneDump{State: ln.state, Stats: ln.stats}

	waiting := ln.items
	if !ln.startedAt.IsZero() && len(waiting) > 0 {
		head := waiting[0]
		d.InFlight = &InFlight{
			Hook:       head.hook,
			Kind:       head.kind,
			Bindings:   head.bindings(),
			Batch:      len(head.contexts),
			StartedAt:  ln.startedAt.Format(time.RFC3339),
			RunningFor: now.Sub(ln.startedAt).String(),
		}
		waiting = waiting[1:]
	}

	d.Waiting.Count = len(waiting)
	if len(waiting) > 0 {
		d.Waiting.ByHook = make(map[string]int)
		d.Waiting.OldestAge = now.Sub(waiting[0].delivered).String()
	}

	for i, it := range waiting {
		d.Waiting.ByHook[it.hook]++

		if i < dumpItems {
			d.Waiting.Items = append(d.Waiting.Items, WaitingItem{
				Hook: it.hook, Kind: it.kind, Contexts: len(it.contexts), Age: now.Sub(it.delivered).String(),
			})
		}
	}

	if ln.failure.attempts > 0 && len(ln.items) > 0 {
		d.Failure = &FailureDump{
			Hook:      ln.items[0].hook,
			Error:     ln.failure.err,
			Attempts:  ln.failure.attempts,
			NextRetry: ln.backoff.Until().Sub(now).String(),
		}
	}

	return d
}
