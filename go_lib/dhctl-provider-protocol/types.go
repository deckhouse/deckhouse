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

// Package dhctlproviderprotocol defines the wire types and subprocess protocol
// used between dhctl and external provider binaries (dhctl-provider-<name>).
//
// An external binary receives a request on stdin and writes a response to stdout.
// All messages are newline-terminated JSON. See PROTOCOL.md for the full spec.
package dhctlproviderprotocol

// ProtocolVersion is the current version of this protocol.
// dhctl always sends this value in the version field of every request.
// A binary built with this package rejects requests whose version differs.
const ProtocolVersion = "1"

// OperationBootstrap is the operation value sent during cluster bootstrap.
const OperationBootstrap = "bootstrap"

// OperationConverge is the operation value sent during cluster converge.
const OperationConverge = "converge"

// OperationDestroy is the operation value sent during cluster destroy.
const OperationDestroy = "destroy"

// CredentialsSecretType is the Kubernetes Secret type that marks provider credentials.
const CredentialsSecretType = "cloud-provider.deckhouse.io/credentials"

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

// PrepareInput is the input payload shared by both validate and prepare calls.
type PrepareInput struct {
	// ProviderName is the cloud provider identifier (e.g. "dvp", "aws").
	ProviderName string `json:"providerName"`
	// ClusterPrefix is an optional prefix applied to cloud resource names.
	ClusterPrefix string `json:"clusterPrefix,omitempty"`
	// Layout is the provider layout name (e.g. "Standard").
	Layout string `json:"layout,omitempty"`
	// Operation is one of OperationBootstrap, OperationConverge, OperationDestroy.
	Operation string `json:"operation,omitempty"`
	// ProviderClusterConfig holds the parsed providerClusterConfiguration section.
	ProviderClusterConfig map[string]interface{} `json:"providerClusterConfiguration,omitempty"`
	// Vars is the structured provider data (node groups, instance classes,
	// credential secrets, module settings) collected by dhctl. Always
	// populated on both subcommands.
	Vars *CloudProviderVars `json:"vars,omitempty"`
	// ModuleConfig holds the cloud-provider module configuration values.
	ModuleConfig map[string]interface{} `json:"moduleConfig,omitempty"`
}

// PrepareResult is returned by the prepare subcommand on success.
type PrepareResult struct {
	// Vars is the structured provider variables to pass to Terraform/OpenTofu.
	Vars *CloudProviderVars `json:"vars,omitempty"`
	// ProviderClusterConfig is the (possibly mutated) providerClusterConfiguration.
	ProviderClusterConfig map[string]interface{} `json:"providerClusterConfiguration,omitempty"`
}

// ValidateRequest is the JSON object written to stdin for the validate subcommand.
type ValidateRequest struct {
	// Version must equal ProtocolVersion. The binary rejects mismatched versions.
	Version string       `json:"version"`
	Input   PrepareInput `json:"input"`
}

// ValidateResponse is the JSON object written to stdout after validate.
// A non-empty Error means validation failed; the binary exits 0 regardless.
type ValidateResponse struct {
	Error string `json:"error,omitempty"`
}

// PrepareRequest is the JSON object written to stdin for the prepare subcommand.
type PrepareRequest struct {
	// Version must equal ProtocolVersion. The binary rejects mismatched versions.
	Version string       `json:"version"`
	Input   PrepareInput `json:"input"`
}

// PrepareResponse is the JSON object written to stdout after prepare.
// A non-empty Error means preparation failed; the binary exits 0 regardless.
type PrepareResponse struct {
	Result *PrepareResult `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}
