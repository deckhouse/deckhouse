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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const defaultReleaseChannel = "Stable"

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
}, createDeckhouseModuleSourceAndPolicy)

func createDeckhouseModuleSourceAndPolicy(input *go_hook.HookInput) error {
	deckhouseRepo := input.Values.Get("global.modulesImages.registry.base").String() + "/modules"
	deckhouseDockerCfg := input.Values.Get("global.modulesImages.registry.dockercfg").String()
	deckhouseCA := input.Values.Get("global.modulesImages.registry.CA").String()
	releaseChannel := defaultReleaseChannel

	if len(input.Snapshots["deckhouse-secret"]) > 0 {
		ds := input.Snapshots["deckhouse-secret"][0].(deckhouseSecret)
		releaseChannel = strcase.ToCamel(ds.ReleaseChannel)
	}

	ms := v1alpha1.ModuleSource{}
	if len(input.Snapshots["sources"]) > 0 {
		ms = input.Snapshots["sources"][0].(v1alpha1.ModuleSource)
	}

	if len(ms.Spec.ReleaseChannel) > 0 {
		// take releaseChannel from existing ModuleSource if it's set
		releaseChannel = sanitizeReleaseChannel(strcase.ToCamel(ms.Spec.ReleaseChannel))
	}

	// if not exists, ensure deckhouse ModuleUpdatePolicy
	deckhouseMup := &v1alpha1.ModuleUpdatePolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleUpdatePolicy",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "deckhouse",
		},
		Spec: v1alpha1.ModuleUpdatePolicySpec{
			ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"source": "deckhouse",
					},
				},
			},
			ReleaseChannel: releaseChannel,
			Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
				Mode: "Auto",
			},
		},
	}
	input.PatchCollector.Create(deckhouseMup, object_patch.IgnoreIfExists())

	if moduleSourceUpToDate(&ms, deckhouseRepo, deckhouseDockerCfg, deckhouseCA) {
		// return if ModuleSource deckhouse already exists and all params are equal
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
			ReleaseChannel: ms.Spec.ReleaseChannel,
			Registry: v1alpha1.ModuleSourceSpecRegistry{
				Scheme:    "HTTPS",
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

func sanitizeReleaseChannel(rc string) string {
	switch rc {
	case "Alpha", "Beta", "EarlyAccess", "Stable", "RockSolid":
		return rc
	default:
		return defaultReleaseChannel
	}
}
