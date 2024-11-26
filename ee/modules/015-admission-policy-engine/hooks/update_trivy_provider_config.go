/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	insecureRegistryPrefix = "trivy.insecureRegistry."
	trivyProvider          = "trivy-provider"
	trivyProviderNs        = "d8-admission-policy-engine"
	registryCAKey          = "TRIVY_REGISTRY_CA"
	insecureKey            = "TRIVY_INSECURE"
)

type trivySettings struct {
	InsecureDbRegistry string
	InsecureRegistries map[string]string
	CustomCA           string
}

// hook for updating trivy provider's configmap in accordance with the operator-trivy module's settings
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/admission-policy-engine/trivy_provider_config",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "trivy_config",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"trivy-operator-trivy-config"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-operator-trivy"},
				},
			},
			FilterFunc:                   filterCM,
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
		},
		{
			Name:       "trivy_provider_config",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{trivyProvider},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{trivyProviderNs},
				},
			},
			FilterFunc:                   filterCM,
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
		},
	},
}, updateConfig)

func filterCM(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	data, found, err := unstructured.NestedStringMap(obj.Object, "data")
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("data field not found")
	}

	var settings trivySettings
	insecureRegistries := make(map[string]string, 0)
	for key, value := range data {
		switch {
		case key == insecureKey:
			settings.InsecureDbRegistry = value

		case strings.HasPrefix(key, insecureRegistryPrefix):
			insecureRegistries[key] = value

		case key == registryCAKey:
			settings.CustomCA = value
		}
	}
	settings.InsecureRegistries = insecureRegistries

	return settings, nil
}

func updateConfig(input *go_hook.HookInput) error {
	if set.NewFromValues(input.Values, "global.enabledModules").Has("operator-trivy") && input.ConfigValues.Get("admissionPolicyEngine.denyVulnerableImages.enabled").Bool() {
		var (
			restartRequired bool
			// default settings
			trivyConfig = trivySettings{
				InsecureDbRegistry: "false",
			}
		)
		if len(input.Snapshots["trivy_config"]) != 0 {
			trivyConfig = input.Snapshots["trivy_config"][0].(trivySettings)
		}

		resultingCm := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      trivyProvider,
				Namespace: trivyProviderNs,
				Labels: map[string]string{
					"heritage": "deckhouse",
					"module":   "admission-policy-engine",
				},
			},
			Data: make(map[string]string, 0),
		}
		customCA := input.Values.Get("global.modulesImages.registry.CA").String()

		if len(input.Snapshots["trivy_provider_config"]) == 0 {
			restartRequired = true
			if len(customCA) != 0 {
				resultingCm.Data[registryCAKey] = customCA
			}

			resultingCm.Data[insecureKey] = trivyConfig.InsecureDbRegistry

			for k, v := range trivyConfig.InsecureRegistries {
				resultingCm.Data[k] = v
			}
		} else {
			providerConfig := input.Snapshots["trivy_provider_config"][0].(trivySettings)
			if (customCA != providerConfig.CustomCA) || (trivyConfig.InsecureDbRegistry != providerConfig.InsecureDbRegistry) || !maps.Equal(trivyConfig.InsecureRegistries, providerConfig.InsecureRegistries) {
				restartRequired = true
			}

			if len(customCA) != 0 {
				resultingCm.Data[registryCAKey] = customCA
			}
			resultingCm.Data[insecureKey] = trivyConfig.InsecureDbRegistry
			for k, v := range trivyConfig.InsecureRegistries {
				resultingCm.Data[k] = v
			}
		}

		if restartRequired {
			input.PatchCollector.Create(resultingCm, object_patch.UpdateIfExists())

			templatePatch := map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"annotations": map[string]string{
								"restartedAt": time.Now().Format(time.RFC3339),
							},
						},
					},
				},
			}
			input.PatchCollector.MergePatch(templatePatch, "apps/v1", "StatefulSet", trivyProviderNs, trivyProvider, object_patch.IgnoreMissingObject())
		}

		return nil
	}

	// either operator-trivy or trivy-provider is disabled, clean up the configmap
	input.PatchCollector.Delete("v1", "ConfigMap", "d8-admission-policy-engine", trivyProvider)

	return nil
}
