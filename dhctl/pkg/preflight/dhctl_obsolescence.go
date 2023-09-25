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
	"os"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const dhctlVersionMismatchError = "" +
	"Installation aborted: %w.\n" +
	"There is a possibility that you will not be able to install latest versions of Deckhouse correctly with this image.\n" +
	`To fix this add "--pull=always" flag to your "docker run" cmdline or run dhctl with $DHCTL_CLI_PREFLIGHT_SKIP_INCOMPATIBLE_VERSION_CHECK env set to "1"`

var (
	ErrInstallerVersionMismatch           = errors.New("your installer image is outdated")
	ErrDeckhouseDigestFileHashAlgMismatch = errors.New("digest hash algorithm does not match")
)

// imageDescriptorProvider returns image manifest data, mainly image digest.
type imageDescriptorProvider interface {
	Descriptor(ref name.Reference, opts ...remote.Option) (*v1.Descriptor, error)
}

type buildDigestProvider interface {
	ThisBuildDigest() (v1.Hash, error)
}

// remoteDescriptorProvider returns image manifest data from remote registry.
type remoteDescriptorProvider struct{}

func (remoteDescriptorProvider) Descriptor(ref name.Reference, opts ...remote.Option) (*v1.Descriptor, error) {
	return remote.Head(ref, opts...)
}

type dhctlBuildDigestProvider struct {
	DigestFilePath string
}

func (p *dhctlBuildDigestProvider) ThisBuildDigest() (v1.Hash, error) {
	deckhouseImageDigestFile, err := os.ReadFile(p.DigestFilePath)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("read image digest from %s: %w", p.DigestFilePath, err)
	}

	digestParts := strings.Split(string(deckhouseImageDigestFile), ":")
	return v1.Hash{
		Algorithm: digestParts[0],
		Hex:       strings.TrimSpace(digestParts[1]), // trim trailing newline
	}, nil
}

func (pc *PreflightCheck) CheckDhctlVersionObsolescence() error {
	log.DebugLn("Checking if dhctl version is compatible with release to be installed")
	if app.AppVersion == "local" {
		log.DebugLn("dhctl version check is skipped for local builds")
		return nil
	}
	if app.PreflightSkipDeckhouseVersionCheck {
		log.WarnLn("dhctl compatibility check is skipped")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	currentDeckhouseImageDigest, err := pc.fetchDeckhouseImageHashFromReleaseChannel(ctx)
	if err != nil {
		return fmt.Errorf("fetch deckhouse image hash: %w", err)
	}

	dhctlImageDigest, err := pc.buildDigestProvider.ThisBuildDigest()
	if err != nil {
		return fmt.Errorf("read digest of this dhctl-compatible build: %w", err)
	}

	if currentDeckhouseImageDigest.Algorithm != dhctlImageDigest.Algorithm {
		return fmt.Errorf(
			"%w: dhctl installer knows %q hash, but current deckhouse image has %q",
			ErrDeckhouseDigestFileHashAlgMismatch,
			currentDeckhouseImageDigest.Algorithm,
			dhctlImageDigest.Algorithm,
		)
	}

	if currentDeckhouseImageDigest.Hex != dhctlImageDigest.Hex {
		return fmt.Errorf(dhctlVersionMismatchError, ErrInstallerVersionMismatch)
	}

	log.InfoLn("Checked if dhctl version is compatible successfully")
	return nil
}

func (pc *PreflightCheck) fetchDeckhouseImageHashFromReleaseChannel(ctx context.Context) (*v1.Hash, error) {
	creds, err := pc.findRegistryAuthCredentials()
	if err != nil {
		return nil, fmt.Errorf("parse ClusterConfiguration.deckhouse.registryDockerCfg: %w", err)
	}

	imageReference, err := name.ParseReference(pc.installConfig.GetImage())
	if err != nil {
		return nil, fmt.Errorf("parse image refernce: %w", err)
	}

	descriptor, err := pc.imageDescriptorProvider.Descriptor(imageReference, remote.WithContext(ctx), remote.WithAuth(creds))
	if err != nil {
		return nil, fmt.Errorf("pull deckhouse image manifest from registry: %w", err)
	}
	hash := descriptor.Digest
	return &hash, nil
}

func (pc *PreflightCheck) findRegistryAuthCredentials() (authn.Authenticator, error) {
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
