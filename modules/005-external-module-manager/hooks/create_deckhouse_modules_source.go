// Copyright 2023 Flant JSC
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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/iancoleman/strcase"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/005-external-module-manager/hooks/internal/apis/v1alpha1"
)

type deckhouseSecret struct {
	Bundle         string
	ReleaseChannel string
}

func filterDeckhouseSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return deckhouseSecret{
		Bundle:         string(secret.Data["bundle"]),
		ReleaseChannel: string(secret.Data["releaseChannel"]),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// ensure crds hook has order 5, for creating a module source we should use greater number
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 6},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "sources",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ModuleSource",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			FilterFunc: filterSource,
		},
		{
			Name:       "deckhouse-secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-discovery"},
			},
			FilterFunc: filterDeckhouseSecret,
		},
	},
}, createDeckhouseModuleSource)

func createDeckhouseModuleSource(input *go_hook.HookInput) error {
	if input.Values.Get("global.modulesImages.registry.address").String() != "registry.deckhouse.io" {
		// For now, modules are only stored in the base deckhouse registry,
		// for other registries deploying this resource by default will only cause an error.
		return nil
	}

	if input.Values.Get("global.modulesImages.registry.path").String() == "/deckhouse/ce" {
		// For CE, there are no modules for now, but will be some in the future!
		return nil
	}

	newms := v1alpha1.ModuleSource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleSource",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "deckhouse",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
		Spec: v1alpha1.ModuleSourceSpec{
			Registry: v1alpha1.ModuleSourceSpecRegistry{
				Repo:      input.Values.Get("global.modulesImages.registry.base").String() + "/modules",
				DockerCFG: input.Values.Get("global.modulesImages.registry.dockercfg").String(),
			},
		},
	}

	ca := input.Values.Get("global.modulesImages.registry.CA").String()
	if ca != "" {
		newms.Spec.Registry.CA = ca
	}

	if len(input.Snapshots["deckhouse-secret"]) > 0 {
		ds := input.Snapshots["deckhouse-secret"][0].(deckhouseSecret)
		newms.Spec.ReleaseChannel = strcase.ToKebab(ds.ReleaseChannel)
	}

	if len(input.Snapshots["sources"]) > 0 {
		ms := input.Snapshots["sources"][0].(v1alpha1.ModuleSource)

		// Keep some options that users configured manually to prevent overriding
		// In the future, instead, it is possible to use the server-side apply instead of subscribing to the object.
		newms.Spec.ReleaseChannel = ms.Spec.ReleaseChannel
	}

	o, err := sdk.ToUnstructured(&newms)
	if err != nil {
		return err
	}

	input.PatchCollector.Create(o, object_patch.UpdateIfExists())

	return nil
}
