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

// Package providerdata defines the shared data types and subprocess protocol
// used between dhctl and external provider binaries.
//
// An external preparator binary receives a PrepareRequest on stdin and writes
// a PrepareResponse (or ValidateResponse) to stdout. All messages are
// newline-terminated JSON.
package providerdata

const OperationBootstrap = "bootstrap"

type CloudProviderVars struct {
	Settings        map[string]interface{}            `json:"settings,omitempty"`
	NodeGroups      map[string]map[string]interface{} `json:"nodeGroups,omitempty"`
	InstanceClasses map[string]map[string]interface{} `json:"instanceClasses,omitempty"`
	Secrets         map[string]map[string]interface{} `json:"secrets,omitempty"`
}

type PrepareInput struct {
	ProviderName          string                 `json:"providerName"`
	ClusterPrefix         string                 `json:"clusterPrefix,omitempty"`
	Layout                string                 `json:"layout,omitempty"`
	Operation             string                 `json:"operation,omitempty"`
	ProviderClusterConfig map[string]interface{} `json:"providerClusterConfiguration,omitempty"`
	ResourcesYAML         string                 `json:"resourcesYAML,omitempty"`
	ModuleConfig          map[string]interface{} `json:"moduleConfig,omitempty"`
}

type PrepareResult struct {
	Vars                  *CloudProviderVars     `json:"vars,omitempty"`
	ProviderClusterConfig map[string]interface{} `json:"providerClusterConfiguration,omitempty"`
}

// ValidateRequest is sent to the external binary for the "validate" subcommand.
type ValidateRequest struct {
	Input PrepareInput `json:"input"`
}

// ValidateResponse is returned by the external binary after "validate".
type ValidateResponse struct {
	Error string `json:"error,omitempty"`
}

// PrepareRequest is sent to the external binary for the "prepare" subcommand.
type PrepareRequest struct {
	Input PrepareInput `json:"input"`
}

// PrepareResponse is returned by the external binary after "prepare".
type PrepareResponse struct {
	Result *PrepareResult `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}
