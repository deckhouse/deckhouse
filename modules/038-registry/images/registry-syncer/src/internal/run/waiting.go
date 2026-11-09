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

package run

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Waiting is how long this replica has waited, while nobody is full and nobody leads, for the replica
// the rule chose to take the lease, and which replicas it has passed over for not taking it.
//
// The rule chooses from reports, and a report says nothing about whether its replica still acts on it.
// On a DVP stand (H2b) the fullest follower's syncer was cut from the API server while its node stayed
// Ready: its entry stood as it was last written, with 277 digests and a copy error from the leader, and
// when the leader was powered off that frozen entry was the choice. The lost-node filter never met it,
// its node being Ready; the empty replica, which would have begun, waited for it in silence, and nobody
// led for four minutes: the API server let the frozen one through again after three, and it took the
// lease a minute after that. Passing over any replica
// with an error, as the rule once did, only moved the stall: it waits the same way on a frozen entry
// without one. A syncer that is wedged, or crash-looping on a Ready node, freezes its entry too.
//
// So the wait is bounded: a replica chosen for longer than For without taking the lease is passed over,
// and the rule chooses again among the rest. For is longer than the chosen replica needs to take the
// lease — to see the choice within a review, then to watch the lease unchanged for its duration as
// client-go does — so in the ordinary case it takes it first, and the others see it lead and stop
// waiting. See passOverAfter in cmd.
//
// Kept here, by this replica, rather than as a heartbeat in the status: every replica writes that one
// object and the controller watches it, so a needless write multiplies into a reconciliation of the
// whole layout (see report.Publisher); and without one a replica with nothing new to report writes
// nothing for hours, so its silence cannot be told from its being frozen.
//
// The wait for a replica is not restarted while the spell lasts — until this replica sees somebody lead,
// the lease held live. A choice that alternates, on a healthError that comes and goes, on a store that is
// writable one review and not the next, or on a node counted and then not, would otherwise restart it
// each time and the election would never begin; and a fresh elector waits a whole lease duration every
// time it is entered anyway.
//
// Nor is it restarted by a review that shows somebody full, or a leader by the roles, while nobody is
// seen leading. Such a review is as likely a degraded read as the end of the spell: a node or the pods
// that cannot be read keep the reports they would decide (see presentReplicas in cmd), and in H2b that
// is the dead leader's, full and Leader. Each failed read in the minutes after a master dies, when the
// API server is least steady, restarted every wait, and two minutes of clean reviews was the bound. A
// full replica that is really there takes the lease, and ends the spell that way.
//
// No replica is passed over while the lease cannot be read, which is no evidence about anybody, nor
// while it is held live: nobody can take it then, so a chosen replica not taking it is not a failure —
// the holder may be one whose report was dropped, or that has not reported yet. The wait already run
// stands, and resumes once the lease shows nobody leading.
//
// A replica that cannot serve, its own healthError set, passes nobody over: it waits on any replica that
// can, also on one it passed over while it could serve itself — see whyNotLead. It gains nothing by
// leading, and once it led the incumbent rule would keep it there after a healthy replica came back.
//
// The price: a less full replica, or an empty one, may begin and fill anew, and once it leads it keeps
// the lease while nobody is full, also after the fuller one comes back.
type Waiting struct {
	// For is how long a chosen replica may go without taking the lease before it is passed over. Zero
	// never passes anybody over: the rule as it is without this.
	For time.Duration

	// Log says when a replica is passed over; nil says nothing.
	Log *slog.Logger

	mu sync.Mutex
	// since is when each replica was first this one's choice in this spell.
	since map[string]time.Time
	// passed is the replicas passed over in this spell.
	passed map[string]bool
}

// WhyNotLead is the package's WhyNotLead, passing over the replicas this one has waited on for longer
// than For while nobody is full and nobody leads. now is when lease was read.
func (w *Waiting) WhyNotLead(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, currentSet string, lease Lease, now time.Time,
) string {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Each round passes over a replica not passed over before, and never this one, so there are at most
	// as many rounds as there are replicas.
	for {
		why, choice := whyNotLead(self, replicas, currentSet, lease, w.passed)
		switch {
		case choice == "":
			// Somebody leads, and the spell is over: the next one waits afresh. Somebody full, or a leader
			// by the roles, with nobody seen leading, may be a degraded read, and changes nothing.
			if lease.Known && lease.Live {
				w.since, w.passed = nil, nil
			}
			return why
		case choice == self:
			return why
		case !lease.Known || lease.Live:
			return w.waiting(why, choice)
		}

		if w.since == nil {
			w.since, w.passed = map[string]time.Time{}, map[string]bool{}
		}
		since, ok := w.since[choice]
		if !ok {
			since = now
			w.since[choice] = now
		}
		if w.For <= 0 || now.Sub(since) < w.For {
			return w.waiting(why, choice)
		}

		w.passed[choice] = true
		if w.Log != nil {
			w.Log.Info("passing over a replica that was chosen to begin and has not taken the lease",
				"replica", choice, "waited", now.Sub(since).Round(time.Second).String())
		}
	}
}

// waiting is the reason a replica gives while it waits on the one the rule chose. Fixed, with no
// countdown, so that it is logged once rather than at every review.
func (w *Waiting) waiting(why, choice string) string {
	if w.For <= 0 {
		return why
	}
	return fmt.Sprintf("%s; %s is passed over if it has not taken the lease %s after this replica first chose it",
		why, choice, w.For)
}
