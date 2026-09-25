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

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils"
	cca "github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils/check-cloud-api"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

const CloudAPICheckName preflight.CheckName = "cloud-api-accessibility"

// sshClientSource is the part of the SSH provider initializer this check needs: the client to
// run the request from, and which backend it is, since clissh and gossh spell a port-forward
// differently. It is an interface rather than the concrete initializer so the tunnel can be stood
// up locally in tests; production passes *providerinitializer.SSHProviderInitializer.
type sshClientSource interface {
	GetSSHProvider(ctx context.Context) (libcon.SSHProvider, error)
	IsLegacyMode() bool
}

type CloudAPICheck struct {
	MetaConfig *config.MetaConfig
	// SSHProviderInitializer rather than a built provider: the suite is constructed before the
	// infrastructure exists, and lib-connection copies the host list into the provider when it
	// is built. The provider captured at construction therefore has no hosts, Client() fails
	// with "hosts is empty in session or default config" — and this check used to answer that
	// by returning nil, which the runner printed as a ✓. The check only ever really ran on a
	// resumed bootstrap.
	SSHProviderInitializer sshClientSource
	// Endpoint renders user@host:port from the configuration, for the failures that happen
	// before there is a client to read it off. It is exactly the case this check has to name:
	// with ssh-credential skipped, this is the first thing to touch the connection.
	Endpoint EndpointFunc
}

func (CloudAPICheck) Description() string {
	return "the cloud provider API is reachable from the master node"
}

func (CloudAPICheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (CloudAPICheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c CloudAPICheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", errors.New("no configuration was loaded from --config")
	}

	cloudAPIConfig, err := c.endpoint()
	if err != nil {
		return "", err
	}
	if cloudAPIConfig == nil {
		return "", preflight.NotApplicable("no cloud API endpoint is known for provider %q", c.MetaConfig.ProviderName)
	}

	sshClient, err := c.masterClient(ctx)
	if err != nil {
		return "", err
	}

	proxyURL, noProxyAddresses, err := utils.GetProxyFromMetaConfig(c.MetaConfig)
	if err != nil {
		return "", fmt.Errorf("reading ClusterConfiguration.proxy: %w", err)
	}

	// Either the request goes through the proxy, or noProxy exempts this endpoint and it goes
	// straight out. Whichever it is, that is also the address the tunnel has to reach.
	throughProxy := proxyURL != nil && !utils.ShouldSkipProxyCheck(cloudAPIConfig.URL, noProxyAddresses)
	tunnelTarget := cloudAPIConfig.URL
	if throughProxy {
		tunnelTarget = proxyURL
	}

	legacyMode := c.SSHProviderInitializer.IsLegacyMode()
	tun, err := utils.SetupSSHTunnelToProxyAddr(ctx, sshClient, tunnelTarget, legacyMode)
	if err != nil {
		return "", tunnelFailure(sshClient, tunnelTarget, err)
	}
	defer tun.Stop()

	if err := c.request(ctx, cloudAPIConfig, proxyURL, throughProxy); err != nil {
		return "", err
	}

	if throughProxy {
		return fmt.Sprintf("%s answers from the master node via proxy %s", cloudAPIConfig.URL, proxyURL.Redacted()), nil
	}
	return fmt.Sprintf("%s answers from the master node", cloudAPIConfig.URL), nil
}

// endpoint resolves the provider's API address out of the configuration.
func (c CloudAPICheck) endpoint() (*cca.CloudAPIConfig, error) {
	if !cca.KnownProviders(c.MetaConfig.ProviderName) {
		return nil, nil
	}

	providerConfig, ok := c.MetaConfig.ProviderClusterConfig["provider"]
	if !ok || len(providerConfig) == 0 {
		return nil, preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("%s.provider", providerDocumentKind(c.MetaConfig.ProviderName)),
			Observed: "the document has no provider section",
			Expected: "the connection parameters of the cloud the cluster is being created in",
			Fix:      fmt.Sprintf("add a provider section to %s in --config", providerDocumentKind(c.MetaConfig.ProviderName)),
		})
	}

	cloudAPIConfig, err := cca.EndpointFor(c.MetaConfig.ProviderName, providerConfig)
	if err != nil {
		return nil, preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the cloud API endpoint of provider %q", c.MetaConfig.ProviderName),
			Observed: err.Error(),
			Expected: "an address the master node can reach",
			Fix:      fmt.Sprintf("correct the provider section of %s in --config", providerDocumentKind(c.MetaConfig.ProviderName)),
		})
	}
	return cloudAPIConfig, nil
}

