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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	modulesSnapName             = "modules"
	packageVersionsSnapName     = "package-versions"
	deckhouseDeploymentSnapName = "deckhouse-deployment"

	// embeddedRepositoryName labels the package versions of the modules the image ships.
	embeddedRepositoryName = "embedded"

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
				ApiVersion:                   "deckhouse.io/v1alpha2",
				Kind:                         "Module",
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					var module v1alpha2.Module

					err := sdk.FromUnstructured(obj, &module)
					if err != nil {
						return nil, fmt.Errorf("failed to convert Module object to struct: %v", err)
					}

					if !module.IsEmbedded() {
						return nil, nil
					}

					return embeddedModuleModel{Name: module.Name, Version: module.Spec.PackageVersion}, nil
				},
			},
			// whether an embedded module is critical is what its package version says
			{
				Name:                         packageVersionsSnapName,
				ExecuteHookOnEvents:          go_hook.Bool(false),
				ExecuteHookOnSynchronization: go_hook.Bool(false),
				ApiVersion:                   "deckhouse.io/v1alpha1",
				Kind:                         "ModulePackageVersion",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{v1alpha1.ModulePackageVersionLabelRepository: embeddedRepositoryName},
				},
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					var version v1alpha1.ModulePackageVersion

					err := sdk.FromUnstructured(obj, &version)
					if err != nil {
						return nil, fmt.Errorf("failed to convert ModulePackageVersion object to struct: %v", err)
					}

					critical := version.Status.PackageMetadata != nil && version.Status.PackageMetadata.Critical

					return packageVersionModel{Package: version.Spec.PackageName, Version: version.Spec.PackageVersion, Critical: critical}, nil
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
	moduleNames, err := criticalModuleNames(input)
	if err != nil {
		return nil, err
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

// embeddedModuleModel is an embedded module and the package version it runs.
type embeddedModuleModel struct {
	Name    string
	Version string
}

// packageVersionModel is a package version of an embedded module and whether it is critical.
type packageVersionModel struct {
	Package  string
	Version  string
	Critical bool
}

// criticalModuleNames returns the camel-cased names of the critical embedded modules. A
// module whose package version is not synced yet cannot be told apart from a non-critical
// one and is skipped with a warning rather than failing the whole check.
func criticalModuleNames(input *go_hook.HookInput) ([]string, error) {
	modules, err := helpers.SnapshotToList[embeddedModuleModel](input, modulesSnapName)
	if err != nil {
		return nil, fmt.Errorf("cannot get modules snapshot: %w", err)
	}

	versions, err := helpers.SnapshotToList[packageVersionModel](input, packageVersionsSnapName)
	if err != nil {
		return nil, fmt.Errorf("cannot get package versions snapshot: %w", err)
	}

	critical := make(map[string]bool, len(versions))
	for _, version := range versions {
		critical[version.Package+"/"+version.Version] = version.Critical
	}

	names := make([]string, 0, len(modules))
	for _, module := range modules {
		isCritical, synced := critical[module.Name+"/"+module.Version]
		if !synced {
			input.Logger.Warn("embedded module has no synced package version, skip it", "module", module.Name, "version", module.Version)
			continue
		}

		if isCritical {
			names = append(names, strcase.ToCamel(module.Name))
		}
	}

	return names, nil
}
