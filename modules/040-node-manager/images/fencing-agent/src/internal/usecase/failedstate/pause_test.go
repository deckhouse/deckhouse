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

package failedstate

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/logtest"
)

func removeRecord(store *stubStore, name string) {
	store.states = slices.DeleteFunc(store.states, func(state v1alpha1.FencingFailedNodeState) bool {
		return state.Name == name
	})
}

func storedDetectedAt(store *stubStore, name string) (detectedAt time.Time, ok bool) {
	for _, state := range store.states {
		if state.Name == name && state.Status.Failed != nil {
			return state.Status.Failed.DetectedAt.Time, true
		}
	}

	return time.Time{}, false
}

func markStamps(store *stubStore) []time.Time {
	stamps := make([]time.Time, 0, len(store.marks))

	for _, mark := range store.marks {
		stamps = append(stamps, mark.DetectedAt.Time)
	}

	return stamps
}

func sameInstants(got, want []time.Time) bool {
	return slices.EqualFunc(got, want, time.Time.Equal)
}

func ownRecord(self, detectedBy string, ct time.Time) v1alpha1.FencingFailedNodeState {
	return v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: self, UID: types.UID("cr-" + self), CreationTimestamp: metav1.NewTime(ct)},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{
				DetectedAt: metav1.NewMicroTime(ct),
				DetectedBy: detectedBy,
				Reason:     v1alpha1.FailedReasonMemberlistDead,
			},
		},
	}
}

func (h *harness) pauseByOwnRecord(t *testing.T, self, detectedBy string, failed ...string) {
	t.Helper()

	h.store.states = append(h.store.states, ownRecord(self, detectedBy, h.clock.now))
	h.failPeer(t.Context(), failed...)
	h.clock.advance(takeoverDelay)
	h.failPeer(t.Context(), failed...)

	if !h.writer.paused {
		t.Fatalf("the writer is not paused, want the own failed record to pause it after %s", takeoverDelay)
	}
}

func deleteCalls(store *stubStore) []string {
	return slices.DeleteFunc(slices.Clone(store.calls), func(call string) bool {
		return !strings.HasPrefix(call, "delete:")
	})
}

func TestVanishedRecordIsRecreatedWithAFreshDetectedAt(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	firstFailure := h.clock.now
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Second)
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Minute - time.Second)
	vanished := h.clock.now
	removeRecord(store, failed)
	store.calls = nil
	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"create:" + failed, "mark:" + failed}) {
		t.Fatalf("calls = %v, want the record re-created in the pass that saw it vanish", store.calls)
	}

	if got, want := markStamps(store), []time.Time{firstFailure, vanished}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: the re-created record carries the moment it is written", got, want)
	}
}

func TestRecordTheCacheNeverShowedKeepsTheFirstDetectedAt(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	store.invisible[failed] = true
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	firstFailure := h.clock.now
	h.failPeer(t.Context(), failed)

	for range 2 {
		h.clock.advance(time.Second)
		h.failPeer(t.Context(), failed)
	}

	if got, ok := storedDetectedAt(store, failed); !ok || !got.Equal(firstFailure) {
		t.Fatalf("stored detectedAt = %s (on record: %t), want %s: an informer lag must not move detectedAt",
			got, ok, firstFailure)
	}

	if got, want := store.recorded[failed].DetectedAt.Time, firstFailure; !got.Equal(want) {
		t.Fatalf("recorded detectedAt = %s, want %s from the only write that set the verdict", got, want)
	}

	created := slices.DeleteFunc(slices.Clone(h.events.normal), func(reason string) bool { return reason != reasonStateCreated })
	if len(created) != 1 {
		t.Errorf("normal events = %v, want one %s: the lag repeats writes, not the incident", h.events.normal, reasonStateCreated)
	}

	delete(store.invisible, failed)
	store.calls = nil

	for range 2 {
		h.clock.advance(time.Second)
		h.failPeer(t.Context(), failed)
	}

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none once the cache shows the record", store.calls)
	}

	if got, ok := storedDetectedAt(store, failed); !ok || !got.Equal(firstFailure) {
		t.Errorf("stored detectedAt = %s (on record: %t), want %s once the cache shows the record", got, ok, firstFailure)
	}
}

func TestRecordRemovedBeforeTheCacheShowedItIsRecreatedWithAFreshDetectedAt(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	store.uniqueUIDs = true
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	firstFailure := h.clock.now
	h.failPeer(t.Context(), failed)

	if got, ok := storedDetectedAt(store, failed); !ok || !got.Equal(firstFailure) {
		t.Fatalf("stored detectedAt = %s (on record: %t), want the first sighting %s", got, ok, firstFailure)
	}

	for range 3 {
		removeRecord(store, failed)
		h.clock.advance(time.Second)
		recreated := h.clock.now
		store.calls = nil
		h.failPeer(t.Context(), failed)

		if !slices.Equal(store.calls, []string{"create:" + failed, "mark:" + failed}) {
			t.Fatalf("calls = %v, want the record re-created in the pass after it was removed", store.calls)
		}

		if got, ok := storedDetectedAt(store, failed); !ok || !got.Equal(recreated) {
			t.Fatalf("stored detectedAt = %s (on record: %t), want the re-creation %s, not the first sighting %s",
				got, ok, recreated, firstFailure)
		}
	}
}

func TestRecordRemovedByAnAgentThatSeesThePeerAliveIsRecreatedOncePerHandoverStep(t *testing.T) {
	const (
		failed = "worker-3"
		passes = 20
	)

	writer := rankOneFor(t, nodeGroupSize, failed)
	remover := writerFor(failed)

	if rank := domain.WriterRank(groupNames(nodeGroupSize), failed, remover); rank != 0 {
		t.Fatalf("the test needs %s first in line for the record of %s while it sees the peer alive, its rank is %d",
			remover, failed, rank)
	}

	store := newStore()
	store.uniqueUIDs = true
	shared := &clock{now: time.Date(2026, 6, 2, 15, 0, 0, 0, time.UTC)}
	holdsFailed := newHarnessWith(t, nodeGroupSize, writer, store, withClock(shared))
	seesAlive := newHarnessWith(t, nodeGroupSize, remover, store, withClock(shared))

	holdsFailed.settle(t.Context())
	seesAlive.settle(t.Context())

	type creation struct{ at, detectedAt time.Time }

	var creations []creation

	for range passes {
		shared.advance(500 * time.Millisecond)

		_, before := storedDetectedAt(store, failed)
		holdsFailed.failPeer(t.Context(), failed)

		if detectedAt, after := storedDetectedAt(store, failed); after && !before {
			creations = append(creations, creation{at: shared.now, detectedAt: detectedAt})
		}

		shared.advance(500 * time.Millisecond)
		seesAlive.settle(t.Context())

		if _, left := storedDetectedAt(store, failed); left {
			t.Fatalf("the record of %s is still there after the pass of %s, want it removed as soon as it is seen", failed, remover)
		}
	}

	if len(creations) < 3 {
		t.Fatalf("records created = %v, want at least three in %d passes to judge the re-creations", creations, passes)
	}

	for i := 1; i < len(creations); i++ {
		prev, next := creations[i-1], creations[i]

		if gap := next.at.Sub(prev.at); gap < takeoverDelay {
			t.Errorf("record re-created %s after the previous one at %s, want at least the handover step %s from the vanish",
				gap, next.at, takeoverDelay)
		}

		if !next.detectedAt.Equal(next.at) {
			t.Errorf("record re-created at %s carries detectedAt %s, want the moment it was written", next.at, next.detectedAt)
		}
	}
}

