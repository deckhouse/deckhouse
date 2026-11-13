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

package nodestatus

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/metrics"
	"fencing-agent/internal/domain"
)

const nodeName = "w1"

type recordedEvent struct{ kind, reason, message string }

type fakeEvents struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (f *fakeEvents) Normal(reason, message string)  { f.add("Normal", reason, message) }
func (f *fakeEvents) Warning(reason, message string) { f.add("Warning", reason, message) }

func (f *fakeEvents) add(kind, reason, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.events = append(f.events, recordedEvent{kind, reason, message})
}

func newWriter(t *testing.T, node *corev1.Node) (*Writer, *fake.Clientset, *fakeEvents) {
	t.Helper()

	client := fake.NewClientset(node)
	events := &fakeEvents{}

	return New(client, nodeName, events, metrics.New(nodeName), log.NewNop()), client, events
}

func node1() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"node-manager.deckhouse.io/fencing-enabled": "",
				"node-manager.deckhouse.io/fencing-mode":    "Watchdog",
				"keep":                                      "me",
			},
		},
		Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
	}
}

func getNode(t *testing.T, client *fake.Clientset) *corev1.Node {
	t.Helper()

	node, err := client.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get node: %v", err)
	}

	return node
}

func fencingCondition(t *testing.T, node *corev1.Node) corev1.NodeCondition {
	t.Helper()

	for _, c := range node.Status.Conditions {
		if c.Type == conditionType {
			return c
		}
	}

	t.Fatalf("node has no Fencing condition: %+v", node.Status.Conditions)

	return corev1.NodeCondition{}
}

func patchCount(client *fake.Clientset, subresource string) int {
	n := 0

	for _, action := range client.Actions() {
		if patch, ok := action.(clienttesting.PatchAction); ok && patch.GetSubresource() == subresource {
			n++
		}
	}

	return n
}

func TestRemoveLegacyLabels(t *testing.T) {
	w, client, _ := newWriter(t, node1())

	if err := w.RemoveLegacyLabels(context.Background()); err != nil {
		t.Fatalf("remove legacy labels: %v", err)
	}

	if labels := getNode(t, client).Labels; len(labels) != 1 || labels["keep"] != "me" {
		t.Errorf("labels are %v, want only keep=me", labels)
	}
}

func TestLegacyLabelsAreRemovedOnce(t *testing.T) {
	w, client, _ := newWriter(t, node1())

	if err := w.RemoveLegacyLabels(context.Background()); err != nil {
		t.Fatalf("remove legacy labels: %v", err)
	}

	w.SetProfile("slow")

	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var patches []string

	for _, action := range client.Actions() {
		if patch, ok := action.(clienttesting.PatchAction); ok {
			patches = append(patches, string(patch.GetPatch()))
		}
	}

	want := []string{
		`{"metadata":{"labels":{"node-manager.deckhouse.io/fencing-enabled":null,"node-manager.deckhouse.io/fencing-mode":null}}}`,
		`{"metadata":{"annotations":{"` + ProfileAnnotation + `":"slow"}}}`,
	}
	if !slices.Equal(patches, want) {
		t.Errorf("patches are\n%q\nwant\n%q", patches, want)
	}
}

func TestStateIsPublishedWithOneMetadataPatch(t *testing.T) {
	w, client, _ := newWriter(t, node1())

	w.SetCondition(domain.ReasonArmed, "watchdog is armed")
	w.SetProfile("slow")
	w.SetCPSource(domain.CPSourceProxy)

	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	node := getNode(t, client)

	if node.Annotations[ProfileAnnotation] != "slow" || node.Annotations[CPSourceAnnotation] != "proxy" {
		t.Errorf("annotations are %v", node.Annotations)
	}

	if got := patchCount(client, ""); got != 1 {
		t.Errorf("got %d metadata patches, want 1", got)
	}

	c := fencingCondition(t, node)
	if c.Status != corev1.ConditionTrue || c.Reason != "Armed" || c.Message != "watchdog is armed" {
		t.Errorf("condition is %+v", c)
	}

	if len(node.Status.Conditions) != 2 {
		t.Errorf("the other conditions are lost: %+v", node.Status.Conditions)
	}

	// Nothing changed: no more writes.
	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if patchCount(client, "")+patchCount(client, "status") != 2 {
		t.Errorf("a flush without changes wrote to the node")
	}
}

