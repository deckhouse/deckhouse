// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const dhctlEditionMismatchError = "" +
	"%w\nThere is a possibility that you will not be able to install latest versions of Deckhouse correctly with this image.\n" +
	`To fix this, check that the labels of the installer and the version being installed match, or use the --preflight-skip-deckhouse-edition-check flag`

// imageDescriptorProvider returns image manifest data, mainly image digest.
type imageDescriptorProvider interface {
	ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error)
}

// remoteDescriptorProvider returns image manifest data from remote registry.
type remoteDescriptorProvider struct{}

func (remoteDescriptorProvider) ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error) {
	image, err := remote.Image(ref, opts...)
	if err != nil {
		return &v1.ConfigFile{}, err
	}
	return image.ConfigFile()
}

func (pc *Checker) CheckDhctlEdition(ctx context.Context) error {
	log.DebugLn("Checking if dhctl version is compatible with release to be installed")
	if app.AppVersion == "local" {
		log.DebugLn("dhctl version check is skipped for local builds")
		return nil
	}
	if app.PreflightSkipDeckhouseEditionCheck {
		log.WarnLn("Dhctl compatibility check is skipped")
		return nil
	}

	currentDeckhouseImageConfig, err := pc.getDeckhouseImageConfig(ctx)
	if err != nil {
		return fmt.Errorf("Cannot fetch deckhouse image config: %w.", err)
	}
	if currentDeckhouseImageConfig == nil ||
		currentDeckhouseImageConfig.Config.Labels == nil ||
		currentDeckhouseImageConfig.Config.Labels["io.deckhouse.edition"] != app.AppEdition {
		return fmt.Errorf(dhctlEditionMismatchError, errors.New(
			fmt.Sprintf("Your edition installer image does not match.\n `crane config %s:%s | jq -r jq -r '.config.Labels.\"io.deckhouse.editio\"'`", pc.installConfig.GetImage(true), app.AppVersion)))
	}

	return nil
}

func (pc *Checker) getDeckhouseImageConfig(ctx context.Context) (*v1.ConfigFile, error) {
	creds, err := pc.findRegistryAuthCredentials()
	if err != nil {
		return nil, fmt.Errorf("parse ClusterConfiguration.deckhouse.registryDockerCfg: %w", err)
	}

	versionTagRef, err := name.ParseReference(pc.installConfig.GetImage(true))
	if err != nil {
		return nil, fmt.Errorf("parse image reference: %w", err)
	}

	config, err := pc.imageDescriptorProvider.ConfigFile(versionTagRef, remote.WithContext(ctx), remote.WithAuth(creds))
	if err != nil {
		return nil, fmt.Errorf("pull deckhouse image ConfigFile from registry: %w", err)
	}

	return config, nil
}

func (pc *Checker) findRegistryAuthCredentials() (authn.Authenticator, error) {
	buf, err := base64.StdEncoding.DecodeString(pc.installConfig.Registry.DockerCfg)
	if err != nil {
		return nil, fmt.Errorf("decode dockerCfg: %w", err)
	}

	decodedDockerCfg := struct {
		Auths map[string]struct {
			Auth     string `json:"auth,omitempty"`
			User     string `json:"username,omitempty"`
			Password string `json:"password,omitempty"`
		} `json:"auths"`
	}{}
	if err := json.Unmarshal(buf, &decodedDockerCfg); err != nil {
		return nil, fmt.Errorf("decode dockerCfg: %w", err)
	}

	if decodedDockerCfg.Auths == nil {
		return authn.Anonymous, nil
	}
	registryAuth, hasRegistryCreds := decodedDockerCfg.Auths[pc.installConfig.Registry.Address]
	if !hasRegistryCreds {
		return authn.Anonymous, nil
	}

	if registryAuth.Auth != "" {
		return authn.FromConfig(authn.AuthConfig{
			Auth: registryAuth.Auth,
		}), nil
	}

	if registryAuth.User != "" && registryAuth.Password != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: registryAuth.User,
			Password: registryAuth.Password,
		}), nil
	}

	return authn.Anonymous, nil
}
