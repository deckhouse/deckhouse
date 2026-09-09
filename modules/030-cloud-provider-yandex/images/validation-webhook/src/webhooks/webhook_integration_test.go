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
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	ycmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/meta"
)

func createValidYandexWebhookCluster() {
	deleteYandexWebhookCluster()

	err := testK8sClient.Create(testCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ycmeta.Namespace}})
	if err != nil {
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
	}

	objects := []client.Object{
		yandexCredentialIntegrationSecret(),
		yandexNodeGroupIntegrationObject("master", "master-yandex"),
		yandexNodeGroupIntegrationObject("worker", "worker"),
		yandexInstanceClassIntegrationObject("master-yandex", map[string]any{"etcdDiskSizeGB": int64(10)}),
		yandexInstanceClassIntegrationObject("worker", map[string]any{}),
	}

	for _, obj := range objects {
		Expect(testK8sClient.Create(testCtx, obj)).To(Succeed())
	}
}

func deleteYandexWebhookCluster() {
	objects := []client.Object{
		yandexModuleConfigIntegrationObject(nil),
		yandexNodeGroupIntegrationObject("worker", "worker"),
		yandexNodeGroupIntegrationObject("master", "master-yandex"),
		yandexInstanceClassIntegrationObject("worker", map[string]any{}),
		yandexInstanceClassIntegrationObject("master-yandex", map[string]any{}),
		yandexCredentialIntegrationSecret(),
	}

	for _, obj := range objects {
		_ = testK8sClient.Delete(testCtx, obj)
	}
}

func clientObjectKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: name}
}

func yandexCredentialIntegrationSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: ycmeta.Namespace},
		Type:       cpapi.CredentialsSecretType,
		Data: map[string][]byte{
			cpapi.CredentialSecretAuthSchemeKey: []byte(cpapi.AuthSchemeServiceAccount),
			cpapi.CredentialSecretSecretKey:     []byte(validWebhookServiceAccountJSON()),
		},
	}
}

func yandexNodeGroupIntegrationObject(name, className string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(nodeGroupGVK())
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{
		"nodeType": string(cpapi.NodeTypeCloudPermanent),
		"cloudInstances": map[string]any{
			// The node count is what ValidateNodeGroupExternalIPAddresses compares against
			// settings.nodes.parameters.externalIPAddresses, so the fixture has to carry it.
			"maxPerZone": int64(2),
			"classReference": map[string]any{
				"kind": "YandexInstanceClass",
				"name": className,
			},
		},
	}
	return obj
}

func yandexInstanceClassIntegrationObject(name string, spec map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(instanceClassGVK())
	obj.SetName(name)
	obj.Object["spec"] = spec
	return obj
}
