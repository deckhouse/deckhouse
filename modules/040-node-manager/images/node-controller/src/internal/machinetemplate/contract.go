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

// Package machinetemplate implements the v2 CAPI machine-template contract: the single
// capi/template.yaml file a cloud-provider module ships, the sandbox its go-template is
// rendered in, and the rollout decision made from the InstanceClass values.
//
// The package is deliberately self-contained (it does not reuse the legacy machineclass
// renderer): that engine emulates a helm values tree and is deleted once the last provider
// migrates, and nothing here should have to be untangled from it on the way out.
package machinetemplate

import (
	"fmt"
	"slices"
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"
)

// ContractVersionV2 is the only version this package renders. The version is what selects the
// engine at runtime, so a future contract change ships as v3 next to v2 rather than as a silent
// behaviour change under the same name.
const ContractVersionV2 = "v2"

// Contract is the parsed capi/template.yaml of a cloud-provider module.
type Contract struct {
	Version string `json:"version"`

	// RolloutFields lists the InstanceClass spec fields whose change requires recreating the
	// machines, as dot-paths into the spec ("rootDisk.size"). The provider owns this list: only
	// the provider team knows which of its fields the cloud cannot change on a live VM.
	//
	// It is compared against the snapshot at decision time and is not part of the snapshot
	// itself, so shipping a new list never rolls machines on its own.
	RolloutFields []string `json:"rolloutFields"`

	// ProviderRolloutFields is the same list for the cloud-provider config — the second input the
	// template renders from. Most providers leave it empty: their config feeds the machine but its
	// change was never a reason to recreate one. vcd is the exception, and its
	// VCDClusterConfiguration schema tells the user so.
	ProviderRolloutFields []string `json:"providerRolloutFields"`

	MachineDeployment MachineDeploymentContract `json:"machineDeployment"`

	// Template is the go-template of the infrastructure MachineTemplate. It renders
	// apiVersion, kind and spec only — metadata is stamped by node-controller.
	Template string `json:"template"`

	// parsed is the compiled Template. It is compiled once, in ParseContract, both to validate the
	// contract at load time and so that Render does not recompile it per zone.
	parsed *template.Template
}

// MachineDeploymentContract carries what the provider needs in the generic MachineDeployment
// that node-controller builds itself. It replaces the v1 machine-deployment-spec-patch.yaml,
// whose ${zone} string substitution into a raw YAML patch was only ever used for one field.
type MachineDeploymentContract struct {
	// AdditionalFields maps a dot-path inside spec.template.spec to a go-template rendered in the
	// same sandbox, with the same context, as the machine template itself — so a provider that
	// needs a second field does not need a node-controller release to name a new "source".
	// Today one provider uses one entry: openstack's `failureDomain: "{{ .zone }}"`.
	AdditionalFields map[string]string `json:"additionalFields"`

	// parsedFields are AdditionalFields compiled at parse time, keyed by the same dot-path.
	parsedFields map[string]*template.Template
}

// reservedMachineDeploymentFields are the parts of spec.template.spec node-controller decides
// itself. infrastructureRef in particular is the whole design: its name identifies the current
// generation, so a provider able to write it could switch machines onto an object node-controller
// knows nothing about.
var reservedMachineDeploymentFields = []string{"infrastructureRef", "bootstrap", "clusterName"}

// ParseContract parses and validates capi/template.yaml. Every problem is reported as an error
// rather than defaulted away: a provider file that does not say what it means must fail loudly at
// load time, not render something plausible into a cluster.
func ParseContract(data []byte) (*Contract, error) {
	c := &Contract{}
	if err := yaml.UnmarshalStrict(data, c); err != nil {
		return nil, fmt.Errorf("parse machine-template contract: %w", err)
	}

	if c.Version != ContractVersionV2 {
		return nil, fmt.Errorf("unsupported machine-template contract version %q, want %q", c.Version, ContractVersionV2)
	}
	if strings.TrimSpace(c.Template) == "" {
		return nil, fmt.Errorf("machine-template contract has an empty template")
	}
	if len(c.RolloutFields) == 0 {
		return nil, fmt.Errorf("machine-template contract has no rolloutFields: the provider must state which InstanceClass fields recreate machines")
	}

	if err := validateRolloutFields("rolloutFields", c.RolloutFields); err != nil {
		return nil, err
	}
	if err := validateRolloutFields("providerRolloutFields", c.ProviderRolloutFields); err != nil {
		return nil, err
	}

	c.MachineDeployment.parsedFields = make(map[string]*template.Template, len(c.MachineDeployment.AdditionalFields))
	for path, value := range c.MachineDeployment.AdditionalFields {
		if err := validateFieldPath(path); err != nil {
			return nil, fmt.Errorf("machineDeployment.additionalFields: %w", err)
		}
		if root, _, _ := strings.Cut(path, "."); slices.Contains(reservedMachineDeploymentFields, root) {
			return nil, fmt.Errorf("machineDeployment.additionalFields[%s]: %s belongs to node-controller and cannot be set by a provider",
				path, root)
		}
		parsed, err := parseTemplate(value)
		if err != nil {
			return nil, fmt.Errorf("machineDeployment.additionalFields[%s]: %w", path, err)
		}
		c.MachineDeployment.parsedFields[path] = parsed
	}

	parsed, err := parseTemplate(c.Template)
	if err != nil {
		return nil, err
	}
	c.parsed = parsed

	return c, nil
}

// validateRolloutFields checks one of the two rollout lists. contractKey names it in the error, so
// the provider reads which of the two it got wrong.
func validateRolloutFields(contractKey string, fields []string) error {
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if err := validateFieldPath(field); err != nil {
			return fmt.Errorf("%s: %w", contractKey, err)
		}
		if _, dup := seen[field]; dup {
			return fmt.Errorf("%s: duplicate field %q", contractKey, field)
		}
		seen[field] = struct{}{}
	}
	return nil
}

// validateFieldPath rejects a path with an empty segment, which also covers the empty path itself
// (strings.Split("", ".") is [""]).
func validateFieldPath(path string) error {
	if slices.Contains(strings.Split(path, "."), "") {
		return fmt.Errorf("malformed field path %q", path)
	}
	return nil
}
