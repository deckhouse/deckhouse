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

// BuildMasterPayload renders the cloud-init the first master boots with: the
// node has no sshd and no bashible, so everything dhctl would otherwise upload
// has to be in here. Base64-encoded, as the "cloudConfig" tfvar contract expects.
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

// BuildJoinPayload renders the cloud-init an additional master boots with: the
// first master's payload minus the ControlPlaneConfig, plus the CA and token to
// reach an existing apiserver. control-plane-manager promotes the node later.
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

// buildCloudConfig wraps the payload documents into one #cloud-config. The node
// ignores write_files "encoding"/"permissions", and the provider's terraform
// appends its own block, so no top-level key it also emits may appear here.
func buildCloudConfig(nodeConfig *nodeConfig, controlPlaneConfig *controlPlaneConfig) (string, error) {
	nodeConfigYAML, err := yaml.Marshal(nodeConfig)
	if err != nil {
		return "", fmt.Errorf("marshal nodeConfig: %w", err)
	}

	files := []map[string]any{
		{"path": nodeConfigPath, "content": string(nodeConfigYAML)},
	}

	// Only the cluster-creating node gets one: the document is an order to
	// generate a cluster CA, and a joining master must never mint a second CA —
	// it gets its control plane from control-plane-manager after joining.
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
