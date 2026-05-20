/*
Copyright The ORAS Authors.
Copyright 2026 Flant JSC

Modifications made by Flant JSC as part of the Deckhouse project.

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

package types

import (
	"encoding/json"

	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type FetchAll = func(dgst digest.Digest) ([]byte, error)

// Successors returns the nodes directly pointed to by the current descriptor
// (e.g. config and layers of a manifest, or manifests in an index).
func Successors(fetchAll FetchAll, desc ShortDescriptor) ([]ociv1.Descriptor, error) {
	ret, err := ManifestSuccessors(fetchAll, desc)
	if err != nil || len(ret) != 0 {
		return ret, err
	}

	ret, err = ManifestListSuccessors(fetchAll, desc)
	return ret, err
}

func ManifestListSuccessors(fetchAll FetchAll, desc ShortDescriptor) ([]ociv1.Descriptor, error) {
	switch desc.MediaType {
	case MediaTypeDockerManifestList:
		content, err := fetchAll(desc.Digest)
		if err != nil {
			return nil, err
		}

		// OCI manifest index schema can be used to marshal docker manifest list
		var index ociv1.Index
		if err := json.Unmarshal(content, &index); err != nil {
			return nil, err
		}

		return index.Manifests, nil

	case ociv1.MediaTypeImageIndex:
		content, err := fetchAll(desc.Digest)
		if err != nil {
			return nil, err
		}

		var index ociv1.Index
		if err := json.Unmarshal(content, &index); err != nil {
			return nil, err
		}

		var nodes []ociv1.Descriptor
		if index.Subject != nil {
			nodes = append(nodes, *index.Subject)
		}

		return append(nodes, index.Manifests...), nil
	}

	return nil, nil
}

func ManifestSuccessors(fetchAll FetchAll, desc ShortDescriptor) ([]ociv1.Descriptor, error) {
	switch desc.MediaType {
	case MediaTypeDockerManifest:
		content, err := fetchAll(desc.Digest)
		if err != nil {
			return nil, err
		}

		// OCI manifest schema can be used to marshal docker manifest
		var manifest ociv1.Manifest
		if err := json.Unmarshal(content, &manifest); err != nil {
			return nil, err
		}

		return append([]ociv1.Descriptor{manifest.Config}, manifest.Layers...), nil

	case ociv1.MediaTypeImageManifest:
		content, err := fetchAll(desc.Digest)
		if err != nil {
			return nil, err
		}

		var manifest ociv1.Manifest
		if err := json.Unmarshal(content, &manifest); err != nil {
			return nil, err
		}

		var nodes []ociv1.Descriptor
		if manifest.Subject != nil {
			nodes = append(nodes, *manifest.Subject)
		}

		nodes = append(nodes, manifest.Config)
		return append(nodes, manifest.Layers...), nil

	case MediaTypeArtifactManifest:
		content, err := fetchAll(desc.Digest)
		if err != nil {
			return nil, err
		}

		var manifest ArtifactManifest
		if err := json.Unmarshal(content, &manifest); err != nil {
			return nil, err
		}

		var nodes []ociv1.Descriptor
		if manifest.Subject != nil {
			nodes = append(nodes, *manifest.Subject)
		}

		return append(nodes, manifest.Blobs...), nil
	}

	return nil, nil
}

// GetTagFromAnnotation returns the short tag from the descriptor annotations, or empty string if unset.
func GetTagFromAnnotation(desc ociv1.Descriptor) string {
	return desc.Annotations[ShortTagAnnotation]
}

func IsManifest(mediaType string) bool {
	switch mediaType {
	case MediaTypeDockerManifest, ociv1.MediaTypeImageManifest, MediaTypeArtifactManifest:
		return true
	default:
		return false
	}
}

func IsManifestList(mediaType string) bool {
	switch mediaType {
	case MediaTypeDockerManifestList, ociv1.MediaTypeImageIndex:
		return true
	default:
		return false
	}
}
