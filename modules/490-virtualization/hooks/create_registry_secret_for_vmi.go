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
	vmisSnapshot            = "vmis"
	secretsSnapshot         = "registry_secrets_namespaces"
	d8RegistrySnapshot      = "d8_registry_secret"
	kubevirtVMIsCRDSnapshot = "vmHandlerKubevirtVMICRD"

	virtRegistrySecretName = "virt-deckhouse-registry"
)

type registrySecret struct {
	Namespace string
	Config    string
}

var createRegistrySecretForVMIHookConfig = &go_hook.HookConfig{
	Queue: "/modules/virtualization/registry-secrets",
	Kubernetes: []go_hook.KubernetesConfig{
		// A binding with dynamic kind has index 0 for simplicity.
		{
			Name:       vmisSnapshot,
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyNamespaceFilter,
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
		{
			Name:       kubevirtVMIsCRDSnapshot,
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"virtualmachineinstances.kubevirt.io"},
			},
			FilterFunc: applyCRDExistenseFilter,
		},
	},
}

var _ = sdk.RegisterFunc(createRegistrySecretForVMIHookConfig, handleVMI)

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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
				"kubevirt.deckhouse.io/virt-registry-secret": "true",
				"deckhouse.io/registry-secret":               "true",
			},
		},

		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(dockerCfg),
		},

		Type: corev1.SecretTypeDockerConfigJson,
	}
}

// handleVMI
//
// synopsis:
//
//	Every namespace running virtual machines, must contain registry
//	secret to be able pulling the virt-launcher image.
//	The virt-launcher image re-pushed in deckhouse registry to make
//	it possible to run in closed environments.
//	If in namespace delete all VMIs then registry secret must be deleted.
//	We do not use ownerReferences because virt-controller may to create
//	multiple VMIs in one namespace in one time.
//
//	We have patched virt-controller for specifying image pull-secrets
//	for envery VMI pod. This is temproray solution, until kubevirt
//	will have native opportunity for specifying registrySecrets.
func handleVMI(input *go_hook.HookInput) error {
	// KubeVirt manages it's own CRDs, so we need to wait for them before starting the watch
	if createRegistrySecretForVMIHookConfig.Kubernetes[0].Kind == "" {
		if len(input.Snapshots[kubevirtVMIsCRDSnapshot]) > 0 {
			// KubeVirt installed
			input.LogEntry.Infof("KubeVirt VirtualMachine CRD installed, update kind for binding VirtualMachines.kubevirt.io")
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       vmisSnapshot,
				Action:     "UpdateKind",
				ApiVersion: "kubevirt.io/v1",
				Kind:       "VirtualMachineInstance",
			})
			// Save new kind as current kind.
			createRegistrySecretForVMIHookConfig.Kubernetes[0].Kind = "VirtualMachineInstance"
			createRegistrySecretForVMIHookConfig.Kubernetes[0].ApiVersion = "kubevirt.io/v1"
			// Binding changed, hook will be restarted with new objects in snapshot.
			return nil
		}
		// KubeVirt is not yet installed, do nothing
		return nil
	}

	// Start main hook logic
	d8RegistrySnap := input.Snapshots[d8RegistrySnapshot]
	if len(d8RegistrySnap) == 0 {
		input.LogEntry.Warnln("Registry secret not found. Skip")
		return nil
	}

	registryCfg := d8RegistrySnap[0].(registrySecret).Config

	vmisNss := set.NewFromSnapshot(input.Snapshots[vmisSnapshot])

	// namespace -> .dockerconfigjson content
	secretsByNs := map[string]string{}

	for _, sRaw := range input.Snapshots[secretsSnapshot] {
		regSecret := sRaw.(registrySecret)
		secretsByNs[regSecret.Namespace] = regSecret.Config
	}

	// create secrets
	for ns := range vmisNss {
		secretContent, secretExists := secretsByNs[ns]
		// secret already exists in namespace. do not create or patch
		if !secretExists || secretContent != registryCfg {
			secret := prepareVirtRegistrySecret(ns, registryCfg)
			input.PatchCollector.Create(secret, object_patch.UpdateIfExists())
		}
	}

	// gc secrets
	for ns := range secretsByNs {
		if vmisNss.Has(ns) {
			// a secret exists in namespace, and exists one more VMI. do not delete secret
			continue
		}

		input.PatchCollector.Delete("v1", "Secret", ns, virtRegistrySecretName)
	}

	return nil
}
