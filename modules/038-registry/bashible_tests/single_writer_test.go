/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package bashible_tests pins the one invariant the whole registry design rests on:
// exactly one thing configures the container runtime on a node.
//
// The two implementations both write `/etc/containerd/registry.d`, and an explicit host
// directory there takes precedence over the agent's `_default` fallback — so a leftover
// from the wrong writer does not merely add noise, it routes pulls around the agent and
// around the in-cluster cache with it. Which writer is active is decided by one field of
// the node context, and these tests render the real templates to check that the decision
// reaches every step that could act on it.
//
// Rendered with the production function map and the production renderer, because the
// gates use `has`, `with` and `not` — and a test with its own function map would be
// checking a template nobody serves.
package bashible_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// candi is the repository's bashible tree, relative to this package.
const candi = "../../../candi/bashible/common-steps"

// nodeContext is the part of the bashible context these steps read.
//
// Only the registry subtree differs between the two cases; everything else is filler that
// keeps the templates renderable.
func nodeContext(t *testing.T, registry map[string]any) map[string]any {
	t.Helper()

	return map[string]any{
		"cri":               "Containerd",
		"runType":           "Normal",
		"kubernetesVersion": "1.31",
		"k8s":               map[string]any{"1.31": map[string]any{"patch": 5}},
		"nodeGroup":         map[string]any{"name": "master"},
		"normal":            map[string]any{"moduleSourcesCA": map[string]any{}},
		"deckhouseImageRef": "registry.example.com/deckhouse/ee:v1",
		"registry":          registry,
		"images": map[string]any{
			"registry": map[string]any{
				"registryAgent":      "sha256:agent",
				"registryProxy":      "sha256:proxy",
				"dockerDistribution": "sha256:distribution",
				"dockerAuth":         "sha256:auth",
				"syncer":             "sha256:syncer",
			},
			"nodeManager": map[string]any{"kubernetesApiProxy": "sha256:api-proxy"},
			"registrypackages": map[string]any{
				"pause": "sha256:pause", "kubernetesApiProxy": "sha256:api-proxy",
				"registryProxy": "sha256:proxy", "registryAgent": "sha256:agent",
				"cfssl165": "sha256:cfssl", "d8": "sha256:d8", "yq4471": "x",
				"d8Curl891": "x", "e2fsprogs1472": "x", "iptables189": "x", "socat1734": "x",
				"jq171": "x", "nfsMount282": "x", "lsblk2402": "x", "which223": "x",
				"growpart033": "x", "virtWhat125": "x", "containerd1734": "x",
				"kubernetesCni162": "x", "kubelet1315": "x", "crictl131": "x",
				"tomlMerge01": "x", "d8CaUpdater200225": "x",
			},
		},
	}
}

// legacyRegistry is a cluster the legacy implementation manages: it names hosts, and the
// bashible step writes a drop-in directory per host.
func legacyRegistry() map[string]any {
	return map[string]any{
		"registryModuleEnable": true,
		"mode":                 "Proxy",
		"version":              "legacy-1",
		"imagesBase":           registry_const.HostWithPath,
		"proxyEndpoints":       []any{"10.0.0.1:5001"},
		"hosts": map[string]any{
			registry_const.Host: map[string]any{
				"mirrors": []any{map[string]any{
					"scheme": "https", "host": "10.0.0.1:5001", "ca": "CA",
					"auth":     map[string]any{"username": "ro", "password": "p", "auth": ""},
					"rewrites": []any{},
				}},
			},
		},
		"bootstrap": map[string]any{
			"init": map[string]any{
				"ca":      map[string]any{"cert": "CA", "key": "KEY"},
				"ro_user": map[string]any{"name": "ro", "password": "p", "password_hash": "h"},
				"rw_user": map[string]any{"name": "rw", "password": "p", "password_hash": "h"},
			},
			"proxy": map[string]any{"ca": ""},
		},
	}
}

// localRegistry is the legacy implementation in its Local mode, which is the only one
// some of the bootstrap steps act in.
func localRegistry() map[string]any {
	registry := legacyRegistry()
	registry["mode"] = "Local"
	return registry
}

