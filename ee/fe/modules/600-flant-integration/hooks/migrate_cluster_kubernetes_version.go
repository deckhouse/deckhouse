/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type ClusterConfigurationYaml struct {
	Content []byte
}

func applySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	cc := &ClusterConfigurationYaml{}

	ccYaml, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf(`"cluster-configuration.yaml" not found in "d8-cluster-configuration" Secret`)
	}

	cc.Content = ccYaml

	return cc, err
}

var (
	_ = sdk.RegisterFunc(&go_hook.HookConfig{
		OnStartup: &go_hook.OrderedConfig{Order: 20},
		Queue:     "/modules/flant-integration",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "clusterConfiguration",
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cluster-configuration"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				FilterFunc: applySecretFilter,
			},
		},
	}, migrateClusterKubernetesVersion)
)

func migrateClusterKubernetesVersion(input *go_hook.HookInput) error {
	currentConfig, ok := input.Snapshots["clusterConfiguration"]
	if !ok || len(currentConfig) == 0 {
		input.LogEntry.Info(`cannot find kube-system/d8-cluster-configuration secret, or cannot find "cluster-configuration.yaml" in data, skipping`)
		return nil
	}

	// FilterResult is a YAML encoded as a JSON string. Unmarshal it.
	configYamlBytes := currentConfig[0].(*ClusterConfigurationYaml)

	var metaConfig *config.MetaConfig
	metaConfig, err := config.ParseConfigFromData(string(configYamlBytes.Content))
	if err != nil {
		return err
	}

	kubernetesVersionFromMetaConfig, err := rawMessageToString(metaConfig.ClusterConfig["kubernetesVersion"])
	if err != nil {
		return err
	}

	if kubernetesVersionFromMetaConfig != config.DefaultKubernetesVersion {
		// No need to patch secret
		return nil
	}

	b, err := json.Marshal("Automatic")
	if err != nil {
		return err
	}
	metaConfig.ClusterConfig["kubernetesVersion"] = b

	c, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(c)
	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"cluster-configuration.yaml": encoded,
		},
	}
	input.PatchCollector.MergePatch(patch, "v1", "Secret", "kube-system", "d8-cluster-configuration")

	return nil
}

func rawMessageToString(message json.RawMessage) (string, error) {
	var result string
	b, err := message.MarshalJSON()
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(b, &result)
	return result, err
}
