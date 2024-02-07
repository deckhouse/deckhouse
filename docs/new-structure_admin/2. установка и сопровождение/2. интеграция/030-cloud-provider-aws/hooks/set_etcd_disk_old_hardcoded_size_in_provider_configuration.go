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

// TODO: can be removed after release 1.45

package hooks

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
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
			ExecuteHookOnEvents: pointer.Bool(false),
			FilterFunc:          applyProviderClusterConfigurationSecretFilter,
		},
	},
}, patchClusterConfiguration)

func patchClusterConfiguration(input *go_hook.HookInput) error {
	if len(input.Snapshots["provider_cluster_configuration"]) == 0 {
		return fmt.Errorf("%s", "Can't find Secret d8-provider-cluster-configuration in Namespace kube-system")
	}

	secret := input.Snapshots["provider_cluster_configuration"][0].(*v1.Secret)

	data := secret.Data["cloud-provider-cluster-configuration.yaml"]

	var clusterConfiguration map[string]interface{}

	err := yaml.Unmarshal(data, &clusterConfiguration)
	if err != nil {
		return err
	}

	value, _, err := unstructured.NestedString(clusterConfiguration, "masterNodeGroup", "instanceClass", "etcdDisk", "type")
	if err != nil {
		return err
	}

	// skip if values are set
	if value != "" {
		return nil
	}

	etcdDiskValues := map[string]interface{}{
		"sizeGb": int64(150),
		"type":   "gp2",
	}

	err = unstructured.SetNestedField(clusterConfiguration, etcdDiskValues, "masterNodeGroup", "instanceClass", "etcdDisk")
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(nil)
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	err = enc.Encode(clusterConfiguration)
	if err != nil {
		return err
	}

	patch := map[string]interface{}{
		"data": map[string]string{
			"cloud-provider-cluster-configuration.yaml": base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	}

	input.PatchCollector.MergePatch(patch, "v1", "Secret", secret.Namespace, secret.Name)

	return nil
}
