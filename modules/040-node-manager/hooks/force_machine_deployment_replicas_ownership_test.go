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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
