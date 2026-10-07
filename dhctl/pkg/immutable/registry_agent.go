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

package immutable

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const (
	// registryAgentStaticPodName is the name node-controller recognises the agent by
	// (registryAgentStaticPodName in modules/040-node-manager/.../nodeconfig/constants.go).
	// It hands registry.d to the agent only on a node whose config carries this entry,
	// so the node keeps the agent when node-controller takes its document over.
	registryAgentStaticPodName = "registry-agent"

	registryOwnerAgent = "agent"

	// registryDigestsKey is the registry module in images_digests.json, and
	// registryAgentImage its node agent there.
	registryDigestsKey = "registry"
	registryAgentImage = "registryAgent"

	// registryAgentDigestPlaceholder stands in registry-agent.yaml for the agent image's
	// digest, which the module's template renders in the same place. See applyRegistryAgent.
	registryAgentDigestPlaceholder = "__REGISTRY_AGENT_IMAGE_DIGEST__"
)

// registryAgentManifestFile is the manifest of the registry module's NodeStaticPodRequest
// (modules/038-registry/templates/v2/node-static-pod.yaml), rendered for an Engine
// node. The two are kept equal by a template test of that module.
//
//go:embed registry-agent.yaml
var registryAgentManifestFile string

// registryAgentManifest is the file without its final newline, which the request's
// manifest does not have either: the template's closing tag trims it.
var registryAgentManifest = strings.TrimRight(registryAgentManifestFile, "\n")

// applyRegistryAgent puts the node agent on a node that is installed with it.
//
// On a bashible node the agent comes from a bootstrap step, before anything pulls
// through it: the `deckhouse-registry` secret names the in-cluster address, and that
// address is served by the agent on the node. An Engine node runs no such step, and the
// module's NodeStaticPodRequest cannot be the way the agent arrives on the first master:
// the module is deployed by the Deckhouse whose image the agent has to serve. So the
// agent comes in the node's own document, in the same order as on bashible.
//
// Its image is local — the containerd extension imports it — and it routes by the
// upstream in spec.registry until the API server answers, so it needs nothing from the
// cluster to start. With it, registry.d is the agent's: nodelet stops writing it, and the
// agent's _default host carries every pull, the control plane's included.
//
// The manifest carries the agent image's digest, as the module's own request does: the image
// is the same local tag in every release, so the digest is the one thing that makes the
// manifest change when the agent does — and with it, kubelet start the new one.
func applyRegistryAgent(spec *nodeSpec, metaConfig *config.MetaConfig) error {
	if !metaConfig.Registry.AgentOwnsRuntime {
		return nil
	}

	images, err := digestGroup(metaConfig.Images.ConvertToMap(), registryDigestsKey)
	if err != nil {
		return err
	}
	digest := images[registryAgentImage]
	if digest == "" {
		return fmt.Errorf("the installer image carries no %q digest for the registry module", registryAgentImage)
	}

	spec.StaticPods = append(spec.StaticPods, staticPod{
		Name:     registryAgentStaticPodName,
		Manifest: strings.Replace(registryAgentManifest, registryAgentDigestPlaceholder, digest, 1),
	})
	spec.ContainerRuntime.RegistryOwner = registryOwnerAgent
	return nil
}