func TestRecreatedRecordIsStampedOncePerVanish(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	firstFailure := h.clock.now
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Second)
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Minute)
	firstVanish := h.clock.now
	removeRecord(store, failed)
	h.failPeer(t.Context(), failed)

	store.calls = nil

	for range 5 {
		h.clock.advance(time.Second)
		h.failPeer(t.Context(), failed)
	}

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none while the re-created record is visible", store.calls)
	}

	h.clock.advance(time.Minute)
	secondVanish := h.clock.now
	removeRecord(store, failed)
	h.failPeer(t.Context(), failed)

	if got, want := markStamps(store), []time.Time{firstFailure, firstVanish, secondVanish}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: one stamp per vanish", got, want)
	}
}

func TestRecreationAfterAVanishWaitsItsTurnFromTheVanish(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, rankOneFor(t, nodeGroupSize, failed), store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	store.states = append(store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:              failed,
			UID:               "cr-" + failed,
			CreationTimestamp: metav1.NewTime(h.clock.now),
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{
				DetectedAt: metav1.NewMicroTime(h.clock.now),
				DetectedBy: writerFor(failed),
			},
		},
	})

	h.clock.advance(time.Second)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none: the designated writer recorded the incident", store.calls)
	}

	h.clock.advance(time.Minute - time.Second)
	removeRecord(store, failed)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none in the pass that saw the record vanish: the designated writer goes first", store.calls)
	}

	h.clock.advance(takeoverDelay - time.Millisecond)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none before the handover step counted from the vanish has passed", store.calls)
	}

	h.clock.advance(time.Millisecond)
	recreated := h.clock.now
	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"create:" + failed, "mark:" + failed}) {
		t.Fatalf("calls = %v, want the record re-created once the handover step has passed", store.calls)
	}

	if got, want := markStamps(store), []time.Time{recreated}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: the moment of re-creation, not of the vanish", got, want)
	}
}

func TestFailedRecreationIsStampedWhenTheWriteFinallySucceeds(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	firstFailure := h.clock.now
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Second)
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Minute)
	vanished := h.clock.now
	removeRecord(store, failed)
	store.failMark = errors.New("api server is unavailable")
	h.failPeer(t.Context(), failed)

	inc := h.writer.incidents[failed]
	if inc == nil || inc.retryAfter.IsZero() {
		t.Fatalf("calls = %v, want the failed mark to schedule a retry", store.calls)
	}

	h.clock.advance(inc.retryAfter.Sub(h.clock.now))
	written := h.clock.now
	store.failMark = nil
	store.calls = nil
	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"mark:" + failed}) {
		t.Fatalf("calls = %v, want the retry to mark the re-created record", store.calls)
	}

	if got, want := markStamps(store), []time.Time{firstFailure, vanished, written}; !sameInstants(got, want) {
		t.Fatalf("detectedAt of the marks = %v, want %v: the write that went through carries its own moment", got, want)
	}

	h.clock.advance(time.Second)
	store.calls = nil
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none in a pass without a vanish", store.calls)
	}

	if got := store.recorded[failed].DetectedAt; !got.Time.Equal(written) {
		t.Errorf("stored detectedAt = %s, want %s from the write that went through", got, written)
	}

	if inc.restamp {
		t.Errorf("restamp is still set after the write went through")
	}
}

func TestVanishedRecordWithoutAVerdictIsRecreatedWithAFreshDetectedAt(t *testing.T) {
	const failed = "worker-3"

	store := newStore()
	h := newHarness(t, writerFor(failed), store)

	h.settle(t.Context())

	firstFailure := h.clock.now
	store.failMark = errors.New("api server is unavailable")
	h.failPeer(t.Context(), failed)

	inc := h.writer.incidents[failed]
	if inc == nil || inc.retryAfter.IsZero() {
		t.Fatalf("calls = %v, want the failed mark to schedule a retry", store.calls)
	}

	h.clock.advance(inc.retryAfter.Sub(h.clock.now))
	h.failPeer(t.Context(), failed)

	h.clock.advance(inc.retryAfter.Sub(h.clock.now))
	vanished := h.clock.now
	removeRecord(store, failed)
	store.failMark = nil
	store.calls = nil
	h.failPeer(t.Context(), failed)

	if !slices.Equal(store.calls, []string{"create:" + failed, "mark:" + failed}) {
		t.Fatalf("calls = %v, want the record re-created in the pass that saw it vanish", store.calls)
	}

	if got, want := markStamps(store), []time.Time{firstFailure, firstFailure, vanished}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: a record without a verdict has vanished too", got, want)
	}
}

func TestWriterRecordsNoPeerWhileItsOwnFailedRecordExists(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)
	store := newStore()
	h := newHarness(t, self, store)

	h.settle(t.Context())

	store.states = append(store.states, ownRecord(self, "worker-2", h.clock.now))
	h.settle(t.Context())
	h.clock.advance(takeoverDelay)

	for range 3 {
		h.failPeer(t.Context(), failed)
		h.clock.advance(time.Second)
	}

	if !h.writer.paused {
		t.Fatalf("the writer is not paused, want the own failed record to pause it after %s", takeoverDelay)
	}

	if slices.ContainsFunc(store.calls, func(call string) bool {
		return strings.HasPrefix(call, "create:") || strings.HasPrefix(call, "mark:")
	}) {
		t.Errorf("calls = %v, want no create and no mark while a peer records this node as failed", store.calls)
	}

	if h.writer.incidents[failed] == nil {
		t.Errorf("no incident is open for %s, want the failure timed during the pause", failed)
	}
}

