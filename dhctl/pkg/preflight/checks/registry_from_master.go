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
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/checks/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

// RegistryFromMasterCheck asks the master node itself whether it can reach the registry.
//
// registry-reachable asks the same question of the installer host, which is often a laptop or a
// CI runner with completely different egress; registry-access-through-proxy asks it of the node,
// but only when a proxy is configured. The gap between them is the common one: a master with no
// NAT, no route, a security group that blocks 443, or a private CA the node does not have. It
// surfaces as bashible looping on "kubernetes-api-proxy not running after 200s", or as fifteen
// minutes of "Deckhouse pod found: … (Pending)" with no ImagePullBackOff to read.
type RegistryFromMasterCheck struct {
	MetaConfig             *config.MetaConfig
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
}

const RegistryFromMasterCheckName preflight.CheckName = "registry-access-from-master"

func (RegistryFromMasterCheck) Description() string {
	return "the master node can reach the container registry"
}

func (RegistryFromMasterCheck) Phase() preflight.Phase {
	return preflight.PhasePostInfra
}

func (RegistryFromMasterCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c RegistryFromMasterCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil {
		return "", fmt.Errorf("dhctl was given no cluster configuration")
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	address, _ := registry.AddressAndPath()
	registryURL := registryV2URL(c.MetaConfig)
	endpoint := registryURL.String()

	// A node that is configured to reach the registry through a proxy is not expected to reach it
	// directly, and this probe goes direct. Asking anyway made the check demand egress the cluster
	// was never going to have: on a static cluster behind 192.168.199.254:8888 it reported HTTP 000
	// while registry-access-through-proxy, which asks the same question the right way, passed.
	proxyURL, noProxy, err := utils.GetProxyFromMetaConfig(c.MetaConfig)
	if err != nil {
		return "", fmt.Errorf("reading ClusterConfiguration.proxy: %w", err)
	}
	if proxyURL != nil && !utils.ShouldSkipProxyCheck(registryURL, noProxy) {
		return "", preflight.NotApplicable(
			"the nodes reach %s through the proxy in ClusterConfiguration.proxy, which "+
				"registry-access-through-proxy is what asks about", address)
	}

	nodeInterface, err := c.nodeInterface(ctx)
	if err != nil {
		return "", err
	}
	if nodeInterface == nil {
		return "", preflight.NotApplicable("dhctl was given no SSH host to make the request from")
	}
	host := hostPhrase(nodeInterface)

	// The node is asked with whatever it has. curl and wget are what a Deckhouse-supported
	// distribution ships; if neither is there the question cannot be asked from here, and saying
	// so is better than reporting the registry as unreachable.
	probe, err := nodeRegistryProbe(ctx, nodeInterface, registry.CA != "")
	if err != nil {
		return "", preflight.NotApplicable("neither curl nor wget is installed on %s", host)
	}

	cmd := nodeInterface.Command(probe.binary, probe.args(endpoint)...)
	stdout, _, runErr := cmd.Output(ctx)
	status := strings.TrimSpace(string(stdout))

	if runErr != nil && status == "" {
		observed, fix := probe.classify(runErr, registryURL.Hostname(), address, c.registryMode())
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from %s", endpoint, host),
			Observed: observed,
			Expected: "an answer from the registry API to the node",
			Fix:      fix,
			Err:      runErr,
		}
	}

	// Any answer means the node reached it. 401 is the registry asking for credentials, which is
	// exactly as good as 200 for the question being asked here.
	switch status {
	case "200", "401":
		return fmt.Sprintf("%s answers from %s (HTTP %s)", address, host, status), nil
	case "":
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from %s", endpoint, host),
			Observed: "the node got no answer",
			Expected: "an answer from the registry API to the node",
			Fix:      fmt.Sprintf("give the node egress to %s, or set ClusterConfiguration.proxy", address),
		}
	default:
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from %s", endpoint, host),
			Observed: fmt.Sprintf("the node got HTTP %s, which is not how a registry answers /v2/", status),
			Expected: "HTTP 200 or 401 from the registry API",
			Fix:      fmt.Sprintf("check from the node what answers at %s", endpoint),
		}
	}
}

func (c RegistryFromMasterCheck) nodeInterface(ctx context.Context) (nodeCommandRunner, error) {
	resolve := nodeInterfaceResolverFor(c.SSHProviderInitializer)
	nodeInterface, err := resolve(ctx)
	if err != nil {
		return nil, err
	}
	if hostLabel(nodeInterface) == "" {
		// No SSH behind it: this would be the installer host, which registry-reachable covers.
		return nil, nil
	}
	return nodeInterface, nil
}