// agentRegistry is the same cluster once the controller-based implementation owns it.
func agentRegistry() map[string]any {
	return map[string]any{
		"registryModuleEnable": true,
		"mode":                 "Managed",
		"version":              "v2-1",
		"imagesBase":           registry_const.HostWithPath,
		"proxyEndpoints":       []any{},
		"hosts": map[string]any{
			registry_const.Host: map[string]any{
				"mirrors": []any{map[string]any{
					"scheme": "https", "host": registry_const.Host, "ca": "",
					"auth":     map[string]any{"username": "", "password": "", "auth": ""},
					"rewrites": []any{},
				}},
			},
		},
		"agent": map[string]any{
			"endpoint":   registry_const.ProxyHost,
			"dropInFile": registry_const.AgentDropInFile,
			"layout":     `{"backends":[{"name":"Upstream","host":"registry.deckhouse.io"}]}`,
		},
	}
}

func render(t *testing.T, step string, registry map[string]any) string {
	t.Helper()

	path := filepath.Join(candi, step)
	content, err := os.ReadFile(path)
	require.NoError(t, err, "cannot read %s", path)

	rendered, err := template.RenderTemplate(filepath.Base(step), content, nodeContext(t, registry))
	require.NoError(t, err, "cannot render %s", step)

	return rendered.Content.String()
}

// effective strips comments and blank lines, leaving what the step actually does.
//
// Needed because a fully gated-out step still renders its license header and its
// explanatory comments, and "renders nothing" has to mean "does nothing" rather than
// "produces no bytes".
func effective(rendered string) string {
	var kept []string
	for _, line := range strings.Split(rendered, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, "\n")
}

// TestOnlyOneWriterConfiguresTheRuntime is the invariant itself, from both sides.
func TestOnlyOneWriterConfiguresTheRuntime(t *testing.T) {
	const step = "all/030_configure_containerd_registry.sh.tpl"

	t.Run("without an agent the bashible step writes the drop-ins", func(t *testing.T) {
		body := render(t, step, legacyRegistry())

		require.Contains(t, body, "mkdir -p \"/etc/containerd/registry.d/"+registry_const.Host+"\"")
		require.Contains(t, body, "hosts_state_file=")
	})

	t.Run("with an agent the bashible step does nothing at all", func(t *testing.T) {
		body := effective(render(t, step, agentRegistry()))

		require.Empty(t, body,
			"step 030 still acts while the agent owns the directory; an explicit host "+
				"directory takes precedence over the agent's _default and would route "+
				"pulls around it")
	})
}

// TestTheAgentStepFollowsTheSameDecision is the other half: the step that installs the
// agent must be silent exactly when step 030 is not, and must clean up after itself.
func TestTheAgentStepFollowsTheSameDecision(t *testing.T) {
	const step = "all/053_configure_registry_agent.sh.tpl"

	t.Run("with an agent it installs the static pod", func(t *testing.T) {
		body := render(t, step, agentRegistry())

		require.Contains(t, body, "/etc/kubernetes/manifests/registry-agent.yaml")
		require.Contains(t, body, "--listen-address="+registry_const.ProxyHost)
		require.Contains(t, body, "--containerd-registry-dir=${drop_in_root}")
		// The routing a node uses before it has ever reached the API server, without
		// which a node joining a new cluster cannot pull the control plane.
		require.Contains(t, body, "bootstrap-layout.json")
		require.Contains(t, body, `"backends"`)
	})

	t.Run("without an agent it removes what it left behind", func(t *testing.T) {
		body := effective(render(t, step, legacyRegistry()))

		require.Contains(t, body, "rm -f /etc/kubernetes/manifests/registry-agent.yaml")
		require.NotContains(t, body, "bb-sync-file /etc/kubernetes/manifests/registry-agent.yaml",
			"the step both installs and removes the agent in the same run")
	})
}

// TestTheDropInPathIsTakenFromTheContext keeps the step and the agent from disagreeing
// about which directory is being handed over. The path is a contract between three
// parties — the agent writes it, this step waits for it, containerd reads it — and each
// spelling it out separately is how they drift.
func TestTheDropInPathIsTakenFromTheContext(t *testing.T) {
	for _, step := range []string{
		"all/053_configure_registry_agent.sh.tpl",
		"all/035_prefetch_node_images.sh.tpl",
	} {
		t.Run(step, func(t *testing.T) {
			body := render(t, step, agentRegistry())
			require.Contains(t, body, registry_const.AgentDropInFile)
		})
	}
}

