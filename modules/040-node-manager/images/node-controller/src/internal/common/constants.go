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

	// InstanceClassAPIVersionKey is published by the cloud provider module next to
	// instanceClassKind. See InstanceClassAPIVersion for why the version is data and not
	// something node-controller is allowed to work out for itself.
	InstanceClassAPIVersionKey = "instanceClassAPIVersion"
)
