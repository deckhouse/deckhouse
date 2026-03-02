/*
Copyright 2025 Flant JSC

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

// nolint:gci
package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/utils/ptr"
	clusterv1b1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dvpapi "dvp-common/api"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
)

const ProviderIDPrefix = "dvp://"

const (
	// OrphanedVMAnnotation marks DeckhouseMachine when VM deletion timed out
	OrphanedVMAnnotation = "dvp.deckhouse.io/orphaned-vm"
	// OrphanedVMTimestampAnnotation records when VM became orphaned
	OrphanedVMTimestampAnnotation = "dvp.deckhouse.io/orphaned-vm-timestamp"
)

// DeckhouseMachineReconciler reconciles a DeckhouseMachine object
type DeckhouseMachineReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	DVP         *dvpapi.DVPCloudAPI
	ClusterUUID string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhousemachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhousemachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhousemachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeckhouseMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) { // nolint:nonamedreturns
	logger := log.FromContext(ctx)

	dvpMachine := &infrastructurev1a1.DeckhouseMachine{}
	err := r.Client.Get(ctx, req.NamespacedName, dvpMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("Error getting DeckhouseMachine: %w", err)
	}
	logger = logger.WithValues("dvp_machine", dvpMachine.Name, "dvp_machine_ns", dvpMachine.Namespace)

	machine, err := capiutil.GetOwnerMachine(ctx, r.Client, dvpMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}
	logger = logger.WithValues("machine", machine.Name)

	cluster, err := capiutil.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}
	logger = logger.WithValues("cluster", cluster.Name)

	dvpCluster := &infrastructurev1a1.DeckhouseCluster{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}, dvpCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("Error getting DeckhouseCluster: %w", err)
	}
	logger = logger.WithValues("dvp_cluster", dvpCluster.Name, "dvp_cluster_ns", dvpCluster.Namespace)

	if annotations.IsPaused(cluster, dvpMachine) {
		logger.Info("DeckhouseMachine or linked Cluster is marked as paused. Will not reconcile.")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(dvpMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the dvpMachine when exiting this function, so we can persist any DeckhouseMachine changes.
	defer func() {
		if err := patchDeckhouseMachine(ctx, patchHelper, dvpMachine); err != nil {
			result = ctrl.Result{}
			reterr = err
		}
	}()

	// Handle deleted machines
	if !dvpMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDeleteOperation(ctx, logger, dvpMachine)
	}

	// Handle other kinds of changes
	return r.reconcileUpdates(ctx, logger, cluster, machine, dvpMachine)
}

func patchDeckhouseMachine(
	ctx context.Context,
	patchHelper *patch.Helper,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
	options ...patch.Option,
) error {
	conditions.SetSummary(dvpMachine,
		conditions.WithConditions(infrastructurev1a1.VMReadyCondition),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []clusterv1b1.ConditionType{
			clusterv1b1.ReadyCondition,
			infrastructurev1a1.VMReadyCondition,
		}},
	)
	return patchHelper.Patch(ctx, dvpMachine, options...)
}

func (r *DeckhouseMachineReconciler) reconcileUpdates(
	ctx context.Context,
	logger logr.Logger,
	cluster *clusterv1b1.Cluster,
	machine *clusterv1b1.Machine,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) (ctrl.Result, error) {
	if dvpMachine.Status.FailureReason != nil || dvpMachine.Status.FailureMessage != nil {
		logger.Info("DeckhouseMachine has failed, will not reconcile. See DeckhouseMachine status for details.")
		return ctrl.Result{}, nil
	}

	// If DeckhouseMachine is not under finalizer yet, set it now.
	if controllerutil.AddFinalizer(dvpMachine, infrastructurev1a1.MachineFinalizer) {
		return ctrl.Result{}, nil
	}

	if !cluster.Status.InfrastructureReady {
		logger.Info("Waiting for Cluster infrastructure to become Ready")
		conditions.MarkFalse(
			dvpMachine,
			infrastructurev1a1.VMReadyCondition,
			infrastructurev1a1.WaitingForClusterInfrastructureReason,
			clusterv1b1.ConditionSeverityInfo,
			"",
		)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap cloud-init secret reference is missing from Machine")
		conditions.MarkFalse(
			dvpMachine,
			infrastructurev1a1.VMReadyCondition,
			infrastructurev1a1.WaitingForBootstrapScriptReason,
			clusterv1b1.ConditionSeverityInfo,
			"",
		)
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling DeckhouseMachine")

	vm, err := r.getOrCreateVM(ctx, machine, dvpMachine)
	if err != nil {
		logger.Info("No VM can be found or created for Machine, see DeckhouseMachine status for details")
		conditions.MarkFalse(
			dvpMachine,
			infrastructurev1a1.VMReadyCondition,
			infrastructurev1a1.VMErrorReason,
			clusterv1b1.ConditionSeverityError,
			"No VM can be found or created for Machine: %v",
			err,
		)
		return ctrl.Result{}, fmt.Errorf("find or create VM: %w", err)
	}
	logger = logger.WithValues("vm_name", vm.Name, "vm_ns", vm.Namespace)

	// Node usually joins the cluster if the CSR generated by kubelet with the node name is approved.
	// The approval happens if the Machine InternalDNS matches the node name, so we add it here along with hostname.
	dvpMachine.Status.Addresses = []infrastructurev1a1.VMAddress{
		{Type: clusterv1b1.MachineHostName, Address: vm.Name},
		{Type: clusterv1b1.MachineInternalDNS, Address: vm.Name},
	}
	dvpMachine.Spec.ProviderID = ProviderIDPrefix + vm.Name

	switch vm.Status.Phase {
	case v1alpha2.MachineRunning:
		// If VM is running, fetch its IP addr and add it to dvpMachine.Status.Addresses
		logger.Info("VM is Running")
		conditions.MarkTrue(dvpMachine, infrastructurev1a1.VMReadyCondition)
		dvpMachine.Status.Ready = true
		dvpMachine.Status.Addresses = append(dvpMachine.Status.Addresses, []infrastructurev1a1.VMAddress{
			{Type: clusterv1b1.MachineInternalIP, Address: vm.Status.IPAddress},
			{Type: clusterv1b1.MachineExternalIP, Address: vm.Status.IPAddress},
		}...)

		// TODO(mvasl) DVP does not support detaching of provisioning secrets yet, but one day it will.
		// We should detach and remove cloud-init secret we created after vm is bootstrapped and joined the cluster
		// if machine.Status.NodeRef != nil && vm.Spec.Provisioning != nil {
		// 	cloudInitSecretName := "cloud-init-" + dvpMachine.Name
		// 	logger.Info("Removing Cloud-Init secret from VirtualMachine", "secret", cloudInitSecretName)
		// 	vm.Spec.Provisioning = nil
		// 	if err = r.DVP.ComputeService.DeleteCloudInitProvisioningSecret(ctx, cloudInitSecretName); err != nil {
		// 		return ctrl.Result{}, fmt.Errorf("delete cloud-init secret %q: %w", cloudInitSecretName, err)
		// 	}
		// }
	case v1alpha2.MachineStopped:
		// VM is stopped. With "AlwaysOnUnlessStoppedManually" run policy this can be expected
		// (e.g., manual stop for maintenance). We do not force-start the VM here.
		logger.Info("VM is in Stopped state; manual stop is allowed by runPolicy. Not forcing start", "state", vm.Status.Phase)
		dvpMachine.Status.Ready = false
		conditions.MarkFalse(
			dvpMachine,
			infrastructurev1a1.VMReadyCondition,
			infrastructurev1a1.VMInStoppedStateReason,
			clusterv1b1.ConditionSeverityWarning,
			"VM is in Stopped state; manual stop is allowed by runPolicy. Not forcing start",
		)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	case v1alpha2.MachineDegraded:
		// If VM is in some bad state and cannot be booted up, check if it has NodeRef.
		// If the machine has a NodeRef then it must have been working at some point,
		// so the error could be something temporary.
		// If not, it is more likely a configuration error, so we record failure and never retry.
		logger.Error(fmt.Errorf("VM in degraded state"), "VM failed",
			"vm_phase", vm.Status.Phase,
			"vm_name", vm.Name,
			"has_node_ref", machine.Status.NodeRef != nil,
			"requested_memory", dvpMachine.Spec.Memory.String(),
			"requested_cpu_cores", dvpMachine.Spec.CPU.Cores,
			"vm_class", dvpMachine.Spec.VMClassName,
		)

		if machine.Status.NodeRef == nil {
			// VM never successfully started - likely a resource or configuration error
			err = fmt.Errorf("VM state %q indicates failure, likely due to resource constraints or configuration error", vm.Status.Phase)
			dvpMachine.Status.FailureReason = ptr.To("CreateError")
			dvpMachine.Status.FailureMessage = ptr.To(fmt.Sprintf(
				"VM failed to start (vmClass: %s, memory: %s, CPU: %d cores). Check parent DVP cluster for detailed error: %s",
				dvpMachine.Spec.VMClassName,
				dvpMachine.Spec.Memory.String(),
				dvpMachine.Spec.CPU.Cores,
				err.Error(),
			))
		} else {
			// VM was working before, this might be temporary
			logger.Info("VM had NodeRef before entering degraded state, may be temporary issue",
				"node_name", machine.Status.NodeRef.Name,
			)
		}

		conditions.MarkFalse(
			dvpMachine,
			infrastructurev1a1.VMReadyCondition,
			infrastructurev1a1.VMInFailedStateReason,
			clusterv1b1.ConditionSeverityError,
			"VM in degraded state: %s",
			vm.Status.Phase,
		)
		return ctrl.Result{}, nil
	default:
		// The other states are normal (for example, migration or shutdown) but we don't want to proceed until it's up
		// due to potential conflict or unexpected actions
		logger.Info("Waiting for VM state to become Running", "state", vm.Status.Phase)
		dvpMachine.Status.Ready = false
		conditions.MarkUnknown(
			dvpMachine,
			infrastructurev1a1.VMReadyCondition,
			infrastructurev1a1.VMNotReadyReason,
			"VM is not ready, state is %s",
			vm.Status.Phase,
		)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("Reconciled DeckhouseMachine successfully")
	return ctrl.Result{}, nil
}

func (r *DeckhouseMachineReconciler) reconcileDeleteOperation(
	ctx context.Context,
	logger logr.Logger,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) (ctrl.Result, error) { // nolint:unparam
	logger.Info("Reconciling DeckhouseMachine delete operation")

	vm, err := r.DVP.ComputeService.GetVMByName(ctx, dvpMachine.Name)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			logger.Error(err, "Corresponding VirtualMachine resource was not found, will consider this VM as properly deleted")

			controllerutil.RemoveFinalizer(dvpMachine, infrastructurev1a1.MachineFinalizer)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("cannot get VirtualMachine: %w", err)
	}

	disksToDetach, disksToDelete, err := r.DVP.ComputeService.GetDisksForDetachAndDelete(ctx, vm, true)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting disks for detach and delete: %w", err)
	}

	vmHostname, err := r.DVP.ComputeService.GetVMHostname(vm)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.DVP.ComputeService.DetachDisksFromVM(ctx, disksToDetach, vmHostname)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Try to delete VM with timeout
	vmDeletionFailed := false
	if err = r.DVP.ComputeService.DeleteVM(ctx, dvpMachine.Name); err != nil {
		switch {
		case errors.Is(err, cloudprovider.InstanceNotFound):
			logger.Info("VirtualMachine already deleted during DeleteVM call, continuing")
		case errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout"): // Check if it's a timeout error - in this case, proceed with cleanup
			logger.Error(err, "VM deletion timed out, VM may still be terminating in parent DVP cluster. Proceeding with cleanup to unblock DeckhouseMachine deletion.",
				"vm_name", dvpMachine.Name,
			)
			vmDeletionFailed = true

			// Mark the VM as orphaned for manual cleanup
			if dvpMachine.Annotations == nil {
				dvpMachine.Annotations = make(map[string]string)
			}
			dvpMachine.Annotations[OrphanedVMAnnotation] = dvpMachine.Name
			dvpMachine.Annotations[OrphanedVMTimestampAnnotation] = time.Now().Format(time.RFC3339)

			// Continue with disk cleanup despite VM deletion timeout
		default:
			// For other errors, fail the reconciliation
			return ctrl.Result{}, fmt.Errorf("delete VirtualMachine: %w", err)
		}
	}

	merr := &multierror.Error{}
	for _, disk := range disksToDelete {
		logger.Info("Removing VirtualDisk", "disk_name", disk)
		if err = r.DVP.DiskService.RemoveDiskByName(ctx, disk); err != nil {
			merr = multierror.Append(merr, fmt.Errorf("delete VirtualDisk %s: %w", disk, err))
		}
	}

	if err = merr.ErrorOrNil(); err != nil {
		return ctrl.Result{}, fmt.Errorf("delete VirtualDisks: %w", err)
	}

	controllerutil.RemoveFinalizer(dvpMachine, infrastructurev1a1.MachineFinalizer)

	if vmDeletionFailed {
		logger.Info("Reconciled Machine delete with orphaned VM - manual cleanup may be required in parent DVP cluster",
			"orphaned_vm", dvpMachine.Name,
		)
	} else {
		logger.Info("Reconciled Machine delete successfully")
	}

	return ctrl.Result{}, nil
}

func (r *DeckhouseMachineReconciler) getOrCreateVM(
	ctx context.Context,
	machine *clusterv1b1.Machine,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) (*v1alpha2.VirtualMachine, error) {
	vm, err := r.DVP.ComputeService.GetVMByName(ctx, dvpMachine.Name)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			vm, err = r.createVM(ctx, machine, dvpMachine)
			return vm, err
		}
		return nil, fmt.Errorf("cannot get VirtualMachine: %w", err)
	}

	return vm, nil
}

// cleanupVMResources removes resources created during VM provisioning
func (r *DeckhouseMachineReconciler) cleanupVMResources(
	ctx context.Context,
	dvpMachine *infrastructurev1a1.DeckhouseMachine, // nolint:unparam
	cloudInitSecretName string,
	createdDiskNames []string,
) {
	logger := log.FromContext(ctx)

	// Delete cloud-init secret
	if cloudInitSecretName != "" {
		if err := r.DVP.ComputeService.DeleteCloudInitProvisioningSecret(ctx, cloudInitSecretName); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("Cleanup skipped: cloud-init secret not found (already deleted or never created)", "secretName", cloudInitSecretName)
			} else {
				logger.Error(err, "Failed to cleanup cloud-init secret", "secretName", cloudInitSecretName)
			}
		}
	}

	// Delete disks (boot and additional)
	for _, diskName := range createdDiskNames {
		if err := r.DVP.DiskService.RemoveDiskByName(ctx, diskName); err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("Cleanup skipped: disk not found (already deleted or never created)", "diskName", diskName)
			} else {
				logger.Error(err, "Failed to cleanup disk", "diskName", diskName)
			}
		}
	}
}

func (r *DeckhouseMachineReconciler) createVM(
	ctx context.Context,
	machine *clusterv1b1.Machine,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) (*v1alpha2.VirtualMachine, error) {
	logger := log.FromContext(ctx)

	if machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, fmt.Errorf("clusterv1b1.Machine does not contain bootstrap script")
	}

	// Validate VirtualMachineClass and image exist before creating VM
	if err := r.validateVMResources(ctx, dvpMachine); err != nil {
		return nil, fmt.Errorf("resource validation failed: %w", err)
	}

	var createdDiskNames []string
	cloudInitSecretName := "cloud-init-" + dvpMachine.Name

	bootstrapDataSecret := &corev1.Secret{}
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{Namespace: machine.GetNamespace(), Name: *machine.Spec.Bootstrap.DataSecretName},
		bootstrapDataSecret,
	); err != nil {
		logger.Info("Failed to get bootstrap data secret, cleaning up created resources", "error", err.Error())
		r.cleanupVMResources(ctx, dvpMachine, cloudInitSecretName, createdDiskNames)
		return nil, fmt.Errorf("Cannot get cloud-init data secret: %w", err)
	}

	cloudInitScript, hasBootstrapScript := bootstrapDataSecret.Data["value"]
	if !hasBootstrapScript {
		logger.Info("Bootstrap script not found in secret, cleaning up created resources")
		r.cleanupVMResources(ctx, dvpMachine, cloudInitSecretName, createdDiskNames)
		return nil, fmt.Errorf("Expected to find a cloud-init script in secret %s/%s", bootstrapDataSecret.Namespace, bootstrapDataSecret.Name)
	}

	bootDiskName := dvpMachine.Name + "-boot"
	blockDeviceRefs := []v1alpha2.BlockDeviceSpecRef{
		{Kind: v1alpha2.DiskDevice, Name: bootDiskName},
	}

	for i := range dvpMachine.Spec.AdditionalDisks {
		addDiskName := fmt.Sprintf("%s-additional-disk-%d", dvpMachine.Name, i)
		blockDeviceRefs = append(blockDeviceRefs, v1alpha2.BlockDeviceSpecRef{
			Kind: v1alpha2.DiskDevice,
			Name: addDiskName,
		})
	}

	runPolicy := dvpMachine.Spec.RunPolicy
	if runPolicy == "" {
		runPolicy = string(v1alpha2.AlwaysOnUnlessStoppedManually)
	}

	// LiveMigrationPolicy: apply from spec or use default for masters
	liveMigrationPolicy := dvpMachine.Spec.LiveMigrationPolicy
	if liveMigrationPolicy == "" {
		// For control plane nodes (masters), default to PreferForced due to high memory activity
		if machine != nil && capiutil.IsControlPlaneMachine(machine) {
			liveMigrationPolicy = string(v1alpha2.PreferForcedMigrationPolicy)
		} else {
			// For worker nodes, default to PreferSafe for safer live migrations
			liveMigrationPolicy = string(v1alpha2.PreferSafeMigrationPolicy)
		}
	}

	vm, err := r.DVP.ComputeService.CreateVM(ctx, &v1alpha2.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: dvpMachine.Name,
			Labels: map[string]string{
				"deckhouse.io/managed-by":       "deckhouse",
				"dvp.deckhouse.io/cluster-uuid": r.ClusterUUID,
				"dvp.deckhouse.io/hostname":     dvpMachine.Name,
			},
		},
		Spec: v1alpha2.VirtualMachineSpec{
			RunPolicy:                v1alpha2.RunPolicy(runPolicy),
			LiveMigrationPolicy:      v1alpha2.LiveMigrationPolicy(liveMigrationPolicy),
			OsType:                   v1alpha2.GenericOs,
			Bootloader:               v1alpha2.BootloaderType(dvpMachine.Spec.Bootloader),
			VirtualMachineClassName:  dvpMachine.Spec.VMClassName,
			EnableParavirtualization: true,
			Provisioning: &v1alpha2.Provisioning{
				Type: v1alpha2.ProvisioningTypeUserDataRef,
				UserDataRef: &v1alpha2.UserDataRef{
					Kind: "Secret",
					Name: cloudInitSecretName,
				},
			},
			CPU: v1alpha2.CPUSpec{
				Cores:        dvpMachine.Spec.CPU.Cores,
				CoreFraction: dvpMachine.Spec.CPU.Fraction,
			},
			Memory: v1alpha2.MemorySpec{
				Size: dvpMachine.Spec.Memory,
			},
			BlockDeviceRefs: blockDeviceRefs,
		},
	})
	if err != nil {
		// Cleanup resources on VM creation failure
		logger.Info("VM creation failed, cleaning up created resources", "error", err.Error())
		r.cleanupVMResources(ctx, dvpMachine, cloudInitSecretName, createdDiskNames)

		// Log the request details for debugging
		logger.Error(err, "Failed to create VM in parent DVP cluster",
			"vm_name", dvpMachine.Name,
			"vm_class", dvpMachine.Spec.VMClassName,
			"requested_memory", dvpMachine.Spec.Memory.String(),
			"requested_cpu_cores", dvpMachine.Spec.CPU.Cores,
			"requested_cpu_fraction", dvpMachine.Spec.CPU.Fraction,
		)

		errMsg := err.Error()

		// Check for permission/forbidden errors (e.g., vmClass mismatch or quota)
		if strings.Contains(errMsg, "forbidden") || strings.Contains(errMsg, "Forbidden") {
			return nil, fmt.Errorf("VM creation blocked in parent DVP cluster - possible vmClass resource limits exceeded or permission issue (vmClass: %s, memory: %s, CPU: %d cores @ %s): %w",
				dvpMachine.Spec.VMClassName,
				dvpMachine.Spec.Memory.String(),
				dvpMachine.Spec.CPU.Cores,
				dvpMachine.Spec.CPU.Fraction,
				err)
		}

		// Check for resource constraint errors
		if strings.Contains(errMsg, "insufficient") ||
			strings.Contains(errMsg, "exceeded") ||
			strings.Contains(errMsg, "quota") ||
			strings.Contains(errMsg, "limit") {
			return nil, fmt.Errorf("VM creation failed due to resource constraints - requested resources may exceed available capacity or vmClass limits (vmClass: %s, memory: %s, CPU: %d cores): %w",
				dvpMachine.Spec.VMClassName,
				dvpMachine.Spec.Memory.String(),
				dvpMachine.Spec.CPU.Cores,
				err)
		}

		// Generic error with context
		return nil, fmt.Errorf("create VM failed (vmClass: %s, memory: %s, CPU: %d cores): %w",
			dvpMachine.Spec.VMClassName,
			dvpMachine.Spec.Memory.String(),
			dvpMachine.Spec.CPU.Cores,
			err)
	}

	if err = r.DVP.ComputeService.CreateCloudInitProvisioningSecret(
		ctx,
		r.ClusterUUID,
		dvpMachine.Name,
		cloudInitSecretName,
		cloudInitScript,
		vm.Name,
		vm.UID,
	); err != nil {
		logger.Info("Cloud-init secret creation failed, cleaning up created resources", "error", err.Error())
		r.cleanupVMResources(ctx, dvpMachine, cloudInitSecretName, createdDiskNames)
		return nil, fmt.Errorf("Cannot create cloud-init provisioning secret: %w", err)
	}

	if _, err = r.DVP.DiskService.CreateDiskFromDataSource(
		ctx,
		r.ClusterUUID,
		dvpMachine.Name,
		bootDiskName,
		dvpMachine.Spec.RootDiskSize,
		dvpMachine.Spec.RootDiskStorageClass,
		&v1alpha2.VirtualDiskDataSource{
			Type: v1alpha2.DataSourceTypeObjectRef,
			ObjectRef: &v1alpha2.VirtualDiskObjectRef{
				Kind: v1alpha2.VirtualDiskObjectRefKind(dvpMachine.Spec.BootDiskImageRef.Kind),
				Name: dvpMachine.Spec.BootDiskImageRef.Name,
			},
		},
		[]metav1.OwnerReference{
			{
				APIVersion: "virtualization.deckhouse.io/v1alpha2",
				Kind:       "VirtualMachine",
				Name:       vm.Name,
				UID:        vm.UID,
			},
		},
	); err != nil {
		logger.Info("Boot disk creation failed, cleaning up created resources", "error", err.Error())
		r.cleanupVMResources(ctx, dvpMachine, cloudInitSecretName, createdDiskNames)
		return nil, fmt.Errorf("Cannot create boot disk: %w", err)
	}
	createdDiskNames = append(createdDiskNames, bootDiskName)

	for i, d := range dvpMachine.Spec.AdditionalDisks {
		addDiskName := fmt.Sprintf("%s-additional-disk-%d", dvpMachine.Name, i)
		if _, err = r.DVP.DiskService.CreateDisk(
			ctx,
			r.ClusterUUID,
			dvpMachine.Name,
			addDiskName,
			d.Size.Value(),
			d.StorageClass,
			[]metav1.OwnerReference{{
				APIVersion: "virtualization.deckhouse.io/v1alpha2",
				Kind:       "VirtualMachine",
				Name:       vm.Name,
				UID:        vm.UID,
			}}); err != nil {
			logger.Info("Additional disk creation failed, cleaning up created resources", "error", err.Error(), "diskName", addDiskName)
			r.cleanupVMResources(ctx, dvpMachine, cloudInitSecretName, createdDiskNames)
			return nil, fmt.Errorf("Cannot create additional disk %s: %w", addDiskName, err)
		}
		createdDiskNames = append(createdDiskNames, addDiskName)
	}

	return vm, nil
}

// validateVMResources validates that VirtualMachineClass and boot image exist in parent DVP cluster
func (r *DeckhouseMachineReconciler) validateVMResources(
	ctx context.Context,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) error {
	logger := log.FromContext(ctx)
	dvpNamespace := r.DVP.ProjectNamespace()

	// Validate VirtualMachineClass exists
	vmClassName := dvpMachine.Spec.VMClassName
	vmClassGVK := schema.GroupVersionKind{
		Group:   "virtualization.deckhouse.io",
		Version: "v1alpha2",
		Kind:    "VirtualMachineClass",
	}

	vmClass := &unstructured.Unstructured{}
	vmClass.SetGroupVersionKind(vmClassGVK)
	err := r.DVP.Service.GetClient().Get(ctx, client.ObjectKey{Name: vmClassName}, vmClass)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "VirtualMachineClass not found in parent DVP cluster",
				"vmClassName", vmClassName)
			return fmt.Errorf("VirtualMachineClass '%s' not found in parent DVP cluster. "+
				"Please ensure the VirtualMachineClass exists before creating VMs. "+
				"Available VirtualMachineClasses can be listed with: kubectl get virtualmachineclasses",
				vmClassName)
		}
		return fmt.Errorf("failed to validate VirtualMachineClass '%s': %w", vmClassName, err)
	}
	logger.V(1).Info("VirtualMachineClass validated successfully", "vmClassName", vmClassName)

	// Validate boot image exists
	imageKind := dvpMachine.Spec.BootDiskImageRef.Kind
	imageName := dvpMachine.Spec.BootDiskImageRef.Name

	// Validate image kind
	if imageKind != "ClusterVirtualImage" && imageKind != "VirtualImage" {
		return fmt.Errorf("unsupported boot disk image kind '%s', must be either 'ClusterVirtualImage' or 'VirtualImage'",
			imageKind)
	}

	// Build image GVK and ObjectKey
	imageGVK := schema.GroupVersionKind{
		Group:   "virtualization.deckhouse.io",
		Version: "v1alpha2",
		Kind:    imageKind,
	}

	imageKey := client.ObjectKey{Name: imageName}
	if imageKind == "VirtualImage" {
		imageKey.Namespace = dvpNamespace
	}

	// Validate image exists
	image := &unstructured.Unstructured{}
	image.SetGroupVersionKind(imageGVK)
	err = r.DVP.Service.GetClient().Get(ctx, imageKey, image)
	if err != nil {
		if apierrors.IsNotFound(err) {
			namespaceInfo := ""
			if imageKind == "VirtualImage" {
				namespaceInfo = fmt.Sprintf(" in namespace '%s'", dvpNamespace)
			}

			logger.Error(err, fmt.Sprintf("%s not found in parent DVP cluster", imageKind),
				"imageName", imageName, "namespace", dvpNamespace)

			resourceName := strings.ToLower(imageKind) + "s"
			kubectlCmd := fmt.Sprintf("kubectl get %s", resourceName)
			if imageKind == "VirtualImage" {
				kubectlCmd += fmt.Sprintf(" -n %s", dvpNamespace)
			}

			return fmt.Errorf("%s '%s' not found%s in parent DVP cluster. "+
				"Please ensure the image exists before creating VMs. "+
				"Available %ss can be listed with: %s",
				imageKind, imageName, namespaceInfo, imageKind, kubectlCmd)
		}

		namespaceInfo := ""
		if imageKind == "VirtualImage" {
			namespaceInfo = fmt.Sprintf(" in namespace '%s'", dvpNamespace)
		}
		return fmt.Errorf("failed to validate %s '%s'%s: %w", imageKind, imageName, namespaceInfo, err)
	}

	logger.V(1).Info("Boot disk image validated successfully", "imageKind", imageKind, "imageName", imageName)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeckhouseMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1a1.DeckhouseMachine{}).
		Named("deckhousemachine").
		Complete(r)
}
