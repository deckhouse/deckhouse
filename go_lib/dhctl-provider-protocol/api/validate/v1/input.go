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

// Input is the input payload for the validate call.
type Input struct {
	// ProviderName is the cloud provider identifier (e.g. "dvp", "aws").
	ProviderName string `json:"providerName"`
	// ClusterPrefix is an optional prefix applied to cloud resource names.
	ClusterPrefix string `json:"clusterPrefix,omitempty"`
	// Layout is the provider layout name (e.g. "Standard").
	Layout string `json:"layout,omitempty"`
	// Operation is one of OperationBootstrap, OperationConverge, OperationDestroy.
	Operation Operation `json:"operation,omitempty"`
	// ProviderClusterConfig holds the parsed providerClusterConfiguration section.
	ProviderClusterConfig map[string]interface{} `json:"providerClusterConfiguration,omitempty"`
	// CloudProviderVars is the structured provider data (node groups, instance
	// classes, credential secrets, module settings) collected by dhctl.
	CloudProviderVars *CloudProviderVars `json:"vars,omitempty"`
}

// CloudProviderVars holds the structured data extracted from provider resources
// and passed to the Terraform/OpenTofu configuration.
type CloudProviderVars struct {
	// Settings holds module-level provider settings (from ModuleConfig).
	Settings map[string]interface{} `json:"settings,omitempty"`
	// NodeGroups maps node group name to its full resource object.
	NodeGroups map[string]map[string]interface{} `json:"nodeGroups,omitempty"`
	// InstanceClasses maps instance class name to its full resource object.
	InstanceClasses map[string]map[string]interface{} `json:"instanceClasses,omitempty"`
	// Secrets maps secret name to its full resource object.
	Secrets map[string]map[string]interface{} `json:"secrets,omitempty"`
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
