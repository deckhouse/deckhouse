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
// renderer): the legacy engine emulates a helm values tree and is deleted once the last
// provider migrates, while this one is also the library a provider team runs in its own CI
// to check its template.
package machinetemplate

import (
	"fmt"
	"slices"
	"strings"

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

	MachineDeployment MachineDeploymentContract `json:"machineDeployment"`

	// Template is the go-template of the infrastructure MachineTemplate. It renders
	// apiVersion, kind and spec only — metadata is stamped by node-controller.
	Template string `json:"template"`
}

// MachineDeploymentContract carries what the provider needs in the generic MachineDeployment
// that node-controller builds itself. It replaces the v1 machine-deployment-spec-patch.yaml,
// whose ${zone} string substitution into a raw YAML patch was only ever used for one field.
type MachineDeploymentContract struct {
	// AdditionalFields maps a dot-path inside spec.template.spec to a context source name.
	// Today the single known source is "zone" (openstack needs failureDomain: zone).
	AdditionalFields map[string]string `json:"additionalFields"`
}

// MachineDeploymentFieldSourceZone is the only source an additionalFields entry may name.
// Adding a source is a contract change: it must be documented in the contract spec first,
// otherwise a template starts depending on data the spec does not promise.
const MachineDeploymentFieldSourceZone = "zone"

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

	seen := make(map[string]struct{}, len(c.RolloutFields))
	for _, field := range c.RolloutFields {
		if err := validateFieldPath(field); err != nil {
			return nil, fmt.Errorf("rolloutFields: %w", err)
		}
		if _, dup := seen[field]; dup {
			return nil, fmt.Errorf("rolloutFields: duplicate field %q", field)
		}
		seen[field] = struct{}{}
	}

	for path, source := range c.MachineDeployment.AdditionalFields {
		if err := validateFieldPath(path); err != nil {
			return nil, fmt.Errorf("machineDeployment.additionalFields: %w", err)
		}
		if source != MachineDeploymentFieldSourceZone {
			return nil, fmt.Errorf("machineDeployment.additionalFields[%s]: unknown source %q, known sources: %s",
				path, source, MachineDeploymentFieldSourceZone)
		}
	}

	if _, err := parseTemplate(c.Template); err != nil {
		return nil, err
	}

	return c, nil
}

func validateFieldPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty field path")
	}
	if slices.Contains(strings.Split(path, "."), "") {
		return fmt.Errorf("malformed field path %q", path)
	}
	return nil
}
