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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func filterSource(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ms v1alpha1.ModuleSource

	err := sdk.FromUnstructured(obj, &ms)
	if err != nil {
		return nil, err
	}

	if ms.Spec.Registry.Scheme == "" {
		// fallback to default https protocol
		ms.Spec.Registry.Scheme = "HTTPS"
	}

	// remove unused fields
	newms := v1alpha1.ModuleSource{
		TypeMeta: ms.TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name: ms.Name,
		},
		Spec: ms.Spec,
		Status: v1alpha1.ModuleSourceStatus{
			ModuleErrors: ms.Status.ModuleErrors,
		},
	}

	return newms, nil
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
	},
}, createDeckhouseModuleSourceAndPolicy)

func createDeckhouseModuleSourceAndPolicy(input *go_hook.HookInput) error {
	deckhouseRepo := input.Values.Get("global.modulesImages.registry.base").String() + "/modules"
	deckhouseDockerCfg := input.Values.Get("global.modulesImages.registry.dockercfg").String()
	deckhouseCA := input.Values.Get("global.modulesImages.registry.CA").String()

	ms := v1alpha1.ModuleSource{}
	if len(input.Snapshots["sources"]) > 0 {
		ms = input.Snapshots["sources"][0].(v1alpha1.ModuleSource)
	}

	if moduleSourceUpToDate(&ms, deckhouseRepo, deckhouseDockerCfg, deckhouseCA) {
		// return if ModuleSource deckhouse already exists and all params are equal
		return nil
	}

	// get scheme from values
	scheme := strings.ToUpper(input.Values.Get("global.modulesImages.registry.scheme").String())
	switch scheme {
	case "HTTP", "HTTPS":
	// pass

	default:
		scheme = "HTTPS"
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
			ReleaseChannel: ms.Spec.ReleaseChannel,
			Registry: v1alpha1.ModuleSourceSpecRegistry{
				Scheme:    scheme,
				Repo:      deckhouseRepo,
				DockerCFG: deckhouseDockerCfg,
				CA:        deckhouseCA,
			},
		},
	}

	o, err := sdk.ToUnstructured(&newms)
	if err != nil {
		return err
	}

	input.PatchCollector.Create(o, object_patch.UpdateIfExists())

	return nil
}

func moduleSourceUpToDate(ms *v1alpha1.ModuleSource, repo, cfg, ca string) bool {
	if ms.Spec.Registry.Repo != repo {
		return false
	}

	if ca != "" && ms.Spec.Registry.CA != ca {
		return false
	}

	if ms.Spec.Registry.DockerCFG != cfg {
		return false
	}

	return true
}
