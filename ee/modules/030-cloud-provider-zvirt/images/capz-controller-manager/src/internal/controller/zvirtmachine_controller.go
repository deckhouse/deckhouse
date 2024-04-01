/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	ovirt "github.com/ovirt/go-ovirt-client/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrastructurev1 "github.com/deckhouse/deckhouse/api/v1"
	"github.com/deckhouse/deckhouse/internal/controller/utils"
)

const ProviderIDPrefix = "zvirt://"

// ZvirtMachineReconciler reconciles a ZvirtMachine's
type ZvirtMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Zvirt  ovirt.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=zvirtmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=zvirtmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=zvirtmachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ZvirtMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := ctrl.LoggerFrom(ctx)

	zvMachine := &infrastructurev1.ZvirtMachine{}
	err := r.Client.Get(ctx, req.NamespacedName, zvMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("Error getting ZvirtMachine: %w", err)
	}

	machine, err := capiutil.GetOwnerMachine(ctx, r.Client, zvMachine.ObjectMeta)
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

	zvCluster := &infrastructurev1.ZvirtCluster{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}, zvCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("Error getting ZvirtCluster: %w", err)
	}

	if annotations.IsPaused(cluster, zvMachine) {
		logger.Info("ZvirtMachine or linked Cluster is marked as paused. Will not reconcile.")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(zvMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the zvMachine when exiting this function, so we can persist any ZvirtMachine changes.
	defer func() {
		if err := patchZvirtMachine(ctx, patchHelper, zvMachine); err != nil {
			result = ctrl.Result{}
			reterr = err
		}
	}()

	// Handle deleted machines
	if !zvMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, logger, machine, zvMachine)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, logger, cluster, machine, zvMachine, zvCluster)
}

func patchZvirtMachine(
	ctx context.Context,
	patchHelper *patch.Helper,
	zvirtMachine *infrastructurev1.ZvirtMachine,
	options ...patch.Option,
) error {
	conditions.SetSummary(zvirtMachine,
		conditions.WithConditions(infrastructurev1.VMReadyCondition),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrastructurev1.VMReadyCondition,
		}},
	)
	return patchHelper.Patch(ctx, zvirtMachine, options...)
}

const (
	expectedNICName = "eth0"
)