// registryProbe is the command the node is asked with, and how to read its answer.
type registryProbe struct {
	binary string
	args   func(endpoint string) []string
	// classify turns the probe's exit status into what went wrong and what to do about it.
	classify func(err error, hostname, address, mode string) (observed, fix string)
}

// curlExitCodes are the ones worth telling apart. They are the four things that stop a node from
// pulling an image, and each sends the reader somewhere different — which the single sentence
// "the node could not reach the registry" did not.
//
// The name not resolving is the one worth the most: it is what a node with the wrong resolver
// does, and it surfaces from bashible as "etcd not running after 200s", because crictl could not
// resolve the registry and never pulled the image. Nothing in that message mentions DNS.
func curlClassify(err error, hostname, address, mode string) (string, string) {
	status, ok := exitStatus(err)
	if !ok {
		return "the node could not reach the registry", registryEgressFix(address)
	}

	switch status {
	case 6:
		return fmt.Sprintf("the node cannot resolve %s", hostname),
			fmt.Sprintf("check /etc/resolv.conf on the node and give it a resolver that knows %s, "+
				"or add the address to /etc/hosts on the node", hostname)
	case 7:
		return fmt.Sprintf("the node could not connect to %s", address), registryEgressFix(address)
	case 28:
		return fmt.Sprintf("the node timed out reaching %s", address), registryEgressFix(address)
	case 35, 60:
		return "the node rejected the registry certificate",
			fmt.Sprintf("put the registry CA into %s, or correct the certificate the registry serves",
				registryCAField(mode))
	default:
		return "the node could not reach the registry", registryEgressFix(address)
	}
}

// wgetClassify is the same question of a node with no curl. wget collapses every network failure
// into one status, so only the certificate is separable.
func wgetClassify(err error, _, address, mode string) (string, string) {
	if status, ok := exitStatus(err); ok && status == 5 {
		return "the node rejected the registry certificate",
			fmt.Sprintf("put the registry CA into %s, or correct the certificate the registry serves",
				registryCAField(mode))
	}
	return "the node could not reach the registry", registryEgressFix(address)
}

func registryEgressFix(address string) string {
	return fmt.Sprintf("give the node egress to %s: add a route or a NAT gateway, and allow it in the "+
		"security group. If the node goes through a proxy, set ClusterConfiguration.proxy.", address)
}

func nodeRegistryProbe(ctx context.Context, nodeInterface nodeCommandRunner, insecure bool) (registryProbe, error) {
	if err := nodeInterface.Command("command", "-v", "curl").Run(ctx); err == nil {
		return registryProbe{
			binary: "curl",
			args: func(endpoint string) []string {
				args := []string{"-s", "-o", "/dev/null", "-w", "%{http_code}", "--max-time", "20"}
				if insecure {
					// The node has no CA yet — bashible installs it later — so a registry
					// signed by a private one would fail verification here on a node that is
					// perfectly able to pull from it once configured. The question this check
					// asks is whether the node reaches the registry; whether the certificate
					// is trustworthy is registry-reachable's, and it has the CA to judge with.
					args = append(args, "-k")
				}
				return append(args, endpoint)
			},
			classify: curlClassify,
		}, nil
	}

	if err := nodeInterface.Command("command", "-v", "wget").Run(ctx); err == nil {
		return registryProbe{
			binary: "wget",
			args: func(endpoint string) []string {
				args := []string{"-q", "-O", "/dev/null", "-T", "20", "--server-response"}
				if insecure {
					args = append(args, "--no-check-certificate")
				}
				return append(args, endpoint)
			},
			classify: wgetClassify,
		}, nil
	}

	return registryProbe{}, fmt.Errorf("neither curl nor wget is available")
}

func RegistryFromMaster(meta *config.MetaConfig, sshProviderInitializer *providerinitializer.SSHProviderInitializer) preflight.Check {
	check := RegistryFromMasterCheck{MetaConfig: meta, SSHProviderInitializer: sshProviderInitializer}
	return preflight.Check{
		Name:        RegistryFromMasterCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.NodeCheckTimeout,
		Run:         check.Run,
	}
}

// registryMode is the mode the registry is configured in; see registrySection.
func (c RegistryFromMasterCheck) registryMode() string {
	if c.MetaConfig == nil {
		return ""
	}
	return string(c.MetaConfig.Registry.Settings.Mode)
}