func TestRecordWrittenDuringThePauseEndsTheRetryOfItsIncident(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)
	store := newStore()
	h := newHarness(t, self, store)

	h.settle(t.Context())

	store.failCreate = errors.New("api server is unavailable")
	h.failPeer(t.Context(), failed)

	inc := h.writer.incidents[failed]
	if inc == nil || inc.attempts == 0 || inc.retryAfter.IsZero() {
		t.Fatalf("calls = %v, want the failed create to schedule a retry", store.calls)
	}

	store.failCreate = nil
	h.pauseByOwnRecord(t, self, otherThan(failed), failed)

	if inc.attempts == 0 || inc.retryAfter.IsZero() {
		t.Fatalf("calls = %v, want the retry still pending when the pause starts", store.calls)
	}

	record := failedRecordAt(failed, types.UID("cr-"+failed), h.clock.now)
	record.Status.Failed.DetectedBy = otherThan(failed)
	store.states = append(store.states, record)
	h.failPeer(t.Context(), failed)

	if inc.attempts != 0 || !inc.retryAfter.IsZero() {
		t.Errorf("incident = {attempts: %d, retryAfter: %s}, want no attempts and no retry once %s is on record, paused or not",
			inc.attempts, inc.retryAfter, failed)
	}
}

func TestWriterRemovesNoOtherRecordWhileItsOwnFailedRecordExists(t *testing.T) {
	self := writerFor("worker-3")

	rows := []struct {
		name       string
		member     string
		record     func(now time.Time) v1alpha1.FencingFailedNodeState
		seenBefore bool
		wait       time.Duration
	}{
		{
			name:   "failed verdict on a peer back in gossip",
			member: "worker-3",
			record: func(now time.Time) v1alpha1.FencingFailedNodeState {
				state := failedRecordAt("worker-3", "cr-worker-3", now)
				state.Status.Failed.DetectedBy = "worker-9"

				return state
			},
		},
		{
			name:   "record without a verdict that stopped changing",
			member: "worker-3",
			record: func(time.Time) v1alpha1.FencingFailedNodeState {
				return v1alpha1.FencingFailedNodeState{ObjectMeta: metav1.ObjectMeta{Name: "worker-3", UID: "cr-worker-3"}}
			},
			wait: fallbackTTL + time.Second,
		},
		{
			name:   "record of a node outside the group",
			member: "worker-7",
			record: func(now time.Time) v1alpha1.FencingFailedNodeState {
				state := failedRecordAt("worker-7", "cr-worker-7", now)
				state.Status.Failed.DetectedBy = "worker-9"

				return state
			},
			seenBefore: true,
			wait:       unexpectedGrace + time.Second,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			if rank := domain.WriterRank(groupNames(nodeGroupSize), row.member, self); rank != 0 {
				t.Fatalf("the test needs %s first in line for the record of %s, its rank is %d", self, row.member, rank)
			}

			store := newStore()
			h := newHarness(t, self, store)

			if row.seenBefore {
				store.states = append(store.states, row.record(h.clock.now))
				h.settle(t.Context())

				if len(store.calls) != 0 {
					t.Fatalf("calls = %v, want none before the pause: the record is not due yet", store.calls)
				}
			}

			h.pauseByOwnRecord(t, self, "worker-2")

			if !row.seenBefore {
				store.states = append(store.states, row.record(h.clock.now))
			}

			h.settle(t.Context())
			h.clock.advance(row.wait)
			h.settle(t.Context())

			if deletes := deleteCalls(store); len(deletes) != 0 {
				t.Errorf("deletes = %v, want none while a peer records this node as failed", deletes)
			}
		})
	}
}

func TestPausedWriterRemovesItsOwnVerdictOnAPeerBackInGossip(t *testing.T) {
	const peer = "worker-3"

	rows := []struct {
		name string
		self string
		rank int
	}{
		{
			name: "first in line removes it in the pass that sees it",
			self: writerFor(peer),
			rank: 0,
		},
		{
			name: "next in line removes it once the handover step has passed",
			self: rankOneFor(t, nodeGroupSize, peer),
			rank: 1,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			if rank := domain.WriterRank(groupNames(nodeGroupSize), peer, row.self); rank != row.rank {
				t.Fatalf("the test needs %s at rank %d for the record of %s, its rank is %d", row.self, row.rank, peer, rank)
			}

			store := newStore()
			h := newHarness(t, row.self, store)

			h.pauseByOwnRecord(t, row.self, peer)

			verdict := failedRecordAt(peer, types.UID("cr-"+peer), h.clock.now)
			verdict.Status.Failed.DetectedBy = row.self
			store.states = append(store.states, verdict)

			h.settle(t.Context())

			if row.rank > 0 {
				if deletes := deleteCalls(store); len(deletes) != 0 {
					t.Fatalf("deletes = %v, want none in the pass that first sees the record: the agent first in line goes first", deletes)
				}

				h.clock.advance(takeoverDelay - time.Millisecond)
				h.settle(t.Context())

				if deletes := deleteCalls(store); len(deletes) != 0 {
					t.Fatalf("deletes = %v, want none before the handover step counted from the pass that first saw the record", deletes)
				}

				h.clock.advance(time.Millisecond)
				h.settle(t.Context())
			}

			if got, want := deleteCalls(store), []string{"delete:" + peer + ":cr-" + peer}; !slices.Equal(got, want) {
				t.Errorf("deletes = %v, want %v: the own verdict on the peer back in gossip and nothing else", got, want)
			}

			if want := []string{reasonStateCleared}; !slices.Equal(h.events.normal, want) {
				t.Errorf("normal events = %v, want %v as for any other removal", h.events.normal, want)
			}

			if !h.writer.paused {
				t.Errorf("the writer resumed, want it paused while the own failed record stands")
			}
		})
	}
}

func TestPausedWriterKeepsItsOwnVerdictOnAPeerStillGone(t *testing.T) {
	rows := []struct {
		name   string
		member string
	}{
		{name: "peer still out of gossip", member: "worker-3"},
		{name: "node outside the group", member: "worker-9"},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			self := writerFor(row.member)
			store := newStore()
			h := newHarness(t, self, store)

			h.settle(t.Context())

			verdict := failedRecordAt(row.member, types.UID("cr-"+row.member), h.clock.now)
			verdict.Status.Failed.DetectedBy = self
			store.states = append(store.states, ownRecord(self, otherThan(row.member), h.clock.now), verdict)

			h.failPeer(t.Context(), row.member)
			h.clock.advance(takeoverDelay)
			h.failPeer(t.Context(), row.member)

			if !h.writer.paused {
				t.Fatalf("the writer is not paused, want the own failed record to pause it after %s", takeoverDelay)
			}

			h.clock.advance(unexpectedGrace + time.Second)
			h.failPeer(t.Context(), row.member)

			if deletes := deleteCalls(store); len(deletes) != 0 {
				t.Errorf("deletes = %v, want none: gossip does not see %s", deletes, row.member)
			}
		})
	}
}

