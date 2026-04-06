/*
Copyright 2026 Flant JSC

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret_publishapi_selfsigned_ca_migration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes-api-ca-key-pair"},
			},
			FilterFunc: filterApiCASecret,
		},
	},
}, copyCAPairToModuleValues)

func filterApiCASecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to Secret: %v", err)
	}

	return secret.Data, nil
}

func copyCAPairToModuleValues(_ context.Context, input *go_hook.HookInput) error {
	// if !(input.Values.Get("controlPlaneManager.apiserver.publishAPI.ingress.https.mode").Value() == "SelfSigned") {
	// 	fmt.Println("https mode is not SelfSigned, skipping")
	// 	return nil
	// }
	fmt.Println("Getting selfsigned publishAPI CA")
	keyPairs := input.Snapshots.Get("secret_publishapi_selfsigned_ca_migration")
	fmt.Println(keyPairs[0])

	selfSignedKeyPair := make(map[string]string)
	if len(keyPairs) > 0 {
		err := keyPairs[0].UnmarshalTo(&selfSignedKeyPair)

		if err != nil {
			return fmt.Errorf("failed to unmarshal 'secret_publishapi_selfsigned_ca_migration' snapshot: %w", err)
		}

		fmt.Println("Setting key pair into internal values")
		input.Values.Set("controlPlaneManager.internal.selfSignedCA", selfSignedKeyPair)

	} else {
		fmt.Println("'kubernetes-api-ca-key-pair' secret appears to not have data")
	}
	return nil
}