func TestCPSourceAnnotationFollowsTheSource(t *testing.T) {
	w, client, _ := newWriter(t, node1())

	w.SetProfile("slow")
	w.SetCPSource(domain.CPSourceSelf)

	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	w.SetCPSource(domain.CPSourceProxy)

	if err := w.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if got := getNode(t, client).Annotations[CPSourceAnnotation]; got != "proxy" {
		t.Errorf("cp source annotation is %q, want proxy", got)
	}
}

func TestConditionStatusAndTransitionTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, client, _ := newWriter(t, node1())
		ctx := context.Background()

		flush := func(reason domain.Reason) corev1.NodeCondition {
			time.Sleep(time.Minute)
			w.SetCondition(reason, "")

			if err := w.Flush(ctx); err != nil {
				t.Fatalf("flush: %v", err)
			}

			return fencingCondition(t, getNode(t, client))
		}

		waiting := flush(domain.ReasonWaitingForControlPlane)
		noTarget := flush(domain.ReasonNoEvacuationTarget)
		armed := flush(domain.ReasonArmed)

		if waiting.Status != corev1.ConditionFalse || armed.Status != corev1.ConditionTrue {
			t.Errorf("statuses are %s and %s", waiting.Status, armed.Status)
		}

		if noTarget.Reason != "NoEvacuationTarget" || !noTarget.LastTransitionTime.Equal(&waiting.LastTransitionTime) {
			t.Errorf("a new reason with the same status moved the transition time: %v -> %v",
				waiting.LastTransitionTime, noTarget.LastTransitionTime)
		}

		if !armed.LastTransitionTime.After(noTarget.LastTransitionTime.Time) {
			t.Errorf("a new status did not move the transition time: %v -> %v",
				noTarget.LastTransitionTime, armed.LastTransitionTime)
		}
	})
}

func TestEventsOnReasonChangeOnly(t *testing.T) {
	w, _, events := newWriter(t, node1())

	w.SetCondition(domain.ReasonWaitingForControlPlane, "")
	w.SetCondition(domain.ReasonWaitingForControlPlane, "still waiting")
	w.SetCondition(domain.ReasonArmed, "watchdog is armed")
	w.SetCondition(domain.ReasonKernelPanicMismatch, "kernel.panic is 10")
	w.SetCondition(domain.ReasonWatchdogDeviceMissing, "no device")

	want := []recordedEvent{
		{"Normal", "WaitingForControlPlane", "WaitingForControlPlane"},
		{"Normal", "Armed", "watchdog is armed"},
		{"Warning", "KernelPanicMismatch", "kernel.panic is 10"},
		{"Warning", "WatchdogDeviceMissing", "no device"},
	}

	if len(events.events) != len(want) {
		t.Fatalf("events are %v, want %v", events.events, want)
	}

	for i := range want {
		if events.events[i] != want[i] {
			t.Errorf("event %d is %v, want %v", i, events.events[i], want[i])
		}
	}
}

func TestRunRetriesAndWritesTheLastState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, client, _ := newWriter(t, node1())

		failures := 3
		client.PrependReactor("patch", "nodes", func(clienttesting.Action) (bool, runtime.Object, error) {
			if failures > 0 {
				failures--

				return true, nil, errors.New("api is down")
			}

			return false, nil, nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			defer close(done)
			w.Run(ctx)
		}()

		w.SetCPSource(domain.CPSourceSelf)
		synctest.Wait()

		if got := getNode(t, client).Annotations[CPSourceAnnotation]; got != "" {
			t.Fatalf("the annotation is %q while the API fails", got)
		}

		// The state changes while the write is failing: only the last one must be written.
		w.SetCPSource(domain.CPSourceProxy)
		w.SetCondition(domain.ReasonArmed, "")

		time.Sleep(time.Minute)
		synctest.Wait()

		node := getNode(t, client)
		if node.Annotations[CPSourceAnnotation] != "proxy" || fencingCondition(t, node).Reason != "Armed" {
			t.Errorf("the last state is not written: %v, %+v", node.Annotations, node.Status.Conditions)
		}

		cancel()
		<-done
	})
}

