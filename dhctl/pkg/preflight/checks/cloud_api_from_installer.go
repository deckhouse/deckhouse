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
	"net/http"
	"net/url"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils"
	cca "github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils/check-cloud-api"
)

const CloudAPIFromInstallerCheckName preflight.CheckName = "cloud-api-from-installer"

// CloudAPIFromInstallerCheck asks whether dhctl itself reaches the cloud API it is about to
// drive.
//
// This is a different subject from cloud-api-accessibility, which asks whether the MASTER reaches
// it — the master needs it for the cloud-controller-manager, and that check runs post-infra
// through a tunnel. Nothing asked the same question of the installer, and the installer is who
// creates the infrastructure: with no egress to the API, the run got as far as the infrastructure
// plan and came back as three provider stack traces, one per resource that happened to read the
// API, each ending in the same dial timeout. Seen live on a runner whose container could reach
// the registry but not the cloud API.
//
// Pre-infra, so it costs nothing and nothing has been created when it speaks.
type CloudAPIFromInstallerCheck struct {
	MetaConfig *config.MetaConfig
}

func (CloudAPIFromInstallerCheck) Description() string {
	return "the installer reaches the cloud provider API it is about to drive"
}

func (c CloudAPIFromInstallerCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", errors.New("meta config is required")
	}

	endpoint, err := c.endpoint()
	if err != nil || endpoint == nil {
		return "", err
	}

	client, err := utils.BuildHTTPClientFromEnvironment(utils.TLSOptions{
		ServerName: endpoint.URL.Hostname(),
		Insecure:   endpoint.Insecure,
		CACert:     endpoint.CACert,
	})
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  endpoint.Field,
			Observed: err.Error(),
			Expected: "TLS settings the request can be made with",
			Fix:      "correct the CA certificate in the provider section of the <Provider>ClusterConfiguration document",
		})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("building the request to %s: %w", endpoint.URL, err)
	}

	// The same resolution the infrastructure utility will make: dhctl hands it HTTP_PROXY and
	// HTTPS_PROXY out of its own environment (pkg/infrastructure/terraform/cmd.go), so the
	// installer's environment — not ClusterConfiguration.proxy, which is about the cluster's
	// nodes — is what decides the path both of them take.
	proxyURL, _ := utils.ProxyFromEnvironment(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", c.transportFailure(endpoint, proxyURL, err)
	}
	defer resp.Body.Close()

	// Any answer means the API was reached, which is all this check is about: the credentials are
	// the infrastructure utility's business, and it has its own, so 401 and 404 are passes.
	if resp.StatusCode >= 500 {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from the installer", endpoint.URL),
			Observed: fmt.Sprintf("HTTP %d", resp.StatusCode),
			Expected: "any answer below 500",
			Fix:      c.egressFix(endpoint, proxyURL),
		}
	}

	if proxyURL != nil {
		return fmt.Sprintf("%s answers, through proxy %s", endpoint.URL, proxyURL.Redacted()), nil
	}
	return fmt.Sprintf("%s answers", endpoint.URL), nil
}

// endpoint resolves the API address out of the <Provider>ClusterConfiguration, and returns nil
// where there is nothing to resolve.
//
// A configuration that declares no provider section at all is not a failure here: that is the
// shape where the provider is configured through its own ModuleConfig and Secret, whose form
// belongs to the provider and is validated by it. Reading each provider's ModuleConfig here would
// put N unrelated schemas into dhctl with no guarantee they stay alike, so the check says what it
// could not look at instead of guessing.
func (c CloudAPIFromInstallerCheck) endpoint() (*cca.CloudAPIConfig, error) {
	if !cca.KnownProviders(c.MetaConfig.ProviderName) {
		return nil, preflight.NotApplicable("no cloud API endpoint is known for provider %q", c.MetaConfig.ProviderName)
	}

	providerConfig, ok := c.MetaConfig.ProviderClusterConfig["provider"]
	if !ok || len(providerConfig) == 0 {
		return nil, preflight.NotApplicable(
			"there is no %sClusterConfiguration with a provider section to read the API address from",
			c.MetaConfig.ProviderName)
	}

	endpoint, err := cca.EndpointFor(c.MetaConfig.ProviderName, providerConfig)
	if err != nil {
		return nil, preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the cloud API endpoint of provider %q", c.MetaConfig.ProviderName),
			Observed: err.Error(),
			Expected: "an address the installer can reach",
			Fix:      "correct the provider section of the <Provider>ClusterConfiguration document in --config",
		})
	}
	if endpoint == nil {
		return nil, preflight.NotApplicable("no cloud API endpoint is known for provider %q", c.MetaConfig.ProviderName)
	}
	return endpoint, nil
}

func (c CloudAPIFromInstallerCheck) transportFailure(endpoint *cca.CloudAPIConfig, proxyURL *url.URL, err error) error {
	failure := &preflight.Failure{
		Checked:  fmt.Sprintf("GET %s from the installer", endpoint.URL),
		Observed: classifyNetworkError(err),
		Expected: "the API to answer",
		Fix:      c.egressFix(endpoint, proxyURL),
		Err:      err,
	}

	// A certificate problem is settled: retrying changes nothing about it.
	if isCertificateError(err) {
		failure.Fix = fmt.Sprintf(
			"if the API uses a private CA, put it in the provider section of the %sClusterConfiguration document; "+
				"if its certificate is not yet valid, check the clock where dhctl runs",
			c.MetaConfig.ProviderName,
		)
		return preflight.Permanent(failure)
	}
	return failure
}

// egressFix points at the installer's own network. The advice cloud-api-accessibility gives —
// NAT, route, security group for the master node — is about a machine that does not exist yet
// when this check runs, and would send the operator to the wrong side of the problem.
func (c CloudAPIFromInstallerCheck) egressFix(endpoint *cca.CloudAPIConfig, proxyURL *url.URL) string {
	if proxyURL != nil {
		return fmt.Sprintf(
			"check that %s reaches %s; it comes from HTTP_PROXY/HTTPS_PROXY in dhctl's own environment, "+
				"which is also what the infrastructure utility is handed",
			proxyURL.Redacted(), endpoint.URL.Host,
		)
	}
	return fmt.Sprintf(
		"give dhctl itself egress to %s — this is the installer's network, not the cluster's, and in a container "+
			"it is the container's — and check %s",
		endpoint.URL.Host, endpoint.Field,
	)
}

func CloudAPIFromInstaller(meta *config.MetaConfig) preflight.Check {
	check := CloudAPIFromInstallerCheck{MetaConfig: meta}
	return preflight.Check{
		Name:        CloudAPIFromInstallerCheckName,
		Description: check.Description(),
		Phase:       preflight.PhasePreInfra,
		Retry:       preflight.NetworkRetry,
		Timeout:     preflight.LongCheckTimeout,
		Run:         check.Run,
	}
}
