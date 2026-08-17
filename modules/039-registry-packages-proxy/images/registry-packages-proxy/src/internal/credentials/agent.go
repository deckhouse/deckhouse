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

package credentials

import (
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

// Fetching through the node agent, which is how anything on a node fetches once the registry module
// manages the pull path.
//
// The cluster records `registry.d8-system.svc:5001` everywhere — image references, ModuleSources,
// statuses — and that address is a KEY, not somewhere anybody dials. The agent listens on the
// loopback of every node and decides per request whether the answer comes from the in-cluster store
// or from an upstream. A container runtime is redirected into it by a drop-in file; a process has to
// dial it deliberately, which is what this does.
//
// Three things about the agent are not properties of any registry the cluster was told about:
//
//   - it serves HTTPS whatever the registry behind it speaks;
//   - its authority is generated on the node and never leaves it, so no cluster object can carry it —
//     every node has a different one, which is why it is mounted from the host;
//   - nothing authenticates to it. It holds the credentials for whatever is behind it and asks the
//     client for none.
//
// The last one is why credentials are CLEARED rather than left to go unused: a docker config is
// looked up by host, so one carrying no entry for the host being dialled makes building the client
// fail outright, before a request is made.
//
// The same translation already exists for the Deckhouse controller, in
// deckhouse-controller/pkg/controller/module-controllers/utils. It is repeated rather than imported
// because this proxy is its own Go module; the constants below are pinned to their source by a test.
const (
	// storeHost and storePath are the in-cluster address, as recorded. Source of truth:
	// `Host` and `Path` in go_lib/registry/const/registry.go.
	storeHost = "registry.d8-system.svc:5001"
	storePath = "system/deckhouse"

	// proxyHost is the same registry as dialled: the agent, on this node's loopback.
	// Source of truth: `ProxyHost`.
	proxyHost = "127.0.0.1:5001"

	// agentCAFile is the authority the agent signs what it serves with. Source of truth:
	// `AgentCAFile`. Mounted from the host — see the deployment.
	agentCAFile = "/etc/kubernetes/registry-agent/pki/ca.crt"

	// agentScheme is fixed, whatever the registry behind the agent speaks.
	agentScheme = "https"
)

// servedByTheAgent reports whether a repository is one the node agent answers for.
//
// Both spellings, because the same registry is written one way and dialled another: a request that
// names the agent as its own registry is a loop, and the agent refuses it.
func servedByTheAgent(repository string) bool {
	for _, host := range []string{storeHost, proxyHost} {
		if repository == host || strings.HasPrefix(repository, host+"/") {
			return true
		}
	}
	return false
}

// throughTheAgent rewrites a client configuration to fetch through the agent, and returns it
// unchanged for any repository the agent does not serve.
//
// Returned as a copy: the configuration it is given is shared between callers and holds the
// repository as the cluster records it, which is what the map is keyed by and what belongs in
// anything written down.
func throughTheAgent(config *registry.ClientConfig, readCA func() ([]byte, error)) *registry.ClientConfig {
	if config == nil || !servedByTheAgent(config.Repository) {
		return config
	}

	// The authority first, and its absence means the agent is not there — so the request goes the way
	// it went before rather than to the agent.
	//
	// This is the deployment circle the design ADR names outright: "deployment is a closed circle: the
	// registry components, the agent among them, have to be pulled through the registry itself, and it
	// is not up yet". The agent is a static pod installed by bashible, and the package it is installed
	// FROM is fetched through this proxy. A proxy that dials the agent unconditionally therefore waits
	// for what is waiting for it: measured on a fresh cluster as `rpp-get [registry-agent] attempt 14
	// failed … HTTP 500: dial tcp 127.0.0.1:5001: connect: connection refused`, thirty times over,
	// after which no node ever joined.
	//
	// bashible writes this authority before the module has any PKI of its own, so its presence is the
	// earliest honest signal that fetching through the agent is possible at all.
	authority, err := readCA()
	if err != nil || len(authority) == 0 {
		return config
	}

	dialled := *config
	// The host is replaced, not prefixed, and either spelling is stripped: a configuration already
	// naming the agent would otherwise become `127.0.0.1:5001127.0.0.1:5001/…`, which fails as a
	// malformed repository rather than as anything a reader could act on.
	dialled.Repository = proxyHost + strings.TrimPrefix(strings.TrimPrefix(config.Repository, storeHost), proxyHost)
	dialled.Scheme = agentScheme
	dialled.Auth = ""
	dialled.CA = string(authority)

	return &dialled
}

// readAgentCA reads the authority from the host mount, on every call.
//
// Not cached: the agent is installed by a bashible pass, so on a node that is still coming up the
// file appears after this process starts. A value cached at startup would be an empty one for the
// life of the pod.
func readAgentCA() ([]byte, error) {
	return os.ReadFile(agentCAFile)
}