func (r *ZvirtMachineReconciler) reconcileNormal(
	ctx context.Context,
	logger logr.Logger,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine,
	zvMachine *infrastructurev1.ZvirtMachine,
	zvCluster *infrastructurev1.ZvirtCluster,
) (ctrl.Result, error) {
	var err error

	if zvMachine.Status.FailureReason != nil || zvMachine.Status.FailureMessage != nil {
		logger.Info("ZvirtMachine has failed, will not reconcile. See ZvirtMachine status for details.")
		return ctrl.Result{}, nil
	}

	// If ZvirtMachine is not under finalizer yet, set it now.
	if controllerutil.AddFinalizer(zvMachine, infrastructurev1.MachineFinalizer) {
		return ctrl.Result{}, nil
	}

	if !cluster.Status.InfrastructureReady {
		logger.Info("Waiting for Cluster infrastructure to become Ready")
		conditions.MarkFalse(
			zvMachine,
			infrastructurev1.VMReadyCondition,
			infrastructurev1.WaitingForClusterInfrastructureReason,
			clusterv1.ConditionSeverityInfo,
			"",
		)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap cloud-init secret reference is missing from Machine")
		conditions.MarkFalse(
			zvMachine,
			infrastructurev1.VMReadyCondition,
			infrastructurev1.WaitingForBootstrapScriptReason,
			clusterv1.ConditionSeverityInfo,
			"",
		)
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling ZvirtMachine")

	vm, err := r.getOrCreateVM(ctx, machine, zvMachine, zvCluster)
	if err != nil {
		logger.Info("No VM can be found or created for Machine, see ZvirtMachine status for details")
		conditions.MarkFalse(
			zvMachine,
			infrastructurev1.VMReadyCondition,
			infrastructurev1.VMErrorReason,
			clusterv1.ConditionSeverityError,
			"No VM can be found or created for Machine: %v",
			err,
		)
		return ctrl.Result{}, fmt.Errorf("Find or create new VM for ZvirtMachine: %w", err)
	}

	vmid := vm.ID()
	vmMisconfigured, err := r.checkIfVirtualMachineIsMisconfigured(ctx, vmid, zvMachine, logger)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Check if VM is misconfigured and should be recreated: %w", err)
	}

	zVirtClient := r.Zvirt.WithContext(ctx)
	if vmMisconfigured {
		// TODO(mvasl) We probably should detach all of the disks except bootable before removing VM.
		_ = zVirtClient.StopVM(vmid, true)
		if err = zVirtClient.RemoveVM(vmid); err != nil {
			return ctrl.Result{}, fmt.Errorf("Cannot delete misconfigured VM: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	zvMachine.Spec.ID = string(vmid)
	zvMachine.Spec.ProviderID = string(ProviderIDPrefix + vmid)

	// Node usually joins the cluster if the CSR generated by kubelet with the node name is approved.
	// The approval happens if the Machine InternalDNS matches the node name, so we add it here along with hostname.
	zvMachine.Status.Addresses = []infrastructurev1.VMAddress{
		{Type: clusterv1.MachineHostName, Address: machine.Name},
		{Type: clusterv1.MachineInternalDNS, Address: machine.Name},
	}

	vmStatus := vm.Status()
	switch vmStatus {
	case ovirt.VMStatusUp:
		logger.Info("VM state is UP", "id", vmid)
		conditions.MarkTrue(zvMachine, infrastructurev1.VMReadyCondition)
		zvMachine.Status.Ready = true

		addrs, err := zVirtClient.WaitForNonLocalVMIPAddress(vmid)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("Tired of waiting for VM to get IP address: %w", err)
		}

		machineAddress := ""
		nicAddrs, hasAddr := addrs[expectedNICName]
		if !hasAddr {
			return ctrl.Result{}, fmt.Errorf(
				"Expected vNIC %q to be attached to VM %q and configured non-loopback IP address",
				zvMachine.Spec.NicName,
				machine.Name,
			)
		}

		for _, ip := range nicAddrs {
			if !ip.IsLoopback() {
				machineAddress = ip.String()
			}
		}

		if machineAddress == "" {
			return ctrl.Result{}, fmt.Errorf(
				"Expected vNIC %q to be attached to VM %q and configured non-loopback IP address",
				zvMachine.Spec.NicName,
				machine.Name,
			)
		}

		zvMachine.Status.Addresses = append(zvMachine.Status.Addresses, []infrastructurev1.VMAddress{
			{Type: clusterv1.MachineInternalIP, Address: machineAddress},
			{Type: clusterv1.MachineExternalIP, Address: machineAddress},
		}...)
	case ovirt.VMStatusNotResponding,
		ovirt.VMStatusPaused,
		ovirt.VMStatusSuspended,
		ovirt.VMStatusUnassigned,
		ovirt.VMStatusUnknown:
		// If the machine has a NodeRef then it must have been working at some point,
		// so the error could be something temporary.
		// If not, it is more likely a configuration error, so we record failure and never retry.
		logger.Info("VM failed", "id", vmid, "state", vmStatus)
		if machine.Status.NodeRef == nil {
			err = fmt.Errorf("VM state %q is unexpected", vmStatus)
			zvMachine.Status.FailureReason = pointer.String(string(capierrors.UpdateMachineError))
			zvMachine.Status.FailureMessage = pointer.String(err.Error())
		}
		conditions.MarkFalse(
			zvMachine,
			infrastructurev1.VMReadyCondition,
			infrastructurev1.VMInFailedStateReason,
			clusterv1.ConditionSeverityError,
			"",
		)
		return ctrl.Result{}, nil
	case ovirt.VMStatusDown:
		logger.Info("VM is DOWN, starting it", "id", vmid)
		if err = zVirtClient.StartVM(vmid); err != nil {
			return ctrl.Result{}, fmt.Errorf("Cannot start VM %q : %w", vmid, err)
		}
		_, err = zVirtClient.WaitForVMStatus(vmid, ovirt.VMStatusUp)
		if err != nil {
			conditions.MarkFalse(
				zvMachine,
				infrastructurev1.VMReadyCondition,
				infrastructurev1.VMErrorReason,
				clusterv1.ConditionSeverityError,
				"%v", err,
			)
			return ctrl.Result{}, err
		}
	default:
		// The other states are normal (for example, migration or shutoff) but we don't want to proceed until it's up
		// due to potential conflict or unexpected actions
		logger.Info("Waiting for VM to become UP", "id", vmid, "status", vmStatus)
		conditions.MarkUnknown(
			zvMachine,
			infrastructurev1.VMReadyCondition,
			infrastructurev1.VMNotReadyReason,
			"VM is not ready, state is %s",
			vmStatus,
		)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("Reconciled ZvirtMachine successfully")
	return ctrl.Result{}, nil
}

func (r *ZvirtMachineReconciler) getOrCreateVM(
	ctx context.Context,
	machine *clusterv1.Machine,
	zvMachine *infrastructurev1.ZvirtMachine,
	zvCluster *infrastructurev1.ZvirtCluster,
	retryStrategy ...ovirt.RetryStrategy,
) (ovirt.VM, error) {
	vm, foundVM, err := r.findVMForMachine(ctx, machine, retryStrategy...)
	if err != nil {
		return nil, fmt.Errorf("Lookup VM by name of Machine: %w", err)
	}

	if foundVM {
		return vm, nil
	}

	return r.createVM(ctx, machine, zvMachine, zvCluster, retryStrategy...)
}

func (r *ZvirtMachineReconciler) createVM(
	ctx context.Context,
	machine *clusterv1.Machine,
	zvMachine *infrastructurev1.ZvirtMachine,
	zvCluster *infrastructurev1.ZvirtCluster,
	retryStrategy ...ovirt.RetryStrategy,
) (ovirt.VM, error) {
	dataSecretName := *machine.Spec.Bootstrap.DataSecretName
	bootstrapDataSecret := &corev1.Secret{}
	if err := r.Client.Get(
		ctx,
		client.ObjectKey{
			Namespace: machine.GetNamespace(),
			Name:      dataSecretName,
		},
		bootstrapDataSecret,
	); err != nil {
		return nil, fmt.Errorf("Cannot get cloud-init data secret: %w", err)
	}

	cloudInitScript, hasBootstrapScript := bootstrapDataSecret.Data["value"]
	if !hasBootstrapScript {
		return nil, fmt.Errorf("Expected to find a cloud-init script in secret %s/%s", bootstrapDataSecret.Namespace, bootstrapDataSecret.Name)
	}

	vmConfig, err := r.vmConfigFromZvirtMachineSpec(machine.Name, &zvMachine.Spec, cloudInitScript)
	if err != nil {
		return nil, err
	}

	zVirtClient := r.Zvirt.WithContext(ctx)

	tpl, err := zVirtClient.GetTemplateByName(zvMachine.Spec.TemplateName, retryStrategy...)
	if err != nil {
		return nil, fmt.Errorf("Cannot get VM template %q: %w", zvMachine.Spec.TemplateName, err)
	}

	vm, err := zVirtClient.CreateVM(ovirt.ClusterID(zvCluster.Spec.ID), tpl.ID(), machine.Name, vmConfig, retryStrategy...)
	if err != nil {
		return nil, fmt.Errorf("Cannot create VM: %w", err)
	}

	vmid := vm.ID()

	if _, err = zVirtClient.WaitForVMStatus(vmid, ovirt.VMStatusDown, retryStrategy...); err != nil {
		return nil, fmt.Errorf("Tired of waiting for VM to be created: %w", err)
	}

	disks, err := zVirtClient.ListDiskAttachments(vmid, retryStrategy...)
	if err != nil {
		return nil, fmt.Errorf("Cannot resize VM boot disk: %w", err)
	}
	if len(disks) == 0 {
		return nil, fmt.Errorf("VM created without disks, check if your template is configured correctly: %w", err)
	}

	diskResized := false
	for _, diskAttach := range disks {
		if diskAttach.Bootable() && diskAttach.Active() {
			diskParams, err := ovirt.UpdateDiskParams().
				WithProvisionedSize(uint64(zvMachine.Spec.RootDiskSize * 1024 * 1024 * 1024))
			if err != nil {
				return nil, fmt.Errorf("Cannot resize VM boot disk: %w", err)
			}

			if _, err = zVirtClient.UpdateDisk(diskAttach.DiskID(), diskParams); err != nil {
				return nil, fmt.Errorf("Cannot resize VM boot disk: %w", err)
			}

			diskResized = true
			break
		}
	}
	if !diskResized {
		return nil, fmt.Errorf("Cannot find any active bootable disks on created VM, check if your template is configured correctly: %w", err)
	}

	_, err = zVirtClient.CreateNIC(
		vmid,
		ovirt.VNICProfileID(zvMachine.Spec.VNICProfileID),
		zvMachine.Spec.NicName,
		nil,
		retryStrategy...,
	)
	if err != nil {
		return nil, fmt.Errorf("Attach vNIC to the VM: %w", err)
	}

	if err = zVirtClient.StartVM(vmid, retryStrategy...); err != nil {
		return nil, fmt.Errorf("Cannot start VM: %w", err)
	}

	addrs, err := zVirtClient.WaitForNonLocalVMIPAddress(vmid, retryStrategy...)
	if err != nil {
		return nil, fmt.Errorf("Tired of waiting for VM to get IP address: %w", err)
	}

	machineAddress := ""
	nicAddrs, hasAddr := addrs[expectedNICName]
	if !hasAddr {
		return nil, fmt.Errorf(
			"Expected vNIC %q to be attached to VM %q and configured with non-loopback IP address",
			zvMachine.Spec.NicName,
			machine.Name,
		)
	}

	for _, ip := range nicAddrs {
		if !ip.IsLoopback() {
			machineAddress = ip.String()
		}
	}

	if machineAddress == "" {
		return nil, fmt.Errorf(
			"Expected vNIC %q to be attached to VM %q and configured with non-loopback IP address",
			zvMachine.Spec.NicName,
			machine.Name,
		)
	}

	return vm, nil
}

func (r *ZvirtMachineReconciler) checkIfVirtualMachineIsMisconfigured(
	ctx context.Context,
	vmid ovirt.VMID,
	zvMachine *infrastructurev1.ZvirtMachine,
	logger logr.Logger,
	retryStrategy ...ovirt.RetryStrategy,
) (bool, error) {
	zVirtClient := r.Zvirt.WithContext(ctx)

	nics, err := zVirtClient.ListNICs(vmid, retryStrategy...)
	if err != nil {
		return false, fmt.Errorf("Error checking if VM was configured properly: %w", err)
	}

	disks, err := zVirtClient.ListDiskAttachments(vmid, retryStrategy...)
	if err != nil {
		return false, fmt.Errorf("Error checking if VM was configured properly: %w", err)
	}

	if len(nics) != 1 || string(nics[0].VNICProfileID()) != zvMachine.Spec.VNICProfileID {
		logger.Info("VM is not configured properly and will be replaced: expected NIC not present")
		return true, nil
	}

	diskFound := false
	for _, diskAttachment := range disks {
		if diskAttachment.Active() && diskAttachment.Bootable() {
			diskFound = true
			break
		}
	}
	if !diskFound {
		logger.Info("VM is not configured properly and will be replaced: expected disk not attached")
		return true, nil
	}

	return false, nil
}

func (r *ZvirtMachineReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	machine *clusterv1.Machine,
	zvMachine *infrastructurev1.ZvirtMachine,
) (ctrl.Result, error) {
	logger.Info("Reconciling Machine delete")
	zVirtClient := r.Zvirt.WithContext(ctx)

	vm, vmFound, err := r.findVMForMachine(ctx, machine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Find zVirt VM for Machine: %w", err)
	}
	if vmFound {
		vmid := vm.ID()
		if err = zVirtClient.ShutdownVM(vmid, true); err != nil {
			return ctrl.Result{}, fmt.Errorf("Shutdown zVirt VM: %w", err)
		}

		disks, err := zVirtClient.ListDiskAttachments(vmid)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("List zVirt VM disks: %w", err)
		}
		for _, disk := range disks {
			if !disk.Bootable() {
				if err = disk.Remove(); err != nil {
					return ctrl.Result{}, fmt.Errorf("Detach disk %v from vm %s. %w", disk.DiskID(), vm.Name(), err)
				}
			}
		}
		if err = zVirtClient.RemoveVM(vmid); err != nil {
			return ctrl.Result{}, fmt.Errorf("Delete zVirt VM: %w", err)
		}
	} else {
		logger.Info("VM not found in zVirt, nothing to do")
	}

	controllerutil.RemoveFinalizer(zvMachine, infrastructurev1.MachineFinalizer)
	logger.Info("Reconciled Machine delete successfully")
	return ctrl.Result{}, nil
}

func (r *ZvirtMachineReconciler) findVMForMachine(
	ctx context.Context,
	machine *clusterv1.Machine,
	retryStrategy ...ovirt.RetryStrategy,
) (ovirt.VM, bool, error) {
	vm, err := r.Zvirt.WithContext(ctx).GetVMByName(machine.Name, retryStrategy...)
	switch {
	case err != nil && ovirt.HasErrorCode(err, ovirt.ENotFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}

	return vm, true, nil
}

func (r *ZvirtMachineReconciler) vmConfigFromZvirtMachineSpec(
	hostname string,
	machineSpec *infrastructurev1.ZvirtMachineSpec,
	cloudInitScript []byte,
) (ovirt.BuildableVMParameters, error) {
	ramBytes := int64(machineSpec.Memory) * 1024 * 1024

	vmType := ovirt.VMTypeHighPerformance

	vmConfig := ovirt.NewCreateVMParams()
	vmConfig = vmConfig.MustWithClone(true).
		MustWithCPU(
			ovirt.NewVMCPUParams().
				MustWithTopo(
					ovirt.MustNewVMCPUTopo(uint(machineSpec.CPU.Cores), uint(machineSpec.CPU.Threads), uint(machineSpec.CPU.Sockets)),
				),
		)
	vmConfig = vmConfig.MustWithMemory(ramBytes)
	vmConfig = vmConfig.MustWithVMType(vmType)
	vmConfig = vmConfig.WithMemoryPolicy(
		ovirt.NewMemoryPolicyParameters().
			MustWithBallooning(false).
			MustWithMax(ramBytes).
			MustWithGuaranteed(ramBytes),
	)

	encodedCloudInit, err := utils.XMLEncode(cloudInitScript)
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare cloud-init script for VM: %w", err)
	}

	vmConfig, err = vmConfig.WithInitialization(ovirt.NewInitialization(encodedCloudInit, hostname))
	if err != nil {
		return nil, fmt.Errorf("Cannot prepare cloud-init script for VM: %w", err)
	}

	return vmConfig, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZvirtMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1.ZvirtMachine{}).
		Complete(r)
}
