// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checks

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	cfgregistry "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/registryutil"
)

type imageDescriptorProvider interface {
	ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error)
}

type remoteDescriptorProvider struct{}

func (remoteDescriptorProvider) ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error) {
	image, err := remote.Image(ref, opts...)
	if err != nil {
		return &v1.ConfigFile{}, err
	}

	return image.ConfigFile()
}

func deckhouseImageConfig(
	ctx context.Context,
	metaConfig *config.MetaConfig,
	installer *config.DeckhouseInstaller,
	descriptor imageDescriptorProvider,
) (*v1.ConfigFile, error) {
	registry := metaConfig.Registry.Settings.RemoteData

	image, err := installer.GetRemoteImage(ctx, true)
	if err != nil {
		return nil, err
	}

	ref, err := parseDeckhouseImageReference(image, string(registry.Scheme))
	if err != nil {
		return nil, err
	}

	client, err := registryutil.NewRegistryClient(ctx, string(registry.Scheme), registry.CA)
	if err != nil {
		return nil, err
	}

	if descriptor == nil {
		descriptor = remoteDescriptorProvider{}
	}

	return descriptor.ConfigFile(
		ref,
		remote.WithContext(ctx),
		remote.WithAuth(registryAuth(registry)),
		remote.WithTransport(client.Transport),
	)
}

func parseDeckhouseImageReference(image, scheme string) (name.Reference, error) {
	if strings.ToLower(scheme) == "http" {
		return name.ParseReference(image, name.Insecure)
	}

	return name.ParseReference(image)
}

func registryAuth(registry cfgregistry.Data) authn.Authenticator {
	if registry.Username != "" && registry.Password != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: registry.Username,
			Password: registry.Password,
		})
	}

	return authn.Anonymous
}
