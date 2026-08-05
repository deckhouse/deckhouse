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
	"testing"

	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

func newWebhookAdmissionStateBuilderFactory(t *testing.T, objects ...runtime.Object) *ycval.AdmissionStateBuilderFactory {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	client := clientfake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return ycval.NewAdmissionStateBuilderFactory(client, cpvaladmission.StateBuilderConfig{
		ModuleName:       ycmeta.ModuleName,
		NamespaceName:    ycmeta.Namespace,
		InstanceClassGVK: ycicv1.GroupVersionKind,
	})
}

func validYandexClusterObjects() []runtime.Object {
	return []runtime.Object{
		yandexCredentialSecret(validWebhookServiceAccountJSON()),
		yandexNodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
		yandexInstanceClassObject("master-yandex"),
	}
}

func yandexNodeGroupObject(name string, nodeType cpapi.NodeType) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"})
	obj.SetName(name)
	spec := map[string]any{"nodeType": string(nodeType)}
	if name == "master" {
		spec["cloudInstances"] = map[string]any{
			"classReference": map[string]any{
				"kind": "YandexInstanceClass",
				"name": "master-yandex",
			},
		}
	}
	obj.Object["spec"] = spec
	return obj
}

func yandexStaticNodeGroupObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"})
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{"nodeType": "Static"}
	return obj
}

func yandexInstanceClassObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: ycicv1.GroupVersionKind.Group, Version: ycicv1.GroupVersionKind.Version, Kind: ycicv1.GroupVersionKind.Kind})
	obj.SetName(name)
	if name == "master-yandex" {
		obj.Object["spec"] = map[string]any{"etcdDiskSizeGB": int64(10)}
	}
	return obj
}

func yandexCredentialSecret(token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.CredentialSecretName,
			Namespace: ycmeta.Namespace,
		},
		Type: cpapi.CredentialsSecretType,
		StringData: map[string]string{
			cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeServiceAccount),
			cpapi.CredentialSecretSecretKey:     token,
		},
	}
}

// validWebhookServiceAccountJSON returns a credential accepted by ycval.CredentialsValidator:
// Yandex supports the serviceAccount (IAM key JSON) and apiToken auth schemes, not kubeconfig.
func validWebhookServiceAccountJSON() string {
	return `{"id": "test", "service_account_id": "test"}`
}
