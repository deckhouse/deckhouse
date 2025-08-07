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
	"sort"

	gcr_name "github.com/google/go-containerregistry/pkg/name"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func buildRepoQueue(info clusterImagesInfo, repo gcr_name.Repository, checkMode registry_const.CheckModeType) ([]queueItem, error) {
	images := make(map[string]string)

	switch checkMode {
	case registry_const.Soft:
		// Only deckhouse container image
		newImageRef, err := updateImageRepo(info.DeckhouseContainerImageRef, repo)
		if err != nil {
			return nil, fmt.Errorf("failed to update image reference: %w", err)
		}

		// Need to be a tag
		if _, ok := newImageRef.(gcr_name.Tag); !ok {
			return nil, fmt.Errorf("expected deckhouse image reference to be a tag, but got: %s", newImageRef.String())
		}
		images[newImageRef.String()] = "deckhouse/containers/deckhouse"
	default:
		images = make(map[string]string, len(info.ModulesImagesDigests)+len(info.DeckhouseImagesRefs))

		// Module images
		for digest, info := range info.ModulesImagesDigests {
			image := repo.Digest(digest).String()
			images[image] = info
		}

		// Deckhouse images
		for info, image := range info.DeckhouseImagesRefs {
			newImageRef, err := updateImageRepo(image, repo)
			if err != nil {
				return nil, err
			}
			images[newImageRef.String()] = info
		}
	}

	ret := make([]queueItem, 0, len(images))
	for image, info := range images {
		item := queueItem{
			Info:  info,
			Image: image,
		}
		ret = append(ret, item)
	}

	// for stable order
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Image < ret[j].Image
	})

	return ret, nil
}

func updateImageRepo(imageRef string, newRepo gcr_name.Repository) (gcr_name.Reference, error) {
	ref, err := gcr_name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference %q: %w", imageRef, err)
	}
	switch refType := ref.(type) {
	case gcr_name.Digest:
		return newRepo.Digest(refType.DigestStr()), nil
	case gcr_name.Tag:
		return newRepo.Tag(refType.TagStr()), nil
	default:
		return nil, fmt.Errorf("unknown reference type for image %q", imageRef)
	}
}