// masterClient resolves the SSH client now, rather than reusing one captured before the master
// existed.
//
// It does not wait for the machine any more. It used to, because nothing else in this phase did:
// the phase runs between creating the master and the bootstrap's own wait for SSH on it. ssh-
// credential now goes first and carries that wait, so a second one here probed a connection that
// had just been proven — 50 milliseconds, and a "Waiting for SSH connection" box in the middle of
// the report that read as though the wait were happening twice.
//
// If the connection is broken anyway — ssh-credential turned off by name — this is the first
// thing in the run to touch it, so it has to name the failure itself. It used to return the
// error the provider handed it, which the runner prints raw: a wrong --ssh-user came back as
// "no SSH connection to the master node: ... dial: transient error, may succeed on retry",
// naming neither the user nor the host. The comment here used to claim the tunnel below would
// classify it, which is only true when a client was obtained and the forward then failed.
func (c CloudAPICheck) masterClient(ctx context.Context) (libcon.SSHClient, error) {
	sshProvider, err := c.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil {
		return nil, c.connectionFailure(err)
	}

	sshClient, err := sshProvider.Client(ctx)
	if err != nil {
		return nil, c.connectionFailure(err)
	}

	return sshClient, nil
}

// connectionFailure names a connection that was never made. The verdict is noConnection's, the
// same one every other check gets, so the reader sees one answer to one question however they
// arrived at it.
//
// It deliberately does not reproduce what ssh-credential would have said. This branch is reached
// when that check did not run, which on a bootstrap means the operator turned it off by name: an
// answer that repeats its advice hands back the verdict they declined to read, under a different
// check's name. What is owed here is the opposite of the OpenStack report that started this — not
// advice about sshd's AllowTcpForwarding for a login that never happened, but the plain fact that
// there was no connection, and to whom.
func (c CloudAPICheck) connectionFailure(err error) error {
	label := ""
	if c.Endpoint != nil {
		label = c.Endpoint()
	}
	return noConnection(label, err)
}

// request asks the endpoint through the tunnel and turns the answer into a verdict.
func (c CloudAPICheck) request(ctx context.Context, cloudAPIConfig *cca.CloudAPIConfig, proxyURL *url.URL, throughProxy bool) error {
	tlsOpts := utils.TLSOptions{
		ServerName: cloudAPIConfig.URL.Hostname(),
		Insecure:   cloudAPIConfig.Insecure,
		CACert:     cloudAPIConfig.CACert,
	}

	var (
		client *http.Client
		err    error
	)
	if throughProxy {
		client, err = utils.BuildHTTPClientWithLocalhostProxy(proxyURL, tlsOpts)
	} else {
		client, err = utils.BuildHTTPClientThroughTunnel(tlsOpts)
	}
	if err != nil {
		return preflight.Permanent(&preflight.Failure{
			Checked:  cloudAPIConfig.Field,
			Observed: err.Error(),
			Expected: "a usable CA bundle",
			Fix:      fmt.Sprintf("correct the CA certificate in the provider section of %s", providerDocumentKind(c.MetaConfig.ProviderName)),
		})
	}

	client.CheckRedirect = utils.StopAtFirstAnswer

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cloudAPIConfig.URL.String(), nil)
	if err != nil {
		return fmt.Errorf("building the request to %s: %w", cloudAPIConfig.URL, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		if throughProxy {
			// The proxy refused the CONNECT rather than failing to connect. Its status is
			// returned as an error, never as a response, so this is the only place the
			// 407 below can be reached from for an https endpoint — which every cloud API is.
			if refusal := proxyRefusal(proxyConnectStatus(err), proxyURL, cloudAPIConfig.URL); refusal != nil {
				return refusal
			}
		}
		return c.transportFailure(cloudAPIConfig, proxyURL, throughProxy, err)
	}
	defer resp.Body.Close()

	// Any answer at all means the node reached the API: this check is about connectivity, not
	// about credentials, so 401 and 404 are passes. 5xx is the exception — it is what a proxy
	// that cannot reach the API upstream returns, which is the failure being looked for.
	if resp.StatusCode >= 500 {
		return &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from the master node", cloudAPIConfig.URL),
			Observed: fmt.Sprintf("HTTP %d", resp.StatusCode),
			Expected: "an answer from the API (any status below 500)",
			Fix:      c.proxyOrNetworkFix(cloudAPIConfig, proxyURL, throughProxy),
		}
	}
	if throughProxy && resp.StatusCode == http.StatusProxyAuthRequired {
		return proxyRefusal(resp.StatusCode, proxyURL, cloudAPIConfig.URL)
	}
	return nil
}

