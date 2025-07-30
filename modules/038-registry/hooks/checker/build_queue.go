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
	"fmt"
	"maps"
	"sort"
	"strings"

	gcr_name "github.com/google/go-containerregistry/pkg/name"
)

func buildRepoQueue(info clusterImagesInfo, repo gcr_name.Repository) ([]queueItem, error) {
	currentRepo, err := gcr_name.NewRepository(info.Repo)
	if err != nil {
		return nil, fmt.Errorf("cannot parse registry base %q: %w", info.Repo, err)
	}

	images := make(map[string]string)
	for d, info := range info.ModulesImagesDigests {
		image := currentRepo.Digest(d).String()
		images[image] = info
	}

	// Merge deckhouse images to modules images
	maps.Copy(images, collectDeckhouseQueueImages(info.DeckhouseImages, currentRepo))

	ret := make([]queueItem, 0, len(images))
	for image, info := range images {
		ref, err := gcr_name.ParseReference(image)
		if err != nil {
			return ret, fmt.Errorf("cannot parse image %q (%q) reference: %w", image, info, err)
		}

		if !strings.HasPrefix(ref.String(), currentRepo.String()) {
			return ret, fmt.Errorf("image %q (%q) ref not starts with repository %q", ref.String(), info, currentRepo.String())
		}

		imagePath := strings.TrimPrefix(ref.String(), currentRepo.String())

		item := queueItem{
			Info:  info,
			Image: repo.String() + imagePath,
		}

		ret = append(ret, item)
	}

	// for stable order
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Image < ret[j].Image
	})

	return ret, nil
}

func collectDeckhouseQueueImages(deckhouseImages deckhouseImagesModel, repo gcr_name.Repository) map[string]string {
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

	return images
}
