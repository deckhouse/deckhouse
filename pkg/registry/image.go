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

// DescriptorInterface defines methods for accessing descriptor information
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
