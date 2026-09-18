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
	"fmt"
	"net/http"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// RegistryReachableCheck asks the registry an anonymous question, so that everything which can go
// wrong before credentials are even offered — DNS, TCP, TLS, a private CA, the wrong scheme,
// something that is not a registry at the address — is reported as itself.
//
// It was split out of registry-credentials, which reported six unrelated causes under one name
// and one sentence, "authentication failed".
type RegistryReachableCheck struct {
	MetaConfig *config.MetaConfig
}

const RegistryReachableCheckName preflight.CheckName = "registry-reachable"

func (RegistryReachableCheck) Description() string {
	return "the container registry answers from the installer host"
}

func (RegistryReachableCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (RegistryReachableCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c RegistryReachableCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("meta config is required")
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	address, _ := registry.AddressAndPath()
	endpoint := registryV2URL(c.MetaConfig).String()

	client, err := prepareAuthHTTPClient(ctx, c.MetaConfig)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  registryCAField,
			Observed: err.Error(),
			Expected: "a PEM bundle the request can be made with",
			Fix:      "correct " + registryCAField,
		})
	}

	req, err := prepareRegistryRequest(ctx, c.MetaConfig, "")
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", registryTransportFailure(endpoint, address, string(registry.Scheme), err)
	}
	defer resp.Body.Close()

	// 200 and 401 are both the registry answering: one lets anyone in, the other wants
	// credentials. Either way the address is a registry and it is reachable, which is the whole
	// question here — whether the credentials are right is the next check.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("GET %s", endpoint),
			Observed: fmt.Sprintf("HTTP %d, which is not how a registry answers /v2/", resp.StatusCode),
			Expected: "HTTP 200 or 401 from the registry API",
			Fix: fmt.Sprintf("check %s; something other than a container registry is answering at %s "+
				"(a reverse proxy, a load balancer, an error page)", registryImagesRepoField, address),
		})
	}

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("GET %s", endpoint),
			Observed: "the answer carries no Docker-Distribution-API-Version: registry/2.0 header",
			Expected: "a registry API v2 endpoint",
			Fix: fmt.Sprintf("check %s, and that no reverse proxy in front of the registry strips "+
				"the header", registryImagesRepoField),
		})
	}

	if resp.StatusCode == http.StatusOK {
		return fmt.Sprintf("%s answers at /v2/ and allows anonymous access", address), nil
	}
	return fmt.Sprintf("%s answers at /v2/ and asks for credentials", address), nil
}

// registryTransportFailure names what went wrong before any HTTP answer came back. A private CA,
// a name that does not resolve and the wrong scheme are three different problems with three
// different fixes; they used to arrive as the single word "authentication failed".
func registryTransportFailure(endpoint, address, scheme string, err error) error {
	failure := &preflight.Failure{
		Checked:  fmt.Sprintf("GET %s from the installer host", endpoint),
		Observed: classifyNetworkError(err),
		Expected: "the registry to answer",
		Err:      err,
	}

	switch {
	case isCertificateError(err):
		failure.Fix = fmt.Sprintf("put the registry CA into %s; if the certificate is not yet valid, check the clock on this host", registryCAField)
		return preflight.Permanent(failure)

	case strings.Contains(err.Error(), "server gave HTTP response to HTTPS client"):
		failure.Observed = fmt.Sprintf("%s answered with plain HTTP while the configured scheme is %s", address, scheme)
		failure.Fix = "set the registry scheme to HTTP, or enable TLS on the registry"
		return preflight.Permanent(failure)

	default:
		failure.Fix = fmt.Sprintf("check %s. If the installer host reaches the Internet through a proxy, "+
			"export HTTPS_PROXY and NO_PROXY inside the installer container", registryImagesRepoField)
		return failure
	}
}

// The two configuration paths every registry failure points at. Spelled out once: the field moved
// from InitConfiguration to a ModuleConfig, and both spellings are still in use.
const (
	registryImagesRepoField = `.spec.settings.registry.<mode>.imagesRepo in the "deckhouse" ModuleConfig (InitConfiguration.deckhouse.imagesRepo)`
	registryCAField         = `.spec.settings.registry.<mode>.ca in the "deckhouse" ModuleConfig (InitConfiguration.deckhouse.registryCA)`
)

func RegistryReachable(meta *config.MetaConfig) preflight.Check {
	check := RegistryReachableCheck{MetaConfig: meta}
	return preflight.Check{
		Name:        RegistryReachableCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
