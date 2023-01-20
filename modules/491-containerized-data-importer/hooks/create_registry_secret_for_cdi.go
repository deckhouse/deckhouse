/*
Copyright 2023 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	cdiPodsSnapshot    = "cdi_pods"
	secretsSnapshot    = "registry_secrets_namespaces"
	d8RegistrySnapshot = "d8_registry_secret"

	virtRegistrySecretName = "cdi-deckhouse-registry"
)

type registrySecret struct {
	Namespace string
	Config    string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/containerized-data-importer/registry-secrets",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       cdiPodsSnapshot,
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyCDIPodFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                          "containerized-data-importer",
					"app.kubernetes.io/managed-by": "cdi-controller",
				},
			},
		},
		{
			Name:       secretsSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyRegistrySecretFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{virtRegistrySecretName},
			},
		},
		{
			Name:       d8RegistrySnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-registry"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyRegistrySecretFilter,
		},
	},
}, handleCDIPod)

func applyCDIPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetNamespace(), nil
}

func applyRegistrySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s corev1.Secret
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	ns := s.GetNamespace()

	conf, ok := s.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return "", fmt.Errorf("registry auth conf is not in registry secret %s/%s", ns, obj.GetName())
	}

	return registrySecret{
		Namespace: ns,
		Config:    string(conf),
	}, nil
}

func prepareVirtRegistrySecret(namespace, dockerCfg string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtRegistrySecretName,
			Namespace: namespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"kubevirt.deckhouse.io/cdi-registry-secret": "true",
				"deckhouse.io/registry-secret":              "true",
			},
		},

		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(dockerCfg),
		},

		Type: corev1.SecretTypeDockerConfigJson,
	}
}

// handleCDIPod
//
// synopsis:
//
//	Every namespace running cdi workload pod, must contain registry
//	secret to be able pulling cdi images.
//	The cdi images re-pushed in deckhouse registry to make
//	them possible to run in closed environments.
//	After CDI pod is finished the registry secret must be deleted.
//
//	We have patched cdi-controller for specifying image pull-secrets
//	for envery VMI pod. This is temproray solution, until kubevirt
//	will have native opportunity for specifying registrySecrets.
func handleCDIPod(input *go_hook.HookInput) error {
	d8RegistrySnap := input.Snapshots[d8RegistrySnapshot]
	if len(d8RegistrySnap) == 0 {
		input.LogEntry.Warnln("Registry secret not found. Skip")
		return nil
	}

	registryCfg := d8RegistrySnap[0].(registrySecret).Config

	cdiPods := set.NewFromSnapshot(input.Snapshots[cdiPodsSnapshot])

	// namespace -> .dockerconfigjson content
	secretsByNs := map[string]string{}

	for _, sRaw := range input.Snapshots[secretsSnapshot] {
		regSecret := sRaw.(registrySecret)
		secretsByNs[regSecret.Namespace] = regSecret.Config
	}

	// create secrets
	for ns := range cdiPods {
		secretContent, secretExists := secretsByNs[ns]
		// secret already exists in namespace. do not create or patch
		if !secretExists || secretContent != registryCfg {
			secret := prepareVirtRegistrySecret(ns, registryCfg)
			input.PatchCollector.Create(secret, object_patch.UpdateIfExists())
		}
	}

	// gc secrets
	for ns := range secretsByNs {
		if cdiPods.Has(ns) {
			// a secret exists in namespace, and exists one more VMI. do not delete secret
			continue
		}

		input.PatchCollector.Delete("v1", "Secret", ns, virtRegistrySecretName)
	}

	return nil
}
