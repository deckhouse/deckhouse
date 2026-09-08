// Copyright 2025 Flant JSC
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

package registry

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// ImageGetOption is some configuration that modifies options for a get request.
type ImageGetOption interface {
	// ApplyToImageGet applies this configuration to the given image get options.
	ApplyToImageGet(*ImageGetOptions)
}

type ImageGetOptions struct {
	// Platform selects one image out of a multi-arch index.
	//
	// Leaving it nil does NOT mean "the host's platform": go-containerregistry
	// falls back to a hardcoded linux/amd64, so an index resolves to its amd64
	// child even on an arm64 machine. Set it explicitly whenever the answer
	// must match the caller's architecture.
	Platform *v1.Platform
}

// ManifestGetOption is some configuration that modifies options for a manifest
// get request.
type ManifestGetOption interface {
	// ApplyToManifestGet applies this configuration to the given manifest get options.
	ApplyToManifestGet(*ManifestGetOptions)
}

type ManifestGetOptions struct {
	// Platform resolves a multi-arch index to the manifest of one child image.
	//
	// Leaving it nil returns the manifest as served, which for a multi-arch
	// reference is the index itself - not any single image's manifest.
	Platform *v1.Platform
}

// ImagePushOption is some configuration that modifies options for a put request.
type ImagePushOption interface {
	// ApplyToImagePush applies this configuration to the given image put options.
	ApplyToImagePush(*ImagePushOptions)
}

type ImagePushOptions struct {
}

// ListTagsOption is some configuration that modifies options for a list tags request.
type ListTagsOption interface {
	// ApplyToListTags applies this configuration to the given list tags options.
	ApplyToListTags(*ListTagsOptions)
}

type ListTagsOptions struct {
	// Last tag for pagination continuation
	Last string
	// Maximum number of results to return (0 means no limit)
	N int
}

// ListRepositoriesOption is some configuration that modifies options for a list repositories request.
type ListRepositoriesOption interface {
	// ApplyToListRepositories applies this configuration to the given list repositories options.
	ApplyToListRepositories(*ListRepositoriesOptions)
}

type ListRepositoriesOptions struct {
	// Last repository name for pagination continuation
	Last string
	// Maximum number of results to return (0 means no limit)
	N int
}
