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
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/errs"
)

const (
	// CredentialsSecretType is the Kubernetes Secret type that marks provider
	// credentials. It identifies a credential Secret everywhere: in the resources a
	// user supplies, in the cluster, and in the Secrets map below.
	CredentialsSecretType = "cloud-provider.deckhouse.io/credentials"

	// The phase of the host's work a call belongs to. Plain strings rather than an
	// enum: the host is the only producer of the value and carries it as a string
	// end to end.
	OperationBootstrap = "bootstrap"
	OperationConverge  = "converge"
	OperationDestroy   = "destroy"
)

// Input is everything a plugin needs to check a cluster's configuration before the
// host touches infrastructure.
type Input struct {
	// ProviderName is the cloud provider identifier (e.g. "dvp", "aws").
	ProviderName string `json:"providerName"`
	// ClusterPrefix is an optional prefix applied to cloud resource names.
	ClusterPrefix string `json:"clusterPrefix,omitempty"`
	// Layout is the provider layout name (e.g. "Standard").
	Layout string `json:"layout,omitempty"`
	// Operation is one of OperationBootstrap, OperationConverge, OperationDestroy.
	Operation string `json:"operation,omitempty"`
	// ProviderClusterConfig holds the parsed providerClusterConfiguration section.
	ProviderClusterConfig map[string]any `json:"providerClusterConfiguration,omitempty"`
	// CloudProviderVars is the structured provider data (node groups, instance
	// classes, credential secrets, module settings) the host collected. Nil when
	// there was nothing to collect, so a plugin must check before use.
	CloudProviderVars *CloudProviderVars `json:"vars,omitempty"`
}

// CloudProviderVars holds the structured data extracted from provider resources
// and passed to the Terraform/OpenTofu configuration.
type CloudProviderVars struct {
	// Settings holds module-level provider settings (from ModuleConfig).
	Settings map[string]any `json:"settings,omitempty"`
	// NodeGroups maps node group name to its full resource object.
	NodeGroups map[string]map[string]any `json:"nodeGroups,omitempty"`
	// InstanceClasses maps instance class name to its full resource object.
	InstanceClasses map[string]map[string]any `json:"instanceClasses,omitempty"`
	// Secrets maps secret name to its full resource object.
	Secrets map[string]map[string]any `json:"secrets,omitempty"`
}

// Validate reports whether the input is well-formed. Run calls it before a plugin
// sees the input, so every transport rejects the same requests.
//
// An unknown operation is refused rather than passed through: a plugin decides what
// to check from this field, and one it does not recognise would silently get the
// checks of some other phase.
func (i Input) Validate() error {
	switch i.Operation {
	case OperationBootstrap, OperationConverge, OperationDestroy:
		return nil
	case "":
		return fmt.Errorf("%w: operation requared", errs.ErrInvalidRequest)
	default:
		return fmt.Errorf("%w: operation unknown: %q", errs.ErrInvalidRequest, i.Operation)
	}
}
