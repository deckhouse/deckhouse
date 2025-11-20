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
