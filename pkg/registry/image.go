package registry

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type Image interface {
	v1.Image
	Extract() io.ReadCloser
}

// ManifestInterface defines methods for accessing manifest information
type Manifest interface {
	GetSchemaVersion() int64
	GetMediaType() string
	GetConfig() Descriptor
	GetLayers() []Descriptor
	GetAnnotations() map[string]string
	GetSubject() Descriptor
}

// Descriptor defines methods for accessing descriptor information
type Descriptor interface {
	GetMediaType() string
	GetSize() int64
	GetDigest() v1.Hash
	GetData() []byte
	GetURLs() []string
	GetAnnotations() map[string]string
	GetPlatform() *v1.Platform
	GetArtifactType() string
}
