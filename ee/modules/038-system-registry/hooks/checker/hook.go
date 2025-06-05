/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package checker

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ettle/strcase"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	gcr_name "github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	deckhouse_types "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

const (
	modulesSnapName             = "modules"
	deckhouseDeploymentSnapName = "deckhouse-deployment"

	moduleDigestsValuesPath = "global.modulesImages.digests"
	registryBaseValuesPath  = "global.modulesImages.registry.base"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/system-registry/checker",
		Schedule: []go_hook.ScheduleConfig{
			{
				Name:    "checker",
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

					r := module.Properties.Requirements

					if r != nil && strings.ToLower(r.Bootstrapped) == "true" {
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
	func(input *go_hook.HookInput) error {
		startTime := time.Now()
		paramsAccessor := helpers.NewValuesAccessor[Params](input, "systemRegistry.internal.checker.params")
		stateAccessor := helpers.NewValuesAccessor[State](input, "systemRegistry.internal.checker.state")

		state := stateAccessor.Get()

		inputs := Inputs{
			Params: prepareParams(paramsAccessor.Get()),
		}
		paramsAccessor.Set(inputs.Params) // testing

		inputs.ImagesInfo.Repo = input.Values.Get(registryBaseValuesPath).String()

		var err error

		inputs.ImagesInfo.DeckhouseImages, err = helpers.SnapshotToSingle[deckhouseImagesModel](input, deckhouseDeploymentSnapName)
		if err != nil {
			return fmt.Errorf("cannot get deckhouse deployment snapshot: %w", err)
		}

		inputs.ImagesInfo.ModulesImagesDigests, err = getModulesImagesDigests(input)
		if err != nil {
			return fmt.Errorf("cannot get modules images: %w", err)
		}

		if err := state.process(inputs); err != nil {
			return err
		}

		var imagesCount int64
		for _, v := range state.Queues {
			imagesCount += int64(len(v.Items)) + int64(len(v.Retry))
		}

		stateAccessor.Set(state)

		executionDuration := time.Since(startTime)
		input.Logger.Warn(
			"ImageChecker Run",
			"images.count", imagesCount,
			"execution.start", startTime,
			"execution.duration", executionDuration.String(),
		)

		return nil
	},
)

func getModulesImagesDigests(input *go_hook.HookInput) (map[string]string, error) {
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

func prepareParams(value Params) Params {
	repoRef1, err := gcr_name.NewRepository("fake-registry.local/flant/deckhouse")
	if err != nil {
		panic(err)
	}

	repoRef2, err := gcr_name.NewRepository("test-registry.local/flant/dkp")
	if err != nil {
		panic(err)
	}

	repos := map[string]gcr_name.Repository{
		"fake": repoRef1,
		"test": repoRef2,
	}

	if value.Registries == nil {
		value.Registries = make(map[string]RegistryParams)
	}
	for k, v := range repos {
		item := value.Registries[k]

		item.Address = v.Registry.Name()
		item.Scheme = strings.ToUpper(v.Registry.Scheme())

		value.Registries[k] = item
	}

	return value
}
