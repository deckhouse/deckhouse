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

package get_cni_secret

import (
	"encoding/base64"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func applyCNISecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return "", err
	}

	if _, ok := secret.Data["cni"]; !ok {
		return "", fmt.Errorf("kube-system/d8-cni-configuration secret data field `cni` is absent: %q", secret.Data)
	}

	dataYAML, err := yaml.Marshal(secret.Data)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(dataYAML), nil
}

func RegisterHook(moduleName string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "cni_secret",
				ApiVersion: "v1",
				Kind:       "Secret",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cni-configuration"},
				},
				FilterFunc: applyCNISecretFilter,
			},
		},
	}, setCNISecretData(moduleName))
}

func setCNISecretData(moduleName string) func(input *go_hook.HookInput) error {
	return func(input *go_hook.HookInput) error {
		cniSecretSnap := input.Snapshots["cni_secret"]
		if len(cniSecretSnap) == 0 {
			input.Logger.Info("No cni secret received, skipping setting values")
			return nil
		}

		if len(cniSecretSnap) > 1 {
			input.Logger.Info("Multiple secret received, skipping setting values")
			return nil
		}

		path := fmt.Sprintf("%s.internal.cniSecretData", moduleName)
		input.Values.Set(path, cniSecretSnap[0].(string))
		return nil
	}
}
