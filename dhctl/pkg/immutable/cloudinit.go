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
	"fmt"

	"sigs.k8s.io/yaml"
)

const (
	// nodeConfigPath is where the on-node loader reads the node config from. It
	// selects the entry by the "nodeconfig.yml"/"nodeconfig.yaml" suffix of the
	// path, so the name matters and the directory does not.
	nodeConfigPath = "/config/nodeconfig.yaml"

	// controlPlaneConfigPath names the payload entry. The node does not receive
	// this file — the initramfs delivers only the node config — it reads
	// cloud-init itself and picks the entry out by this name. So the path is a
	// label, and the name is the contract.
	controlPlaneConfigPath = "/config/controlplane.yaml"
)

// BuildCloudConfig wraps both payload documents into the single #cloud-config
// document the VM boots with.
//
// The generator on the node accepts plain content only — the "encoding" and
// "permissions" keys of write_files are ignored — and the provider's terraform
// concatenates this document with a block of its own, so no top-level key it
// also emits (hostname, users, ssh_authorized_keys) may appear here: a
// duplicate key breaks the parsing of the whole user-data.
func BuildCloudConfig(nodeConfig *NodeConfig, controlPlaneConfig *ControlPlaneConfig) (string, error) {
	if nodeConfig == nil {
		return "", fmt.Errorf("build cloud config: node config is nil")
	}
	if controlPlaneConfig == nil {
		return "", fmt.Errorf("build cloud config: control-plane config is nil")
	}

	nodeConfigYAML, err := yaml.Marshal(nodeConfig)
	if err != nil {
		return "", fmt.Errorf("marshal NodeConfig: %w", err)
	}
	controlPlaneYAML, err := yaml.Marshal(controlPlaneConfig)
	if err != nil {
		return "", fmt.Errorf("marshal ControlPlaneConfig: %w", err)
	}

	cloudConfig := map[string]any{
		"write_files": []map[string]any{
			{"path": nodeConfigPath, "content": string(nodeConfigYAML)},
			{"path": controlPlaneConfigPath, "content": string(controlPlaneYAML)},
		},
	}

	body, err := yaml.Marshal(cloudConfig)
	if err != nil {
		return "", fmt.Errorf("marshal cloud-config: %w", err)
	}

	return "#cloud-config\n" + string(body), nil
}
