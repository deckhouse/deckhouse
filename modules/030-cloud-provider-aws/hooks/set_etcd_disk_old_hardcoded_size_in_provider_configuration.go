/*
Copyright 2022 Flant JSC

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
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-provider-cluster-configuration"},
			},
			FilterFunc: applyProviderClusterConfigurationSecretFilter,
		},
	},
}, patchClusterConfiguration)

func patchClusterConfiguration(input *go_hook.HookInput) error {
	if len(input.Snapshots["provider_cluster_configuration"]) == 0 {
		return fmt.Errorf("%s", "Can't find Secret d8-provider-cluster-configuration in Namespace kube-system")
	}

	secret := input.Snapshots["provider_cluster_configuration"][0].(*v1.Secret)

	clusterConfiguration := string(secret.Data["cloud-provider-cluster-configuration.yaml"])

	if len(clusterConfiguration) == 0 {
		return fmt.Errorf("%s", "Something went wrong, cloud-provider-cluster-configuration.yaml has zero size")
	}

	// If "etcdDisk" is present in the config, then the hook has already worked.
	if strings.Contains(clusterConfiguration, "etcdDisk") {
		return nil
	}

	insert := `    etcdDisk:
      sizeGb: 150
      type: gp2
`

	// "masterNodeGroup:\n" - 17 characters
	// "  instanceClass:\n" - 17 characters
	beforeInsert := clusterConfiguration[:strings.Index(clusterConfiguration, "masterNodeGroup:")+17+17]
	afterInsert := clusterConfiguration[strings.Index(clusterConfiguration, "masterNodeGroup:")+17+17:]

	newClusterConfiguration := beforeInsert + insert + afterInsert

	patch := map[string]interface{}{
		"data": map[string]string{
			"cloud-provider-cluster-configuration.yaml": base64.StdEncoding.EncodeToString([]byte(newClusterConfiguration)),
		},
	}

	input.PatchCollector.MergePatch(patch, "v1", "Secret", secret.Namespace, secret.Name)

	return nil
}
