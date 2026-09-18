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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

type RegistryCredentialsCheck struct {
	MetaConfig    *config.MetaConfig
	InstallConfig *config.DeckhouseInstaller
}

const RegistryCredentialsCheckName preflight.CheckName = "registry-credentials"

func (RegistryCredentialsCheck) Description() string {
	return "the registry accepts the configured credentials"
}

func (RegistryCredentialsCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (RegistryCredentialsCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

// Run asks only whether the credentials are accepted. Everything that can go wrong before that —
// DNS, TCP, TLS, a private CA, the wrong scheme, something that is not a registry at the address
// — belongs to registry-reachable, which this check declares a dependency on, so a registry that
// cannot be reached at all is reported once instead of twice.
func (c RegistryCredentialsCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil || c.InstallConfig == nil {
		return "", fmt.Errorf("metaConfig and installConfig are required")
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	address, repoPath := registry.AddressAndPath()
	user := registry.Username

	authData := registry.AuthBase64()
	if authData == "" {
		// Nothing to check: the registry is used anonymously. Whether an anonymous pull actually
		// works is deckhouse-image-available's question, not this one.
		return "", preflight.NotApplicable("no registry credentials are configured; the registry is used anonymously")
	}

	client, err := prepareAuthHTTPClient(ctx, c.MetaConfig)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  registryCAField,
			Observed: err.Error(),
			Expected: "a PEM bundle the request can be made with",
			Fix:      "correct " + registryCAField,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Basic first, then bearer: a registry that takes Basic answers immediately, and one that
	// wants a token says so with a 401 and a WWW-Authenticate header.
	basicErr := checkBasicRegistryAuth(ctx, c.MetaConfig, authData, client)
	if basicErr == nil {
		return fmt.Sprintf("authenticated to %s as %q (basic auth)", address, user), nil
	}
	if !errors.Is(basicErr, ErrAuthRegistryFailed) {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s as %q", registryV2URL(c.MetaConfig), user),
			Observed: classifyNetworkError(basicErr),
			Expected: "the registry to answer",
			Fix:      fmt.Sprintf("check %s", registryImagesRepoField),
			Err:      basicErr,
		}
	}

	if err := checkTokenRegistryAuth(ctx, c.MetaConfig, authData, client); err != nil {
		return "", c.authFailure(address, repoPath, user, err)
	}
	return fmt.Sprintf("authenticated to %s as %q (bearer token)", address, user), nil
}

// authFailure turns the last answer into the sentence the reader acts on. Credentials that are
// turned down are turned down on every attempt, so it does not ask for another one — the old
// policy spent five attempts and ten HTTP requests on an HTTP 401.
func (c RegistryCredentialsCheck) authFailure(address, repoPath, user string, err error) error {
	failure := &preflight.Failure{
		Checked:  fmt.Sprintf("the credentials for %s as %q", address, user),
		Expected: "the registry to accept them",
		Err:      err,
	}

	switch {
	case errors.Is(err, ErrRegistryPullDenied):
		failure.Observed = fmt.Sprintf("the credentials authenticate but have no pull permission on %s", strings.TrimLeft(repoPath, "/"))
		failure.Fix = fmt.Sprintf("grant pull access to %q on %s, or correct %s",
			user, strings.TrimLeft(repoPath, "/"), registryImagesRepoField)

	case errors.Is(err, ErrRegistryBearerUnsupported):
		// The old text advised enabling bearer auth here, which is wrong for a Basic-only
		// registry (Nexus, Harbor with basic, registry:2 behind htpasswd): the password is
		// simply wrong, and there is nothing to enable.
		failure.Observed = fmt.Sprintf("%s uses basic authentication and rejected the credentials", address)
		failure.Fix = registryCredentialsFix

	default:
		failure.Observed = fmt.Sprintf("%s rejected the credentials for %q (HTTP 401)", address, user)
		failure.Fix = registryCredentialsFix
	}

	return preflight.Permanent(failure)
}

const registryCredentialsFix = `check .spec.settings.registry.direct.license (or username/password) in the "deckhouse" ModuleConfig, ` +
	`or InitConfiguration.deckhouse.registryDockerCfg; the dockercfg entry must be keyed by the registry address exactly`

func RegistryCredentials(meta *config.MetaConfig, cfg *config.DeckhouseInstaller) preflight.Check {
	check := RegistryCredentialsCheck{
		MetaConfig:    meta,
		InstallConfig: cfg,
	}
	return preflight.Check{
		Name:        RegistryCredentialsCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
