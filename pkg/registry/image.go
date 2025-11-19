package registry

import (
	"io"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

type ClientImage interface {
	v1.Image
	Extract() io.ReadCloser
}
