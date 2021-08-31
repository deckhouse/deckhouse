// Copyright 2021 Flant CJSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1apps "k8s.io/api/apps/v1"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	imageModulesD8RegistrySnap     = "d8_deployment"
	imageModulesD8RegistryConfSnap = "d8_registry_secret"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       imageModulesD8RegistrySnap,
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyD8ImageFilter,
		},
		{
			Name:       imageModulesD8RegistryConfSnap,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-registry"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyD8RegistrySecretFilter,
		},
	},
}, discoveryDeckhouseRegistry)

func applyD8ImageFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var deployment v1apps.Deployment
	err := sdk.FromUnstructured(obj, &deployment)
	if err != nil {
		return nil, err
	}

	// get werf stages repo
	// get it from deckhouse image
	image := deployment.Spec.Template.Spec.Containers[0].Image
	// remove branch or channel
	image = strings.Split(image, ":")[0]
	// dev-deckhouse image is 'dev' name . remove it
	// because stages is in repo
	image = strings.TrimSuffix(image, "/dev")

	return image, nil
}

func applyD8RegistrySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	registryCnf, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return nil, fmt.Errorf("not deckhouse found docker config in secret")
	}

	return registryCnf, nil
}

func discoveryDeckhouseRegistry(input *go_hook.HookInput) error {
	registrySnap := input.Snapshots[imageModulesD8RegistrySnap]
	registryConfSnap := input.Snapshots[imageModulesD8RegistryConfSnap]

	if len(registrySnap) == 0 {
		return fmt.Errorf("not found deckhouse deployment")
	}

	if len(registryConfSnap) == 0 {
		return fmt.Errorf("not found deckhouse registry conf secret")
	}

	registryConfRaw := registryConfSnap[0].([]byte)
	// yes, we store base64 encoded string but in secret object store decoded data
	// In values we store base64-encoded docker config because in this form it is applied in other places.
	registryConfEncoded := base64.StdEncoding.EncodeToString(registryConfRaw)

	input.Values.Set("global.modulesImages.registry", registrySnap[0].(string))
	input.Values.Set("global.modulesImages.registryDockercfg", registryConfEncoded)

	return nil
}
