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

// This migration hook creates module update policies for all but deckhouse module sources on first run.
// After creation, d8-system namespace gets annotated and migration doesn't fire up anymore.
package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	defaultUpdateMode   = "Auto"
	migrationAnnotation = "modules.deckhouse.io/ensured-update-policies"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// ensure crds hook has order 5, for creating a module source we should use greater number
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 6},
}, dependency.WithExternalDependencies(createModuleUpdatePolicies))

var msResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "modulesources"}

func createModuleUpdatePolicies(input *go_hook.HookInput, dc dependency.Container) error {
	k8sCli, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	d8System, err := k8sCli.CoreV1().Namespaces().Get(context.TODO(), "d8-system", metav1.GetOptions{})
	if err != nil {
		return err
	}

	// have already run the migration
	if _, migrated := d8System.ObjectMeta.Annotations[migrationAnnotation]; migrated {
		return nil
	}

	// get all modulesources
	moduleSources, err := k8sCli.Dynamic().Resource(msResource).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, source := range moduleSources.Items {
		// skip deckhouse source as its update policy is ensured by another hook
		if source.GetName() == "deckhouse" {
			continue
		}
		moduleSource := &v1alpha1.ModuleSource{}
		err := sdk.FromUnstructured(&source, moduleSource)
		if err != nil {
			return err
		}

		// if not exists, ensure ModuleUpdatePolicy
		deckhouseMup := &v1alpha1.ModuleUpdatePolicy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ModuleUpdatePolicy",
				APIVersion: "deckhouse.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: moduleSource.ObjectMeta.Name,
			},
			Spec: v1alpha1.ModuleUpdatePolicySpec{
				ModuleReleaseSelector: v1alpha1.ModuleUpdatePolicySpecReleaseSelector{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"source": moduleSource.ObjectMeta.Name,
						},
					},
				},
				ReleaseChannel: moduleSource.Spec.ReleaseChannel,
				Update: v1alpha1.ModuleUpdatePolicySpecUpdate{
					Mode: "Auto",
				},
			},
		}
		input.PatchCollector.Create(deckhouseMup, object_patch.IgnoreIfExists())
	}

	// annotate d8-system as migrated
	d8SystemPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				migrationAnnotation: "",
			},
		},
	}
	input.PatchCollector.MergePatch(d8SystemPatch, "v1", "Namespace", "", "d8-system")

	return nil
}
