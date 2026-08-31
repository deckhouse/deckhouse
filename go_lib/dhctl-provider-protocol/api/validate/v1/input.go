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

package v1

import (
	"encoding/json"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
)

// CredentialsSecretType marks a provider credential Secret, both in the
// cluster and in the Secrets map below.
const CredentialsSecretType = "cloud-provider.deckhouse.io/credentials"

type Operation string

const (
	OperationBootstrap Operation = "bootstrap"
	OperationConverge  Operation = "converge"
	OperationDestroy   Operation = "destroy"
)

// Input is everything a validator needs to check a cluster's configuration before
// the caller touches infrastructure.
type Input struct {
	ProviderName          string             `json:"providerName"`
	ClusterPrefix         string             `json:"clusterPrefix,omitempty"`
	Layout                string             `json:"layout,omitempty"`
	Operation             Operation          `json:"operation,omitempty"`
	ProviderClusterConfig map[string]any     `json:"providerClusterConfiguration,omitempty"`
	CloudProviderVars     *CloudProviderVars `json:"vars,omitempty"`
}

// CloudProviderVars is the provider data the caller collected from the cluster and
// from the user's resources. Every map is name to the full resource object.
type CloudProviderVars struct {
	Settings        map[string]any            `json:"settings,omitempty"`
	NodeGroups      map[string]map[string]any `json:"nodeGroups,omitempty"`
	InstanceClasses map[string]map[string]any `json:"instanceClasses,omitempty"`
	Secrets         map[string]map[string]any `json:"secrets,omitempty"`
}

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

func (i Input) ToRequest() (*ValidateRequest, error) {
	ret, err := json.Marshal(i)
	if err != nil {
		return nil, fmt.Errorf("encode validate input: %w", err)
	}

	return &ValidateRequest{
		InputJson: ret,
	}, nil
}

func InputFromRequest(req *ValidateRequest) (Input, error) {
	var ret Input
	if err := json.Unmarshal(req.GetInputJson(), &ret); err != nil {
		return ret, fmt.Errorf("decode validate input: %w", err)
	}

	return ret, nil
}
