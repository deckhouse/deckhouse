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
		return "", fmt.Errorf("dhctl was given no cluster configuration")
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	address, _ := registry.AddressAndPath()
	endpoint := registryV2URL(c.MetaConfig).String()

	client, err := prepareAuthHTTPClient(ctx, c.MetaConfig)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  registryCAField(c.registryMode()),
			Observed: err.Error(),
			Expected: "a valid PEM certificate bundle",
			Fix:      "correct " + registryCAField(c.registryMode()),
		})
	}

	req, err := prepareRegistryRequest(ctx, c.MetaConfig, "")
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", registryTransportFailure(endpoint, address, string(registry.Scheme), c.registryMode(), err)
	}
	defer resp.Body.Close()

	// 200 and 401 are both the registry answering: one lets anyone in, the other wants
	// credentials. Either way the address is a registry and it is reachable, which is the whole
	// question here — whether the credentials are right is the next check.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("GET %s", endpoint),
			Observed: fmt.Sprintf("%s answered with HTTP %d, which is not how a registry answers /v2/", address, resp.StatusCode),
			Expected: "HTTP 200 or 401 from the registry API",
			Fix: fmt.Sprintf("check %s, and any reverse proxy or load balancer in front of the registry",
				registryImagesRepoField(c.registryMode())),
		})
	}

	if resp.Header.Get("Docker-Distribution-API-Version") != "registry/2.0" {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("GET %s", endpoint),
			Observed: "the answer carries no Docker-Distribution-API-Version: registry/2.0 header",
			Expected: "a registry API v2 endpoint",
			Fix: fmt.Sprintf("check %s, and that no reverse proxy in front of the registry strips "+
				"the header", registryImagesRepoField(c.registryMode())),
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
func registryTransportFailure(endpoint, address, scheme, mode string, err error) error {
	failure := &preflight.Failure{
		Checked:  fmt.Sprintf("GET %s from the installer host", endpoint),
		Observed: classifyNetworkError(err),
		Expected: "an answer from the registry",
		Err:      err,
	}

	switch {
	case isCertificateError(err):
		failure.Fix = fmt.Sprintf("put the registry CA into %s. If the certificate is not yet valid, check the clock on this host.", registryCAField(mode))
		return preflight.Permanent(failure)

	case strings.Contains(err.Error(), "server gave HTTP response to HTTPS client"):
		failure.Observed = fmt.Sprintf("%s answered with plain HTTP while the configured scheme is %s", address, scheme)
		failure.Fix = "set the registry scheme to HTTP, or enable TLS on the registry"
		return preflight.Permanent(failure)

	default:
		failure.Fix = fmt.Sprintf("check %s. If the installer host reaches the Internet through a proxy, "+
			"export HTTPS_PROXY and NO_PROXY inside the installer container", registryImagesRepoField(mode))
		return failure
	}
}

// The two configuration paths every registry failure points at. Spelled out once: the field moved
// from InitConfiguration to a ModuleConfig, and both spellings are still in use.
//
// The mode is filled in rather than left as a placeholder. The fields live under the section of
// the mode in use — registry.direct.imagesRepo, registry.proxy.imagesRepo — so a message printing
// "<mode>" names a path that is in nobody's document, and the reader has to work out which of the
// four sections is theirs before they can act on it.
//
// A registry the registry ModuleConfig names has neither: its upstream is primary.upstream there,
// whether the user wrote that ModuleConfig or the installer did because nothing else named a
// registry. Pointing such a reader at the deckhouse ModuleConfig sends them to a section their
// configuration does not have.
//
// The same goes for a registry whose resolved mode is not the one the user wrote: a Direct section
// of the deckhouse ModuleConfig is installed as InitConfiguration, and InitConfiguration as legacy
// Unmanaged. Printing the resolved mode named registry.unmanaged, which neither of those users
// wrote, so each is pointed at the field they did write.
func registryImagesRepoField(mode string) string {
	switch mode {
	case registryFromModule:
		return `.spec.settings.primary.upstream.host and .path in the "registry" ModuleConfig`
	case registryFoldedDirect:
		return `.spec.settings.registry.direct.imagesRepo in the "deckhouse" ModuleConfig`
	case registryFromInitConfiguration:
		return "InitConfiguration.deckhouse.imagesRepo"
	}
	return fmt.Sprintf(`.spec.settings.registry.%s.imagesRepo in the "deckhouse" ModuleConfig (InitConfiguration.deckhouse.imagesRepo)`, registrySection(mode))
}

func registryCAField(mode string) string {
	switch mode {
	case registryFromModule:
		return `.spec.settings.primary.upstream.ca in the "registry" ModuleConfig`
	case registryFoldedDirect:
		return `.spec.settings.registry.direct.ca in the "deckhouse" ModuleConfig`
	case registryFromInitConfiguration:
		return "InitConfiguration.deckhouse.registryCA"
	}
	return fmt.Sprintf(`.spec.settings.registry.%s.ca in the "deckhouse" ModuleConfig (InitConfiguration.deckhouse.registryCA)`, registrySection(mode))
}

// Stand-ins for a mode where the mode the registry resolved to does not say where its fields live.
// None of them is the name of a mode, so none can be mistaken for one.
const (
	// registryFromModule: the registry comes from the registry ModuleConfig. The fields then live
	// there, under primary.upstream, and no mode section of the deckhouse ModuleConfig applies.
	registryFromModule = "registry module"

	// registryFoldedDirect: the user wrote registry.mode: Direct in the deckhouse ModuleConfig, and
	// the installer read it as InitConfiguration (see registry.FoldLegacyDirectIntoInit).
	registryFoldedDirect = "Direct folded into InitConfiguration"

	// registryFromInitConfiguration: the user wrote the registry into InitConfiguration.
	registryFromInitConfiguration = "InitConfiguration"
)

// configuredRegistryMode is what the checks pass to the field helpers above: the mode of the
// deckhouse settings, or one of the stand-ins above when the document the registry came from is not
// the deckhouse section of that mode. Empty when no configuration was loaded, which registrySection
// reports as a placeholder rather than guessing.
func configuredRegistryMode(meta *config.MetaConfig) string {
	if meta == nil {
		return ""
	}
	switch {
	case meta.Registry.FromRegistryModule:
		return registryFromModule
	case meta.Registry.FoldedFromDirect:
		return registryFoldedDirect
	case meta.Registry.FromInitConfiguration:
		return registryFromInitConfiguration
	}
	return string(meta.Registry.Settings.Mode)
}

// registrySection is the ModuleConfig key of a registry mode, which is its name lowercased. An
// unset mode keeps the placeholder: guessing a section would be worse than admitting the gap.
func registrySection(mode string) string {
	if mode == "" {
		return "<mode>"
	}
	return strings.ToLower(mode)
}

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

// registryMode is the mode the registry is configured in, used to name the ModuleConfig section
// the fields live under. Empty when no configuration was loaded, which registrySection reports
// as a placeholder rather than guessing.
func (c RegistryReachableCheck) registryMode() string {
	return configuredRegistryMode(c.MetaConfig)
}
