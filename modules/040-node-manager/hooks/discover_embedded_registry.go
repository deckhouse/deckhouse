/*
Copyright 2024 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	embeddedRegistryPort = "5001"
	embeddedRegistry     = "embedded-registry.d8-system.svc"
)

type registryCredentials struct {
	Name     string
	Password string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/node-manager/discover-embedded-registry",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "system_registry",
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "system-registry", // TODO change to embedded-registry
					"tier":      "control-plane",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: embeddedRegistryPodFilter,
		},
		{
			Name:       "registry_pki_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-system"},
			},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-pki"},
			},
			FilterFunc: filterRegistryPkiSecret,
		},
		{
			Name:       "registry_user_ro_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-user-ro"},
			},
			FilterFunc: filterRegistryUserRoSecret,
		},
	},
}, handleEmbeddedRegistryData)

func embeddedRegistryPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pod object from unstructured: %v", err)
	}
	return fmt.Sprintf("%s:%s", pod.Status.HostIP, embeddedRegistryPort), nil
}

func filterRegistryPkiSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}

	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	caCertBytes, exists := secret.Data["registry-ca.crt"]

	if !exists {
		return nil, nil
	}
	return string(caCertBytes), nil
}

func filterRegistryUserRoSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	nameBytes, nameExists := secret.Data["name"]
	passwordBytes, passwordExists := secret.Data["password"]
	if !nameExists || !passwordExists {
		return nil, nil
	}

	return registryCredentials{
		Name:     string(nameBytes),
		Password: string(passwordBytes),
	}, nil
}

func handleEmbeddedRegistryData(input *go_hook.HookInput) error {
	endpointsSet := set.NewFromSnapshot(input.Snapshots["system_registry"])
	endpointsList := endpointsSet.Slice() // sorted

	if len(endpointsList) == 0 {
		return nil
	}

	// Set embedded registry endpoints
	input.LogEntry.Infof("found embedded registry endpoints: %v", endpointsList)
	input.Values.Set("nodeManager.internal.systemRegistry.addresses", endpointsList) // TODO systemRegistry to embeddedRegistry here and in code below

	// Get embedded registry CA from snapshot
	caCertSnap := input.Snapshots["registry_pki_secret"]
	if len(caCertSnap) == 0 {
		input.LogEntry.Warn("Secret registry-pki not found or empty")
		return nil
	}
	// Set embedded registry CA value
	caCert := caCertSnap[0].(string)
	input.LogEntry.Infof("found embedded registry CA")
	input.Values.Set("nodeManager.internal.systemRegistry.registryCA", caCert)

	// Get embedded registry credentials from snapshot
	credsSnap := input.Snapshots["registry_user_ro_secret"]

	if len(credsSnap) == 0 {
		input.LogEntry.Warn("Secret registry-user-ro not found or empty")
		return nil
	}

	registryCreds, exists := credsSnap[0].(registryCredentials)
	if !exists {
		input.LogEntry.Warn("Failed to parse registry-user-ro secret")
		return nil
	}

	// If credentials are present, set them
	if registryCreds.Name != "" && registryCreds.Password != "" {
		input.LogEntry.Infof("found embedded registry credentials")
		input.Values.Set("nodeManager.internal.systemRegistry.auth", map[string]string{
			"username": registryCreds.Name,
			"password": registryCreds.Password,
		})
		// Set embedded registry embeddedRegistry only if credentials are present
		input.LogEntry.Infof("setting embedded registry embeddedRegistry to %s", embeddedRegistry)
		input.Values.Set("nodeManager.internal.systemRegistry.address", embeddedRegistry+":"+embeddedRegistryPort)
	}

	return nil
}