// TestThePrefetchWaitsForTheAgent covers an ordering that cannot be fixed by renumbering.
//
// Every reference the prefetch pulls names the in-cluster registry address, which under
// the agent is reachable only through the agent — and the agent is a static pod, so it
// does not start until the kubelet does, long after this step runs. Without the wait the
// prefetch fails on every master that joins an existing cluster.
func TestThePrefetchWaitsForTheAgent(t *testing.T) {
	const step = "all/035_prefetch_node_images.sh.tpl"

	withAgent := render(t, step, agentRegistry())
	require.Contains(t, withAgent, "agent_hosts_file=")
	require.Contains(t, withAgent, "the node agent did not configure the runtime")

	withoutAgent := render(t, step, legacyRegistry())
	require.NotContains(t, withoutAgent, "agent_hosts_file=",
		"the wait is for the agent only; without one there is nothing to wait for")
}

// TestTheAgentImageIsImportedLocally is what makes the agent startable at all: it is on
// the pull path of every image, so it cannot itself be pulled from a registry.
func TestTheAgentImageIsImportedLocally(t *testing.T) {
	const step = "all/034_ctr_import_local_images.sh.tpl"

	withAgent := render(t, step, agentRegistry())
	require.Contains(t, withAgent, `ctr_import_image deckhouse.local/images:registry-agent "/opt/deckhouse/images/registry-agent.tar"`)
	require.Contains(t, withAgent, `bb-package-install "registry-agent:sha256:agent"`)

	withoutAgent := render(t, step, legacyRegistry())
	require.NotContains(t, withoutAgent, `bb-package-install "registry-agent:`,
		"every existing cluster would fetch a package it has no use for")
}

// TestTheLegacyBootstrapPathStaysInert is the convergence check with cluster bootstrap.
//
// The legacy implementation brings up a temporary registry on the first master — the
// igniter — and those steps are gated on its own mode names. `Unmanaged` appears in both
// vocabularies, so the inertness below is not as self-evident as it looks, and it is worth
// pinning: a step of the old bootstrap path firing under the new implementation would put
// a second registry on the node the agent is meant to be the only route to.
func TestTheLegacyBootstrapPathStaysInert(t *testing.T) {
	// Each step is checked against the legacy mode it actually acts in, so that "inert
	// under the agent" is a fact about the gate rather than about a fixture that happened
	// to miss it. Filling in the wrong mode here is how this test would quietly stop
	// testing anything.
	steps := map[string]map[string]any{
		"cluster-bootstrap/020_install_registry_igniter.sh.tpl": legacyRegistry(),
		"cluster-bootstrap/021_fill_registry_igniter.sh.tpl":    localRegistry(),
		"cluster-bootstrap/070_install_registry.sh.tpl":         legacyRegistry(),
		"cluster-bootstrap/073_init_registry_secrets.sh.tpl":    legacyRegistry(),
	}

	for step, legacy := range steps {
		t.Run(step, func(t *testing.T) {
			require.NotEmpty(t, effective(render(t, step, legacy)),
				"this step does nothing even in the legacy mode it belongs to, so the "+
					"assertion below would pass on a typo")

			require.Empty(t, effective(render(t, step, agentRegistry())))
		})
	}
}

// TestTheAgentResolvesThroughTheNode pins the one line that decides whether a node can
// join the cluster at all.
//
// The agent is a static pod so that a node can pull images when the cluster cannot help
// it, and the names it resolves are upstream registries outside the cluster. Pointed at
// cluster DNS it depends on the very thing it exists to outlive, and on a joining node that
// dependency is circular: kube-dns needs its image, the image needs this agent to resolve
// the upstream, and the agent needs kube-dns. The node stays NotReady and is replaced
// forever — which is exactly what a test cluster did, while every unit test passed.
func TestTheAgentResolvesThroughTheNode(t *testing.T) {
	body := render(t, "all/053_configure_registry_agent.sh.tpl", agentRegistry())

	require.Contains(t, body, "dnsPolicy: Default",
		"the agent must resolve through the node's resolver")
	require.NotContains(t, body, "ClusterFirstWithHostNet",
		"cluster DNS makes the agent depend on the cluster it has to work without")
	// Host networking is what makes the node's resolver the right one to inherit, so the
	// two belong together.
	require.Contains(t, body, "hostNetwork: true")
}

