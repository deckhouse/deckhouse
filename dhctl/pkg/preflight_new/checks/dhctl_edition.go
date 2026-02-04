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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	cfgregistry "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type DhctlEditionCheck struct {
	MetaConfig *config.MetaConfig
	Installer  *config.DeckhouseInstaller

	descriptor imageDescriptorProvider
}

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

const DhctlEditionCheckName preflightnew.CheckName = "dhctl-edition"

func (DhctlEditionCheck) Description() string {
	return "dhctl edition matches deckhouse image"
}

func (DhctlEditionCheck) Phase() preflightnew.Phase {
	return preflightnew.PhasePreInfra
}

func (DhctlEditionCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.RetryPolicy{Attempts: 1}
}

func (c DhctlEditionCheck) Run(ctx context.Context) error {
	if c.MetaConfig == nil || c.Installer == nil {
		return fmt.Errorf("metaConfig and installConfig are required")
	}

	imageConfig, err := c.deckhouseImageConfig(ctx)
	if err != nil {
		return fmt.Errorf("cannot fetch deckhouse image config: %w", err)
	}

	labels := imageConfig.Config.Labels
	if labels == nil || labels["io.deckhouse.edition"] != app.AppEdition {
		return fmt.Errorf(
			"your edition installer image does not match: dhctl edition %s, image edition %s",
			app.AppEdition,
			labels["io.deckhouse.edition"],
		)
	}

	return nil
}

func (c DhctlEditionCheck) deckhouseImageConfig(ctx context.Context) (*v1.ConfigFile, error) {
	registry := c.MetaConfig.Registry.Settings.RemoteData
	image := c.Installer.GetRemoteImage(true)

	ref, err := c.parseReference(image, string(registry.Scheme))
	if err != nil {
		return nil, err
	}

	client, err := tlsClient(registry.CA, string(registry.Scheme))
	if err != nil {
		return nil, err
	}

	creds, err := registryAuth(registry)
	if err != nil {
		return nil, err
	}

	return c.provider().ConfigFile(
		ref,
		remote.WithContext(ctx),
		remote.WithAuth(creds),
		remote.WithTransport(client.Transport),
	)
}

func (DhctlEditionCheck) parseReference(image, scheme string) (name.Reference, error) {
	if strings.ToLower(scheme) == "http" {
		return name.ParseReference(image, name.Insecure)
	}
	return name.ParseReference(image)
}

func registryAuth(registry cfgregistry.Data) (authn.Authenticator, error) {
	if registry.Username != "" && registry.Password != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: registry.Username,
			Password: registry.Password,
		}), nil
	}
	return authn.Anonymous, nil
}

func tlsClient(ca, scheme string) (*http.Client, error) {
	client := &http.Client{}
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if strings.ToLower(scheme) == "http" || len(ca) == 0 {
		client.Transport = transport
		return client, nil
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM([]byte(ca)); !ok {
		return nil, fmt.Errorf("invalid cert in CA PEM")
	}

	transport.TLSClientConfig = &tls.Config{RootCAs: certPool}
	client.Transport = transport
	return client, nil
}

func (c DhctlEditionCheck) provider() imageDescriptorProvider {
	if c.descriptor != nil {
		return c.descriptor
	}
	return remoteDescriptorProvider{}
}

func DhctlEdition(meta *config.MetaConfig, cfg *config.DeckhouseInstaller) preflightnew.Check {
	check := DhctlEditionCheck{
		MetaConfig: meta,
		Installer:  cfg,
	}
	preflightCheck := preflightnew.Check{
		Name:        DhctlEditionCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
	if app.AppVersion == "local" || app.AppEdition == "local" {
		preflightCheck.Disable()
	}
	return preflightCheck
}
