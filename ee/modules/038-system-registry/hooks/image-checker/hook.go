/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package imagechecker

import (
	"errors"
	"fmt"
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
		Queue: "/modules/system-registry/image-checker",
		Schedule: []go_hook.ScheduleConfig{
			{
				Name:    "image-checker",
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
						InitContainers: make(map[string]gcr_name.Reference),
						Containers:     make(map[string]gcr_name.Reference),
					}

					for _, c := range initContainers {
						ref, err := gcr_name.ParseReference(c.Image)
						if err != nil {
							return nil, fmt.Errorf(
								"cannot parse reference %q for deckhouse init container %q: %w",
								c.Image, c.Name, err,
							)
						}

						ret.InitContainers[c.Name] = ref
					}

					for _, c := range containers {
						ref, err := gcr_name.ParseReference(c.Image)
						if err != nil {
							return nil, fmt.Errorf(
								"cannot parse reference %q for deckhouse container %q: %w",
								c.Image, c.Name, err,
							)
						}

						ret.Containers[c.Name] = ref
					}

					return ret, nil
				},
			},
		},
	},
	func(input *go_hook.HookInput) error {
		startTime := time.Now()

		log := input.Logger

		images, err := collectImages(input)
		if err != nil {
			return fmt.Errorf("cannot collect images: %w", err)
		}

		logImages := make(map[string]string)
		for k, v := range images {
			logImages[k] = v.String()
		}

		executionDuration := time.Since(startTime)
		log.Warn(
			"ImageChecker Run",
			"images.items", logImages,
			"images.count", len(logImages),
			"execution.start", startTime,
			"execution.duration", executionDuration.String(),
		)

		return nil
	},
)

func collectImages(input *go_hook.HookInput) (map[string]gcr_name.Reference, error) {
	registryBase := input.Values.Get(registryBaseValuesPath).String()

	repo, err := gcr_name.NewRepository(registryBase)
	if err != nil {
		return nil, fmt.Errorf("cannot parse registryBase %q value: %w", registryBase, err)
	}

	images, err := collectModulesImages(input, repo)
	if err != nil {
		return images, fmt.Errorf("cannot collect module images: %w", err)
	}

	if len(images) == 0 {
		return images, fmt.Errorf("modules has no images")
	}

	deckhouseImages, err := helpers.SnapshotToSingle[deckhouseImagesModel](input, deckhouseDeploymentSnapName)
	if err != nil {
		return images, fmt.Errorf("cannot get deckhouse deployment snapshot: %w", err)
	}

	for name, image := range deckhouseImages.InitContainers {
		if !strings.HasPrefix(image.Context().RepositoryStr(), repo.RepositoryStr()) {
			continue
		}

		key := fmt.Sprintf("deckhouse/init-containers/%v", name)
		images[key] = image
	}

	for name, image := range deckhouseImages.Containers {
		if !strings.HasPrefix(image.Context().RepositoryStr(), repo.RepositoryStr()) {
			continue
		}

		key := fmt.Sprintf("deckhouse/containers/%v", name)
		images[key] = image
	}

	return images, nil
}

func collectModulesImages(input *go_hook.HookInput, repo gcr_name.Repository) (map[string]gcr_name.Reference, error) {
	moduleNames, err := helpers.SnapshotToList[string](input, modulesSnapName)
	if err != nil {
		return nil, fmt.Errorf("cannot get modules snapshot: %w", err)
	}

	if len(moduleNames) == 0 {
		return nil, fmt.Errorf("modules snapshot contains no entries")
	}

	images := make(map[string]gcr_name.Reference)
	for _, name := range moduleNames {
		valuesPath := fmt.Sprintf("%v.%v", moduleDigestsValuesPath, name)

		moduleImages, err := helpers.GetValue[map[string]string](input, valuesPath)
		if err != nil && !errors.Is(err, helpers.ErrNoValue) {
			return nil, fmt.Errorf("cannot get images digests for module %q: %w", name, err)
		}

		for n, d := range moduleImages {
			key := fmt.Sprintf("module/%v/%v", strcase.ToKebab(name), strcase.ToKebab(n))
			images[key] = repo.Digest(d)
		}
	}

	return images, nil
}
