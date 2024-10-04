/*
Copyright 2021 Flant JSC

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

package shared

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

// common part for filtering secret with Configuration checksums

// ConfigurationChecksum map of NodeGroup configurations: [nodeGroupName]: <checksum>
type ConfigurationChecksum map[string]string

func ConfigurationChecksumHookConfig() go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:                   "configuration_checksums_secret",
		WaitForSynchronization: ptr.To(false),
		ApiVersion:             "v1",
		Kind:                   "Secret",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-instance-manager"},
			},
		},
		NameSelector: &types.NameSelector{
			MatchNames: []string{"configuration-checksums"},
		},
		FilterFunc: filterChecksumSecret,
	}
}

func filterChecksumSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	data := make(map[string]string, len(sec.Data))
	for k, v := range sec.Data {
		data[k] = string(v)
	}

	return ConfigurationChecksum(data), nil
}
