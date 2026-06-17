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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/meta"
)

func newWebhookAdmissionStateBuilder(t *testing.T, objects ...runtime.Object) *cpvaladmission.StateBuilder {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}

	client := clientfake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return cpvaladmission.NewStateBuilder(client, cpvaladmission.StateBuilderConfig{
		ModuleName:                   dvpmeta.ModuleName,
		NamespaceName:                dvpmeta.Namespace,
		InstanceClassKind:            dvpmeta.InstanceClassKind,
	})
}

func validDVPClusterObjects() []runtime.Object {
	return []runtime.Object{
		dvpModuleConfigObject(),
		dvpCredentialSecret(validWebhookKubeconfigB64()),
		dvpNodeGroupObject("master", cpapi.NodeTypeCloudPermanent),
		dvpInstanceClassObject("master-dvp"),
	}
}

func dvpModuleConfigObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"})
	obj.SetName(dvpmeta.ModuleName)
	obj.Object["spec"] = map[string]any{"enabled": true, "version": int64(2)}
	return obj
}

func dvpNodeGroupObject(name string, nodeType cpapi.NodeType) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"})
	obj.SetName(name)
	spec := map[string]any{"nodeType": string(nodeType)}
	if name == "master" {
		spec["cloudInstances"] = map[string]any{
			"classReference": map[string]any{
				"kind": dvpmeta.InstanceClassKind,
				"name": "master-dvp",
			},
		}
	}
	obj.Object["spec"] = spec
	return obj
}

func dvpStaticNodeGroupObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"})
	obj.SetName(name)
	obj.Object["spec"] = map[string]any{"nodeType": "Static"}
	return obj
}

func dvpInstanceClassObject(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: dvpmeta.InstanceClassKind})
	obj.SetName(name)
	if name == "master-dvp" {
		obj.Object["spec"] = map[string]any{"etcdDisk": map[string]any{}}
	}
	return obj
}

func dvpCredentialSecret(token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.CredentialSecretName,
			Namespace: dvpmeta.Namespace,
		},
		Type: cpapi.CredentialsSecretType,
		StringData: map[string]string{
			cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeKubeconfig),
			cpapi.CredentialSecretSecretKey:     token,
		},
	}
}

func validWebhookKubeconfigB64() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu"
}
