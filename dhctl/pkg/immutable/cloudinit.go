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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
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
	// CandiDir is the installer's own candi directory, where the control-plane
	// templates come from.
	CandiDir string
	// GlobalOptions is what the operator's control-plane-manager settings are
	// read through: resolving them needs that module's schema, and the schema
	// store is built from these.
	GlobalOptions *options.GlobalOptions
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

// JoinPayloadInput is everything BuildJoinPayload needs. The three cluster
// facts come from the running cluster, not from the installer's own idea of it.
type JoinPayloadInput struct {
	// NodeName is the name this master registers under.
	NodeName string
	// MetaConfig is the parsed cluster configuration.
	MetaConfig *config.MetaConfig
	// CACert is the cluster CA, base64-encoded, as the cluster holds it.
	CACert string
	// BootstrapToken is the group's current bootstrap token.
	BootstrapToken string
	// APIServerEndpoints are the apiservers already serving the cluster.
	APIServerEndpoints []string
}

// BuildJoinPayload renders the cloud-init an additional master boots with.
//
// It is the first master's payload minus the ControlPlaneConfig, plus the CA
// and the token that let kubelet reach an apiserver somebody else is already
// running. The node joins as an ordinary member of the master group and
// control-plane-manager makes a control-plane node out of it afterwards —
// the same path a classic cluster's second and third masters take.
func BuildJoinPayload(ctx context.Context, in JoinPayloadInput) (string, error) {
	switch {
	case in.CACert == "":
		return "", errors.New("build join payload: cluster CA is empty")
	case in.BootstrapToken == "":
		return "", errors.New("build join payload: bootstrap token is empty")
	case len(in.APIServerEndpoints) == 0:
		return "", errors.New("build join payload: no apiserver endpoints")
	}

	nodeConfig, err := buildNodeConfig(ctx, nodeConfigInput{
		NodeName:   in.NodeName,
		MetaConfig: in.MetaConfig,
		Join: &joinInput{
			CACert:             in.CACert,
			BootstrapToken:     in.BootstrapToken,
			APIServerEndpoints: in.APIServerEndpoints,
		},
	})
	if err != nil {
		return "", fmt.Errorf("build node config: %w", err)
	}

	document, err := buildCloudConfig(nodeConfig, nil)
	if err != nil {
		return "", fmt.Errorf("build cloud config: %w", err)
	}

	return base64.StdEncoding.EncodeToString([]byte(document)), nil
}

// buildCloudConfig wraps the payload documents into the single #cloud-config
// document the VM boots with.
//
// The generator on the node accepts plain content only — the "encoding" and
// "permissions" keys of write_files are ignored — and the provider's terraform
// concatenates this document with a block of its own, so no top-level key it
// also emits (hostname, users, ssh_authorized_keys) may appear here: a
// duplicate key breaks the parsing of the whole user-data.
func buildCloudConfig(nodeConfig *nodeConfig, controlPlaneConfig *controlPlaneConfig) (string, error) {
	nodeConfigYAML, err := yaml.Marshal(nodeConfig)
	if err != nil {
		return "", fmt.Errorf("marshal nodeConfig: %w", err)
	}

	files := []map[string]any{
		{"path": nodeConfigPath, "content": string(nodeConfigYAML)},
	}

	// Only the node that creates the cluster gets one. A joining master must not:
	// the document is an order to generate a cluster CA and start an apiserver
	// against it, and a second CA in one cluster is not something the node could
	// recover from. It gets its control plane the way every other master does —
	// from control-plane-manager, after it has joined.
	if controlPlaneConfig != nil {
		controlPlaneYAML, err := yaml.Marshal(controlPlaneConfig)
		if err != nil {
			return "", fmt.Errorf("marshal controlPlaneConfig: %w", err)
		}
		files = append(files, map[string]any{
			"path": controlPlaneConfigPath, "content": string(controlPlaneYAML),
		})
	}

	cloudConfig := map[string]any{"write_files": files}

	body, err := yaml.Marshal(cloudConfig)
	if err != nil {
		return "", fmt.Errorf("marshal cloud-config: %w", err)
	}

	return "#cloud-config\n" + string(body), nil
}