func TestSettersDoNotBlockWhileTheAPIFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, client, _ := newWriter(t, node1())

		client.PrependReactor("patch", "nodes", func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("api is down")
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			defer close(done)
			w.Run(ctx)
		}()

		for range 100 {
			w.SetCondition(domain.ReasonWaitingForControlPlane, "")
			w.SetCondition(domain.ReasonArmed, "")
			w.SetCPSource(domain.CPSourceProxy)
			w.SetCPSource(domain.CPSourceSelf)
		}

		cancel()
		<-done
	})
}

// The start of the agent removes the 1.0 labels before the arm. An API server
// that accepts the request and does not answer must not stop the start.
func TestRemoveLegacyLabelsHasATimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	client, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	defer func(d time.Duration) { writeTimeout = d }(writeTimeout)
	writeTimeout = 100 * time.Millisecond

	w := New(client, nodeName, &fakeEvents{}, metrics.New(nodeName), log.NewNop())

	start := time.Now()
	if err := w.RemoveLegacyLabels(context.Background()); err == nil {
		t.Fatal("RemoveLegacyLabels returned nil without an answer of the API server")
	}

	if d := time.Since(start); d > 5*writeTimeout {
		t.Fatalf("RemoveLegacyLabels returned after %v, want about %v", d, writeTimeout)
	}
}

func TestConditionIsLoggedOncePerChange(t *testing.T) {
	var out bytes.Buffer

	w, _, _ := newWriter(t, node1())
	w.logger = log.NewLogger(log.WithOutput(&out))

	w.SetCondition(domain.ReasonArmed, "watchdog is armed")
	w.SetCondition(domain.ReasonArmed, "watchdog is armed")
	w.SetCondition(domain.ReasonArmed, "the feed is stopped")

	if got := strings.Count(out.String(), "fencing condition"); got != 2 {
		t.Fatalf("log has %d records, want 2: %q", got, out.String())
	}
}

// WriteCondition writes the condition at once. A failed write does not come to
// the Node later: Run and Flush do not write it.
func TestWriteCondition(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, client, _ := newWriter(t, node1())

		var down atomic.Bool
		client.PrependReactor("patch", "nodes", func(clienttesting.Action) (bool, runtime.Object, error) {
			if down.Load() {
				return true, nil, errors.New("api is down")
			}

			return false, nil, nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			defer close(done)
			w.Run(ctx)
		}()

		w.SetCondition(domain.ReasonArmed, "")
		synctest.Wait()

		down.Store(true)

		if err := w.WriteCondition(ctx, domain.ReasonAgentStopping, ""); err == nil {
			t.Fatal("WriteCondition returned nil while the API fails")
		}

		down.Store(false)

		if err := w.Flush(ctx); err != nil {
			t.Fatal(err)
		}

		time.Sleep(time.Minute)
		synctest.Wait()

		if got := fencingCondition(t, getNode(t, client)).Reason; got != string(domain.ReasonArmed) {
			t.Fatalf("reason = %q after the failed write, want Armed", got)
		}

		if err := w.WriteCondition(ctx, domain.ReasonAgentStopping, ""); err != nil {
			t.Fatal(err)
		}

		if got := fencingCondition(t, getNode(t, client)).Reason; got != string(domain.ReasonAgentStopping) {
			t.Errorf("reason = %q, want AgentStopping", got)
		}

		cancel()
		<-done
	})
}
