/*
Copyright 2025 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	hubbleSettingsCMSnapshotName = "cilium-agent-hubble-settings-snapshot"
	hubbleSettingsCMName         = "cilium-agent-hubble-settings"
	hubbleSettingsCMNamespace    = "d8-cni-cilium"
)

// HubbleObservabilityConfig represents the structure of the Hubble settings stored in ConfigMap.
type HubbleObservabilityConfig struct {
	ExtendedMetrics struct {
		Enabled    bool `yaml:"enabled"`
		Collectors []struct {
			Name           string `yaml:"name"`
			ContextOptions string `yaml:"contextOptions,omitempty"`
		} `yaml:"collectors"`
	} `yaml:"extendedMetrics"`

	EventLogs struct {
		Enabled       bool     `yaml:"enabled"`
		Allowlist     []string `yaml:"allowList"`
		Denylist      []string `yaml:"denyList"`
		FieldMaskList []string `yaml:"fieldMaskList"`
		FileMaxSizeMb int      `yaml:"fileMaxSizeMb"`
	} `yaml:"flowLogs"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       hubbleSettingsCMSnapshotName,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{hubbleSettingsCMName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{hubbleSettingsCMNamespace},
				},
			},
			FilterFunc: filterHubbleSettings,
		},
	},
}, handleHubbleSettings)

// filterHubbleSettings extracts the settings.yaml content from the ConfigMap.
func filterHubbleSettings(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap
	if err := sdk.FromUnstructured(obj, &cm); err != nil {
		return nil, fmt.Errorf("cannot convert unstructured to ConfigMap: %w", err)
	}

	settings, ok := cm.Data["settings.yaml"]
	if !ok {
		return nil, fmt.Errorf("settings.yaml not found in %s/%s", cm.Namespace, cm.Name)
	}

	return settings, nil
}

// handleHubbleSettings parses the ConfigMap snapshot and stores it into Values.
func handleHubbleSettings(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get(hubbleSettingsCMSnapshotName)

	if len(snaps) == 0 {
		return nil
	}
	if len(snaps) > 1 {
		return fmt.Errorf("multiple snapshots found for %q", hubbleSettingsCMSnapshotName)
	}

	var settingsStr string
	if err := snaps[0].UnmarshalTo(&settingsStr); err != nil {
		return fmt.Errorf("cannot unmarshal: %w", err)
	}

	var settings map[string]interface{}
	if err := yaml.Unmarshal([]byte(settingsStr), &settings); err != nil {
		return fmt.Errorf("cannot unmarshal settings.yaml: %w", err)
	}

	input.Values.Set("cniCilium.internal.hubble.settings", settings)

	return nil
}
