// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhooks

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	dvpicv1alpha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
)

func TestDVPInstanceClassValidatorWithFakeClientValidateUpdate(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validDVPClusterObjects()...)
	validator := NewDVPInstanceClassValidator(factory, &unstructured.Unstructured{})

	updated := dvpInstanceClassObject("master-dvp")
	updated.Object["spec"] = map[string]any{"etcdDisk": map[string]any{"size": "10Gi"}}
	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestDVPInstanceClassValidatorRejectsMasterEtcdDiskRemoval(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validDVPClusterObjects()...)
	validator := NewDVPInstanceClassValidator(factory, &unstructured.Unstructured{})

	updated := dvpInstanceClassObject("master-dvp")
	updated.Object["spec"] = map[string]any{}

	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err == nil || !strings.Contains(err.Error(), "must define spec.etcdDisk") {
		t.Fatalf("ValidateUpdate() error = %v, want master etcdDisk denial", err)
	}
}

func TestDVPInstanceClassValidatorRejectsWorkerEtcdDisk(t *testing.T) {
	t.Parallel()

	workerNodeGroup := dvpNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent)
	workerNodeGroup.Object["spec"] = map[string]any{
		"nodeType": string(cpapi.NodeTypeCloudPermanent),
		"cloudInstances": map[string]any{
			"classReference": map[string]any{
				"kind": dvpicv1alpha1.GroupVersionKind.Kind,
				"name": "worker-dvp",
			},
		},
	}

	factory := newWebhookAdmissionStateBuilderFactory(t, append(validDVPClusterObjects(), workerNodeGroup)...)
	validator := NewDVPInstanceClassValidator(factory, &unstructured.Unstructured{})

	updated := dvpInstanceClassObject("worker-dvp")
	updated.Object["spec"] = map[string]any{
		"etcdDisk": map[string]any{
			"size":         "5Gi",
			"storageClass": "replicated",
		},
	}

	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err == nil || !strings.Contains(err.Error(), "attached to NodeGroup master") {
		t.Fatalf("ValidateUpdate() error = %v, want worker etcdDisk denial", err)
	}
	if !strings.Contains(err.Error(), `worker-dvp.spec.etcdDisk: Invalid value: {"size":"5Gi","storageClass":"replicated"}`) {
		t.Fatalf("ValidateUpdate() error = %q, want etcdDisk field path with the etcdDisk value", err.Error())
	}
}

func TestDVPInstanceClassValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validDVPClusterObjects()...)
	validator := NewDVPInstanceClassValidator(factory, &unstructured.Unstructured{})

	created := dvpInstanceClassObject("worker-dvp")
	_, err := validator.ValidateCreate(context.Background(), created)
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestDVPInstanceClassValidatorWithFakeClientRejectsDeleteInUse(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validDVPClusterObjects()...)
	validator := NewDVPInstanceClassValidator(factory, &unstructured.Unstructured{})

	_, err := validator.ValidateDelete(context.Background(), dvpInstanceClassObject("master-dvp"))
	if err == nil || !strings.Contains(err.Error(), "InstanceClass is used by NodeGroup") {
		t.Fatalf("ValidateDelete() error = %v, want in-use denial", err)
	}
}
