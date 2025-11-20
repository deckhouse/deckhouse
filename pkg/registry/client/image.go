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

package client

import (
	"io"

	"github.com/deckhouse/deckhouse/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

type Image struct {
	v1.Image

	pullReference string
}

func NewImage(img v1.Image, pullReference string) *Image {
	return &Image{Image: img, pullReference: pullReference}
}

// Extract flattens the image to a single layer and returns ReadCloser for fetching the content
// The repository is determined by the chained WithSegment() calls
func (i *Image) Extract() io.ReadCloser {
	return mutate.Extract(i)
}

func (i *Image) GetPullReference() string {
	return i.pullReference
}

type Manifest struct {
	*v1.Manifest
}

// GetSchemaVersion returns the schema version of the manifest
func (m *Manifest) GetSchemaVersion() int64 {
	if m.Manifest == nil {
		return 0
	}
	return m.Manifest.SchemaVersion
}

// GetMediaType returns the media type of the manifest
func (m *Manifest) GetMediaType() string {
	if m.Manifest == nil {
		return ""
	}
	return string(m.Manifest.MediaType)
}

// GetConfig returns the configuration descriptor
func (m *Manifest) GetConfig() registry.Descriptor {
	if m.Manifest == nil {
		return nil
	}
	return &Descriptor{Descriptor: &m.Manifest.Config}
}

// GetLayers returns the layer descriptors
func (m *Manifest) GetLayers() []registry.Descriptor {
	if m.Manifest == nil {
		return nil
	}
	descriptors := make([]registry.Descriptor, len(m.Manifest.Layers))
	for i, layer := range m.Manifest.Layers {
		descriptors[i] = &Descriptor{Descriptor: &layer}
	}
	return descriptors
}

// GetAnnotations returns the annotations associated with the manifest
func (m *Manifest) GetAnnotations() map[string]string {
	if m.Manifest == nil {
		return nil
	}
	return m.Manifest.Annotations
}

// GetSubject returns the subject descriptor if present
func (m *Manifest) GetSubject() registry.Descriptor {
	if m.Manifest == nil || m.Manifest.Subject == nil {
		return nil
	}
	return &Descriptor{Descriptor: m.Manifest.Subject}
}

type Descriptor struct {
	*v1.Descriptor
}

// GetMediaType returns the media type of the descriptor
func (d *Descriptor) GetMediaType() string {
	if d.Descriptor == nil {
		return ""
	}
	return string(d.Descriptor.MediaType)
}

// GetSize returns the size of the described content
func (d *Descriptor) GetSize() int64 {
	if d.Descriptor == nil {
		return 0
	}
	return d.Descriptor.Size
}

// GetDigest returns the digest of the described content
func (d *Descriptor) GetDigest() v1.Hash {
	if d.Descriptor == nil {
		return v1.Hash{}
	}
	return d.Descriptor.Digest
}

// GetData returns the raw data of the descriptor
func (d *Descriptor) GetData() []byte {
	if d.Descriptor == nil {
		return nil
	}
	return d.Descriptor.Data
}

// GetURLs returns the URLs where the content can be accessed
func (d *Descriptor) GetURLs() []string {
	if d.Descriptor == nil {
		return nil
	}
	return d.Descriptor.URLs
}

// GetAnnotations returns the annotations associated with the descriptor
func (d *Descriptor) GetAnnotations() map[string]string {
	if d.Descriptor == nil {
		return nil
	}
	return d.Descriptor.Annotations
}

// GetPlatform returns the platform information for the descriptor
func (d *Descriptor) GetPlatform() *v1.Platform {
	if d.Descriptor == nil {
		return nil
	}
	return d.Descriptor.Platform
}

// GetArtifactType returns the artifact type of the descriptor
func (d *Descriptor) GetArtifactType() string {
	if d.Descriptor == nil {
		return ""
	}
	return d.Descriptor.ArtifactType
}