// TestTheAgentAlwaysGetsTheKubeconfigMount pins the mount whose absence is permanent.
//
// The agent reads its layout with the kubelet's own credentials. Mounted only once
// /etc/kubernetes/kubelet.conf exists — which is what this step did — the question is decided
// at the one moment the answer is "not yet", because the manifest is written before the node
// finishes its TLS bootstrap. It is never revisited: bashible skips a bundle it has already
// applied, so the step does not run again, and that node's agent stays without API access for
// the life of the node. It pulls from the layout it was installed with and reports success, so
// nothing looks wrong; whether it happened at all depended on which came first, this step or
// the kubeconfig.
//
// The directory is mounted instead, unconditionally. That grants nothing new — the agent is
// root in the host's namespaces on that node — and the agent re-reads the path on every
// connection attempt, so the credentials are found whenever they land.
func TestTheAgentAlwaysGetsTheKubeconfigMount(t *testing.T) {
	body := render(t, "all/053_configure_registry_agent.sh.tpl", agentRegistry())

	require.Contains(t, body, "mountPath: /etc/kubernetes\n",
		"the agent must be able to read the kubeconfig whenever it appears")
	require.Contains(t, body, "path: /etc/kubernetes\n")
	require.NotContains(t, body, `if [[ -f /etc/kubernetes/kubelet.conf ]]`,
		"a conditional mount is decided once, at the moment the answer is still no")
	// FileOrCreate would have the kubelet create an empty kubelet.conf, and step 061 skips
	// generating the bootstrap kubeconfig when that file exists.
	require.NotContains(t, body, "type: FileOrCreate")
}

// bundleRegistry is an installation whose images come from a bundle: on the node it looks exactly like
// the legacy Local mode, because it borrows that mode's bootstrap steps to stand a registry up on the
// first master and fill it from the bundle. The one thing that tells them apart is the flag below.
func bundleRegistry() map[string]any {
	registry := localRegistry()
	bootstrap := registry["bootstrap"].(map[string]any)
	bootstrap["fromBundle"] = true
	return registry
}

// How step 073 behaves on the bundle path is pinned in init_secret_test.go. Closing the step
// altogether — which is what this file used to assert — kept the legacy state machine shut and took
// the registry PKI down with it, stranding the installation with "get PKI: secrets \"registry-init\"
// not found" after the store was already full. The secret is uploaded on every path now, and arrives
// pre-annotated as consumed when the images came from a bundle.

// TestTheBundleInstallStillFillsTheStore is the other side of the gate above.
//
// The bundle path needs the borrowed steps: the igniter that serves the bundle through the tunnel, the
// copy into the node's store, and the registry that then holds it. Gating them out along with the init
// secret would leave a cluster with no images at all — and, since the cluster is air-gapped by
// definition here, nowhere to get them.
func TestTheBundleInstallStillFillsTheStore(t *testing.T) {
	steps := []string{
		"cluster-bootstrap/020_install_registry_igniter.sh.tpl",
		"cluster-bootstrap/021_fill_registry_igniter.sh.tpl",
		"cluster-bootstrap/070_install_registry.sh.tpl",
	}

	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			require.NotEmpty(t, effective(render(t, step, bundleRegistry())))
		})
	}
}

// TestTheAgentAuthorityStaysReadable pins the one file under /etc/kubernetes that has a reader
// outside root, against the step that sweeps that whole tree.
//
// registry-packages-proxy verifies the agent with `ca.crt` and runs unprivileged in a pod mounting
// that directory from the host. Step 053 sets the permissions for exactly this reason — and step
// 098 used to undo them on every pass, `find /etc/kubernetes -type d -exec chmod 700`, eight seconds
// later. What that cost is not an error anywhere: an unreadable authority reads to the proxy as
// "there is no agent on this node", so it silently fetched from elsewhere, and on a cache-less
// cluster "elsewhere" is nothing at all once its own cache misses.
//
// Both halves are asserted here because either alone can pass while the pair is broken: the step
// that grants the permissions, and the step that must not take them away.
func TestTheAgentAuthorityStaysReadable(t *testing.T) {
	granting := effective(render(t, "all/053_configure_registry_agent.sh.tpl", agentRegistry()))
	require.Contains(t, granting, `chmod 0755 "${agent_path}" "${pki_path}"`,
		"step 053 no longer opens the agent directory, so nothing unprivileged can reach the authority")
	require.Contains(t, granting, `chmod 0644 "${pki_path}/ca.crt"`,
		"step 053 no longer opens the authority itself")
	require.Contains(t, granting, `install -m 0600 "${staging}/agent-key.pem"`,
		"the private key must stay closed; only the authority is published")

	sweeping := effective(render(t, "all/098_set_permissions.sh.tpl", agentRegistry()))
	for _, line := range strings.Split(sweeping, "\n") {
		if !strings.HasPrefix(line, "find /etc/kubernetes") {
			continue
		}
		require.Contains(t, line, "-path /etc/kubernetes/registry-agent -prune",
			"step 098 sweeps the agent directory again: %s", line)
	}
}

