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
		return "", fmt.Errorf("metaConfig is required")
	}

	nodeInterface, err := c.nodeInterface(ctx)
	if err != nil {
		return "", err
	}
	if nodeInterface == nil {
		return "", preflight.NotApplicable("there is no SSH connection to a node to make the request from")
	}
	host := hostPhrase(nodeInterface)

	registry := c.MetaConfig.Registry.Settings.RemoteData
	address, _ := registry.AddressAndPath()
	endpoint := registryV2URL(c.MetaConfig).String()

	// The node is asked with whatever it has. curl and wget are what a Deckhouse-supported
	// distribution ships; if neither is there the question cannot be asked from here, and saying
	// so is better than reporting the registry as unreachable.
	probe, err := nodeRegistryProbe(ctx, nodeInterface)
	if err != nil {
		return "", preflight.NotApplicable("neither curl nor wget is installed on %s", host)
	}

	cmd := nodeInterface.Command(probe.binary, probe.args(endpoint)...)
	stdout, _, runErr := cmd.Output(ctx)
	status := strings.TrimSpace(string(stdout))

	if runErr != nil && status == "" {
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from %s", endpoint, host),
			Observed: "the node could not reach the registry",
			Expected: "the registry API to answer from the node",
			Fix: fmt.Sprintf("give the node egress to %s — a NAT gateway or a route, and a security group that "+
				"allows it; if the node goes through a proxy, set ClusterConfiguration.proxy", address),
			Err: runErr,
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
			Expected: "the registry API to answer from the node",
			Fix:      fmt.Sprintf("give the node egress to %s, or set ClusterConfiguration.proxy", address),
		}
	default:
		return "", &preflight.Failure{
			Checked:  fmt.Sprintf("GET %s from %s", endpoint, host),
			Observed: fmt.Sprintf("the node got HTTP %s, which is not how a registry answers /v2/", status),
			Expected: "HTTP 200 or 401 from the registry API",
			Fix: fmt.Sprintf("check what answers at %s from the node — a captive portal, a proxy or an "+
				"error page rather than the registry", address),
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
}

func nodeRegistryProbe(ctx context.Context, nodeInterface nodeCommandRunner) (registryProbe, error) {
	if err := nodeInterface.Command("command", "-v", "curl").Run(ctx); err == nil {
		return registryProbe{
			binary: "curl",
			args: func(endpoint string) []string {
				// Only the status code is printed, so the body never reaches the log.
				return []string{"-s", "-o", "/dev/null", "-w", "%{http_code}", "--max-time", "20", endpoint}
			},
		}, nil
	}

	if err := nodeInterface.Command("command", "-v", "wget").Run(ctx); err == nil {
		return registryProbe{
			binary: "wget",
			args: func(endpoint string) []string {
				return []string{"-q", "-O", "/dev/null", "-T", "20", "--server-response", endpoint}
			},
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
