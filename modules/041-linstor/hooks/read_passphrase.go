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

/*
Some features as backup shipping and luks encryption requires master passphrase set
This hook reads secret d8-system/linstor-passphrase and specifies it for LINSTOR.
*/

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applyMasterPassphraseFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert master passphrase secret to secret: %v", err)
	}

	return string(secret.Data["MASTER_PASSPHRASE"]), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_passphrase",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"linstor-passphrase"},
			},
			FilterFunc: applyMasterPassphraseFilter,
		},
	},
}, applyMasterPassphrase)

func applyMasterPassphrase(input *go_hook.HookInput) error {
	var passphrase string

	snaps := input.Snapshots["master_passphrase"]
	for _, snap := range snaps {
		passphrase = snap.(string)
	}

	if passphrase != "" {
		input.Values.Set("linstor.internal.masterPassphrase", passphrase)
	} else {
		input.Values.Remove("linstor.internal.masterPassphrase")
	}

	return nil
}
