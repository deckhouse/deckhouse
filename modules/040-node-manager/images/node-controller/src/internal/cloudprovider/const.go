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

package cloudprovider

// StatusNone is the provider of a NodeGroup whose nodes get no provider steps.
const StatusNone = "None"

// The registration Secret a provider module publishes.
const (
	RegistrationSecretNamespace  = "kube-system"
	RegistrationSecretLabel      = "cloud-provider.deckhouse.io/registration"
	RegistrationSecretNamePrefix = "d8-node-manager-cloud-provider"
)

// The cluster configuration, which names the provider CloudPermanent NodeGroups resolve through.
const (
	clusterConfigSecretNamespace = "kube-system"
	clusterConfigSecretName      = "d8-cluster-configuration"
	clusterConfigSecretKey       = "cluster-configuration.yaml"
	// ClusterConfiguration.clusterType of a cluster that runs in a cloud.
	cloudClusterType = "Cloud"
)

// Keys of the registration Secret that callers outside this package name.
const (
	InstanceClassKindKey = "instanceClassKind"
	// The storage version of the provider's CRD; empty means it has not registered yet, and callers
	// must wait rather than pick one. Never resolve it from discovery: a non-pinned read returns a
	// different value once the conversion webhook is wired or once another RESTMapper wins the
	// cache, and that changes the instance-class checksum — which renames an immutable
	// MachineTemplate and recreates every node in the NodeGroup.
	InstanceClassAPIVersionKey = "instanceClassAPIVersion"
	// What the CAPI keys fall back to when a provider publishes none.
	defaultInfraAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
)
