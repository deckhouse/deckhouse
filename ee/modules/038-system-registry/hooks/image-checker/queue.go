/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package imagechecker

import (
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"strings"

	"github.com/ettle/strcase"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	gcr_name "github.com/google/go-containerregistry/pkg/name"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
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
	maps.Copy(images, deckhouseImages)

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
