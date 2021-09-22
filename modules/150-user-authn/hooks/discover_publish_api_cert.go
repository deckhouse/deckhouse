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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

type PublishAPICert struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

func applyPublishAPICertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return PublishAPICert{Name: obj.GetName(), Data: secret.Data["ca.crt"]}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes-tls", "kubernetes-tls-customcertificate"},
			},
			FilterFunc: applyPublishAPICertFilter,
		},
	},
}, discoverPublishAPICA)

func discoverPublishAPICA(input *go_hook.HookInput) error {
	secretPath := "userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA"

	caSecrets, ok := input.Snapshots["secret"]
	if !ok {
		return nil
	}

	if module.GetHTTPSMode("userAuthn", input) == "OnlyInURI" {
		input.Values.Remove(secretPath)
		return nil
	}

	secret, ok := caSecrets[0].(PublishAPICert)
	if !ok {
		return fmt.Errorf("cannot convert secret to publish api secret")
	}

	if len(secret.Data) > 0 {
		input.Values.Set(secretPath, string(secret.Data))
	} else {
		input.Values.Remove(secretPath)
	}

	return nil
}
