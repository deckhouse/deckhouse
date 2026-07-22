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

package ngconfigmetrics

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(ngConfigurationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		ngConfigurationGVK.GroupVersion().WithKind("NodeGroupConfigurationList"),
		&unstructured.UnstructuredList{},
	)
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

// ngc builds a NodeGroupConfiguration. Pass nodeGroups=nil to omit spec.nodeGroups
// entirely; pass an empty slice for an explicit empty list.
func ngc(name string, nodeGroups []string) *unstructured.Unstructured {
	u := newNodeGroupConfiguration()
	u.SetName(name)
	if nodeGroups != nil {
		anyNGs := make([]any, len(nodeGroups))
		for i, ng := range nodeGroups {
			anyNGs[i] = ng
		}
		_ = unstructured.SetNestedSlice(u.Object, anyNGs, "spec", "nodeGroups")
	}
	return u
}

func reconcile(t *testing.T, r *Reconciler) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func value(t *testing.T, ng string) float64 {
	t.Helper()
	return testutil.ToFloat64(ngConfigurationsTotal.With(prometheus.Labels{"node_group": ng}))
}

func seriesCount(t *testing.T) int {
	t.Helper()
	return testutil.CollectAndCount(ngConfigurationsTotal)
}

// An NGC without spec.nodeGroups defaults to the "*" group.
func TestReconcile_NoNodeGroups_DefaultsToStar(t *testing.T) {
	r := newReconciler(t, ngc("global", nil))
	reconcile(t, r)

	if got := value(t, "*"); got != 1 {
		t.Fatalf("expected node_group=* => 1, got %v", got)
	}
}

// Multiple NGCs targeting the same group sum together.
func TestReconcile_MultipleSameGroup_Summed(t *testing.T) {
	r := newReconciler(t, ngc("a", nil), ngc("b", nil), ngc("c", []string{"worker"}))
	reconcile(t, r)

	if got := value(t, "*"); got != 2 {
		t.Fatalf("expected node_group=* => 2, got %v", got)
	}
	if got := value(t, "worker"); got != 1 {
		t.Fatalf("expected node_group=worker => 1, got %v", got)
	}
}

// An NGC targeting several groups increments each one.
func TestReconcile_ExplicitGroups_CountedEach(t *testing.T) {
	r := newReconciler(t, ngc("multi", []string{"worker", "system"}))
	reconcile(t, r)

	if got := value(t, "worker"); got != 1 {
		t.Fatalf("expected worker => 1, got %v", got)
	}
	if got := value(t, "system"); got != 1 {
		t.Fatalf("expected system => 1, got %v", got)
	}
}

// An explicit empty nodeGroups list yields no series for that configuration.
func TestReconcile_ExplicitEmpty_NoSeries(t *testing.T) {
	r := newReconciler(t, ngc("empty", []string{}))
	reconcile(t, r)

	if got := seriesCount(t); got != 0 {
		t.Fatalf("expected no series, got %d", got)
	}
}

// With no NGCs at all the gauge holds no series.
func TestReconcile_NoConfigurations_Empty(t *testing.T) {
	r := newReconciler(t)
	reconcile(t, r)

	if got := seriesCount(t); got != 0 {
		t.Fatalf("expected no series, got %d", got)
	}
}

// Reset removes series for configurations that no longer target a group.
func TestReconcile_Reset_DropsStaleSeries(t *testing.T) {
	// First converge with a worker-targeting NGC.
	r := newReconciler(t, ngc("w", []string{"worker"}))
	reconcile(t, r)
	if got := value(t, "worker"); got != 1 {
		t.Fatalf("setup: expected worker => 1, got %v", got)
	}

	// Re-converge with only a "*" NGC — the worker series must disappear.
	r2 := newReconciler(t, ngc("g", nil))
	reconcile(t, r2)

	if got := seriesCount(t); got != 1 {
		t.Fatalf("expected exactly 1 series, got %d", got)
	}
	if got := value(t, "*"); got != 1 {
		t.Fatalf("expected * => 1, got %v", got)
	}
}

// The aggregate mirrors the live-cluster baseline: three "*" NGCs plus one
// explicit parity-test-ng NGC.
func TestReconcile_BaselineParity(t *testing.T) {
	r := newReconciler(t,
		ngc("first", nil),
		ngc("second", nil),
		ngc("third", nil),
		ngc("parity", []string{"parity-test-ng"}),
	)
	reconcile(t, r)

	if got := value(t, "*"); got != 3 {
		t.Fatalf("expected * => 3, got %v", got)
	}
	if got := value(t, "parity-test-ng"); got != 1 {
		t.Fatalf("expected parity-test-ng => 1, got %v", got)
	}
}
