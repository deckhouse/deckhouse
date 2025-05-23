/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package imagechecker

import (
	"errors"
	"fmt"
	"math/rand"
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

		log := input.Logger

		repoRef, err := gcr_name.NewRepository("fake-registry.local/flant/deckhouse")
		if err != nil {
			panic(err)
		}

		repos := map[string]gcr_name.Repository{
			"registry": repoRef,
		}

		images, err := buildQueue(input, repos)
		if err != nil {
			return fmt.Errorf("cannot collect images: %w", err)
		}

		executionDuration := time.Since(startTime)
		log.Warn(
			"ImageChecker Run",
			"images.items", images,
			"images.count", len(images),
			"execution.start", startTime,
			"execution.duration", executionDuration.String(),
		)

		return nil
	},
)

func buildQueue(input *go_hook.HookInput, repos map[string]gcr_name.Repository) ([]queueItem, error) {
	if len(repos) == 0 {
		return nil, fmt.Errorf("no repos passed")
	}

	repoStr := input.Values.Get(registryBaseValuesPath).String()
	repo, err := gcr_name.NewRepository(repoStr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse registry base %q: %w", repoStr, err)
	}

	deckhouseImages, err := collectDeckhouseImages(input, repo)
	if err != nil {
		return nil, fmt.Errorf("cannot collect deckhouse images: %w", err)
	}

	images, err := collectModulesImages(input, repo)
	if err != nil {
		return nil, fmt.Errorf("cannot collect module images: %w", err)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("modules has no images")
	}

	// Merge deckhouse images to modules images
	for k, v := range deckhouseImages {
		images[k] = v
	}

	ret := make([]queueItem, 0, len(images)*len(repos))
	for image, info := range images {
		ref, err := gcr_name.ParseReference(image)
		if err != nil {
			return ret, fmt.Errorf("cannot parse image %q (%q) reference: %w", image, info, err)
		}

		if !strings.HasPrefix(ref.String(), repo.String()) {
			return ret, fmt.Errorf("image %q (%q) ref not starts with repository %q", ref.String(), info, repo.String())
		}

		imagePath := strings.TrimPrefix(ref.String(), repo.String())
		for name, addr := range repos {
			item := queueItem{
				Repository: name,
				Info:       info,
				Image:      addr.String() + imagePath,
			}

			ret = append(ret, item)
		}
	}

	// Shake queue
	rand.Shuffle(len(ret), func(i, j int) {
		ret[i], ret[j] = ret[j], ret[i]
	})

	return ret, nil
}

func collectDeckhouseImages(input *go_hook.HookInput, repo gcr_name.Repository) (map[string]string, error) {
	deckhouseImages, err := helpers.SnapshotToSingle[deckhouseImagesModel](input, deckhouseDeploymentSnapName)
	if err != nil {
		return nil, fmt.Errorf("cannot get deckhouse deployment snapshot: %w", err)
	}

	images := make(map[string]string)
	for name, image := range deckhouseImages.InitContainers {
		// workaround for overrideImages
		if !strings.HasPrefix(image, repo.String()) {
			continue
		}

		info := fmt.Sprintf("deckhouse/init-containers/%v", name)
		images[image] = info
	}

	for name, image := range deckhouseImages.Containers {
		// workaround for overrideImages
		if !strings.HasPrefix(image, repo.String()) {
			continue
		}

		info := fmt.Sprintf("deckhouse/containers/%v", name)
		images[image] = info
	}

	return images, nil
}

func collectModulesImages(input *go_hook.HookInput, repo gcr_name.Repository) (map[string]string, error) {
	moduleNames, err := helpers.SnapshotToList[string](input, modulesSnapName)
	if err != nil {
		return nil, fmt.Errorf("cannot get modules snapshot: %w", err)
	}

	if len(moduleNames) == 0 {
		return nil, fmt.Errorf("modules snapshot contains no entries")
	}

	images := make(map[string]string)
	for _, modName := range moduleNames {
		valuesPath := fmt.Sprintf("%v.%v", moduleDigestsValuesPath, modName)

		moduleImages, err := helpers.GetValue[map[string]string](input, valuesPath)
		if err != nil && !errors.Is(err, helpers.ErrNoValue) {
			return nil, fmt.Errorf("cannot get images digests for module %q: %w", modName, err)
		}

		for imgName, dgst := range moduleImages {
			info := fmt.Sprintf("module/%v/%v", strcase.ToKebab(modName), strcase.ToKebab(imgName))
			image := repo.Digest(dgst)

			images[image.String()] = info
		}
	}

	return images, nil
}
