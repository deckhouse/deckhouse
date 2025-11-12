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

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deckhouse/virtualization/api/core/v1alpha2"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/utils/ptr"
	clusterv1b2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
	dvpapi "dvp-common/api"
)

const ProviderIDPrefix = "dvp://"

// DeckhouseMachineReconciler reconciles a DeckhouseMachine object
type DeckhouseMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	DVP    *dvpapi.DVPCloudAPI
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhousemachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhousemachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhousemachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DeckhouseMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
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
		Namespace: cluster.Namespace,
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
	// No SetSummary in v1beta2; individual conditions should be updated with conditions.Set() elsewhere.

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []string{
			string(clusterv1b2.ReadyCondition),
			string(infrastructurev1a1.VMReadyCondition),
		}},
	)
	return patchHelper.Patch(ctx, dvpMachine, options...)
}

func (r *DeckhouseMachineReconciler) reconcileUpdates(
	ctx context.Context,
	logger logr.Logger,
	cluster *clusterv1b2.Cluster,
	machine *clusterv1b2.Machine,
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

	if !conditions.IsTrue(cluster, clusterv1b2.InfrastructureReadyCondition) {
		logger.Info("Waiting for Cluster infrastructure to become Ready")
		conditions.Set(dvpMachine, metav1.Condition{
			Type:               string(infrastructurev1a1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1a1.WaitingForClusterInfrastructureReason,
			Message:            "Cluster infrastructure is not ready yet",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap cloud-init secret reference is missing from Machine")

		conditions.Set(dvpMachine, metav1.Condition{
			Type:               string(infrastructurev1a1.VMReadyCondition),
			Status:             metav1.ConditionFalse, // False instead of MarkFalse
			Reason:             infrastructurev1a1.WaitingForBootstrapScriptReason,
			Message:            "Bootstrap cloud-init secret is missing",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling DeckhouseMachine")

	vm, err := r.getOrCreateVM(ctx, machine, dvpMachine)
	if err != nil {
		logger.Info("No VM can be found or created for Machine, see DeckhouseMachine status for details")

		conditions.Set(dvpMachine, metav1.Condition{
			Type:               string(infrastructurev1a1.VMReadyCondition),
			Status:             metav1.ConditionFalse, // VMReady = False
			Reason:             infrastructurev1a1.VMErrorReason,
			Message:            fmt.Sprintf("No VM can be found or created for Machine: %v", err),
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{}, fmt.Errorf("find or create VM: %w", err)
	}
	logger = logger.WithValues("vm_name", vm.Name, "vm_ns", vm.Namespace)

	// Node usually joins the cluster if the CSR generated by kubelet with the node name is approved.
	// The approval happens if the Machine InternalDNS matches the node name, so we add it here along with hostname.
	dvpMachine.Status.Addresses = []infrastructurev1a1.VMAddress{
		{Type: clusterv1b2.MachineHostName, Address: vm.Name},
		{Type: clusterv1b2.MachineInternalDNS, Address: vm.Name},
	}
	dvpMachine.Spec.ProviderID = ProviderIDPrefix + vm.Name

	switch vm.Status.Phase {
	case v1alpha2.MachineRunning:
		// If VM is running, fetch its IP addr and add it to dvpMachine.Status.Addresses
		logger.Info("VM is Running")
		conditions.Set(dvpMachine, metav1.Condition{
			Type:               string(infrastructurev1a1.VMReadyCondition),
			Status:             metav1.ConditionTrue,
			Reason:             "VMRunning",
			Message:            "VM is running and ready",
			LastTransitionTime: metav1.Now(),
		})
		//dvpMachine.Status.Ready = true
		infraReady := true
		dvpMachine.Status.Initialization.InfrastructureProvisioned = &infraReady
		dvpMachine.Status.Addresses = append(dvpMachine.Status.Addresses, []infrastructurev1a1.VMAddress{
			{Type: clusterv1b2.MachineInternalIP, Address: vm.Status.IPAddress},
			{Type: clusterv1b2.MachineExternalIP, Address: vm.Status.IPAddress},
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
		// VM is stopped, this is unexpected as we use "AlwaysOn" run policy for VM's here.
		// Let's wait and see what happens as this may be a part of migration process or this is a bug in the DVP VM controller.
		logger.Info("VM is in Stopped state, waiting for DVP to bring it back up", "state", vm.Status.Phase)
		//dvpMachine.Status.Ready = false
		infraReady := false
		dvpMachine.Status.Initialization.InfrastructureProvisioned = &infraReady
		conditions.Set(dvpMachine, metav1.Condition{
			Type:               string(infrastructurev1a1.VMReadyCondition),
			Status:             metav1.ConditionFalse, // False instead of MarkFalse
			Reason:             infrastructurev1a1.VMInStoppedStateReason,
			Message:            "VM is in Stopped state, waiting for DVP to bring it back up",
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	case v1alpha2.MachineDegraded:
		// If VM is in some bad state and cannot be booted up, check if it has NodeRef.
		// If the machine has a NodeRef then it must have been working at some point,
		// so the error could be something temporary.
		// If not, it is more likely a configuration error, so we record failure and never retry.
		logger.Info("VM failed", "state", vm.Status.Phase)
		if machine.Status.NodeRef.Name == "" {
			err = fmt.Errorf("VM state %q is unexpected", vm.Status.Phase)
			dvpMachine.Status.FailureReason = ptr.To(string(capierrors.UpdateMachineError))
			dvpMachine.Status.FailureMessage = ptr.To(err.Error())
		}
		conditions.Set(dvpMachine, metav1.Condition{
			Type:               string(infrastructurev1a1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1a1.VMInFailedStateReason,
			Message:            "VM is in a failed state",
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{}, nil
	default:
		// The other states are normal (for example, migration or shutdown) but we don't want to proceed until it's up
		// due to potential conflict or unexpected actions
		logger.Info("Waiting for VM state to become Running", "state", vm.Status.Phase)
		//dvpMachine.Status.Ready = false
		infraReady := false
		dvpMachine.Status.Initialization.InfrastructureProvisioned = &infraReady
		conditions.Set(dvpMachine, metav1.Condition{
			Type:               string(infrastructurev1a1.VMReadyCondition),
			Status:             metav1.ConditionUnknown,
			Reason:             infrastructurev1a1.VMNotReadyReason,
			Message:            fmt.Sprintf("VM is not ready, state is %s", vm.Status.Phase),
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("Reconciled DeckhouseMachine successfully")
	return ctrl.Result{}, nil
}

func (r *DeckhouseMachineReconciler) reconcileDeleteOperation(
	ctx context.Context,
	logger logr.Logger,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) (ctrl.Result, error) {
	logger.Info("Reconciling DeckhouseMachine delete operation")

	vm, err := r.DVP.ComputeService.GetVMByName(ctx, dvpMachine.Name)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			logger.Error(err, "Corresponding VirtualMachine resource was not found, will consider this VM as properly deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("cannot get VirtualMachine: %w", err)
	}

	disksToDetach, disksToDelete, err := r.DVP.ComputeService.GetDisksForDetachAndDelete(ctx, vm, true)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error geting disks for detach and delete: %w", err)
	}

	vmHostname, err := r.DVP.ComputeService.GetVMHostname(vm)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.DVP.ComputeService.DetachDisksFromVM(ctx, disksToDetach, vmHostname)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err = r.DVP.ComputeService.DeleteVM(ctx, dvpMachine.Name); err != nil {
		return ctrl.Result{}, fmt.Errorf("delete VirtualMachine: %w", err)
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
	logger.Info("Reconciled Machine delete successfully")
	return ctrl.Result{}, nil
}

func (r *DeckhouseMachineReconciler) getOrCreateVM(
	ctx context.Context,
	machine *clusterv1b2.Machine,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) (
	vm *v1alpha2.VirtualMachine,
	err error,
) {
	vm, err = r.DVP.ComputeService.GetVMByName(ctx, dvpMachine.Name)
	if err != nil {
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			vm, err = r.createVM(ctx, machine, dvpMachine)
			return vm, err
		}
		return nil, fmt.Errorf("cannot get VirtualMachine: %w", err)
	}

	return vm, nil
}

func (r *DeckhouseMachineReconciler) createVM(
	ctx context.Context,
	machine *clusterv1b2.Machine,
	dvpMachine *infrastructurev1a1.DeckhouseMachine,
) (*v1alpha2.VirtualMachine, error) {
	if machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, fmt.Errorf("clusterv1b2.Machine does not contain bootstrap script")
	}

	bootDisk, err := r.DVP.DiskService.CreateDiskFromDataSource(
		ctx,
		dvpMachine.Name+"-boot",
		dvpMachine.Spec.RootDiskSize,
		dvpMachine.Spec.RootDiskStorageClass,
		&v1alpha2.VirtualDiskDataSource{
			Type: v1alpha2.DataSourceTypeObjectRef,
			ObjectRef: &v1alpha2.VirtualDiskObjectRef{
				Kind: v1alpha2.VirtualDiskObjectRefKind(dvpMachine.Spec.BootDiskImageRef.Kind),
				Name: dvpMachine.Spec.BootDiskImageRef.Name,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("Cannot create boot disk: %w", err)
	}

	bootstrapDataSecret := &corev1.Secret{}
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{Namespace: machine.GetNamespace(), Name: *machine.Spec.Bootstrap.DataSecretName},
		bootstrapDataSecret,
	); err != nil {
		return nil, fmt.Errorf("Cannot get cloud-init data secret: %w", err)
	}

	cloudInitScript, hasBootstrapScript := bootstrapDataSecret.Data["value"]
	if !hasBootstrapScript {
		return nil, fmt.Errorf("Expected to find a cloud-init script in secret %s/%s", bootstrapDataSecret.Namespace, bootstrapDataSecret.Name)
	}

	cloudInitSecretName := "cloud-init-" + dvpMachine.Name
	if err := r.DVP.ComputeService.CreateCloudInitProvisioningSecret(ctx, cloudInitSecretName, cloudInitScript); err != nil {
		return nil, fmt.Errorf("Cannot create cloud-init provisioning secret: %w", err)
	}
	blockDeviceRefs := []v1alpha2.BlockDeviceSpecRef{
		{Kind: v1alpha2.DiskDevice, Name: bootDisk.Name},
	}

	for i, d := range dvpMachine.Spec.AdditionalDisks {
		addDiskName := fmt.Sprintf("%s-additional-disk-%d", dvpMachine.Name, i)
		addDisk, err := r.DVP.DiskService.CreateDisk(ctx, addDiskName, d.Size.Value(), d.StorageClass)
		if err != nil {
			return nil, fmt.Errorf("Cannot create additional disk %s: %w", addDiskName, err)
		}
		blockDeviceRefs = append(blockDeviceRefs, v1alpha2.BlockDeviceSpecRef{
			Kind: v1alpha2.DiskDevice,
			Name: addDisk.Name,
		})
	}

	vm, err := r.DVP.ComputeService.CreateVM(ctx, &v1alpha2.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: dvpMachine.Name,
			Labels: map[string]string{
				"dvp.deckhouse.io/hostname": dvpMachine.Name,
			},
		},
		Spec: v1alpha2.VirtualMachineSpec{
			RunPolicy:                v1alpha2.AlwaysOnPolicy,
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
		return nil, fmt.Errorf("create VM: %w", err)
	}

	return vm, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeckhouseMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1a1.DeckhouseMachine{}).
		Named("deckhousemachine").
		Complete(r)
}
