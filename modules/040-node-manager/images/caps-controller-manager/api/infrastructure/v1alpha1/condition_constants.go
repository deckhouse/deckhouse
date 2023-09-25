package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

// Conditions and Reasons defined on StaticInstance.
const (
	StaticInstanceAddedToNodeGroupCondition clusterv1.ConditionType = "AddedToNodeGroup"

	// StaticInstanceWaitingForNodeGroupReason indicates when a StaticInstance is waiting for a NodeGroup to be assigned.
	StaticInstanceWaitingForNodeGroupReason = "WaitingForNodeGroupToBeAssigned"

	StaticInstanceWaitingForCredentialsRefReason = "WaitingForCredentialsRefToBeAssigned"

	StaticInstanceBootstrapSucceededCondition clusterv1.ConditionType = "BootstrapSucceeded"

	// StaticInstanceWaitingForMachineRefReason indicates when a StaticInstance is registered into a capacity pool and
	// waiting for a StaticInstance.Status.MachineRef to be assigned.
	StaticInstanceWaitingForMachineRefReason = "WaitingForMachineRefToBeAssigned"

	// StaticInstanceWaitingForNodeRefReason indicates when a StaticInstance is registered into a capacity pool and
	// waiting for a StaticInstance.Status.NodeRef to be assigned.
	StaticInstanceWaitingForNodeRefReason = "WaitingForNodeRefToBeAssigned"
)

// Conditions and Reasons defined on StaticMachine.
const (
	// StaticMachineStaticInstanceReadyCondition documents the k8s node is ready and can take on workloads.
	StaticMachineStaticInstanceReadyCondition clusterv1.ConditionType = "StaticInstanceReady"

	// StaticMachineWaitingForClusterInfrastructureReason indicates the cluster that the StaticMachine belongs to
	// is waiting to be owned by the corresponding CAPI Cluster.
	StaticMachineWaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"

	// StaticMachineWaitingForBootstrapDataSecretReason indicates that the bootstrap provider is yet to provide the
	// secret that contains bootstrap information.
	// This secret is available on Machine.Spec.Bootstrap.DataSecretName.
	StaticMachineWaitingForBootstrapDataSecretReason = "WaitingForBootstrapDataSecret"

	// StaticMachineStaticInstancesUnavailableReason indicates that no static instances are available in the capacity pool.
	StaticMachineStaticInstancesUnavailableReason = "StaticInstancesUnavailable"
)

// Reasons common to all Static resources.
const (
	// ClusterOrResourcePausedReason indicates that either
	// Spec.Paused field on the cluster is set to true
	// or the resource is marked with Paused annotation.
	ClusterOrResourcePausedReason = "ClusterOrResourcePaused"
)
