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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/deckhouse/deckhouse/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/types"
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

func NewManifestResultFromBytes(manifestBytes []byte) *ManifestResult {
	return &ManifestResult{
		rawManifest: manifestBytes,
	}
}

type ManifestResult struct {
	rawManifest []byte

	descriptor *v1.Descriptor

	manifest      *v1.Manifest
	indexManifest *v1.IndexManifest
}

func (m *ManifestResult) IsIndex() bool {
	return m.descriptor.MediaType.IsIndex()
}

var ErrIsIndexManifest = fmt.Errorf("manifest is an index")
var ErrIsNotIndexManifest = fmt.Errorf("manifest is not an index")

func (m *ManifestResult) GetDescriptor() registry.Descriptor {
	if m.descriptor == nil {
		return nil
	}

	return &Descriptor{Descriptor: m.descriptor}
}

// GetMediaType returns the media type of the manifest
func (m *ManifestResult) GetMediaType() types.MediaType {
	if m.descriptor == nil {
		return ""
	}
	return m.descriptor.MediaType
}

func (m *ManifestResult) GetManifest() (registry.Manifest, error) {
	if m.IsIndex() {
		return nil, ErrIsIndexManifest
	}

	if m.manifest != nil {
		return &Manifest{manifest: m.manifest}, nil
	}

	err := json.NewDecoder(bytes.NewReader(m.rawManifest)).Decode(&m.manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return &Manifest{manifest: m.manifest}, nil
}

func (m *ManifestResult) GetIndexManifest() (registry.IndexManifest, error) {
	if !m.IsIndex() {
		return nil, ErrIsNotIndexManifest
	}

	if m.indexManifest != nil {
		return &IndexManifest{indexManifest: m.indexManifest}, nil
	}

	err := json.NewDecoder(bytes.NewReader(m.rawManifest)).Decode(&m.indexManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode index manifest: %w", err)
	}

	return &IndexManifest{indexManifest: m.indexManifest}, nil
}

type Manifest struct {
	manifest *v1.Manifest
}

// GetSchemaVersion returns the schema version of the manifest
func (m *Manifest) GetSchemaVersion() int64 {
	if m.manifest == nil {
		return 0
	}
	return m.manifest.SchemaVersion
}

// GetMediaType returns the media type of the manifest
func (m *Manifest) GetMediaType() types.MediaType {
	if m.manifest == nil {
		return ""
	}
	return m.manifest.MediaType
}

// GetConfig returns the configuration descriptor
func (m *Manifest) GetConfig() registry.Descriptor {
	if m.manifest == nil {
		return nil
	}
	return &Descriptor{Descriptor: &m.manifest.Config}
}

// GetLayers returns the layer descriptors
func (m *Manifest) GetLayers() []registry.Descriptor {
	if m.manifest == nil {
		return nil
	}
	descriptors := make([]registry.Descriptor, len(m.manifest.Layers))
	for i, layer := range m.manifest.Layers {
		descriptors[i] = &Descriptor{Descriptor: &layer}
	}
	return descriptors
}

// GetAnnotations returns the annotations associated with the manifest
func (m *Manifest) GetAnnotations() map[string]string {
	if m.manifest == nil {
		return nil
	}
	return m.manifest.Annotations
}

// GetSubject returns the subject descriptor if present
func (m *Manifest) GetSubject() registry.Descriptor {
	if m.manifest == nil || m.manifest.Subject == nil {
		return nil
	}
	return &Descriptor{Descriptor: m.manifest.Subject}
}

type IndexManifest struct {
	indexManifest *v1.IndexManifest
}

// GetSchemaVersion returns the schema version of the index manifest
func (im *IndexManifest) GetSchemaVersion() int64 {
	if im.indexManifest == nil {
		return 0
	}
	return im.indexManifest.SchemaVersion
}

// GetMediaType returns the media type of the index manifest
func (im *IndexManifest) GetMediaType() types.MediaType {
	if im.indexManifest == nil {
		return ""
	}
	return im.indexManifest.MediaType
}

// GetManifests returns the manifest descriptors
func (im *IndexManifest) GetManifests() []registry.Descriptor {
	if im.indexManifest == nil {
		return nil
	}
	descriptors := make([]registry.Descriptor, len(im.indexManifest.Manifests))
	for i, manifest := range im.indexManifest.Manifests {
		descriptors[i] = &Descriptor{Descriptor: &manifest}
	}
	return descriptors
}

// GetAnnotations returns the annotations associated with the index manifest
func (im *IndexManifest) GetAnnotations() map[string]string {
	if im.indexManifest == nil {
		return nil
	}
	return im.indexManifest.Annotations
}

// GetSubject returns the subject descriptor if present
func (im *IndexManifest) GetSubject() registry.Descriptor {
	if im.indexManifest == nil || im.indexManifest.Subject == nil {
		return nil
	}
	return &Descriptor{Descriptor: im.indexManifest.Subject}
}

type Descriptor struct {
	*v1.Descriptor
}

// GetMediaType returns the media type of the descriptor
func (d *Descriptor) GetMediaType() types.MediaType {
	if d.Descriptor == nil {
		return ""
	}
	return d.Descriptor.MediaType
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
