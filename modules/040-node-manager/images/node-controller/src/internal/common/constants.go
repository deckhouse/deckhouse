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

package common

const (
	MachineNamespace                 = "d8-cloud-instance-manager"
	ConfigurationChecksumsSecretName = "configuration-checksums"

	CloudProviderSecretName      = "d8-node-manager-cloud-provider"
	CloudProviderSecretNamespace = "kube-system"

	// CloudProviderRegistrationLabel marks every registration Secret a cloud provider module
	// publishes. CloudProviderSecretName above is only the legacy fixed name — each provider also
	// publishes a per-provider copy, and anything that must see all providers selects by this
	// label instead of the name.
	CloudProviderRegistrationLabel = "cloud-provider.deckhouse.io/registration"

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
)
