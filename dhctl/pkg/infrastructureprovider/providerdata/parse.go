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

package providerdata

import (
	"context"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

const (
	CloudProviderCredentialsSecretType = proto.CredentialsSecretType

	cloudProviderModuleNamePrefix = "cloud-provider-"
)

// CloudProviderModuleName returns the ModuleConfig name for the given provider.
func CloudProviderModuleName(providerName string) string {
	return cloudProviderModuleNamePrefix + strings.ToLower(providerName)
}

// IsCloudPermanentNodeGroup reports whether obj is a CloudPermanent NodeGroup.
func IsCloudPermanentNodeGroup(obj map[string]interface{}) bool {
	nodeType, _, _ := unstructured.NestedString(obj, "spec", "nodeType")
	return nodeType == "CloudPermanent"
}

// CloudProviderVarsFromInput builds CloudProviderVars from a PrepareInput.
// This is the core parsing logic shared between the built-in preparators and
// the external binary.
func CloudProviderVarsFromInput(_ context.Context, input PrepareInput) (*CloudProviderVars, error) {
	cv, err := proto.ParseResourcesYAML(input.ResourcesYAML)
	if err != nil {
		return nil, err
	}

	cv.Settings = input.ModuleConfig

	return cv, nil
}