// transportFailure names the class of a connection that never produced an answer. x509, DNS and
// a refused connection are different problems with different fixes, and all three used to be
// reported as the single sentence "could not reach Cloud API from master node".
func (c CloudAPICheck) transportFailure(cloudAPIConfig *cca.CloudAPIConfig, proxyURL *url.URL, throughProxy bool, err error) error {
	failure := &preflight.Failure{
		Checked:  fmt.Sprintf("GET %s from the master node", cloudAPIConfig.URL),
		Observed: classifyNetworkError(err),
		Expected: "an answer from the API",
		Fix:      c.proxyOrNetworkFix(cloudAPIConfig, proxyURL, throughProxy),
		Err:      err,
	}

	// A certificate problem is settled: retrying changes nothing about it.
	if isCertificateError(err) {
		failure.Fix = fmt.Sprintf(
			"put the private CA of the API into the provider section of %s. "+
				"If the certificate is not yet valid, correct the clock on the master node",
			providerDocumentKind(c.MetaConfig.ProviderName),
		)
		return preflight.Permanent(failure)
	}
	return failure
}

func (c CloudAPICheck) proxyOrNetworkFix(cloudAPIConfig *cca.CloudAPIConfig, proxyURL *url.URL, throughProxy bool) string {
	if throughProxy {
		return fmt.Sprintf(
			"check that %s reaches %s, or add %s to ClusterConfiguration.proxy.noProxy",
			proxyURL.Redacted(), cloudAPIConfig.URL.Host, cloudAPIConfig.URL.Hostname(),
		)
	}
	return fmt.Sprintf(
		"give the master node egress to %s through a NAT, a route or a security group. Check %s",
		cloudAPIConfig.URL.Host, cloudAPIConfig.Field,
	)
}

// tunnelFailure separates "the forward was refused" from "the local end could not be opened".
// The advice about AllowTcpForwarding used to be attached to the local-bind branch, where it
// could not apply: the local end is this host's, and sshd has no say in it.
func tunnelFailure(sshClient libcon.SSHClient, target *url.URL, err error) error {
	// The forward is opened by running ssh, so everything that stops ssh from connecting at all
	// surfaces here as a failed forward — and used to be reported as one. A wrong --ssh-user came
	// back as "check that sshd has AllowTcpForwarding yes", which is advice about a file on a
	// machine the operator was never logged in to.
	if sshNeverConnected(err) {
		return sshLoginFailure(hostLabelOfClient(sshClient), err)
	}

	return &preflight.Failure{
		Checked:  fmt.Sprintf("ssh port forward to %s through %s", target.Host, hostLabelOfClient(sshClient)),
		Observed: classifyNetworkError(err),
		Expected: "a local port forwarded by the master node",
		Fix: "set AllowTcpForwarding yes and DisableForwarding no in sshd_config on the master node. " +
			"Free port " + utils.ProxyTunnelPort + " on this host",
		Err: err,
	}
}

func CloudAPIAccess(meta *config.MetaConfig, sshProviderInitializer *providerinitializer.SSHProviderInitializer, endpoint EndpointFunc) preflight.Check {
	check := CloudAPICheck{
		MetaConfig:             meta,
		SSHProviderInitializer: sshProviderInitializer,
		Endpoint:               endpoint,
	}
	return preflight.Check{
		Name:        CloudAPICheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.LongCheckTimeout,
		Run:         check.Run,
	}
}
