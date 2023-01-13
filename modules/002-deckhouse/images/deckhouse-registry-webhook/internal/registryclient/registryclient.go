package registryclient

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type RCInterface interface {
	CheckImage(registry, image string, authCfg authn.AuthConfig) error
}

type RegistryClient struct{}

func NewRegistryClient() *RegistryClient {
	return &RegistryClient{}
}

func (r RegistryClient) CheckImage(registry, image string, authCfg authn.AuthConfig) error {
	auth := authn.FromConfig(authCfg)
	// To catch the "manifest unknown" error, we should request an image that does not exist
	ref, err := name.ParseReference(fmt.Sprintf("%s/%s", registry, image))
	if err != nil {
		return fmt.Errorf("can't parse reference: %w", err)
	}
	// Trying to get an image that does not exist
	_, err = remote.Get(ref, remote.WithAuth(auth))
	if err != nil {
		if !strings.Contains(err.Error(), "manifest unknown") {
			return fmt.Errorf("registry error: %w", err)
		}
	}

	return nil
}
