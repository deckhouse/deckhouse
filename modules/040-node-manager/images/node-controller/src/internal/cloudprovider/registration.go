/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cloudprovider loads and validates the data published by cloud-provider modules.
package cloudprovider

import (
	"encoding/json"
	"strings"

	"github.com/deckhouse/node-controller/internal/common"
)

// Registration is the typed view of kube-system/d8-node-manager-cloud-provider.
type Registration struct {
	Type   string
	Region string
	Zones  []string

	InstanceClassKind       string
	InstanceClassAPIVersion string
	MachineClassKind        string
	SSHPublicKey            string

	CAPIClusterName                string
	CAPIClusterKind                string
	CAPIClusterAPIVersion          string
	CAPIMachineTemplateKind        string
	CAPIMachineTemplateAPIVersion  string
	CAPIMachineDeploymentSpecPatch string

	// CloudVariables is the provider-owned subtree exposed to templates as .provider.
	CloudVariables map[string]any
}

// HasCAPI reports whether the provider registered a CAPI infrastructure cluster.
func (r Registration) HasCAPI() bool {
	return r.CAPIClusterKind != ""
}

// DecodeRegistration accepts both raw strings and JSON values used by provider Helm templates.
// Validation is performed by the loader for the path that consumes the registration.
func DecodeRegistration(data map[string][]byte) Registration {
	registration := Registration{
		Type:                           decodeString(data["type"]),
		Region:                         decodeString(data["region"]),
		Zones:                          decodeStringSlice(data["zones"]),
		InstanceClassKind:              decodeString(data[common.InstanceClassKindKey]),
		InstanceClassAPIVersion:        decodeString(data[common.InstanceClassAPIVersionKey]),
		MachineClassKind:               decodeString(data["machineClassKind"]),
		SSHPublicKey:                   decodeString(data["sshPublicKey"]),
		CAPIClusterName:                decodeString(data["capiClusterName"]),
		CAPIClusterKind:                decodeString(data["capiClusterKind"]),
		CAPIClusterAPIVersion:          decodeString(data["capiClusterAPIVersion"]),
		CAPIMachineTemplateKind:        decodeString(data["capiMachineTemplateKind"]),
		CAPIMachineTemplateAPIVersion:  decodeString(data["capiMachineTemplateAPIVersion"]),
		CAPIMachineDeploymentSpecPatch: decodeString(data["capiMachineDeploymentSpecPatch"]),
	}

	if raw, ok := data[strings.ToLower(registration.Type)]; ok {
		_ = json.Unmarshal(raw, &registration.CloudVariables)
	}
	if registration.CloudVariables == nil {
		registration.CloudVariables = map[string]any{}
	}

	return registration
}

func decodeSecretData(data map[string][]byte) map[string]any {
	result := make(map[string]any, len(data))
	for key, raw := range data {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			result[key] = string(raw)
			continue
		}
		result[key] = value
	}
	return result
}

func decodeString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return string(raw)
}

func decodeStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}
