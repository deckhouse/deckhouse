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
	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type ShortDescriptor struct {
	// MediaType is the media type of the object this schema refers to.
	MediaType string `json:"mediaType"`

	// Digest is the digest of the targeted content.
	Digest digest.Digest `json:"digest"`

	// Size specifies the size in bytes of the blob.
	Size int64 `json:"size"`
}

func (d *ShortDescriptor) FromDescriptor(desc ociv1.Descriptor) {
	d.MediaType = desc.MediaType
	d.Digest = desc.Digest
	d.Size = desc.Size
}

type ArtifactManifest struct {
	// MediaType is the media type of the object this schema refers to.
	MediaType string `json:"mediaType"`

	// ArtifactType is the IANA media type of the artifact this schema refers to.
	ArtifactType string `json:"artifactType"`

	// Blobs is a collection of blobs referenced by this manifest.
	Blobs []ociv1.Descriptor `json:"blobs,omitempty"`

	// Subject (reference) is an optional link from the artifact to another manifest forming an association between the artifact and the other manifest.
	Subject *ociv1.Descriptor `json:"subject,omitempty"`

	// Annotations contains arbitrary metadata for the artifact manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}
