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

package fencingfailednodestate

import (
	"strings"
	"testing"
	"time"

	"fencing-controller/internal/common"
)

// TestEventsTellTheChangeOfThePhase covers the events of the phase. The first
// write of Suspected is not a change, because an empty phase reads as
// Suspected.
func TestEventsTellTheChangeOfThePhase(t *testing.T) {
	h := newHarness(t, failedState())
	h.recorder.IncludeObject = true

	h.reconcileAt(observedAt)

	if published := h.events(); len(published) != 0 {
		t.Errorf("the first write of Suspected published %v, want no events", published)
	}

	h.writeHeartbeat(observedAt)
	h.reconcileAt(observedAt.Add(time.Second))

	assertAnnounced(t, h.events(), common.ReasonPhaseChanged, "from Suspected to FallbackAlive")

	h.reconcileAt(observedAt.Add(2 * time.Second))

	if published := h.events(); len(published) != 0 {
		t.Errorf("a reconcile without a change of the phase published %v, want no events", published)
	}
}

// TestEventsTellTheProofOfTheFailure covers the event of the phase Proven. It
// tells what proves the failure, the attestation of a witness or the silence
// of the node, and the delay that the controller waited.
func TestEventsTellTheProofOfTheFailure(t *testing.T) {
	for name, tc := range map[string]struct {
		attested bool
		after    time.Duration
		want     []string
	}{
		"attestation": {
			attested: true,
			after:    attestedAfter,
			want:     []string{"from Suspected to Proven", "witness worker-1 attested", "5s"},
		},
		"silence": {
			after: silentAfter,
			want:  []string{"from Suspected to Proven", "node is silent", "10.55s"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			if tc.attested {
				attest(incident, observedAt)
			}

			// The volume keeps the incident in Evicting, so the events of the
			// later phases do not come.
			h := newHarness(t, incident, liveNode(), volumeAttachment("csi-own", nodeName))
			h.reconcileAt(observedAt)
			h.events()

			h.recorder.IncludeObject = true
			h.reconcileAt(observedAt.Add(tc.after))

			var proven []string

			for _, event := range h.events() {
				if strings.Contains(event, "to Proven") {
					proven = append(proven, event)
				}
			}

			if len(proven) == 0 {
				t.Fatalf("no event of the phase Proven")
			}

			for _, want := range tc.want[1:] {
				assertAnnounced(t, proven, common.ReasonPhaseChanged, want)
			}
		})
	}
}

// TestEventsTellTheTaint covers the events of the out-of-service taint on the
// incident and on its Node.
func TestEventsTellTheTaint(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)
	h.events()

	h.recorder.IncludeObject = true
	h.reconcileAt(observedAt.Add(attestedAfter))
	h.assertTainted()

	var taint []string

	for _, event := range h.events() {
		if strings.Contains(event, common.ReasonOutOfServiceTaintSet) {
			taint = append(taint, event)
		}
	}

	assertAnnounced(t, taint, common.ReasonOutOfServiceTaintSet, nodeName)

	h.reconciler.announceTaintRemoved(h.get())

	assertAnnounced(t, h.events(), common.ReasonOutOfServiceTaintRemoved, nodeName)
}

// assertAnnounced checks that the events are one event on the incident and one
// on its Node, and that each message starts with the prefix [fencing]. The
// recorder must include the object in the events.
func assertAnnounced(t *testing.T, events []string, reason, wantInMessage string) {
	t.Helper()

	if len(events) != 2 {
		t.Fatalf("published %v, want one event on the incident and one on the node", events)
	}

	onNode := 0

	for _, event := range events {
		assertEvent(t, event, "", reason, wantInMessage)

		if strings.Contains(event, "involvedObject{kind=Node,") {
			onNode++
		}
	}

	if onNode != 1 {
		t.Errorf("published %v, want one event on the node", events)
	}
}
