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

package derived_status

import (
	"encoding/json"
	"strings"
)

// CloudProviderRegistration is the registration Secret a cloud provider module publishes
// (modules/030-cloud-provider-*/templates/registration.yaml). Its keys are fixed by that template,
// so they are typed here; only CloudVariables stays open, because its shape is the provider's own.
type CloudProviderRegistration struct {
	// Type is the provider name in lower case: aws, yandex, vsphere, dvp...
	Type string

	MachineClassKind  string
	CAPIClusterKind   string
	InstanceClassKind string

	// InstanceClassAPIVersion stays data rather than a constant on purpose: an empty value means
	// the provider has not published it yet, and guessing a version is what this mechanism exists
	// to prevent — a guessed version goes through the provider's conversion webhook, changes the
	// spec, renames an immutable machine template and recreates every VM in the group.
	InstanceClassAPIVersion string

	Zones []string

	// CloudVariables is the provider's own values tree, keyed in the Secret by Type.
	CloudVariables map[string]any
}

// DecodeRegistration reads the Secret data. Values arrive in two encodings: helm writes plain
// strings for scalars (type: {{ b64enc "aws" }}) and JSON for structures (zones, the provider
// tree), so every field tries JSON first and falls back to the raw bytes.
func DecodeRegistration(data map[string][]byte) CloudProviderRegistration {
	reg := CloudProviderRegistration{
		Type:                    decodeString(data["type"]),
		MachineClassKind:        decodeString(data["machineClassKind"]),
		CAPIClusterKind:         decodeString(data["capiClusterKind"]),
		InstanceClassKind:       decodeString(data["instanceClassKind"]),
		InstanceClassAPIVersion: decodeString(data["instanceClassAPIVersion"]),
		Zones:                   decodeStringSlice(data["zones"]),
	}

	if raw, ok := data[strings.ToLower(reg.Type)]; ok {
		var tree map[string]any
		if err := json.Unmarshal(raw, &tree); err == nil {
			reg.CloudVariables = tree
		}
	}
	return reg
}

func decodeString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func decodeStringSlice(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
