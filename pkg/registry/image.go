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
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

type Image interface {
	v1.Image
	Extract() io.ReadCloser
}

type ManifestResult interface {
	GetMediaType() types.MediaType
	GetManifest() (Manifest, error)
	GetIndexManifest() (IndexManifest, error)
	GetDescriptor() Descriptor
}

// ManifestInterface defines methods for accessing manifest information
type Manifest interface {
	GetSchemaVersion() int64
	GetMediaType() types.MediaType
	GetConfig() Descriptor
	GetLayers() []Descriptor
	GetAnnotations() map[string]string
	GetSubject() Descriptor
}

// IndexManifestInterface defines methods for accessing index manifest information
type IndexManifest interface {
	GetSchemaVersion() int64
	GetMediaType() types.MediaType
	GetManifests() []Descriptor
	GetAnnotations() map[string]string
	GetSubject() Descriptor
}

// Descriptor defines methods for accessing descriptor information
type Descriptor interface {
	GetMediaType() types.MediaType
	GetSize() int64
	GetDigest() v1.Hash
	GetData() []byte
	GetURLs() []string
	GetAnnotations() map[string]string
	GetPlatform() *v1.Platform
	GetArtifactType() string
}
