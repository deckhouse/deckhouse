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

	// KubeSystemNamespace holds the cluster-wide objects a node is built from:
	// the bootstrap tokens, the cluster configuration, the projected CA.
	KubeSystemNamespace = "kube-system"

	// ClusterUUIDConfigMapName holds the cluster UUID that seeds hashed resource names and the
	// update epoch. Four packages read it, and a name that drifts in one of them reads as a
	// cluster with no UUID: renamed MachineClasses, a shifted epoch.
	ClusterUUIDConfigMapName = "d8-cluster-uuid"
	ClusterUUIDConfigMapKey  = "cluster-uuid"
)