func TestClearOwnStateRunsAndAnEarlierLifeRecordDoesNotPause(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)
	store := newStore()
	h := newHarness(t, self, store)

	h.settle(t.Context())

	store.states = append(store.states, ownRecord(self, "worker-2", h.writer.startedAt.Add(-time.Hour)))
	h.failPeer(t.Context(), failed)

	if want := []string{"delete:" + self + ":cr-" + self, "create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
		t.Errorf("calls = %v, want %v in one pass", store.calls, want)
	}
}

func TestWriterPauseStartsAtTheProcessStartSecond(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)

	t.Run("a record created in the start second pauses the writer", func(t *testing.T) {
		store := newStore()
		h := newHarnessWith(t, nodeGroupSize, self, store, withClockAt(ownRecordStart))

		h.settle(t.Context())

		store.states = append(store.states, ownRecord(self, "worker-2", clockAt(15, 0, 0)))
		h.settle(t.Context())
		h.clock.advance(takeoverDelay)
		h.failPeer(t.Context(), failed)

		if !h.writer.paused || len(store.calls) != 0 {
			t.Errorf("paused = %t with calls = %v, want paused with none: the record is about this life and has stood for %s",
				h.writer.paused, store.calls, takeoverDelay)
		}
	})

	t.Run("a record created the second before is removed and pauses nothing", func(t *testing.T) {
		store := newStore()
		h := newHarnessWith(t, nodeGroupSize, self, store, withClockAt(ownRecordStart))

		h.settle(t.Context())

		store.states = append(store.states, ownRecord(self, "worker-2", clockAt(14, 59, 59)))
		h.failPeer(t.Context(), failed)

		if want := []string{"delete:" + self + ":cr-" + self, "create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
			t.Errorf("calls = %v, want %v", store.calls, want)
		}
	})
}

func TestOwnFallbackRecordDoesNotPauseTheWriter(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)
	store := newStore()
	h := newHarness(t, self, store)

	h.settle(t.Context())

	store.states = append(store.states, fallbackOnlyRecordAt(self, types.UID("cr-"+self), h.clock.now))
	store.beat(self, h.clock.now)
	h.failPeer(t.Context(), failed)

	if want := []string{"create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
		t.Errorf("calls = %v, want %v: a record without a verdict pauses nothing", store.calls, want)
	}
}

func TestWriterResumesWhenTheOwnFailedRecordGoes(t *testing.T) {
	const (
		failed = "worker-3"

		recordedMsg = "fencing state recorded"
	)

	self := writerFor(failed)
	logs := &bytes.Buffer{}
	store := newStore()
	h := newHarnessWith(t, nodeGroupSize, self, store, withLogger(logtest.NewJSONLogger(logs)))

	h.settle(t.Context())

	store.states = append(store.states, ownRecord(self, otherThan(failed), h.clock.now))
	h.settle(t.Context())

	h.clock.advance(takeoverDelay)
	failedPaused := h.clock.now
	h.failPeer(t.Context(), failed)

	if !h.writer.paused {
		t.Fatalf("the writer is not paused, want the own failed record to pause it")
	}

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none while a peer records this node as failed", store.calls)
	}

	h.clock.advance(time.Minute)
	resumed := h.clock.now
	removeRecord(store, self)
	h.failPeer(t.Context(), failed)

	if h.writer.paused {
		t.Fatalf("the writer is still paused, want it resumed once the own failed record is gone")
	}

	if want := []string{"create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
		t.Fatalf("calls = %v, want %v in the pass that resumes", store.calls, want)
	}

	if got, want := markStamps(store), []time.Time{resumed}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: the moment the writer resumed, not the failure seen paused at %s",
			got, want, failedPaused)
	}

	records := logtest.Drain(t, logs)
	logtest.AssertSnakeCaseKeys(t, records)

	ends := logtest.WithMsg(records, writerResumedMsg)
	if len(ends) != 1 {
		t.Fatalf("pause end logged %d times, want once", len(ends))
	}

	if got := ends[0].Int("open_incidents"); got != 1 {
		t.Errorf("pause end open_incidents = %d, want 1: the incident of %s opened during the pause", got, failed)
	}

	if got, want := ends[0].Str("paused_for"), time.Minute.String(); got != want {
		t.Errorf("pause end paused_for = %q, want %q from the pass that started the pause", got, want)
	}

	want := resumed.UTC().Format(metav1.RFC3339Micro)
	if lines := logtest.WithMsg(records, recordedMsg); len(lines) != 1 || lines[0].Str("detected_at") != want {
		t.Errorf("%q lines = %v, want one with detected_at %q", recordedMsg, lines, want)
	}
}

func TestResumeRestartsTheTakeoverClockOfEveryOpenIncident(t *testing.T) {
	const failed = "worker-3"

	self := rankOneFor(t, nodeGroupSize, failed)
	store := newStore()
	h := newHarness(t, self, store)

	h.settle(t.Context())

	store.states = append(store.states, ownRecord(self, writerFor(failed), h.clock.now))
	h.settle(t.Context())
	h.clock.advance(takeoverDelay)

	failedPaused := h.clock.now
	h.failPeer(t.Context(), failed)

	if !h.writer.paused {
		t.Fatalf("the writer is not paused, want the own failed record to pause it")
	}

	h.clock.advance(takeoverDelay + time.Millisecond)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none while a peer records this node as failed", store.calls)
	}

	h.clock.advance(time.Minute - takeoverDelay - time.Millisecond)
	resumed := h.clock.now
	removeRecord(store, self)
	h.failPeer(t.Context(), failed)

	if h.writer.paused {
		t.Fatalf("the writer is still paused, want it resumed once the own failed record is gone")
	}

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none in the pass that resumes: the handover step counts from the resume", store.calls)
	}

	h.clock.advance(takeoverDelay - time.Millisecond)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none before the handover step counted from the resume has passed", store.calls)
	}

	h.clock.advance(2 * time.Millisecond)
	h.failPeer(t.Context(), failed)

	if want := []string{"create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
		t.Fatalf("calls = %v, want %v once the handover step counted from the resume has passed", store.calls, want)
	}

	if got, want := markStamps(store), []time.Time{resumed}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: the resume, not the failure seen paused at %s", got, want, failedPaused)
	}
}

