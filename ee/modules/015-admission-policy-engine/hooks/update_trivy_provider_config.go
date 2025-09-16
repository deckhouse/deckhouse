/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	insecureRegistryPrefix = "trivy.insecureRegistry."
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

func updateConfig(_ context.Context, input *go_hook.HookInput) error {
	if set.NewFromValues(input.Values, "global.enabledModules").Has("operator-trivy") && input.ConfigValues.Get("admissionPolicyEngine.denyVulnerableImages.enabled").Bool() {
		var (
			// default settings
			trivyConfig = trivySettings{
				InsecureDbRegistry: "false",
			}
		)

		snaps, err := sdkobjectpatch.UnmarshalToStruct[trivySettings](input.Snapshots, "trivy_config")
		if err != nil {
			return fmt.Errorf("failed to unmarshal trivy_config snapshot: %w", err)
		}

		if len(snaps) != 0 {
			trivyConfig = snaps[0]
		}

		trivyData := make(map[string]string, 0)
		customCA := input.Values.Get("global.modulesImages.registry.CA").String()

		if len(customCA) != 0 {
			trivyData[registryCAKey] = customCA
		}

		trivyData[insecureKey] = trivyConfig.InsecureDbRegistry
		for k, v := range trivyConfig.InsecureRegistries {
			trivyData[k] = v
		}

		input.Values.Set("admissionPolicyEngine.internal.trivyConfigData", trivyData)

		return nil
	}

	// either operator-trivy or trivy-provider is disabled, clean up the configmap
	input.Values.Remove("admissionPolicyEngine.internal.trivyConfigData")

	return nil
}
