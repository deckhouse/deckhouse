/*
Copyright The ORAS Authors.
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

const (
	// Deckhouse short tag annotation
	ShortTagAnnotation = "io.deckhouse.image.short_tag"

	// DefaultMediaType is the media type used when no media type is specified.
	DefaultMediaType = "application/octet-stream"

	// MediaTypeDockerManifest is the media type for Docker manifest
	MediaTypeDockerManifest = "application/vnd.docker.distribution.manifest.v2+json"

	// MediaTypeDockerManifestList is the media type for Docker manifest list
	MediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"

	// MediaTypeArtifactManifest is the media type for ORAS artifact manifest
	MediaTypeArtifactManifest = "application/vnd.oci.artifact.manifest.v1+json"
)