func TestRecordThatVanishedDuringThePauseIsStampedWhenItIsRecreated(t *testing.T) {
	const failed = "worker-3"

	self := rankOneFor(t, nodeGroupSize, failed)
	store := newStore()
	h := newHarness(t, self, store)

	h.settle(t.Context())
	h.failPeer(t.Context(), failed)

	store.states = append(store.states, v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:              failed,
			UID:               "cr-" + failed,
			CreationTimestamp: metav1.NewTime(h.clock.now),
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{
				DetectedAt: metav1.NewMicroTime(h.clock.now),
				DetectedBy: writerFor(failed),
			},
		},
	})

	h.clock.advance(time.Second)
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Second)
	h.pauseByOwnRecord(t, self, writerFor(failed), failed)

	h.clock.advance(time.Second)
	removeRecord(store, failed)
	h.failPeer(t.Context(), failed)

	if inc := h.writer.incidents[failed]; inc == nil || !inc.restamp {
		t.Fatalf("incident of %s = %+v, want the vanish noticed during the pause", failed, inc)
	}

	h.clock.advance(time.Minute)
	resumed := h.clock.now
	removeRecord(store, self)
	h.failPeer(t.Context(), failed)

	if h.writer.paused {
		t.Fatalf("the writer is still paused, want it resumed once the own failed record is gone")
	}

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none in the pass that resumes: the handover step counts from the resume", store.calls)
	}

	h.clock.advance(takeoverDelay - time.Millisecond)
	h.failPeer(t.Context(), failed)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none before the handover step counted from the resume has passed", store.calls)
	}

	h.clock.advance(time.Millisecond)
	recreated := h.clock.now
	h.failPeer(t.Context(), failed)

	if want := []string{"create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
		t.Fatalf("calls = %v, want %v once the handover step counted from the resume has passed", store.calls, want)
	}

	if got, want := markStamps(store), []time.Time{recreated}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: the moment of re-creation, not of the resume %s", got, want, resumed)
	}
}

func TestPeerThatRecoversAndFailsAgainDuringThePauseIsTimedAfresh(t *testing.T) {
	const size = 5

	const failed = "worker-5"

	firstWriter := writerForIn(size, failed)
	self := writerForIn(size, failed, firstWriter)

	store := newStore()
	h := newHarnessOfSize(t, size, self, store)

	h.settle(t.Context())

	firstFailure := h.clock.now
	h.failPeer(t.Context(), failed)
	h.settle(t.Context())

	h.clock.advance(time.Hour)
	h.pauseByOwnRecord(t, self, firstWriter)

	h.clock.advance(time.Second)
	secondFailure := h.clock.now
	h.failPeer(t.Context(), failed, firstWriter)

	if len(store.calls) != 0 {
		t.Fatalf("calls = %v, want none while a peer records this node as failed", store.calls)
	}

	h.clock.advance(time.Minute)
	resumed := h.clock.now
	removeRecord(store, self)
	h.failPeer(t.Context(), failed, firstWriter)

	if got := store.recorded[failed].DetectedAt; !got.Time.Equal(resumed) {
		t.Errorf("detectedAt = %s, want the resume %s: the peer failed again at %s during the pause", got, resumed, secondFailure)
	}

	for _, stamp := range markStamps(store) {
		if stamp.Equal(firstFailure) || stamp.Equal(secondFailure) {
			t.Errorf("a mark carries detectedAt %s, want no failure seen before the resume %s", stamp, resumed)
		}
	}
}

func TestWriterPauseIsLoggedOnceAndRaisesNoEvent(t *testing.T) {
	const (
		self        = "worker-1"
		detectedBy  = "worker-2"
		pausePasses = 5
	)

	logs := &bytes.Buffer{}
	store := newStore()
	h := newHarnessWith(t, nodeGroupSize, self, store, withLogger(logtest.NewJSONLogger(logs)))

	h.settle(t.Context())

	records := logtest.Drain(t, logs)
	normal, warnings := slices.Clone(h.events.normal), slices.Clone(h.events.warnings)

	detectedAt := h.clock.now
	store.states = append(store.states, ownRecord(self, detectedBy, detectedAt))

	h.clock.advance(1500 * time.Millisecond)

	for waited := time.Duration(0); waited < takeoverDelay; waited += time.Second {
		h.writer.reconcile(t.Context())
		h.clock.advance(time.Second)

		if h.writer.paused {
			t.Fatalf("the writer paused %s after it first saw the own failed record, want %s", waited, takeoverDelay)
		}
	}

	for range pausePasses {
		h.writer.reconcile(t.Context())
		h.clock.advance(time.Second)
	}

	if !h.writer.paused {
		t.Fatalf("the writer is not paused, want the own failed record to pause it")
	}

	paused := logtest.Drain(t, logs)

	removeRecord(store, self)

	for range 3 {
		h.writer.reconcile(t.Context())
		h.clock.advance(time.Second)
	}

	if h.writer.paused {
		t.Fatalf("the writer is still paused, want it resumed once the own failed record is gone")
	}

	resumed := logtest.Drain(t, logs)
	window := slices.Concat(paused, resumed)

	logtest.AssertSnakeCaseKeys(t, slices.Concat(records, window))

	starts := logtest.WithMsg(paused, writerPausedMsg)
	if len(starts) != 1 || len(logtest.WithMsg(window, writerPausedMsg)) != 1 {
		t.Fatalf("pause start logged %d times during the pause and %d times in all, want once when the pause starts",
			len(starts), len(logtest.WithMsg(window, writerPausedMsg)))
	}

	ends := logtest.WithMsg(resumed, writerResumedMsg)
	if len(ends) != 1 || len(logtest.WithMsg(window, writerResumedMsg)) != 1 {
		t.Fatalf("pause end logged %d times after the pause and %d times in all, want once when the writer resumes",
			len(ends), len(logtest.WithMsg(window, writerResumedMsg)))
	}

	if got := starts[0].Level(); got != "warn" {
		t.Errorf("pause start logged at %q, want warn", got)
	}

	if got, want := starts[0].Str("detected_by"), detectedBy; got != want {
		t.Errorf("pause start detected_by = %q, want %q", got, want)
	}

	if got, want := starts[0].Str("detected_at"), detectedAt.UTC().Format(metav1.RFC3339Micro); got != want {
		t.Errorf("pause start detected_at = %q, want %q", got, want)
	}

	if got := ends[0].Level(); got != "info" {
		t.Errorf("pause end logged at %q, want info", got)
	}

	if got, want := ends[0].Str("paused_for"), (pausePasses * time.Second).String(); got != want {
		t.Errorf("pause end paused_for = %q, want %q", got, want)
	}

	if got := ends[0].Int("open_incidents"); got != 0 {
		t.Errorf("pause end open_incidents = %d, want 0", got)
	}

	for _, record := range window {
		if record.Msg() == writerPausedMsg || record.Msg() == writerResumedMsg {
			continue
		}

		if record.Level() != "debug" {
			t.Errorf("%s line %q around the pause, want every line but its start and end at debug", record.Level(), record.Msg())
		}
	}

	if !slices.Equal(h.events.normal, normal) || !slices.Equal(h.events.warnings, warnings) {
		t.Errorf("events = {normal: %v, warnings: %v}, want no new reason from %v, %v: the pause raises no Event",
			h.events.normal, h.events.warnings, normal, warnings)
	}
}