// TestTheAgentManifestNamesTheImageDigest is what makes a new agent actually start on a node that
// already runs one.
//
// For containerd the image reference in the static pod is a fixed local tag: the agent cannot be pulled
// through the agent, so it is imported from a tar as `deckhouse.local/images:registry-agent`. That tag
// is the same in every build, so the manifest was byte-identical from one build to the next — a new
// package was installed, a new tar imported, and kubelet went on running the container it already had.
//
// Measured on `ly-direct`: the node's bashible configuration named the new agent digest while the
// running agent was the previous build, and restarting the platform, the bashible apiserver and the
// container itself changed nothing. A fix shipped in the agent could not reach an existing cluster.
//
// The digest in an annotation is what makes the file differ, so `bb-sync-file` rewrites it and kubelet
// restarts the pod onto the imported image.
func TestTheAgentManifestNamesTheImageDigest(t *testing.T) {
	const step = "all/053_configure_registry_agent.sh.tpl"

	body := render(t, step, agentRegistry())

	require.Contains(t, body, `registry.deckhouse.io/agent-image-digest: "sha256:agent"`,
		"the manifest has to change when the image does, or kubelet keeps the old container")

	// The image reference itself stays the local tag: pulling the agent through the agent is the
	// circle this whole arrangement avoids.
	require.Contains(t, body, "image: deckhouse.local/images:registry-agent")

	// And the annotation must sit in the file kubelet watches, not somewhere else in the step.
	manifest := body[strings.Index(body, "bb-sync-file /etc/kubernetes/manifests/registry-agent.yaml"):]
	require.Contains(t, manifest[:2000], "agent-image-digest",
		"the digest belongs in the static pod manifest, which is the file that triggers a restart")
}

// TestTheLegacyCleanupOnlyTouchesWhatIsDead pins the step that removes the previous
// implementation's files from a node.
//
// Written as a test about paths rather than about behaviour, because the risk here is not that the
// cleanup fails to run — it is that it runs and takes something with it. It executes as root on every
// node, and `registry-agent` and `registry-proxy` sit next to the two directories it is meant to
// remove: the first is this implementation's own PKI, the second is what a bundle installation
// bootstraps through. A glob or a loop over a wildcard would have reached both.
func TestTheLegacyCleanupOnlyTouchesWhatIsDead(t *testing.T) {
	const step = "all/054_cleanup_legacy_registry.sh.tpl"

	t.Run("with an agent it removes the previous implementation's directories", func(t *testing.T) {
		body := effective(render(t, step, agentRegistry()))

		require.Contains(t, body, "rm -rf /etc/kubernetes/registry/auth")
		require.Contains(t, body, "rm -rf /etc/kubernetes/registry/distribution")

		// Every removal names its path in full. A wildcard in this step is the one way it could
		// take the agent's own material or the bundle bootstrap's image with it.
		for _, forbidden := range []string{
			"rm -rf /etc/kubernetes/registry/*",
			"rm -rf /etc/kubernetes/registry ",
			"registry-agent",
			"registry-proxy",
			"local_data",
		} {
			require.NotContains(t, body, forbidden,
				"the cleanup must not go anywhere near %s", forbidden)
		}
	})

	// Silent while the previous implementation still owns the node: its files are in use then, and
	// this step has no business deciding the handover has happened.
	t.Run("without an agent it does nothing", func(t *testing.T) {
		body := effective(render(t, step, legacyRegistry()))

		require.NotContains(t, body, "rm -rf")
		require.NotContains(t, body, "rmdir")
	})
}
