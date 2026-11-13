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
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
)

func selfNode(name string, annotations map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			UID:         types.UID("uid-" + name),
			Labels:      map[string]string{domain.NodeGroupLabel: "worker"},
			Annotations: annotations,
		},
		Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
			{Type: corev1.NodeExternalIP, Address: "203.0.113.1"},
			{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			{Type: corev1.NodeHostName, Address: name},
		}},
	}
}

func startSelfWatcher(t *testing.T, client *fake.Clientset, nodeName string) *SelfWatcher {
	t.Helper()

	watcher, err := NewSelfWatcher(client, nodeName, log.NewNop())
	if err != nil {
		t.Fatalf("create self watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	go watcher.Run(ctx)

	if !watcher.WaitForSync(ctx) {
		t.Fatal("own node cache did not sync")
	}

	return watcher
}

func eventually(t *testing.T, w *SelfWatcher, check func(domain.NodeSignals) bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if signals, ok := w.Snapshot(); ok && check(signals) {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	signals, ok := w.Snapshot()
	t.Fatalf("%s; final state: %+v (observed %v)", msg, signals, ok)
}

func TestSelfWatcherReportsTheOwnNode(t *testing.T) {
	client := fake.NewClientset(selfNode("worker-1", nil))

	w := startSelfWatcher(t, client, "worker-1")

	eventually(t, w, func(s domain.NodeSignals) bool {
		return s.UID == "uid-worker-1" &&
			s.NodeGroup == "worker" &&
			slices.Equal(s.InternalIPs, []string{"10.0.0.1"}) &&
			slices.Equal(s.ExternalIPs, []string{"203.0.113.1"}) &&
			!s.Maintenance
	}, "the own Node must reach the snapshot")

	moved := selfNode("worker-1", nil)
	moved.Labels[domain.NodeGroupLabel] = "worker-2"

	if _, err := client.CoreV1().Nodes().Update(t.Context(), moved, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update node: %v", err)
	}

	eventually(t, w, func(s domain.NodeSignals) bool {
		return s.NodeGroup == "worker-2"
	}, "a new group label must reach the snapshot")
}

func TestSelfWatcherReportsEveryMaintenanceAnnotation(t *testing.T) {
	for _, annotation := range domain.MaintenanceAnnotations() {
		t.Run(annotation, func(t *testing.T) {
			client := fake.NewClientset(selfNode("worker-1", map[string]string{annotation: ""}))

			w := startSelfWatcher(t, client, "worker-1")

			eventually(t, w, func(s domain.NodeSignals) bool {
				return s.Maintenance && slices.Equal(s.MaintenanceReasons, []string{annotation})
			}, "annotation "+annotation+" must set maintenance")

			if _, err := client.CoreV1().Nodes().Update(t.Context(), selfNode("worker-1", nil), metav1.UpdateOptions{}); err != nil {
				t.Fatalf("update node: %v", err)
			}

			eventually(t, w, func(s domain.NodeSignals) bool {
				return !s.Maintenance && len(s.MaintenanceReasons) == 0
			}, "removing the annotation must clear maintenance")
		})
	}
}

func TestSelfWatcherIgnoresOtherNodes(t *testing.T) {
	client := fake.NewClientset(selfNode("worker-1", nil))

	w := startSelfWatcher(t, client, "worker-1")

	eventually(t, w, func(domain.NodeSignals) bool { return true }, "the own Node must be observed")

	foreign := selfNode("worker-2", map[string]string{domain.FencingDisableAnnotation: ""})
	if _, err := client.CoreV1().Nodes().Create(t.Context(), foreign, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create node: %v", err)
	}

	// Give the informer time to deliver the foreign event.
	time.Sleep(200 * time.Millisecond)

	if s, _ := w.Snapshot(); s.Maintenance || s.UID != "uid-worker-1" {
		t.Errorf("snapshot %+v, want only the own Node", s)
	}
}

func TestSelfWatcherSignalsAChange(t *testing.T) {
	client := fake.NewClientset(selfNode("worker-1", nil))

	w := startSelfWatcher(t, client, "worker-1")

	waitChanged := func(msg string) {
		t.Helper()

		select {
		case <-w.Changed():
		case <-time.After(5 * time.Second):
			t.Fatal(msg)
		}
	}

	waitChanged("the first observation must signal a change")

	if _, err := client.CoreV1().Nodes().Update(t.Context(), selfNode("worker-1", map[string]string{domain.ApprovedAnnotation: ""}), metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update node: %v", err)
	}

	waitChanged("a new maintenance annotation must signal a change")

	// An update that does not change the signals does not signal.
	same := selfNode("worker-1", map[string]string{domain.ApprovedAnnotation: "", "other": "x"})
	if _, err := client.CoreV1().Nodes().Update(t.Context(), same, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("update node: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	select {
	case <-w.Changed():
		t.Fatal("an update without new signals signalled a change")
	default:
	}
}
