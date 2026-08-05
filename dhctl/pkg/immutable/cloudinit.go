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
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
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

// MasterPayloadInput is everything BuildMasterPayload needs.
type MasterPayloadInput struct {
	// NodeName is the name the first master registers under. It is also the name
	// the handoff endpoint's certificate is issued for.
	NodeName string
	// MetaConfig is the parsed cluster configuration.
	MetaConfig *config.MetaConfig
	// StateCache carries the handoff material between bootstrap attempts.
	StateCache state.Cache
}

// BuildMasterPayload renders the cloud-init the first master boots with. The
// node has no sshd and no bashible, so everything dhctl would otherwise upload
// afterwards has to be in here.
//
// The result is base64-encoded because that is what the "cloudConfig" tfvar
// carries: every provider's terraform base64decodes it before writing the
// cloud-init secret, and the only other producer of that variable (the cloud
// config secret read in kubernetes/actions/entity) encodes it too. The document
// underneath stays plain — it is pinned by a golden file.
func BuildMasterPayload(ctx context.Context, in MasterPayloadInput) (string, error) {
	nodeConfig, err := buildNodeConfig(ctx, nodeConfigInput{
		NodeName:   in.NodeName,
		MetaConfig: in.MetaConfig,
	})
	if err != nil {
		return "", fmt.Errorf("build node config: %w", err)
	}

	controlPlaneConfig, err := buildControlPlaneConfig(ctx, in)
	if err != nil {
		return "", fmt.Errorf("build control-plane config: %w", err)
	}

	document, err := buildCloudConfig(nodeConfig, controlPlaneConfig)
	if err != nil {
		return "", fmt.Errorf("build cloud config: %w", err)
	}

	return base64.StdEncoding.EncodeToString([]byte(document)), nil
}

// buildCloudConfig wraps both payload documents into the single #cloud-config
// document the VM boots with.
//
// The generator on the node accepts plain content only — the "encoding" and
// "permissions" keys of write_files are ignored — and the provider's terraform
// concatenates this document with a block of its own, so no top-level key it
// also emits (hostname, users, ssh_authorized_keys) may appear here: a
// duplicate key breaks the parsing of the whole user-data.
func buildCloudConfig(nodeConfig *nodeConfig, controlPlaneConfig *controlPlaneConfig) (string, error) {
	if nodeConfig == nil {
		return "", errors.New("build cloud config: node config is nil")
	}
	if controlPlaneConfig == nil {
		return "", errors.New("build cloud config: control-plane config is nil")
	}

	nodeConfigYAML, err := yaml.Marshal(nodeConfig)
	if err != nil {
		return "", fmt.Errorf("marshal nodeConfig: %w", err)
	}
	controlPlaneYAML, err := yaml.Marshal(controlPlaneConfig)
	if err != nil {
		return "", fmt.Errorf("marshal controlPlaneConfig: %w", err)
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
