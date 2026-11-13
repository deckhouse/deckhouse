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
	"context"
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	fcache "k8s.io/client-go/tools/cache/testing"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// TestNodeChangeReleasesTheReturnedNode covers a Node that came back while its
// incident is Evicted. Nothing writes the object, and nothing requeues it. The
// update of the Node starts the reconcile, so the controller removes the taint
// and deletes the object.
func TestNodeChangeReleasesTheReturnedNode(t *testing.T) {
	incident := failedState()
	incident.Status.Phase = v1alpha1.PhaseEvicted

	node := returnedNode()
	node.Spec.Taints = []corev1.Taint{outOfServiceTaint}

	h := newHarness(t, incident, node)
	h.reconciler.now = func() time.Time { return observedAt.Add(time.Second) }

	// The informers get the objects from fake sources. The manager does not
	// start them, so the test runs them.
	nodes := fcache.NewFakeControllerSource()
	nodes.Add(node)

	informers := &informertest.FakeInformers{
		Scheme: newScheme(t),
		InformersByGVK: map[schema.GroupVersionKind]toolscache.SharedIndexInformer{
			corev1.SchemeGroupVersion.WithKind("Node"): toolscache.NewSharedIndexInformer(nodes, &corev1.Node{}, 0, toolscache.Indexers{}),
			v1alpha1.GroupVersion.WithKind("FencingFailedNodeState"): toolscache.NewSharedIndexInformer(
				fcache.NewFakeControllerSource(), &v1alpha1.FencingFailedNodeState{}, 0, toolscache.Indexers{}),
		},
	}

	mgr, err := ctrl.NewManager(&rest.Config{Host: "http://127.0.0.1:1"}, ctrl.Options{
		Scheme:   newScheme(t),
		NewCache: func(*rest.Config, cache.Options) (cache.Cache, error) { return informers, nil },
		NewClient: func(*rest.Config, client.Options) (client.Client, error) {
			return h.client, nil
		},
		MapperProvider: func(*rest.Config, *http.Client) (meta.RESTMapper, error) { return h.api.RESTMapper(), nil },
		Metrics:        metricsserver.Options{BindAddress: "0"},
		Controller:     config.Controller{SkipNameValidation: ptr.To(true)},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}

	if err := h.reconciler.SetupWithManager(mgr); err != nil {
		t.Fatalf("set up the controller: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan error)

	for _, informer := range informers.InformersByGVK {
		go informer.Run(ctx.Done())
	}

	go func() { stopped <- mgr.Start(ctx) }()

	defer func() {
		cancel()

		if err := <-stopped; err != nil {
			t.Errorf("manager stopped with %v", err)
		}
	}()

	// The kubelet updates the Node. The fake source sends the update until the
	// controller started its watch.
	deadline := time.Now().Add(10 * time.Second)
	for h.exists(&v1alpha1.FencingFailedNodeState{}, "", nodeName) && time.Now().Before(deadline) {
		nodes.Modify(node.DeepCopy())
		time.Sleep(50 * time.Millisecond)
	}

	h.assertIncidentGone()
	h.assertNotTaintedAnyMore()
}

// TestNodeChangeOfANodeWithoutAnIncident covers a Node that has no incident.
// Its change starts no reconcile.
func TestNodeChangeOfANodeWithoutAnIncident(t *testing.T) {
	h := newHarness(t, liveNode())

	if got := h.reconciler.incidentOfNode(t.Context(), liveNode()); len(got) != 0 {
		t.Errorf("requests are %v, want none", got)
	}
}

func (h *harness) assertNotTaintedAnyMore() {
	h.t.Helper()

	if h.tainted() {
		h.t.Errorf("node has the out-of-service taint")
	}
}
