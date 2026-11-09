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

package proxy

import (
	"strings"
	"sync"
	"time"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

const (
	// DemotionCooldown is how long a target that has just failed is tried last.
	//
	// The same as the interval at which the layout is re-read. A leader that has gone away
	// is replaced in the layout, not here, and this only has to cover the time until the
	// next layout can say so — without every pull on the node paying a full timeout to the
	// dead address in the meantime.
	DemotionCooldown = 30 * time.Second

	// MaxDemotionCooldown is how far repeated failures stretch the cooldown.
	//
	// Bounded, because a target that stays demoted is still asked first again once the
	// window closes, and that first request is the only way the agent learns it is back.
	// Five minutes is the longest a recovered leader is passed over in favour of the upstream.
	MaxDemotionCooldown = 5 * time.Minute
)

// demotions remembers which targets have failed recently, so that the next request does not
// begin with them.
//
// A demoted target is moved to the end of the list and never dropped from it. Dropping it would
// turn a slow judgement into an outage: when everything ahead of it fails as well — an air-gapped
// cluster has nothing ahead of the leader at all — it is still the one that might answer. What
// demotion saves is the wait: a leader whose node went silent holds every request for the full
// dial or response timeout, and without this each pull on the node pays it again until the
// layout changes.
//
// Keyed by scheme and host rather than by the target's name. Names are shared — every target of
// a route is called after the route, and the leader and the other replicas are all "Storage" —
// and a key that merged them would demote the replicas that work along with the one that does
// not.
//
// There is no separate half-open state. Once its window closes a target is back in its place,
// the first request tries it, and that request's outcome either clears it or demotes it again
// for twice as long.
type demotions struct {
	// now is the clock, replaceable so the windows can be tested without waiting them out.
	now func() time.Time

	mu      sync.Mutex
	entries map[string]demotion
}

// demotion is one target's record: when its window closes and how long the last one was.
type demotion struct {
	until    time.Time
	cooldown time.Duration
}

func (d *demotions) clock() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}

// fail records a failed attempt at a target, one that began at started.
//
// Only an attempt that began after the window closed — one that tried the target again after it
// had been passed over — says that it is still down, and only that one doubles the window.
// Requests run in parallel — a pull asks for its layers together, a node for its images — and
// all of those in flight to a dead leader fail together; counting each as a repeat would send it
// to the maximum on the first outage. Nor do they all fail at the same moment: a registry whose
// connections stay up while it never answers releases them one by one over the response timeout,
// which is longer than the shortest window, so what tells a repeat apart is when the attempt
// began, not when it failed.
//
// A failure inside the open window changes nothing. One that began before the window closed but
// arrives after it opens the window again for as long as the last one, without doubling: it is
// still news that the target does not answer, just not news of a second outage.
//
// A record whose window closed long ago is a past outage, not this one, and the next failure
// starts from the shortest window again. Clearing such records here is also what keeps the set
// bounded by the addresses currently failing rather than every address that ever has.
func (d *demotions) fail(target *Target, started time.Time) {
	key := demotionKey(target)
	now := d.clock()

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.entries == nil {
		d.entries = map[string]demotion{}
	}
	for other, entry := range d.entries {
		if now.Sub(entry.until) > MaxDemotionCooldown {
			delete(d.entries, other)
		}
	}

	entry, known := d.entries[key]
	switch {
	case !known:
		entry.cooldown = DemotionCooldown
	case now.Before(entry.until):
		return
	case started.Before(entry.until):
		// The same cooldown again: begun before the window closed, this attempt says nothing
		// about whether trying the target again would have helped.
	default:
		entry.cooldown = min(2*entry.cooldown, MaxDemotionCooldown)
	}
	entry.until = now.Add(entry.cooldown)
	d.entries[key] = entry
}

// succeed records that a target answered, which ends whatever it was demoted for.
func (d *demotions) succeed(target *Target) {
	key := demotionKey(target)

	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.entries, key)
}

// demoted reports whether a target is inside its window.
func (d *demotions) demoted(target *Target) bool {
	key := demotionKey(target)
	now := d.clock()

	d.mu.Lock()
	defer d.mu.Unlock()
	entry, known := d.entries[key]
	return known && now.Before(entry.until)
}

// order moves the demoted targets to the end, keeping the order the layout gave everything
// else.
//
// Stable on purpose: the layout's order is a decision of the controller — the leader before the
// upstream — and demotion only says "not first", never "first instead". Two demoted targets keep
// their relative order for the same reason.
//
// Returns, for each position in the new order, whether the target there was moved ahead of one
// the layout put before it. Its answer then stands in for a target that was skipped rather than
// asked, and the caller needs to know which answers those are — see Server.forward.
func (d *demotions) order(targets []Target) []bool {
	promoted := make([]bool, len(targets))
	if len(targets) < 2 {
		return promoted
	}
	kept := make([]Target, 0, len(targets))
	var moved []Target
	for i := range targets {
		if d.demoted(&targets[i]) {
			moved = append(moved, targets[i])
			continue
		}
		promoted[len(kept)] = len(moved) > 0
		kept = append(kept, targets[i])
	}
	copy(targets, kept)
	copy(targets[len(kept):], moved)
	return promoted
}

// demotionKey names the address a target is reached at. Case-folded, as hosts are compared
// everywhere else, so that one registry written two ways is one record.
func demotionKey(target *Target) string {
	scheme := target.Scheme
	if scheme == "" {
		scheme = registryv1alpha1.SchemeHTTPS
	}
	return scheme.Lower() + "://" + strings.ToLower(strings.TrimSpace(target.Host))
}
