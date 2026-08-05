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
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNodeGroupValidatorWithFakeClientAllowsMasterCreateBeforeInstanceClass(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, yandexCredentialSecret(validWebhookServiceAccountJSON()))
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	_, err := validator.ValidateCreate(context.Background(), yandexNodeGroupObject("master", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow master NodeGroup before InstanceClass exists", err)
	}
}

func TestNodeGroupValidatorWithFakeClientValidateUpdate(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	updated := yandexNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestNodeGroupValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	worker := yandexNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent)
	spec := worker.Object["spec"].(map[string]any)
	spec["cloudInstances"] = map[string]any{
		"classReference": map[string]any{
			"kind": "YandexInstanceClass",
			"name": "worker-yandex",
		},
	}

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	_, err := validator.ValidateCreate(context.Background(), worker)
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestNodeGroupValidatorWithFakeClientAllowsStaticNodeGroupWhenStackIncomplete(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t)
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	_, err := validator.ValidateCreate(context.Background(), yandexStaticNodeGroupObject("worker-static"))
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow for Static NodeGroup", err)
	}
}

func TestNodeGroupValidatorWithFakeClientAllowsMasterDemotion(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	oldMaster := yandexNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	newMaster := yandexStaticNodeGroupObject("master")

	_, err := validator.ValidateUpdate(context.Background(), oldMaster, newMaster)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow without preflight requirements", err)
	}
}

func TestNodeGroupValidatorWithFakeClientValidateDeleteAllowsMaster(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	_, err := validator.ValidateDelete(context.Background(), yandexNodeGroupObject("master", cpapi.NodeTypeCloudPermanent))
	if err != nil {
		t.Fatalf("ValidateDelete() error = %v, want allow master deletion without preflight", err)
	}
}

func TestShouldValidateNodeGroup(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewNodeGroupValidator(factory, &unstructured.Unstructured{})

	if !validator.shouldValidateNodeGroup(yandexNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)) {
		t.Fatal("shouldValidateNodeGroup(CloudPermanent) = false, want true")
	}

	if validator.shouldValidateNodeGroup(yandexStaticNodeGroupObject("worker-static")) {
		t.Fatal("shouldValidateNodeGroup(Static) = true, want false")
	}

	oldMaster := yandexNodeGroupObject("master", cpapi.NodeTypeCloudPermanent)
	newStatic := yandexStaticNodeGroupObject("master")
	if !validator.shouldValidateNodeGroupUpdate(oldMaster, newStatic) {
		t.Fatal("shouldValidateNodeGroupUpdate(master demotion) = false, want true")
	}
}