func TestWriterPauseStartsAndEndsWithoutLocalQuorum(t *testing.T) {
	const (
		size = 5
		self = "worker-1"
	)

	failed := []string{"worker-4", "worker-5"}

	logs := &bytes.Buffer{}
	store := newStore()
	h := newHarnessWith(t, size, self, store, withLogger(logtest.NewJSONLogger(logs)))

	h.settle(t.Context())

	failedAt := h.clock.now
	h.failPeer(t.Context(), failed...)

	if len(h.writer.incidents) != len(failed) {
		t.Fatalf("open incidents = %d, want %d before quorum is lost", len(h.writer.incidents), len(failed))
	}

	passBelowQuorum := func() []logtest.Record {
		t.Helper()

		h.alive.members = []string{self, "worker-2"}
		h.writer.reconcile(t.Context())

		if h.writer.view.HasQuorum() {
			t.Fatalf("the writer holds local quorum with alive %v, the test needs it lost", h.alive.members)
		}

		return logtest.Drain(t, logs)
	}

	h.clock.advance(time.Second)
	passBelowQuorum()

	h.clock.advance(time.Second)
	store.states = append(store.states, ownRecord(self, "worker-2", h.clock.now))
	records := passBelowQuorum()

	if h.writer.paused || len(logtest.WithMsg(records, writerPausedMsg)) != 0 {
		t.Fatalf("the writer paused in the pass that first saw the own record, want it paused %s later", takeoverDelay)
	}

	h.clock.advance(takeoverDelay)
	pausedAt := h.clock.now
	records = passBelowQuorum()

	if !h.writer.paused {
		t.Fatalf("the writer is not paused, want the own failed record to pause it in a pass without quorum")
	}

	if starts := logtest.WithMsg(records, writerPausedMsg); len(starts) != 1 {
		t.Errorf("pause start logged %d times in the pass one handover step after the own record was seen without quorum, want once",
			len(starts))
	}

	h.clock.advance(time.Minute)
	resumed := h.clock.now
	removeRecord(store, self)
	records = passBelowQuorum()

	if h.writer.paused {
		t.Fatalf("the writer is still paused, want it resumed in the pass without quorum that saw the own record go")
	}

	logtest.AssertSnakeCaseKeys(t, records)

	ends := logtest.WithMsg(records, writerResumedMsg)
	if len(ends) != 1 {
		t.Fatalf("pause end logged %d times in the pass that saw the own record go without quorum, want once", len(ends))
	}

	if got, want := ends[0].Str("paused_for"), resumed.Sub(pausedAt).String(); got != want {
		t.Errorf("pause end paused_for = %q, want %q", got, want)
	}

	if got, want := ends[0].Int("open_incidents"), len(failed); got != want {
		t.Errorf("pause end open_incidents = %d, want %d", got, want)
	}

	if len(h.writer.incidents) != len(failed) {
		t.Fatalf("open incidents = %d, want %d: a pass without quorum forgets none", len(h.writer.incidents), len(failed))
	}

	for name, inc := range h.writer.incidents {
		if !inc.detectedAt.Equal(resumed) || !inc.clockFrom.Equal(resumed) {
			t.Errorf("incident of %s = {detectedAt: %s, clockFrom: %s}, want both at the resume %s, not the failure at %s",
				name, inc.detectedAt, inc.clockFrom, resumed, failedAt)
		}
	}
}

func TestPausedWriterKeepsObservingHeartbeats(t *testing.T) {
	const (
		node        = "worker-3"
		pausePasses = 10

		removedMsg    = "fencing state removed"
		removedReason = "the peer is back in the gossip network"
	)

	self := writerFor(node)

	rows := []struct {
		name        string
		resumeAfter time.Duration
		removed     bool
	}{
		{
			name:        "heartbeat still fresh when the pause ends",
			resumeAfter: 2 * time.Second,
		},
		{
			name:        "heartbeat still for longer than the TTL when the pause ends",
			resumeAfter: fallbackTTL + time.Second,
			removed:     true,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			logs := &bytes.Buffer{}
			store := newStore()
			h := newHarnessWith(t, nodeGroupSize, self, store, withLogger(logtest.NewJSONLogger(logs)))

			store.states = append(store.states, fallbackRecord(node, types.UID("cr-"+node), h.clock.now))
			h.settle(t.Context())

			store.states = append(store.states, ownRecord(self, otherThan(node), h.clock.now))
			h.settle(t.Context())
			h.clock.advance(takeoverDelay)
			store.beat(node, h.clock.now)
			h.settle(t.Context())

			if !h.writer.paused {
				t.Fatalf("the writer is not paused, want the own failed record to pause it")
			}

			for range pausePasses {
				h.clock.advance(time.Second)
				store.beat(node, h.clock.now)
				h.settle(t.Context())
			}

			if deletes := deleteCalls(store); len(deletes) != 0 {
				t.Fatalf("deletes = %v, want none during the pause", deletes)
			}

			h.clock.advance(row.resumeAfter)
			removeRecord(store, self)
			logs.Reset()
			h.settle(t.Context())

			if h.writer.paused {
				t.Fatalf("the writer is still paused, want it resumed once the own failed record is gone")
			}

			records := logtest.Drain(t, logs)
			logtest.AssertSnakeCaseKeys(t, records)

			removals := logtest.WithMsg(records, removedMsg)
			deletes := deleteCalls(store)

			if !row.removed {
				if len(deletes) != 0 || len(removals) != 0 {
					t.Errorf("deletes = %v with %d %q lines, want none: %s heartbeated %s before the pass that ends the pause",
						deletes, len(removals), removedMsg, node, row.resumeAfter)
				}

				return
			}

			if want := []string{"delete:" + node + ":cr-" + node}; !slices.Equal(deletes, want) {
				t.Errorf("deletes = %v, want %v in the pass that ends the pause: the heartbeat stopped %s before it",
					deletes, want, row.resumeAfter)
			}

			if len(removals) != 1 || removals[0].Str("member") != node || removals[0].Str("reason") != removedReason {
				t.Errorf("%q lines = %v, want one with member %q and reason %q", removedMsg, removals, node, removedReason)
			}
		})
	}
}

const (
	writerWaitsMsg   = "a peer recorded this node as failed, the fencing state writer pauses if the record stays"
	writerPausedMsg  = "a peer recorded this node as failed, the fencing state writer is paused"
	writerResumedMsg = "no peer records this node as failed any more, the fencing state writer resumes"
)

