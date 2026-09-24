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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func reset(t *testing.T) {
	t.Helper()
	updateApprovalNodeGroupNodeStatus.Reset()
	publishedMu.Lock()
	publishedNodes = map[string]map[string]struct{}{}
	publishedMu.Unlock()
}

func seriesFor(t *testing.T, nodeName string) int {
	t.Helper()

	ch := make(chan prometheus.Metric, 1024)
	updateApprovalNodeGroupNodeStatus.Collect(ch)
	close(ch)

	var count int
	for metric := range ch {
		var decoded dto.Metric
		if err := metric.Write(&decoded); err != nil {
			t.Fatalf("read metric: %v", err)
		}
		for _, label := range decoded.Label {
			if label.GetName() == "node" && label.GetValue() == nodeName {
				count++
			}
		}
	}
	return count
}

func TestPruneDropsTheSeriesOfANodeThatLeftTheGroup(t *testing.T) {
	reset(t)

	SetNodeStatusMetrics("old-name", "worker", "UpToDate")
	SetNodeStatusMetrics("new-name", "worker", "UpToDate")

	if seriesFor(t, "old-name") == 0 || seriesFor(t, "new-name") == 0 {
		t.Fatal("both nodes should have series before pruning")
	}

	// What a rename leaves behind: the group now holds only the new name.
	PruneNodeStatusMetrics("worker", map[string]struct{}{"new-name": {}})

	if got := seriesFor(t, "old-name"); got != 0 {
		t.Fatalf("the departed node still has %d series", got)
	}
	if got := seriesFor(t, "new-name"); got == 0 {
		t.Fatal("pruning removed the series of a node that is still in the group")
	}
}

func TestPruneLeavesOtherGroupsAlone(t *testing.T) {
	reset(t)

	SetNodeStatusMetrics("node-a", "worker", "UpToDate")
	SetNodeStatusMetrics("node-b", "system", "UpToDate")

	PruneNodeStatusMetrics("worker", nil)

	if got := seriesFor(t, "node-a"); got != 0 {
		t.Fatalf("the pruned group still has %d series", got)
	}
	if got := seriesFor(t, "node-b"); got == 0 {
		t.Fatal("pruning one group removed the series of another")
	}
}

func TestPruneOfADeletedGroupDropsEverything(t *testing.T) {
	reset(t)

	SetNodeStatusMetrics("node-a", "worker", "UpToDate")
	SetNodeStatusMetrics("node-b", "worker", "Approved")

	PruneNodeStatusMetrics("worker", nil)

	if got := testutil.CollectAndCount(updateApprovalNodeGroupNodeStatus); got != 0 {
		t.Fatalf("expected no series left, got %d", got)
	}

	publishedMu.Lock()
	_, tracked := publishedNodes["worker"]
	publishedMu.Unlock()
	if tracked {
		t.Fatal("the bookkeeping for an empty group should not be kept")
	}
}

func TestPruneOfAnUnknownGroupIsHarmless(t *testing.T) {
	reset(t)

	SetNodeStatusMetrics("node-a", "worker", "UpToDate")
	PruneNodeStatusMetrics("never-seen", nil)

	if got := seriesFor(t, "node-a"); got == 0 {
		t.Fatal("pruning an unknown group removed series of a known one")
	}
}
