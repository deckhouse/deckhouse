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

package decision

import (
	"sync"
	"time"
)

const (
	// restrictionRepeat is how often one rule may be reported while it keeps restricting.
	restrictionRepeat = time.Hour
	// restrictionForget is how long after the last report a rule is dropped from the memo, so a
	// later episode is reported as its own rather than swallowed by the first one.
	restrictionForget = 2 * restrictionRepeat
)

// RestrictionLog decides when the ordering guard is worth a log line.
//
// The guard is evaluated on every request, so a rule binding whose rule never arrives - a
// ClusterRoleBinding the controller left behind, a rule deleted while its bindings linger - would
// otherwise write a line on every request of every subject it names. That is the authorization
// path, and the line goes through the logger's process-wide lock, on every master.
//
// Reporting once per process is the other extreme, and it was the webhook's: the guard firing
// again next week is a different event, and a process that has already said its piece says
// nothing about it. So a rule is reported when it starts restricting, repeated at most hourly
// while it goes on, and forgotten once it stops - which makes the next episode a first one again.
//
// The memo is keyed on the rule alone, never on the subject. A rule binds a Group as readily as a
// User, so keying on the username would add an entry for every distinct authenticated user in
// that group - on a large OIDC cluster, during exactly the window where the guard is firing for
// all of them at once. The rule name is what an operator acts on; the caller puts the first
// username it saw into the message.
type RestrictionLog struct {
	mu sync.Mutex
	// reported maps a rule to when it was last reported.
	reported map[string]time.Time
	// lastSweep is when the whole memo was last cleaned of rules that stopped restricting.
	lastSweep time.Time
	// now is the clock, replaced in tests.
	now func() time.Time
}

// Allow reports whether the caller should write a line about this rule now.
//
// It reads and writes one key. Cleaning the whole memo on every call would be a walk of it per
// authorization request - the guard runs on all of them - and during a rollout on a cluster with
// thousands of rules that is thousands of iterations under this mutex, per request, to expire
// entries that are hours old. The sweep happens on its own schedule instead.
func (l *RestrictionLog) Allow(rule string) bool {
	if l == nil {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if l.now != nil {
		now = l.now()
	}

	if l.reported == nil {
		l.reported = make(map[string]time.Time)
	}
	l.sweepLocked(now)

	if at, seen := l.reported[rule]; seen {
		if now.Sub(at) >= restrictionForget {
			// This rule stopped restricting long enough ago that the next episode is its own.
			delete(l.reported, rule)
		} else if now.Sub(at) < restrictionRepeat {
			return false
		}
	}
	l.reported[rule] = now
	return true
}

// sweepLocked drops the rules that stopped restricting, at most once per restrictionForget. It
// bounds the memo - the keys are rule names, so it can never hold more than there are rules - and
// it is what makes a later episode of the same rule report itself as a new one. The caller holds
// the mutex.
func (l *RestrictionLog) sweepLocked(now time.Time) {
	if !l.lastSweep.IsZero() && now.Sub(l.lastSweep) < restrictionForget {
		return
	}
	l.lastSweep = now
	for name, at := range l.reported {
		if now.Sub(at) >= restrictionForget {
			delete(l.reported, name)
		}
	}
}
