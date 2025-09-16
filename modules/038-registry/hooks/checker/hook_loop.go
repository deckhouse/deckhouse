/*
Copyright 2025 Flant JSC

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

package checker

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/ettle/strcase"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	deckhouse_types "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	modulesSnapName             = "modules"
	deckhouseDeploymentSnapName = "deckhouse-deployment"

	moduleDigestsValuesPath = "global.modulesImages.digests"
	registryBaseValuesPath  = "global.modulesImages.registry.base"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/registry/checker/loop",
		Schedule: []go_hook.ScheduleConfig{
			{
				Name:    "checker loop every 10 sec",
				Crontab: "*/10 * * * * *", // every 10 sec

			},
		},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:                         modulesSnapName,
				ExecuteHookOnEvents:          go_hook.Bool(false),
				ExecuteHookOnSynchronization: go_hook.Bool(false),
				ApiVersion:                   "deckhouse.io/v1alpha1",
				Kind:                         "Module",
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					var module deckhouse_types.Module

					err := sdk.FromUnstructured(obj, &module)
					if err != nil {
						return nil, fmt.Errorf("failed to convert Module object to struct: %v", err)
					}

					if !module.IsEmbedded() {
						return nil, nil
					}

					if !module.Properties.Critical {
						return nil, nil
					}

					return strcase.ToCamel(module.Name), nil
				},
			},
			{
				Name:                         deckhouseDeploymentSnapName,
				ExecuteHookOnEvents:          go_hook.Bool(false),
				ExecuteHookOnSynchronization: go_hook.Bool(false),
				ApiVersion:                   "apps/v1",
				Kind:                         "Deployment",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-system"},
					},
				},
				NameSelector: &types.NameSelector{
					MatchNames: []string{"deckhouse"},
				},
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					var deployment appsv1.Deployment

					err := sdk.FromUnstructured(obj, &deployment)
					if err != nil {
						return nil, fmt.Errorf("cannot convert deckhouse deployment to struct: %v", err)
					}

					containers := deployment.Spec.Template.Spec.Containers
					initContainers := deployment.Spec.Template.Spec.InitContainers

					ret := deckhouseImagesModel{
						InitContainers: make(map[string]string),
						Containers:     make(map[string]string),
					}

					for _, c := range initContainers {
						ret.InitContainers[c.Name] = c.Image
					}

					for _, c := range containers {
						ret.Containers[c.Name] = c.Image
					}

					return ret, nil
				},
			},
		},
	},
	func(ctx context.Context, input *go_hook.HookInput) error {
		var err error

		stateAccessor := helpers.NewValuesAccessor[stateModel](input, valuesStatePath)
		state := stateAccessor.Get()

		inputs := inputsModel{
			Params: GetParams(ctx, input),
		}
		inputs.ImagesInfo.Repo = input.Values.Get(registryBaseValuesPath).String()
		inputs.ImagesInfo.DeckhouseImages, err = helpers.SnapshotToSingle[deckhouseImagesModel](input, deckhouseDeploymentSnapName)
		if err != nil {
			return fmt.Errorf("cannot get deckhouse deployment snapshot: %w", err)
		}

		inputs.ImagesInfo.ModulesImagesDigests, err = getModulesImagesDigests(ctx, input)
		if err != nil {
			return fmt.Errorf("cannot get modules images: %w", err)
		}

		if err := state.Process(input.Logger, inputs); err != nil {
			return err
		}

		stateAccessor.Set(state)

		return nil
	},
)

func getModulesImagesDigests(_ context.Context, input *go_hook.HookInput) (map[string]string, error) {
	moduleNames, err := helpers.SnapshotToList[string](input, modulesSnapName)
	if err != nil {
		return nil, fmt.Errorf("cannot get modules snapshot: %w", err)
	}

	if len(moduleNames) == 0 {
		return nil, fmt.Errorf("modules snapshot contains no entries")
	}

	sort.Strings(moduleNames) // for stable results
	digests := make(map[string]string)
	for _, module := range moduleNames {
		valuesPath := fmt.Sprintf("%v.%v", moduleDigestsValuesPath, module)

		images, err := helpers.GetValue[map[string]string](input, valuesPath)
		if err != nil && !errors.Is(err, helpers.ErrNoValue) {
			return nil, fmt.Errorf("cannot get images digests for module %q: %w", module, err)
		}

		imageNames := make([]string, 0, len(images))
		for k := range images {
			imageNames = append(imageNames, k)
		}
		sort.Strings(imageNames) // for stable results

		for _, image := range imageNames {
			d := images[image]
			if _, ok := digests[d]; ok {
				continue
			}

			info := fmt.Sprintf("module/%v/%v", strcase.ToKebab(module), strcase.ToKebab(image))
			digests[d] = info
		}
	}

	if len(digests) == 0 {
		return nil, fmt.Errorf("modules has no images")
	}

	return digests, nil
}
