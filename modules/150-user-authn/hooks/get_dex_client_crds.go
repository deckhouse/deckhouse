/*
Copyright 2021 Flant JSC

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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

type DexClient struct {
	ID        string `json:"id"`
	EncodedID string `json:"encodedID"`

	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Spec      map[string]interface{} `json:"spec"`

	Secret string `json:"clientSecret"`

	// LegacyID and LegacyEncodedID is formatted with a colons delimiter which is impossible to use as a
	//   basic auth credentials part
	LegacyID        string `json:"legacyID"`
	LegacyEncodedID string `json:"legacyEncodedID"`

	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`

	AllowAccessToKubernetes bool `json:"allowAccessToKubernetes"`
}

type DexClientSecret struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Secret    []byte `json:"spec"`
}

func applyDexClientFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from dex client: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("dex client has no spec field")
	}

	name := obj.GetName()
	namespace := obj.GetNamespace()

	id := fmt.Sprintf("dex-client-%s@%s", name, namespace)
	legacyID := fmt.Sprintf("dex-client-%s:%s", name, namespace)

	labels, _, err := unstructured.NestedStringMap(obj.Object, "spec", "secretMetadata", "labels")
	if err != nil {
		return nil, fmt.Errorf("cannot get secretMetadata.labels: %v", err)
	}
	if labels == nil {
		labels = make(map[string]string)
	}

	annotations, _, err := unstructured.NestedStringMap(obj.Object, "spec", "secretMetadata", "annotations")
	if err != nil {
		return nil, fmt.Errorf("cannot get secretMetadata.annotations: %v", err)
	}
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if value, exists := obj.GetAnnotations()["dexclient.deckhouse.io/allow-access-to-kubernetes"]; exists {
		annotations["dexclient.deckhouse.io/allow-access-to-kubernetes"] = value
	}

	_, allowAccessToKubernetes := obj.GetAnnotations()["dexclient.deckhouse.io/allow-access-to-kubernetes"]

	return DexClient{
		ID:                      id,
		LegacyID:                legacyID,
		EncodedID:               encoding.ToFnvLikeDex(id),
		LegacyEncodedID:         encoding.ToFnvLikeDex(legacyID),
		Name:                    name,
		Namespace:               namespace,
		Spec:                    spec,
		Labels:                  labels,
		Annotations:             annotations,
		AllowAccessToKubernetes: allowAccessToKubernetes,
	}, nil
}

func applyDexClientSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert dex client secret to secret: %v", err)
	}
	name := obj.GetName()
	namespace := obj.GetNamespace()

	id := fmt.Sprintf("%s@%s", name, namespace)
	return DexClientSecret{
		ID:        id,
		Name:      name,
		Namespace: namespace,
		Secret:    secret.Data["clientSecret"],
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "clients",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "DexClient",
			FilterFunc: applyDexClientFilter,
		},
		{
			Name:       "credentials",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "dex-client",
					"name": "credentials",
				},
			},
			FilterFunc: applyDexClientSecretFilter,
		},
	},
}, getDexClient)

func getDexClient(_ context.Context, input *go_hook.HookInput) error {
	clients := input.Snapshots.Get("clients")
	credentials := input.Snapshots.Get("credentials")

	credentialsByID := make(map[string]string, len(credentials))

	for dexSecret, err := range sdkobjectpatch.SnapshotIter[DexClientSecret](credentials) {
		if err != nil {
			return fmt.Errorf("cannot convert dex client secret: failed to iterate over 'credentials' snapshot: %w", err)
		}

		credentialsByID[dexSecret.ID] = string(dexSecret.Secret)
	}

	dexClients := make([]DexClient, 0, len(clients))
	for dexClient, err := range sdkobjectpatch.SnapshotIter[DexClient](clients) {
		if err != nil {
			return fmt.Errorf("cannot convert dex client: failed to iterate over 'clients' snapshot: %w", err)
		}

		existedSecret, ok := credentialsByID[dexClient.ID]
		if !ok {
			existedSecret = pwgen.AlphaNum(20)
		}

		dexClient.Secret = existedSecret
		dexClients = append(dexClients, dexClient)
	}

	input.Values.Set("userAuthn.internal.dexClientCRDs", dexClients)
	return nil
}
