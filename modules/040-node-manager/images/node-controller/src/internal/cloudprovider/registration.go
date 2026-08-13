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

// Package cloudprovider owns everything node-controller knows about cloud providers: the
// registration a provider module publishes, how a NodeGroup picks one, and how controllers watch
// for them. Nothing outside this package selects a registration Secret or decodes its keys.
package cloudprovider

import (
	"encoding/json"
	"strings"
)

const (
	// SecretNamespace is where every provider module publishes its registration.
	SecretNamespace = "kube-system"

	// SecretLabel marks every registration Secret. Selecting by it rather than by one fixed
	// name is what makes several providers in one cluster visible at all.
	SecretLabel = "cloud-provider.deckhouse.io/registration"

	// SecretNamePrefix is what every registration Secret is named with: the bare prefix is the copy
	// under the fixed name, prefix + "-<provider>" is the per-provider one, and every provider
	// module publishes both. The prefix — not either full name — is therefore what identifies a
	// registration, alongside the label.
	SecretNamePrefix = "d8-node-manager-cloud-provider"

	InstanceClassKindKey = "instanceClassKind"

	// InstanceClassAPIVersionKey names the API version every InstanceClass read and watch must
	// use: the storage version of the provider's CRD, published in the registration Secret next
	// to instanceClassKind. An absent key means the provider has not registered yet; callers must
	// wait rather than pick a version of their own.
	//
	// The version is deliberately data, never resolved from discovery. Two independent things
	// make a non-pinned read return different values for the same unchanged object:
	//
	//   - Which version a group resolves to depends on whichever version the RESTMapper happened
	//     to load first, and it is then cached for the whole process lifetime (controller-runtime
	//     pkg/client/apiutil/restmapper.go, "Prepend if preferred version, else append"). Two
	//     pods of the same build can disagree, permanently.
	//   - Reading a non-storage version also changes the answer the moment the CRD's conversion
	//     webhook is wired. Deckhouse CRDs ship without spec.conversion and get it patched in at
	//     runtime, so early in a cluster's life the same read returns the raw stored value
	//     instead of the converted one.
	//
	// Either difference changes the instance-class checksum. That checksum names an immutable
	// infrastructure MachineTemplate, so a changed checksum renames the template, and the rename
	// recreates every node in the NodeGroup.
	InstanceClassAPIVersionKey = "instanceClassAPIVersion"

	// defaultInfraAPIVersion is what the CAPI keys fall back to when a provider publishes none.
	defaultInfraAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
)

// Registration is the registration Secret a cloud provider module publishes
// (modules/030-cloud-provider-*/templates/registration.yaml). Its keys are fixed by that template,
// so they are typed here; only CloudVariables and Data stay open, because their shape is the
// provider's own.
//
// Every key any controller needs is decoded once, so that resolving a provider is a single read
// no matter how many of its fields the caller goes on to use.
type Registration struct {
	// Type is the provider name in lower case: aws, yandex, vsphere, dvp...
	Type string

	MachineClassKind  string
	InstanceClassKind string

	// InstanceClassAPIVersion stays data rather than a constant on purpose: an empty value means
	// the provider has not published it yet, and guessing a version is what this mechanism exists
	// to prevent — a guessed version goes through the provider's conversion webhook, changes the
	// spec, renames an immutable machine template and recreates every VM in the group.
	InstanceClassAPIVersion string

	Zones []string

	CAPI CAPIConfig

	// CloudVariables is the provider's own values tree, keyed in the Secret by Type.
	CloudVariables map[string]any

	// Data is the whole Secret decoded, which the template render context needs verbatim.
	Data map[string]any
}

// CAPIConfig is the CAPI half of the registration. An empty ClusterKind means the provider ships
// no CAPI support and the NodeGroup runs on MCM.
type CAPIConfig struct {
	ClusterName       string
	ClusterKind       string
	ClusterAPIVersion string

	MachineTemplateKind        string
	MachineTemplateAPIVersion  string
	MachineDeploymentSpecPatch string
}

// Empty reports a zero registration, which is what a cluster with no cloud provider yields.
func (r Registration) Empty() bool { return r.Type == "" }

// Decode reads the Secret data. Values arrive in two encodings: helm writes plain strings for
// scalars (type: {{ b64enc "aws" }}) and JSON for structures (zones, the provider tree), so every
// field tries JSON first and falls back to the raw bytes.
func Decode(data map[string][]byte) Registration {
	reg := Registration{
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

	// The default lives here rather than at each use site: the two consumers used to spell it out
	// separately, and a third one would have had to remember to.
	if reg.CAPI.ClusterAPIVersion == "" {
		reg.CAPI.ClusterAPIVersion = defaultInfraAPIVersion
	}
	if reg.CAPI.MachineTemplateAPIVersion == "" {
		reg.CAPI.MachineTemplateAPIVersion = defaultInfraAPIVersion
	}

	if tree, ok := reg.Data[strings.ToLower(reg.Type)].(map[string]any); ok {
		reg.CloudVariables = tree
	}
	return reg
}

func decodeTree(data map[string][]byte) map[string]any {
	res := make(map[string]any, len(data))
	for k, v := range data {
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			res[k] = string(v)
			continue
		}
		res[k] = val
	}
	return res
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
