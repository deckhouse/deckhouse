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

package hooks

import (
	"encoding/json"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	k8sdynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: force_machine_deployment_replicas_ownership ::", func() {
	const state = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-mcm-legacy
  namespace: d8-cloud-instance-manager
  labels:
    node-group: worker
  managedFields:
  - manager: deckhouse-controller
    operation: Update
    apiVersion: machine.sapcloud.io/v1alpha1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        f:replicas: {}
spec:
  replicas: 2
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-mcm-hook
  namespace: d8-cloud-instance-manager
  labels:
    node-group: worker
  managedFields:
  - manager: deckhouse-hook
    operation: Update
    apiVersion: machine.sapcloud.io/v1alpha1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        f:replicas: {}
spec:
  replicas: 3
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: md-capi-legacy
  namespace: d8-cloud-instance-manager
  labels:
    node-group: static
  managedFields:
  - manager: deckhouse-controller
    operation: Update
    apiVersion: cluster.x-k8s.io/v1beta1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        f:replicas: {}
spec:
  replicas: 1
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: md-capi-other
  namespace: d8-cloud-instance-manager
  labels:
    node-group: static
  managedFields:
  - manager: machine-controller-manager
    operation: Update
    apiVersion: cluster.x-k8s.io/v1beta1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        f:replicas: {}
spec:
  replicas: 4
`

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "MachineDeployment", true)

	// appliedPatches records the names of MachineDeployments that received the
	// managedFields ownership-migration patch (a JSON patch, not a server-side
	// apply). A single reactor is installed once the fake client exists; the slice
	// is reset before every run.
	var (
		appliedPatches []string
		spyOnce        sync.Once
	)

	installApplySpy := func() {
		spyOnce.Do(func() {
			fakeDynamic := f.KubeClient().Dynamic().(*k8sdynamicfake.FakeDynamicClient)
			fakeDynamic.PrependReactor("patch", "machinedeployments", func(action k8stesting.Action) (bool, k8sruntime.Object, error) {
				patch := action.(k8stesting.PatchAction)
				if patch.GetPatchType() == apitypes.JSONPatchType {
					appliedPatches = append(appliedPatches, patch.GetName())
				}
				// Short-circuit: record the migration and skip applying the JSON patch,
				// whose managedFields rewrite is not needed for this assertion.
				return true, &unstructured.Unstructured{}, nil
			})
		})
	}

	Context("MachineDeployments with mixed spec.replicas ownership", func() {
		BeforeEach(func() {
			appliedPatches = nil
			f.BindingContexts.Set(f.KubeStateSet(state))
			installApplySpy()
			f.RunHook()
		})

		It("claims ownership only for the legacy-owned MachineDeployments", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(appliedPatches).To(ConsistOf("md-mcm-legacy", "md-capi-legacy"))
		})
	})

	Context("no MachineDeployments", func() {
		BeforeEach(func() {
			appliedPatches = nil
			f.BindingContexts.Set(f.KubeStateSet(``))
			installApplySpy()
			f.RunHook()
		})

		It("does nothing and does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(appliedPatches).To(BeEmpty())
		})
	})
})

func ownerManagedField(manager, subresource, fieldsV1 string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{
		Manager:     manager,
		Operation:   metav1.ManagedFieldsOperationUpdate,
		Subresource: subresource,
		FieldsType:  "FieldsV1",
		FieldsV1:    &metav1.FieldsV1{Raw: []byte(fieldsV1)},
	}
}

func TestReplicasOwnedByLegacyManager(t *testing.T) {
	tests := []struct {
		name          string
		managedFields []metav1.ManagedFieldsEntry
		want          bool
	}{
		{
			name:          "no managed fields",
			managedFields: nil,
			want:          false,
		},
		{
			name:          "legacy manager owns spec.replicas",
			managedFields: []metav1.ManagedFieldsEntry{ownerManagedField(legacyFieldManager, "", `{"f:spec":{"f:replicas":{}}}`)},
			want:          true,
		},
		{
			name:          "legacy manager owns spec.replicas alongside other fields",
			managedFields: []metav1.ManagedFieldsEntry{ownerManagedField(legacyFieldManager, "", `{"f:spec":{".":{},"f:minReadySeconds":{},"f:replicas":{}}}`)},
			want:          true,
		},
		{
			name:          "legacy manager owns spec but not replicas",
			managedFields: []metav1.ManagedFieldsEntry{ownerManagedField(legacyFieldManager, "", `{"f:spec":{"f:minReadySeconds":{}}}`)},
			want:          false,
		},
		{
			name:          "spec.replicas owned by the hook manager, not legacy",
			managedFields: []metav1.ManagedFieldsEntry{ownerManagedField(hookFieldManager, "", `{"f:spec":{"f:replicas":{}}}`)},
			want:          false,
		},
		{
			name:          "legacy manager owns replicas only on the status subresource",
			managedFields: []metav1.ManagedFieldsEntry{ownerManagedField(legacyFieldManager, "status", `{"f:status":{"f:replicas":{}}}`)},
			want:          false,
		},
		{
			name: "mixed managers, legacy owns replicas",
			managedFields: []metav1.ManagedFieldsEntry{
				ownerManagedField("machine-controller-manager", "status", `{"f:status":{"f:replicas":{}}}`),
				ownerManagedField(hookFieldManager, "", `{"f:spec":{"f:minReadySeconds":{}}}`),
				ownerManagedField(legacyFieldManager, "", `{"f:spec":{"f:replicas":{}}}`),
			},
			want: true,
		},
		{
			name:          "legacy manager with nil FieldsV1",
			managedFields: []metav1.ManagedFieldsEntry{{Manager: legacyFieldManager, Operation: metav1.ManagedFieldsOperationUpdate}},
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replicasOwnedByLegacyManager(tt.managedFields); got != tt.want {
				t.Errorf("replicasOwnedByLegacyManager() = %v, want %v", got, tt.want)
			}
		})
	}
}

// specFieldOwned reports whether the (manager, subresource) entry owns
// f:spec/<field> in its managed fields.
func specFieldOwned(entries []metav1.ManagedFieldsEntry, manager, subresource, field string) bool {
	for _, e := range entries {
		if e.Manager != manager || e.Subresource != subresource || e.FieldsV1 == nil {
			continue
		}
		var top map[string]json.RawMessage
		if json.Unmarshal(e.FieldsV1.Raw, &top) != nil {
			continue
		}
		specRaw, ok := top["f:spec"]
		if !ok {
			continue
		}
		var spec map[string]json.RawMessage
		if json.Unmarshal(specRaw, &spec) != nil {
			continue
		}
		if _, ok := spec[field]; ok {
			return true
		}
	}
	return false
}

// managerEntries counts the entries owned by the (manager, subresource) pair.
func managerEntries(entries []metav1.ManagedFieldsEntry, manager, subresource string) int {
	n := 0
	for _, e := range entries {
		if e.Manager == manager && e.Subresource == subresource {
			n++
		}
	}
	return n
}

func TestMigrateReplicasOwnerToHook(t *testing.T) {
	const apiVersion = "machine.sapcloud.io/v1alpha1"

	t.Run("legacy owns only spec.replicas, no hook entry", func(t *testing.T) {
		in := []metav1.ManagedFieldsEntry{
			ownerManagedField(legacyFieldManager, "", `{"f:spec":{"f:replicas":{}}}`),
		}

		out, changed, err := migrateReplicasOwnerToHook(in, apiVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Fatal("expected changed=true")
		}
		if managerEntries(out, legacyFieldManager, "") != 0 {
			t.Errorf("legacy entry must be dropped when it owned only spec.replicas, got %d", managerEntries(out, legacyFieldManager, ""))
		}
		if !specFieldOwned(out, hookFieldManager, "", "f:replicas") {
			t.Error("hook manager must own spec.replicas")
		}
		// The fabricated hook entry must be a well-formed Update entry.
		for _, e := range out {
			if e.Manager == hookFieldManager {
				if e.Operation != metav1.ManagedFieldsOperationUpdate {
					t.Errorf("hook entry operation = %v, want Update", e.Operation)
				}
				if e.APIVersion != apiVersion {
					t.Errorf("hook entry apiVersion = %q, want %q", e.APIVersion, apiVersion)
				}
			}
		}
	})

	t.Run("legacy owns other spec fields too, only replicas is moved", func(t *testing.T) {
		in := []metav1.ManagedFieldsEntry{
			ownerManagedField(legacyFieldManager, "", `{"f:spec":{"f:minReadySeconds":{},"f:replicas":{}}}`),
		}

		out, changed, err := migrateReplicasOwnerToHook(in, apiVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Fatal("expected changed=true")
		}
		if specFieldOwned(out, legacyFieldManager, "", "f:replicas") {
			t.Error("legacy manager must no longer own spec.replicas")
		}
		if !specFieldOwned(out, legacyFieldManager, "", "f:minReadySeconds") {
			t.Error("legacy manager must keep spec.minReadySeconds")
		}
		if !specFieldOwned(out, hookFieldManager, "", "f:replicas") {
			t.Error("hook manager must own spec.replicas")
		}
	})

	t.Run("existing hook entry gains replicas, legacy entry dropped", func(t *testing.T) {
		in := []metav1.ManagedFieldsEntry{
			ownerManagedField(hookFieldManager, "", `{"f:spec":{"f:minReadySeconds":{}}}`),
			ownerManagedField(legacyFieldManager, "", `{"f:spec":{"f:replicas":{}}}`),
		}

		out, changed, err := migrateReplicasOwnerToHook(in, apiVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Fatal("expected changed=true")
		}
		if managerEntries(out, hookFieldManager, "") != 1 {
			t.Errorf("expected replicas merged into the existing hook entry, got %d hook entries", managerEntries(out, hookFieldManager, ""))
		}
		if managerEntries(out, legacyFieldManager, "") != 0 {
			t.Error("legacy entry must be dropped")
		}
		if !specFieldOwned(out, hookFieldManager, "", "f:replicas") || !specFieldOwned(out, hookFieldManager, "", "f:minReadySeconds") {
			t.Error("hook manager must own both spec.replicas and spec.minReadySeconds")
		}
	})

	t.Run("already migrated is a no-op", func(t *testing.T) {
		in := []metav1.ManagedFieldsEntry{
			ownerManagedField(hookFieldManager, "", `{"f:spec":{"f:replicas":{}}}`),
		}

		_, changed, err := migrateReplicasOwnerToHook(in, apiVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if changed {
			t.Error("expected changed=false when the legacy manager does not own spec.replicas")
		}
	})

	t.Run("legacy owns replicas only on the status subresource is untouched", func(t *testing.T) {
		in := []metav1.ManagedFieldsEntry{
			ownerManagedField(legacyFieldManager, "status", `{"f:status":{"f:replicas":{}}}`),
		}

		_, changed, err := migrateReplicasOwnerToHook(in, apiVersion)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if changed {
			t.Error("expected changed=false for a status-subresource entry")
		}
	})
}
