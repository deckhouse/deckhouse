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

package kubeclient

import (
	"context"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

type selfRecorder struct {
	mu      sync.Mutex
	signals domain.NodeSignals
	seen    int
	deleted int
}

func (r *selfRecorder) Observe(signals domain.NodeSignals) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.signals = signals
	r.seen++
}

func (r *selfRecorder) Deleted() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deleted++
}

func (r *selfRecorder) snapshot() (domain.NodeSignals, int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.signals, r.seen, r.deleted
}

func (r *selfRecorder) eventually(t *testing.T, check func(domain.NodeSignals, int, int) bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if check(r.snapshot()) {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	signals, seen, deleted := r.snapshot()
	t.Fatalf("%s; final state: %+v (observed %d, deleted %d)", msg, signals, seen, deleted)
}

func selfNode(name string, annotations map[string]string) *corev1.Node {
	node := node(name, "worker", internal("10.0.0.1"))
	node.Annotations = annotations

	return node
}

func startSelfWatcher(t *testing.T, client *fake.Clientset, nodeName string) *selfRecorder {
	t.Helper()

	recorder := &selfRecorder{}

	watcher, err := NewSelfWatcher(client, nodeName, recorder, log.NewNop())
	if err != nil {
		t.Fatalf("create self watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	go watcher.Run(ctx)

	if !watcher.WaitForSync(ctx) {
		t.Fatal("own node cache did not sync")
	}

	return recorder
}

func TestSelfWatcherReportsMaintenanceAnnotations(t *testing.T) {
	client := fake.NewSimpleClientset(objects(
		selfNode("worker-1", map[string]string{domain.ApprovedAnnotation: ""}),
	)...)

	recorder := startSelfWatcher(t, client, "worker-1")

	recorder.eventually(t, func(signals domain.NodeSignals, _, _ int) bool {
		return signals.Maintenance &&
			len(signals.MaintenanceReasons) == 1 &&
			signals.MaintenanceReasons[0] == domain.ApprovedAnnotation &&
			signals.UID == "uid-worker-1"
	}, "the maintenance annotation of the own Node must reach the store")

	// Annotation removed: fencing must be allowed again.
	updated := selfNode("worker-1", nil)
	if _, err := client.CoreV1().Nodes().Update(t.Context(), updated, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update node: %v", err)
	}

	recorder.eventually(t, func(signals domain.NodeSignals, _, _ int) bool {
		return !signals.Maintenance && len(signals.MaintenanceReasons) == 0
	}, "removing the annotation must reach the store")
}

func TestSelfWatcherReportsEveryMaintenanceAnnotation(t *testing.T) {
	for _, annotation := range domain.MaintenanceAnnotations() {
		t.Run(annotation, func(t *testing.T) {
			client := fake.NewSimpleClientset(objects(
				selfNode("worker-1", map[string]string{annotation: ""}),
			)...)

			recorder := startSelfWatcher(t, client, "worker-1")

			recorder.eventually(t, func(signals domain.NodeSignals, _, _ int) bool {
				return signals.Maintenance
			}, "annotation "+annotation+" must disable fencing")
		})
	}
}

func TestSelfWatcherReportsPlannedRemoval(t *testing.T) {
	client := fake.NewSimpleClientset(objects(selfNode("worker-1", nil))...)

	recorder := startSelfWatcher(t, client, "worker-1")

	recorder.eventually(t, func(signals domain.NodeSignals, seen, _ int) bool {
		return seen > 0 && !signals.PlannedRemoval
	}, "a healthy Node must not look like a removal")

	tainted := selfNode("worker-1", nil)
	tainted.Spec.Taints = []corev1.Taint{{
		Key:    domain.ClusterAutoscalerDeleteTaint,
		Effect: corev1.TaintEffectNoSchedule,
	}}

	if _, err := client.CoreV1().Nodes().Update(t.Context(), tainted, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update node: %v", err)
	}

	recorder.eventually(t, func(signals domain.NodeSignals, _, _ int) bool {
		return signals.PlannedRemoval && signals.RemovalReason == domain.RemovalReasonAutoscaler
	}, "the cluster-autoscaler taint must be reported as a planned removal")
}

func TestSelfWatcherReportsDeletionTimestamp(t *testing.T) {
	deleting := selfNode("worker-1", nil)
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	deleting.Finalizers = []string{"test.deckhouse.io/hold"}

	client := fake.NewSimpleClientset(objects(deleting)...)

	recorder := startSelfWatcher(t, client, "worker-1")

	recorder.eventually(t, func(signals domain.NodeSignals, _, _ int) bool {
		return signals.PlannedRemoval && signals.RemovalReason == domain.RemovalReasonDeleting
	}, "a deletionTimestamp on the own Node must be reported as a planned removal")
}

func TestSelfWatcherReportsDeletion(t *testing.T) {
	client := fake.NewSimpleClientset(objects(selfNode("worker-1", nil))...)

	recorder := startSelfWatcher(t, client, "worker-1")

	recorder.eventually(t, func(_ domain.NodeSignals, seen, _ int) bool {
		return seen > 0
	}, "the own Node must be observed first")

	if err := client.CoreV1().Nodes().Delete(t.Context(), "worker-1", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete node: %v", err)
	}

	recorder.eventually(t, func(_ domain.NodeSignals, _, deleted int) bool {
		return deleted == 1
	}, "deleting the own Node must reach the store")
}

// The field selector is a server-side filter; a watchdog decision must not
// depend on the API honouring it, so the watcher re-checks the name itself.
func TestSelfWatcherIgnoresOtherNodes(t *testing.T) {
	client := fake.NewSimpleClientset(objects(selfNode("worker-1", nil))...)

	recorder := startSelfWatcher(t, client, "worker-1")

	recorder.eventually(t, func(_ domain.NodeSignals, seen, _ int) bool {
		return seen == 1
	}, "the own Node must be observed once")

	foreign := selfNode("worker-2", map[string]string{domain.FencingDisableAnnotation: ""})
	if _, err := client.CoreV1().Nodes().Create(t.Context(), foreign, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	if err := client.CoreV1().Nodes().Delete(t.Context(), "worker-2", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("delete node: %v", err)
	}

	// Give the informer a chance to deliver the foreign events before asserting.
	time.Sleep(200 * time.Millisecond)

	signals, seen, deleted := recorder.snapshot()
	if seen != 1 || deleted != 0 || signals.Maintenance {
		t.Errorf("store saw %d observations and %d deletions with signals %+v, want only the own Node", seen, deleted, signals)
	}
}
