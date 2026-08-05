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
)

func TestYandexInstanceClassValidatorWithFakeClientValidateUpdate(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewYandexInstanceClassValidator(factory, &unstructured.Unstructured{})

	updated := yandexInstanceClassObject("master-yandex")
	updated.Object["spec"] = map[string]any{"etcdDiskSizeGB": int64(10)}
	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err != nil {
		t.Fatalf("ValidateUpdate() error = %v, want allow", err)
	}
}

func TestYandexInstanceClassValidatorRejectsMasterEtcdDiskRemoval(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewYandexInstanceClassValidator(factory, &unstructured.Unstructured{})

	updated := yandexInstanceClassObject("master-yandex")
	updated.Object["spec"] = map[string]any{}

	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err == nil || !strings.Contains(err.Error(), "must define spec.etcdDisk") {
		t.Fatalf("ValidateUpdate() error = %v, want master etcdDisk denial", err)
	}
}

func TestYandexInstanceClassValidatorRejectsWorkerEtcdDisk(t *testing.T) {
	t.Parallel()

	workerNodeGroup := yandexNodeGroupObject("worker", cpapi.NodeTypeCloudPermanent)
	workerNodeGroup.Object["spec"] = map[string]any{
		"nodeType": string(cpapi.NodeTypeCloudPermanent),
		"cloudInstances": map[string]any{
			"classReference": map[string]any{
				"kind": "YandexInstanceClass",
				"name": "worker-yandex",
			},
		},
	}

	factory := newWebhookAdmissionStateBuilderFactory(t, append(validYandexClusterObjects(), workerNodeGroup)...)
	validator := NewYandexInstanceClassValidator(factory, &unstructured.Unstructured{})

	updated := yandexInstanceClassObject("worker-yandex")
	updated.Object["spec"] = map[string]any{"etcdDiskSizeGB": int64(5)}

	_, err := validator.ValidateUpdate(context.Background(), nil, updated)
	if err == nil || !strings.Contains(err.Error(), "attached to NodeGroup master") {
		t.Fatalf("ValidateUpdate() error = %v, want worker etcdDisk denial", err)
	}
	// The rule is provider-agnostic: cpapi.InstanceClassObject exposes the etcd disk value
	// via GetEtcdDisk, so the denial reports the field path with the raw etcdDiskSizeGB value.
	if !strings.Contains(err.Error(), `worker-yandex.spec.etcdDisk: Invalid value: 5`) {
		t.Fatalf("ValidateUpdate() error = %q, want etcdDisk field path with the etcdDiskSizeGB value", err.Error())
	}
}

func TestYandexInstanceClassValidatorWithFakeClientAllowsValidCluster(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewYandexInstanceClassValidator(factory, &unstructured.Unstructured{})

	created := yandexInstanceClassObject("worker-yandex")
	_, err := validator.ValidateCreate(context.Background(), created)
	if err != nil {
		t.Fatalf("ValidateCreate() error = %v, want allow", err)
	}
}

func TestYandexInstanceClassValidatorWithFakeClientRejectsDeleteInUse(t *testing.T) {
	t.Parallel()

	factory := newWebhookAdmissionStateBuilderFactory(t, validYandexClusterObjects()...)
	validator := NewYandexInstanceClassValidator(factory, &unstructured.Unstructured{})

	_, err := validator.ValidateDelete(context.Background(), yandexInstanceClassObject("master-yandex"))
	if err == nil || !strings.Contains(err.Error(), "InstanceClass is used by NodeGroup") {
		t.Fatalf("ValidateDelete() error = %v, want in-use denial", err)
	}
}
