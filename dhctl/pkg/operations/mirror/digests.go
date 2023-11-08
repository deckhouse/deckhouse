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

package mirror

import (
	"encoding/json"
	"fmt"
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

func ExtractImageDigestsFromDeckhouseInstaller(
	mirrorCtx *Context,
	installerTag string,
	installersLayout layout.Path,
) (map[string]struct{}, error) {
	index, err := installersLayout.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("read installer images index: %w", err)
	}
	indexManifest, err := index.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("read installers index manifest: %w", err)
	}

	installerHash := findDigestForInstallerTag(installerTag, indexManifest)
	if installerHash == nil {
		return nil, fmt.Errorf("no image tagged as %q found in index", installerTag)
	}

	img, err := index.Image(*installerHash)
	if err != nil {
		return nil, fmt.Errorf("cannot read image from index: %w", err)
	}

	imagesDigestsJSON, err := readFileFromImage(img, "deckhouse/candi/images_digests.json")
	if err != nil {
		return nil, fmt.Errorf("read digests from %q: %w", installerTag, err)
	}

	digests := map[string]struct{}{}
	if err = parseDigestsFromImagesDigestsJSON(mirrorCtx.DeckhouseRegistryRepo, imagesDigestsJSON, digests); err != nil {
		return nil, fmt.Errorf("cannot parse images_digests.json: %w", err)
	}

	return digests, nil
}

func findDigestForInstallerTag(installerTag string, indexManifest *v1.IndexManifest) *v1.Hash {
	for _, imageManifest := range indexManifest.Manifests {
		for key, value := range imageManifest.Annotations {
			if key == "org.opencontainers.image.ref.name" && value == installerTag {
				tag := imageManifest.Digest
				return &tag
			}
		}
	}
	return nil
}

func parseDigestsFromImagesDigestsJSON(registryRepo string, jsonDigests io.Reader, dst map[string]struct{}) error {
	digestsByModule := map[string]map[string]string{}
	if err := json.NewDecoder(jsonDigests).Decode(&digestsByModule); err != nil {
		return fmt.Errorf("parse images_digests.json: %w", err)
	}

	for _, nameDigestTuple := range digestsByModule {
		for _, imageID := range nameDigestTuple {
			dst[registryRepo+"@"+imageID] = struct{}{}
		}
	}

	return nil
}
