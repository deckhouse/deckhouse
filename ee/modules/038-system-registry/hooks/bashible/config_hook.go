/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/helpers"
	common_models "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models"
	bashible_config "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models/config"
	bashible_input "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models/input"
	"github.com/deckhouse/deckhouse/go_lib/set"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	registry_models "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/bashible"
)

func BashibleConfigHook(order float64, queue string) bool {
	const (
		snapBashibleConfig = "bashibleConfig"
		snapMasterNodesIP  = "masterNodesIP"
	)

	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: order},
		Queue:        queue,
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:          snapMasterNodesIP,
				ApiVersion:    "v1",
				Kind:          "Node",
				LabelSelector: helpers.MasterNodeLabelSelector,
				FilterFunc:    filterNodeInternalIP,
			},
			{
				Name:       snapBashibleConfig,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry-bashible-config"},
				},
				NamespaceSelector: helpers.NamespaceSelector,
				FilterFunc:        bashible_config.FilterSecret,
			},
		},
	}, func(hookInput *go_hook.HookInput) error {
		inputData, err := bashible_input.Get(hookInput)
		if err != nil {
			return fmt.Errorf("failed to get input data: %w", err)
		}

		if inputData == nil {
			// Try to extract the current bashible configuration from the secrets
			currentBashibleConfig, err := bashible_config.ExtractFromSnapSecret(hookInput.Snapshots[snapBashibleConfig])
			if err != nil {
				return fmt.Errorf("failed to extract bashible config from secret: %w", err)
			}

			// If we successfully obtained the current configuration, set it if it doesn't already exist
			if currentBashibleConfig != nil {
				bashible_config.SetIfNotExist(hookInput, *currentBashibleConfig)
			}
			return nil
		}

		var (
			CACert common_models.CertModel
			user   common_models.UserModel
		)
		switch mode := inputData.Mode; mode {
		case registry_const.ModeProxy:
			if modeData := inputData.Proxy; modeData != nil {
				CACert = modeData.CA
				user = modeData.User
			} else {
				return fmt.Errorf("incorrect input data, empty proxy mode data")
			}
		case registry_const.ModeDetached:
			if modeData := inputData.Detached; modeData != nil {
				CACert = modeData.CA
				user = modeData.User
			} else {
				return fmt.Errorf("incorrect input data, empty detached mode data")
			}
		default:
			bashible_config.Remove(hookInput)
			return nil
		}

		masterNodesIPs := extractFromSnapNodeInternalIP(hookInput.Snapshots[snapMasterNodesIP])
		proxyEndpoints := make([]string, 0, len(masterNodesIPs))
		for _, masterNodesIP := range masterNodesIPs {
			proxyEndpoints = append(proxyEndpoints, fmt.Sprintf("%s:%d", masterNodesIP, registry_const.Port))
		}

		mirrors := createMirrors(user, []string{registry_const.ProxyHost})
		prepullMirrors := createMirrors(user, append([]string{registry_const.ProxyHost}, proxyEndpoints...))
		CA := []string{}
		if CACert.Cert != "" {
			CA = append(CA, CACert.Cert)
		}

		bashibleConfigModel := bashible_config.ConfigModel{
			Mode:           inputData.Mode,
			Version:        inputData.Version,
			ImagesBase:     registry_const.HostWithPath,
			ProxyEndpoints: proxyEndpoints,
			Hosts:          []registry_models.HostsObject{{Host: registry_const.Host, CA: CA, Mirrors: mirrors}},
			PrepullHosts:   []registry_models.HostsObject{{Host: registry_const.Host, CA: CA, Mirrors: prepullMirrors}},
		}

		bashible_config.Set(hookInput, bashibleConfigModel)
		return nil
	})
}

func filterNodeInternalIP(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, fmt.Errorf("failed to convert node to struct: %w", err)
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1core.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return nil, nil
}

func extractFromSnapNodeInternalIP(snaps []go_hook.FilterResult) []string {
	return set.NewFromSnapshot(snaps).Slice()
}

func createMirrors(rUser common_models.UserModel, hosts []string) []registry_models.MirrorHostObject {
	mirrors := make([]registry_models.MirrorHostObject, 0, len(hosts))
	for _, host := range hosts {
		mirrors = append(mirrors, registry_models.MirrorHostObject{
			Host:     host,
			Username: rUser.Name,
			Password: rUser.Password,
			Auth:     rUser.Auth(),
			Scheme:   registry_const.Scheme,
		})
	}
	return mirrors
}
