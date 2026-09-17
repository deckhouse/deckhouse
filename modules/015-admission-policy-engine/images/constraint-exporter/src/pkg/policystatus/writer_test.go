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

package policystatus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
)

func policyGVK(kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: policyGroup, Version: policyVersion, Kind: kind}
}

func policyObject(kind, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(policyGVK(kind))
	obj.SetName(name)
	return obj
}

func newTestWriter(t *testing.T, at time.Time, objects ...client.Object) (*Writer, client.Client) {
	t.Helper()

	testScheme := runtime.NewScheme()
	for _, kind := range []string{gatekeeper.SecurityPolicyKind, gatekeeper.OperationPolicyKind} {
		gvk := policyGVK(kind)
		testScheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		testScheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(kind+"List"), &unstructured.UnstructuredList{})
	}

	fakeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(objects...).Build()

	writer := NewWriter(fakeClient)
	writer.now = func() time.Time { return at }

	return writer, fakeClient
}

func readViolations(t *testing.T, c client.Client, kind, name string) map[string]interface{} {
	t.Helper()

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(policyGVK(kind))
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{Name: name}, obj))

	violations, _, err := unstructured.NestedMap(obj.Object, "status", "violations")
	require.NoError(t, err)
	return violations
}

func TestSyncStoresTheSummary(t *testing.T) {
	at := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	writer, c := newTestWriter(t, at, policyObject(gatekeeper.SecurityPolicyKind, "app"))

	summary := Summary{
		Total:         3,
		ByEnforcement: map[string]int64{"deny": 3},
		TopNamespaces: []NamespaceCount{{Namespace: "prod", Count: 3}},
		Sample:        []SampleViolation{{Kind: "Pod", Namespace: "prod", Name: "web", Rule: "D8HostNetwork"}},
	}

	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{
		{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}: summary,
	}))

	stored := readViolations(t, c, gatekeeper.SecurityPolicyKind, "app")
	assert.Equal(t, int64(3), stored["total"])
	assert.Equal(t, at.Format(time.RFC3339), stored[lastUpdateTimeField])
	assert.Equal(t, map[string]interface{}{"deny": int64(3)}, stored["byEnforcement"])
}

func TestSyncLeavesAnUnchangedSummaryAlone(t *testing.T) {
	// Rewriting an unchanged summary would store a new revision of the policy in etcd every audit
	// cycle, which is the cost this section is designed to avoid.
	first := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	writer, c := newTestWriter(t, first, policyObject(gatekeeper.SecurityPolicyKind, "app"))

	owner := Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}
	summary := Summary{Total: 2, ByEnforcement: map[string]int64{"warn": 2}}

	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{owner: summary}))

	writer.now = func() time.Time { return first.Add(time.Hour) }
	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{owner: summary}))

	stored := readViolations(t, c, gatekeeper.SecurityPolicyKind, "app")
	assert.Equal(t, first.Format(time.RFC3339), stored[lastUpdateTimeField],
		"an unchanged summary should keep the timestamp of the change that produced it")
}

func TestSyncRewritesAChangedSummary(t *testing.T) {
	first := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	writer, c := newTestWriter(t, first, policyObject(gatekeeper.SecurityPolicyKind, "app"))

	owner := Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}
	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{owner: {Total: 2}}))

	later := first.Add(time.Hour)
	writer.now = func() time.Time { return later }
	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{owner: {Total: 5}}))

	stored := readViolations(t, c, gatekeeper.SecurityPolicyKind, "app")
	assert.Equal(t, int64(5), stored["total"])
	assert.Equal(t, later.Format(time.RFC3339), stored[lastUpdateTimeField])
}

func TestSyncClearsTheSummaryWhenTheViolationsAreGone(t *testing.T) {
	first := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	writer, c := newTestWriter(t, first, policyObject(gatekeeper.SecurityPolicyKind, "app"))

	owner := Owner{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}
	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{
		owner: {Total: 2, ByEnforcement: map[string]int64{"deny": 2}},
	}))

	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{owner: {Total: 0}}))

	stored := readViolations(t, c, gatekeeper.SecurityPolicyKind, "app")
	assert.Equal(t, int64(0), stored["total"])
	assert.NotContains(t, stored, "byEnforcement", "the counters of an earlier audit should not survive")
}

func TestSyncSkipsAPolicyThatIsGone(t *testing.T) {
	// The audit works on a snapshot, so a policy deleted between the audit and the write is expected.
	writer, _ := newTestWriter(t, time.Now())

	assert.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{
		{Kind: gatekeeper.SecurityPolicyKind, Name: "gone"}: {Total: 1},
	}))
}

func TestSyncWritesEveryPolicyItCan(t *testing.T) {
	// One failing policy should not keep the others from being updated.
	at := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	writer, c := newTestWriter(t, at,
		policyObject(gatekeeper.SecurityPolicyKind, "app"),
		policyObject(gatekeeper.OperationPolicyKind, "labels"),
	)

	require.NoError(t, writer.Sync(context.Background(), map[Owner]Summary{
		{Kind: gatekeeper.SecurityPolicyKind, Name: "app"}:      {Total: 1},
		{Kind: gatekeeper.OperationPolicyKind, Name: "labels"}:  {Total: 2},
		{Kind: gatekeeper.SecurityPolicyKind, Name: "vanished"}: {Total: 3},
	}))

	assert.Equal(t, int64(1), readViolations(t, c, gatekeeper.SecurityPolicyKind, "app")["total"])
	assert.Equal(t, int64(2), readViolations(t, c, gatekeeper.OperationPolicyKind, "labels")["total"])
}
