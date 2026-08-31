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

package validate

import (
	"encoding/json"
	"fmt"

	protogen "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/gen"
	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
)

const (
	// CredentialsSecretType marks a provider credential Secret, both in the
	// cluster and in the Secrets map below.
	CredentialsSecretType = "cloud-provider.deckhouse.io/credentials"

	// The phase of the host's work a call belongs to. Strings, not an enum: the
	// host is the only producer and carries the value as a string end to end.
	OperationBootstrap = "bootstrap"
	OperationConverge  = "converge"
	OperationDestroy   = "destroy"
)

// Input is everything a plugin needs to check a cluster's configuration before the
// host touches infrastructure.
type Input struct {
	ProviderName          string         `json:"providerName"`
	ClusterPrefix         string         `json:"clusterPrefix,omitempty"`
	Layout                string         `json:"layout,omitempty"`
	Operation             string         `json:"operation,omitempty"`
	ProviderClusterConfig map[string]any `json:"providerClusterConfiguration,omitempty"`
	// Nil when the host had nothing to collect, so a plugin must check before use.
	CloudProviderVars *CloudProviderVars `json:"vars,omitempty"`
}

// CloudProviderVars is the provider data the host collected from the cluster and
// from the user's resources. Every map is name to the full resource object.
type CloudProviderVars struct {
	// Settings is the provider ModuleConfig.
	Settings        map[string]any            `json:"settings,omitempty"`
	NodeGroups      map[string]map[string]any `json:"nodeGroups,omitempty"`
	InstanceClasses map[string]map[string]any `json:"instanceClasses,omitempty"`
	Secrets         map[string]map[string]any `json:"secrets,omitempty"`
}

// Validate is called by the server before a plugin sees the input. An unknown
// operation is refused rather than passed through: a plugin decides what to check
// from this field, and one it does not recognise would silently get the checks of
// some other phase.
func (i Input) Validate() error {
	switch i.Operation {
	case OperationBootstrap, OperationConverge, OperationDestroy:
		return nil
	case "":
		return fmt.Errorf("%w: operation required", errs.ErrInvalidRequest)
	default:
		return fmt.Errorf("%w: operation unknown: %q", errs.ErrInvalidRequest, i.Operation)
	}
}

func ToPBRequest(input Input) (*protogen.ValidateRequest, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode validate input: %w", err)
	}
	return &protogen.ValidateRequest{
		InputJson: inputJSON,
	}, nil
}

func FromPBRequest(req *protogen.ValidateRequest) (Input, error) {
	var input Input
	if err := json.Unmarshal(req.GetInputJson(), &input); err != nil {
		return input, fmt.Errorf("decode validate input: %w", err)
	}
	return input, nil
}
