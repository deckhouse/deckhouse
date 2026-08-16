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

// Package cloudprovider owns what node-controller knows about cloud providers: the registration a
// provider module publishes, how a NodeGroup picks one, and how controllers watch for them.
package cloudprovider

import (
	"encoding/json"
	"strings"
)

// Provider is the registration Secret a cloud provider module publishes
// (modules/030-cloud-provider-*/templates/registration.yaml), decoded once per read.
type Provider struct {
	// Lower case: aws, yandex, vsphere, dvp...
	Type string

	MachineClassKind  string
	InstanceClassKind string

	// Empty means the provider has not published it yet; guessing one recreates every VM in the
	// group. See InstanceClassAPIVersionKey.
	InstanceClassAPIVersion string

	Zones []string

	CAPI CAPIConfig

	// The provider own values tree, keyed in the Secret by Type.
	CloudVariables map[string]any

	// The whole Secret decoded, which the template render context needs verbatim.
	Data map[string]any
}

// CAPIConfig is the CAPI half of the registration. An empty ClusterKind means MCM.
type CAPIConfig struct {
	ClusterName       string
	ClusterKind       string
	ClusterAPIVersion string

	MachineTemplateKind        string
	MachineTemplateAPIVersion  string
	MachineDeploymentSpecPatch string
}

// Empty reports a zero provider — what a cluster with no cloud provider yields.
func (p Provider) Empty() bool {
	return p.Type == ""
}

// FromSecretData reads the Secret data. Helm writes scalars as plain strings and structures as JSON, so
// every field tries JSON first and falls back to the raw bytes.
func FromSecretData(data map[string][]byte) Provider {
	p := Provider{
		Type:                    decodeString(data["type"]),
		MachineClassKind:        decodeString(data["machineClassKind"]),
		InstanceClassKind:       decodeString(data[InstanceClassKindKey]),
		InstanceClassAPIVersion: decodeString(data[InstanceClassAPIVersionKey]),
		Zones:                   decodeStringSlice(data["zones"]),
		CAPI: CAPIConfig{
			ClusterName:                decodeString(data["capiClusterName"]),
			ClusterKind:                decodeString(data["capiClusterKind"]),
			ClusterAPIVersion:          decodeString(data["capiClusterAPIVersion"]),
			MachineTemplateKind:        decodeString(data["capiMachineTemplateKind"]),
			MachineTemplateAPIVersion:  decodeString(data["capiMachineTemplateAPIVersion"]),
			MachineDeploymentSpecPatch: decodeString(data["capiMachineDeploymentSpecPatch"]),
		},
		Data: decodeTree(data),
	}

	// Defaulted here rather than at each use site.
	if p.CAPI.ClusterAPIVersion == "" {
		p.CAPI.ClusterAPIVersion = defaultInfraAPIVersion
	}
	if p.CAPI.MachineTemplateAPIVersion == "" {
		p.CAPI.MachineTemplateAPIVersion = defaultInfraAPIVersion
	}

	if tree, ok := p.Data[strings.ToLower(p.Type)].(map[string]any); ok {
		p.CloudVariables = tree
	}

	return p
}

func decodeTree(data map[string][]byte) map[string]any {
	ret := make(map[string]any, len(data))

	for k, v := range data {
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			ret[k] = string(v)
			continue
		}

		ret[k] = val
	}

	return ret
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

	var ret []string
	if err := json.Unmarshal(raw, &ret); err != nil {
		return nil
	}

	return ret
}
