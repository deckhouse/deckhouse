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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/iancoleman/strcase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	defaultUpdateMode   = "Auto"
	migrationAnnotation = "modules.deckhouse.io/ensured-update-policies"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// ensure crds hook has order 5, for creating a module source we should use greater number
	OnStartup: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(createModuleUpdatePolicies))

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
	moduleSources, err := k8sCli.Dynamic().Resource(v1alpha1.ModuleSourceGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, source := range moduleSources.Items {
		moduleSource := &v1alpha1.ModuleSource{}
		err := sdk.FromUnstructured(&source, moduleSource)
		if err != nil {
			return err
		}

		// check if source releaseChannel can be camelCased correctly
		rc, err := camelCaseReleaseChannel(moduleSource.Spec.ReleaseChannel)
		if err != nil {
			input.LogEntry.Warnf("Couldn't create a ModuleUpdatePolicy for %s ModuleSource: %v", moduleSource.ObjectMeta.Name, err)
			continue
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
				ReleaseChannel: rc,
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

func camelCaseReleaseChannel(channel string) (string, error) {
	releaseChannel := strcase.ToCamel(channel)
	switch releaseChannel {
	case "Alpha", "Beta", "EarlyAccess", "Stable", "RockSolid":
		return releaseChannel, nil
	default:
		return "", fmt.Errorf("couldn't properly camelcase release channel")
	}
}