func TestWriterPausesOneHandoverStepAfterItFirstSeesItsOwnRecord(t *testing.T) {
	const (
		self       = "worker-1"
		detectedBy = "worker-2"
	)

	rows := []struct {
		name   string
		stamps time.Duration
	}{
		{name: "record stamped two handover steps before the pass that sees it", stamps: -2 * takeoverDelay},
		{name: "record stamped an hour ahead of this agent's clock", stamps: time.Hour},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			logs := &bytes.Buffer{}
			store := newStore()
			h := newHarnessWith(t, nodeGroupSize, self, store, withLogger(logtest.NewJSONLogger(logs)))

			h.settle(t.Context())
			h.clock.advance(2 * takeoverDelay)

			stamp := h.clock.now.Add(row.stamps)
			store.states = append(store.states, ownRecord(self, detectedBy, stamp))
			h.settle(t.Context())

			h.clock.advance(takeoverDelay - time.Millisecond)
			h.settle(t.Context())

			waiting := logtest.Drain(t, logs)

			if h.writer.paused || len(logtest.WithMsg(waiting, writerPausedMsg)) != 0 {
				t.Fatalf("paused = %t with %d pause lines %s after the pass that first saw the own record, want no pause before %s",
					h.writer.paused, len(logtest.WithMsg(waiting, writerPausedMsg)), takeoverDelay-time.Millisecond, takeoverDelay)
			}

			h.clock.advance(time.Millisecond)
			h.settle(t.Context())

			if !h.writer.paused {
				t.Fatalf("the writer is not paused %s after the pass that first saw the own record, want it paused", takeoverDelay)
			}

			for range 3 {
				h.clock.advance(time.Second)
				h.settle(t.Context())
			}

			records := slices.Concat(waiting, logtest.Drain(t, logs))
			logtest.AssertSnakeCaseKeys(t, records)

			starts := logtest.WithMsg(records, writerPausedMsg)
			if len(starts) != 1 {
				t.Fatalf("pause start logged %d times, want once", len(starts))
			}

			if got, want := starts[0].Str("detected_at"), stamp.UTC().Format(metav1.RFC3339Micro); got != want {
				t.Errorf("pause start detected_at = %q, want %q from the record", got, want)
			}

			if len(store.calls) != 0 {
				t.Errorf("calls = %v, want none: no peer failed and no record is due", store.calls)
			}
		})
	}
}

func TestOwnRecordThatGoesBeforeTheWaitEndsLeavesTheWriterRunning(t *testing.T) {
	const failed = "worker-3"

	self := rankOneFor(t, nodeGroupSize, failed)
	logs := &bytes.Buffer{}
	store := newStore()
	h := newHarnessWith(t, nodeGroupSize, self, store, withLogger(logtest.NewJSONLogger(logs)))

	h.settle(t.Context())

	firstFailure := h.clock.now
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Second)
	store.states = append(store.states, ownRecord(self, writerFor(failed), h.clock.now))
	h.failPeer(t.Context(), failed)

	h.clock.advance(time.Second)
	removeRecord(store, self)
	h.failPeer(t.Context(), failed)

	inc := h.writer.incidents[failed]
	if inc == nil {
		t.Fatalf("no incident is open for %s, want the one opened at %s", failed, firstFailure)
	}

	if !inc.detectedAt.Equal(firstFailure) || !inc.clockFrom.Equal(firstFailure) {
		t.Fatalf("incident of %s = {detectedAt: %s, clockFrom: %s}, want both at the failure %s: the writer never paused",
			failed, inc.detectedAt, inc.clockFrom, firstFailure)
	}

	h.clock.advance(takeoverDelay - 2*time.Second)
	h.failPeer(t.Context(), failed)

	if want := []string{"create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
		t.Fatalf("calls = %v, want %v one handover step after the failure", store.calls, want)
	}

	if got, want := markStamps(store), []time.Time{firstFailure}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v: the failure, not the moment the own record went", got, want)
	}

	records := logtest.Drain(t, logs)
	logtest.AssertSnakeCaseKeys(t, records)

	if starts, ends := logtest.WithMsg(records, writerPausedMsg), logtest.WithMsg(records, writerResumedMsg); len(starts) != 0 || len(ends) != 0 {
		t.Fatalf("pause logged %d times and its end %d times, want neither for a record gone within %s",
			len(starts), len(ends), takeoverDelay)
	}

	h.clock.advance(time.Second)
	store.states = append(store.states, ownRecord(self, writerFor(failed), h.clock.now))
	h.failPeer(t.Context(), failed)

	h.clock.advance(takeoverDelay - time.Millisecond)
	h.failPeer(t.Context(), failed)

	if h.writer.paused {
		t.Fatalf("the writer paused %s after the own record came back, want the wait counted from that pass", takeoverDelay-time.Millisecond)
	}

	h.clock.advance(time.Millisecond)
	h.failPeer(t.Context(), failed)

	if !h.writer.paused {
		t.Fatalf("the writer is not paused %s after the own record came back, want it paused", takeoverDelay)
	}

	records = logtest.Drain(t, logs)
	if starts, ends := logtest.WithMsg(records, writerPausedMsg), logtest.WithMsg(records, writerResumedMsg); len(starts) != 1 || len(ends) != 0 {
		t.Errorf("pause logged %d times and its end %d times after the record came back, want the start once", len(starts), len(ends))
	}
}

func TestOwnRecordRecreatedUnderANewUIDRestartsTheWait(t *testing.T) {
	const self = "worker-1"

	logs := &bytes.Buffer{}
	store := newStore()
	h := newHarnessWith(t, nodeGroupSize, self, store, withLogger(logtest.NewJSONLogger(logs)))

	h.settle(t.Context())

	store.states = append(store.states, ownRecord(self, "worker-2", h.clock.now))
	h.settle(t.Context())

	h.clock.advance(takeoverDelay - time.Second)
	removeRecord(store, self)

	recreated := ownRecord(self, "worker-3", h.clock.now)
	recreated.UID = types.UID("cr-" + self + "-recreated")
	store.states = append(store.states, recreated)
	h.settle(t.Context())

	h.clock.advance(time.Second)
	h.settle(t.Context())

	if h.writer.paused {
		t.Fatalf("the writer paused one handover step after it saw the replaced record, want the wait restarted by the new UID")
	}

	h.clock.advance(takeoverDelay - time.Second - time.Millisecond)
	h.settle(t.Context())

	if h.writer.paused {
		t.Fatalf("the writer paused %s after it saw the new record, want no pause before %s", takeoverDelay-time.Millisecond, takeoverDelay)
	}

	h.clock.advance(time.Millisecond)
	h.settle(t.Context())

	if !h.writer.paused {
		t.Fatalf("the writer is not paused %s after it saw the new record, want it paused", takeoverDelay)
	}

	starts := logtest.WithMsg(logtest.Drain(t, logs), writerPausedMsg)
	if len(starts) != 1 || starts[0].Str("detected_by") != "worker-3" {
		t.Errorf("pause start lines = %v, want one with detected_by %q from the new record", starts, "worker-3")
	}
}

