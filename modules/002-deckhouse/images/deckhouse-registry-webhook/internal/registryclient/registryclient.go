/*
Copyright 2022 Flant JSC

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

package registryclient

import (
	"fmt"
	log "github.com/sirupsen/logrus"
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
		log.Infof("authentication to the registry %s was successful, but manifest %s unknown", registry, image)
		return nil
	}
	log.Infof("authentication to the registry %s was successful, manifest %s found", registry, image)
	return nil
}
