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
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	// Secrets with that label lead to D8CertmanagerOrphanSecretsChecksFailed alerts.
	// argocd.argoproj.io is used by ArgoCD to identify secrets managed by it.
	// app.kubernetes.io/managed-by is used by Helm to identify secrets managed by it.
	// Delete labels that should not be transferred to the secret
	labelKeysToIgnore := []string{
		"app",
		"heritage",
		"module",
		"name",
		"app.kubernetes.io/managed-by",
		"argocd.argoproj.io/secret-type",
		"argocd.argoproj.io/instance",
		"certmanager.k8s.io/certificate-name",
	}
	for _, key := range labelKeysToIgnore {
		delete(labels, key)
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
	for key := range annotations {
		if strings.Contains(key, "werf.io/") || strings.Contains(key, "meta.helm.sh/") {
			delete(annotations, key)
		}
	}

	return DexClient{
		ID:              id,
		LegacyID:        legacyID,
		EncodedID:       encoding.ToFnvLikeDex(id),
		LegacyEncodedID: encoding.ToFnvLikeDex(legacyID),
		Name:            name,
		Namespace:       namespace,
		Spec:            spec,
		Labels:          labels,
		Annotations:     annotations,
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

func getDexClient(input *go_hook.HookInput) error {
	clients := input.Snapshots["clients"]
	credentials := input.Snapshots["credentials"]

	credentialsByID := make(map[string]string, len(credentials))

	for _, secret := range credentials {
		dexSecret, ok := secret.(DexClientSecret)
		if !ok {
			return fmt.Errorf("cannot convert dex client secret")
		}

		credentialsByID[dexSecret.ID] = string(dexSecret.Secret)
	}

	dexClients := make([]DexClient, 0, len(clients))
	for _, client := range clients {
		dexClient, ok := client.(DexClient)
		if !ok {
			return fmt.Errorf("cannot convert dex client")
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
