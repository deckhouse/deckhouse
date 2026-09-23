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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

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

// ValidateCore checks the part of the registration every cloud provider publishes, regardless
// of whether it currently uses CAPI, MCM, or both.
//
// Zones are deliberately not required: OpenStack in hybrid mode publishes an empty list until the
// cloud-data discovery hook has run. An empty list is a per-NodeGroup condition handled where it
// matters, not a reason to reject the whole provider.
func (r Registration) ValidateCore() error {
	missing := make([]string, 0, 6)
	if strings.TrimSpace(r.Type) == "" {
		missing = append(missing, "type")
	}
	if strings.TrimSpace(r.Region) == "" {
		missing = append(missing, "region")
	}
	if strings.TrimSpace(r.InstanceClassKind) == "" {
		missing = append(missing, "instanceClassKind")
	}
	if strings.TrimSpace(r.InstanceClassAPIVersion) == "" {
		missing = append(missing, common.InstanceClassAPIVersionKey)
	}
	if strings.TrimSpace(r.Type) != "" && r.CloudVariables == nil {
		missing = append(missing, "provider subtree "+strings.ToLower(r.Type))
	}
	for _, zone := range r.Zones {
		if strings.TrimSpace(zone) == "" {
			missing = append(missing, "zones")
			break
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("cloud-provider registration has empty required fields: %s", strings.Join(missing, ", "))
	}
	if r.Type != strings.ToLower(r.Type) {
		return fmt.Errorf("cloud-provider registration type %q must be lowercase", r.Type)
	}
	if r.hasAnyCAPIField() {
		if err := r.ValidateCAPI(); err != nil {
			return err
		}
	}
	if !r.HasCAPI() && strings.TrimSpace(r.MachineClassKind) == "" {
		return fmt.Errorf("cloud-provider registration enables neither CAPI nor MCM")
	}
	return nil
}

func (r Registration) hasAnyCAPIField() bool {
	return r.CAPIClusterName != "" || r.CAPIClusterKind != "" || r.CAPIClusterAPIVersion != "" ||
		r.CAPIMachineTemplateKind != "" || r.CAPIMachineTemplateAPIVersion != ""
}

// ValidateCAPI checks the complete CAPI group. Publishing only part of it is never useful: both
// reconcilers need the cluster identity, and the machine reconciler additionally needs the
// provider MachineTemplate GVK.
func (r Registration) ValidateCAPI() error {
	missing := make([]string, 0, 5)
	fields := []struct {
		name  string
		value string
	}{
		{name: "capiClusterName", value: r.CAPIClusterName},
		{name: "capiClusterKind", value: r.CAPIClusterKind},
		{name: "capiClusterAPIVersion", value: r.CAPIClusterAPIVersion},
		{name: "capiMachineTemplateKind", value: r.CAPIMachineTemplateKind},
		{name: "capiMachineTemplateAPIVersion", value: r.CAPIMachineTemplateAPIVersion},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("cloud-provider CAPI registration has empty required fields: %s", strings.Join(missing, ", "))
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "capiClusterAPIVersion", value: r.CAPIClusterAPIVersion},
		{name: "capiMachineTemplateAPIVersion", value: r.CAPIMachineTemplateAPIVersion},
	} {
		gv, err := schema.ParseGroupVersion(field.value)
		if err != nil || gv.Group == "" || gv.Version == "" {
			return fmt.Errorf("cloud-provider CAPI registration has invalid %s %q", field.name, field.value)
		}
	}
	return nil
}

func (r Registration) ValidateMCM() error {
	if strings.TrimSpace(r.MachineClassKind) == "" {
		return fmt.Errorf("cloud-provider MCM registration has empty machineClassKind")
	}
	return nil
}

// DecodeRegistration accepts both raw strings and JSON values used by provider Helm templates.
// Semantic validation is performed by the loader for the path that consumes the registration;
// malformed JSON is rejected here instead of being silently converted to an empty value.
func DecodeRegistration(data map[string][]byte) (Registration, error) {
	zones, err := decodeStringSlice(data["zones"])
	if err != nil {
		return Registration{}, fmt.Errorf("decode cloud-provider registration zones: %w", err)
	}
	registration := Registration{
		Type:                           decodeString(data["type"]),
		Region:                         decodeString(data["region"]),
		Zones:                          zones,
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

	if registration.Type != "" {
		raw, ok := data[strings.ToLower(registration.Type)]
		if ok && len(raw) > 0 {
			if err := json.Unmarshal(raw, &registration.CloudVariables); err != nil {
				return Registration{}, fmt.Errorf("decode cloud-provider registration %s subtree: %w", strings.ToLower(registration.Type), err)
			}
		}
		if ok && len(raw) > 0 && registration.CloudVariables == nil {
			return Registration{}, fmt.Errorf("cloud-provider registration %s subtree is null", strings.ToLower(registration.Type))
		}
	}

	return registration, nil
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

func decodeStringSlice(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}