func TestPausedWriterStaysPausedWhenItsOwnRecordIsRecreatedUnderANewUID(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)
	other := otherThan(failed)

	if rank := domain.WriterRank([]string{self, other}, other, self); rank != 0 {
		t.Fatalf("the test needs %s first in line for the record of %s while %s is gone, its rank is %d", self, other, failed, rank)
	}

	logs := &bytes.Buffer{}
	store := newStore()
	h := newHarnessWith(t, nodeGroupSize, self, store, withLogger(logtest.NewJSONLogger(logs)))

	h.settle(t.Context())

	h.pauseByOwnRecord(t, self, other)

	pausedAt := h.clock.now

	if starts := logtest.WithMsg(logtest.Drain(t, logs), writerPausedMsg); len(starts) != 1 {
		t.Fatalf("pause start logged %d times, want once", len(starts))
	}

	h.clock.advance(time.Second)
	removeRecord(store, self)

	recreated := ownRecord(self, other, h.clock.now)
	recreated.UID = types.UID("cr-" + self + "-recreated")

	verdict := failedRecordAt(other, types.UID("cr-"+other), h.clock.now)
	verdict.Status.Failed.DetectedBy = failed

	store.states = append(store.states, recreated, verdict)
	h.failPeer(t.Context(), failed)

	for elapsed := time.Second; elapsed <= takeoverDelay; elapsed += time.Second {
		h.clock.advance(time.Second)
		h.failPeer(t.Context(), failed)
	}

	if !h.writer.paused {
		t.Fatalf("the writer is not paused %s after its own record was recreated under a new UID, want the pause to go on", takeoverDelay)
	}

	if len(store.calls) != 0 {
		t.Errorf("calls = %v, want none: the writer is first in line to record %s and to remove the record of %s, but it is paused",
			store.calls, failed, other)
	}

	records := logtest.Drain(t, logs)
	logtest.AssertSnakeCaseKeys(t, records)

	if starts, ends := logtest.WithMsg(records, writerPausedMsg), logtest.WithMsg(records, writerResumedMsg); len(starts) != 0 || len(ends) != 0 {
		t.Errorf("pause logged %d more times and its end %d times after the own record was recreated, want neither while a record stands",
			len(starts), len(ends))
	}

	if waits := logtest.WithMsg(records, writerWaitsMsg); len(waits) != 0 {
		t.Errorf("a wait for the pause logged %d times after the own record was recreated, want none: the writer is paused already", len(waits))
	}

	h.clock.advance(time.Second)
	removeRecord(store, self)
	h.failPeer(t.Context(), failed)

	ends := logtest.WithMsg(logtest.Drain(t, logs), writerResumedMsg)
	if len(ends) != 1 {
		t.Fatalf("pause end logged %d times once no own record stands, want once", len(ends))
	}

	if got, want := ends[0].Str("paused_for"), h.clock.now.Sub(pausedAt).String(); got != want {
		t.Errorf("paused_for = %q, want %q counted from the pass that paused over the first record", got, want)
	}
}

func TestWriterRecordsAndClearsLikeAnyWriterWhileItWaitsOutItsOwnRecord(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)
	broken := otherThan(failed)

	if rank := domain.WriterRank(groupNames(nodeGroupSize), failed, self); rank != 0 {
		t.Fatalf("the test needs %s first in line for the record of %s, its rank is %d", self, failed, rank)
	}

	store := newStore()
	store.uniqueUIDs = true
	h := newHarness(t, self, store)

	h.settle(t.Context())

	verdict := failedRecordAt(failed, types.UID("cr-"+failed), h.clock.now)
	verdict.Status.Failed.DetectedBy = broken
	store.states = append(store.states, ownRecord(self, broken, h.clock.now), verdict)
	h.settle(t.Context())

	if got, want := deleteCalls(store), []string{"delete:" + failed + ":cr-" + failed}; !slices.Equal(got, want) {
		t.Fatalf("deletes = %v, want %v in the pass that first sees the own record: the writer is not paused yet", got, want)
	}

	h.clock.advance(time.Second)
	failedAt := h.clock.now
	store.calls = nil
	h.failPeer(t.Context(), failed)

	if want := []string{"create:" + failed, "mark:" + failed}; !slices.Equal(store.calls, want) {
		t.Fatalf("calls = %v, want %v within one handover step of the own record", store.calls, want)
	}

	if got, want := markStamps(store), []time.Time{failedAt}; !sameInstants(got, want) {
		t.Errorf("detectedAt of the marks = %v, want %v", got, want)
	}

	if h.writer.paused {
		t.Fatalf("the writer is paused %s after it first saw the own record, want no pause before %s", time.Second, takeoverDelay)
	}

	h.clock.advance(takeoverDelay - time.Second)
	h.failPeer(t.Context(), failed)

	if !h.writer.paused {
		t.Errorf("the writer is not paused %s after it first saw the own record, want it paused", takeoverDelay)
	}
}

func TestWriterWithoutATakeoverDelayPausesInThePassThatSeesItsOwnRecord(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)

	for _, delay := range []time.Duration{0, -time.Second} {
		t.Run("takeover delay "+delay.String(), func(t *testing.T) {
			logs := &bytes.Buffer{}
			store := newStore()
			h := newHarnessWith(t, nodeGroupSize, self, store, withTakeoverDelay(delay), withLogger(logtest.NewJSONLogger(logs)))

			h.settle(t.Context())

			store.states = append(store.states, ownRecord(self, otherThan(failed), h.clock.now))
			h.failPeer(t.Context(), failed)

			if !h.writer.paused || len(store.calls) != 0 {
				t.Errorf("paused = %t with calls = %v, want paused with none in the pass that first sees the own record",
					h.writer.paused, store.calls)
			}

			if starts := logtest.WithMsg(logtest.Drain(t, logs), writerPausedMsg); len(starts) != 1 {
				t.Errorf("pause start logged %d times, want once", len(starts))
			}
		})
	}
}

func TestWriterWaitsOutItsOwnRecordWithoutLocalQuorum(t *testing.T) {
	const failed = "worker-3"

	self := writerFor(failed)
	store := newStore()
	h := newHarness(t, self, store)

	h.settle(t.Context())
	h.clock.advance(time.Second)

	store.states = append(store.states, ownRecord(self, otherThan(failed), h.clock.now))
	h.alive.members = []string{self}
	h.writer.reconcile(t.Context())

	if h.writer.view.HasQuorum() {
		t.Fatalf("the writer holds local quorum with alive %v, the test needs it lost", h.alive.members)
	}

	h.clock.advance(takeoverDelay)
	h.failPeer(t.Context(), failed)

	if !h.writer.view.HasQuorum() {
		t.Fatalf("the writer has no local quorum with alive %v, the test needs it back", h.alive.members)
	}

	if !h.writer.paused || len(store.calls) != 0 {
		t.Errorf("paused = %t with calls = %v, want paused with none: the own record was first seen %s earlier without quorum",
			h.writer.paused, store.calls, takeoverDelay)
	}
}
