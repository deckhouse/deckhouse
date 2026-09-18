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
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

var ownRecordStart = time.Date(2026, time.September, 16, 15, 0, 0, 700_000_000, time.UTC)

const ownRecordNode = "worker-1"

type ownFailedRecordRow struct {
	name   string
	states []v1alpha1.FencingFailedNodeState
	want   int
}

func failedRecordAt(name string, uid types.UID, created time.Time) v1alpha1.FencingFailedNodeState {
	return v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid, CreationTimestamp: metav1.NewTime(created)},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{DetectedBy: "worker-2"},
		},
	}
}

func fallbackOnlyRecordAt(name string, uid types.UID, created time.Time) v1alpha1.FencingFailedNodeState {
	return v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid, CreationTimestamp: metav1.NewTime(created)},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Fallback: &v1alpha1.FencingFailedNodeStateFallback{Active: true, APIReachable: true},
		},
	}
}

var ownFailedRecordRows = []ownFailedRecordRow{
	{
		name:   "own failed record created in the start second",
		states: []v1alpha1.FencingFailedNodeState{failedRecordAt(ownRecordNode, "own-150000", clockAt(15, 0, 0))},
		want:   0,
	},
	{
		name:   "own failed record created after the start",
		states: []v1alpha1.FencingFailedNodeState{failedRecordAt(ownRecordNode, "own-150001", clockAt(15, 0, 1))},
		want:   0,
	},
	{
		name:   "own failed record from an earlier life",
		states: []v1alpha1.FencingFailedNodeState{failedRecordAt(ownRecordNode, "own-145959", clockAt(14, 59, 59))},
		want:   -1,
	},
	{
		name:   "own fresh record without a failed verdict",
		states: []v1alpha1.FencingFailedNodeState{fallbackOnlyRecordAt(ownRecordNode, "own-fallback", clockAt(15, 0, 1))},
		want:   -1,
	},
	{
		name:   "fresh failed record of another node",
		states: []v1alpha1.FencingFailedNodeState{failedRecordAt("worker-2", "peer-150001", clockAt(15, 0, 1))},
		want:   -1,
	},
	{
		name:   "no records",
		states: nil,
		want:   -1,
	},
	{
		name: "own failed record behind a fresh failed record of another node",
		states: []v1alpha1.FencingFailedNodeState{
			failedRecordAt("worker-2", "peer-150001", clockAt(15, 0, 1)),
			failedRecordAt(ownRecordNode, "own-150002", clockAt(15, 0, 2)),
		},
		want: 1,
	},
}

func clockAt(hour, minute, second int) time.Time {
	return time.Date(2026, time.September, 16, hour, minute, second, 0, time.UTC)
}

func TestOwnFailedRecord(t *testing.T) {
	startedAt := StartOfLife(ownRecordStart)

	for _, row := range ownFailedRecordRows {
		t.Run(row.name, func(t *testing.T) {
			got := OwnFailedRecord(row.states, ownRecordNode, startedAt)

			checkOwnFailedRecord(t, row.states, row.want, got)
		})
	}
}

func TestOwnFailedRecordIgnoresDeletionTimestamp(t *testing.T) {
	deleting := failedRecordAt(ownRecordNode, "own-deleting", clockAt(15, 0, 1))
	deletedAt := metav1.NewTime(clockAt(15, 0, 2))
	deleting.DeletionTimestamp = &deletedAt
	states := []v1alpha1.FencingFailedNodeState{deleting}

	got := OwnFailedRecord(states, ownRecordNode, StartOfLife(ownRecordStart))

	checkOwnFailedRecord(t, states, 0, got)
}

func checkOwnFailedRecord(t *testing.T, states []v1alpha1.FencingFailedNodeState, want int, got *v1alpha1.FencingFailedNodeState) {
	t.Helper()

	if want < 0 {
		if got != nil {
			t.Fatalf("OwnFailedRecord = %s/%s, want nil", got.Name, got.UID)
		}

		return
	}

	wantRecord := &states[want]
	if got == nil {
		t.Fatalf("OwnFailedRecord = nil, want %s/%s", wantRecord.Name, wantRecord.UID)
	}

	if got.Name != wantRecord.Name || got.UID != wantRecord.UID {
		t.Errorf("OwnFailedRecord = %s/%s, want %s/%s", got.Name, got.UID, wantRecord.Name, wantRecord.UID)
	} else if got != wantRecord {
		t.Errorf("OwnFailedRecord returned a copy of %s/%s, want a pointer into the given slice", got.Name, got.UID)
	}
}

func TestStartOfLifeTruncatesToTheSecond(t *testing.T) {
	cases := []struct {
		now  time.Time
		want time.Time
	}{
		{now: time.Date(2026, time.September, 16, 15, 0, 0, 999_999_000, time.UTC), want: clockAt(15, 0, 0)},
		{now: time.Date(2026, time.September, 16, 15, 0, 7, 999_999_000, time.UTC), want: clockAt(15, 0, 7)},
	}

	for _, c := range cases {
		if got := StartOfLife(c.now); !got.Equal(c.want) {
			t.Errorf("StartOfLife(%s) = %s, want %s", c.now.Format(time.RFC3339Nano), got.Format(time.RFC3339Nano), c.want.Format(time.RFC3339Nano))
		}
	}
}

func TestWriterClearsByTheStartItIsGiven(t *testing.T) {
	const self = "worker-1"

	var (
		startedAt = clockAt(15, 0, 0)
		created   = clockAt(15, 0, 2)
		builtAt   = clockAt(15, 0, 5)
	)

	t.Run("with the start of the process the record stays", func(t *testing.T) {
		store := newStore(failedRecordAt(self, "cr-"+self, created))
		h := newHarnessWith(t, nodeGroupSize, self, store, withClockAt(builtAt), withStartedAt(startedAt))

		h.settle(t.Context())

		if len(store.calls) != 0 {
			t.Errorf("calls = %v, want none: the record was written during this life", store.calls)
		}
	})

	t.Run("without a start the writer falls back to its own clock", func(t *testing.T) {
		store := newStore(failedRecordAt(self, "cr-"+self, created))
		h := newHarnessWith(t, nodeGroupSize, self, store, withClockAt(builtAt))

		h.settle(t.Context())

		if want := []string{"delete:" + self + ":cr-" + self}; !slices.Equal(store.calls, want) {
			t.Errorf("calls = %v, want %v: the record predates the moment New was called", store.calls, want)
		}
	})

	t.Run("the fallback start is truncated to the second a record can carry", func(t *testing.T) {
		store := newStore(failedRecordAt(self, "cr-"+self, builtAt))
		h := newHarnessWith(t, nodeGroupSize, self, store, withClockAt(builtAt.Add(700*time.Millisecond)))

		h.settle(t.Context())

		if len(store.calls) != 0 {
			t.Errorf("calls = %v, want none: a record created in the second New was called is about this life", store.calls)
		}
	})
}
