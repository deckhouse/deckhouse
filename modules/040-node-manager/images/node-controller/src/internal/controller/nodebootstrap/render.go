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

package nodebootstrap

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodeconfig"
)

// renderBootstrapData renders the cloud-config userdata a machine boots with: a
// full NodeConfig with the machine's name already filled in — no __NODE_NAME__
// placeholder — plus the bootstrap token kubelet presents on first contact.
func renderBootstrapData(ctx context.Context, cl client.Client, reader client.Reader, ng *v1.NodeGroup, machineName string) ([]byte, error) {
	spec, err := nodeconfig.RenderBootstrapSpec(ctx, cl, reader, ng, machineName)
	if err != nil {
		return nil, fmt.Errorf("render bootstrap spec: %w", err)
	}

	tokens, err := nodecommon.BootstrapTokens(ctx, reader)
	if err != nil {
		return nil, err
	}
	if tokens[ng.Name] == "" {
		return nil, fmt.Errorf("no valid bootstrap token for NodeGroup %s", ng.Name)
	}
	spec.Kubelet.BootstrapToken = tokens[ng.Name]

	return wrapCloudConfig(spec, machineName, ng.Name)
}

// bootstrapDocument is the shape written to /config/nodeconfig.yaml: the
// cluster object minus the status. Spelled out because omitempty does not drop
// the Status struct; dhctl uses a spec-only type for the same reason.
type bootstrapDocument struct {
	APIVersion string                    `json:"apiVersion"`
	Kind       string                    `json:"kind"`
	Metadata   bootstrapMetadata         `json:"metadata"`
	Spec       internalv1alpha1.NodeSpec `json:"spec"`
}

type bootstrapMetadata struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
}

func wrapCloudConfig(spec internalv1alpha1.NodeSpec, machineName, ngName string) ([]byte, error) {
	config := &bootstrapDocument{
		APIVersion: internalv1alpha1.GroupVersion.String(),
		Kind:       "NodeConfig",
		Metadata: bootstrapMetadata{
			Name:   machineName,
			Labels: map[string]string{nodecommon.NodeGroupLabel: ngName},
		},
		Spec: spec,
	}

	configYAML, err := sigsyaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal NodeConfig: %w", err)
	}

	cloudConfig := map[string]any{
		"write_files": []map[string]any{{
			"path":    nodeConfigPath,
			"content": string(configYAML),
		}},
	}
	body, err := sigsyaml.Marshal(cloudConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal cloud-config: %w", err)
	}

	return append([]byte("#cloud-config\n"), body...), nil
}
