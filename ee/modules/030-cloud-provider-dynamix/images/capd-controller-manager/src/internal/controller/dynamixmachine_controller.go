/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/kvmx86"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"repository.basistech.ru/BASIS/decort-golang-sdk/pkg/cloudapi/compute"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"dynamix-common/api"
	infrastructurev1alpha1 "github.com/deckhouse/deckhouse/api/v1alpha1"
)

const ProviderIDPrefix = "dynamix://"

// DynamixMachineReconciler reconciles a DynamixMachine object
type DynamixMachineReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Dynamix *api.DynamixCloudAPI
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=dynamixmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=dynamixmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=dynamixmachines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DynamixMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *DynamixMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := ctrl.LoggerFrom(ctx)

	dynamixMachine := &infrastructurev1alpha1.DynamixMachine{}
	err := r.Client.Get(ctx, req.NamespacedName, dynamixMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("error getting DynamixMachine: %w", err)
	}

	machine, err := capiutil.GetOwnerMachine(ctx, r.Client, dynamixMachine.ObjectMeta)
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

	dynamixCluster := &infrastructurev1alpha1.DynamixCluster{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Namespace: cluster.Spec.InfrastructureRef.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}, dynamixCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("error getting DynamixCluster: %w", err)
	}

	if annotations.IsPaused(cluster, dynamixMachine) {
		logger.Info("DynamixMachine or linked Cluster is marked as paused. Will not reconcile.")

		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(dynamixMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the dynamixMachine when exiting this function, so we can persist any DynamixMachine changes.
	defer func() {
		if err := patchDynamixMachine(ctx, patchHelper, dynamixMachine); err != nil {
			result = ctrl.Result{}
			reterr = err
		}
	}()

	// Handle deleted machines
	if !dynamixMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, logger, machine, dynamixMachine)
	}

	return r.reconcileNormal(ctx, logger, cluster, dynamixCluster, machine, dynamixMachine)
}

func patchDynamixMachine(
	ctx context.Context,
	patchHelper *patch.Helper,
	dynamixMachine *infrastructurev1alpha1.DynamixMachine,
	options ...patch.Option,
) error {
	conditions.SetSummary(dynamixMachine,
		conditions.WithConditions(infrastructurev1alpha1.VMReadyCondition),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrastructurev1alpha1.VMReadyCondition,
		}},
	)

	return patchHelper.Patch(ctx, dynamixMachine, options...)
}

func (r *DynamixMachineReconciler) reconcileNormal(
	ctx context.Context,
	logger logr.Logger,
	cluster *clusterv1.Cluster,
	dynamixCluster *infrastructurev1alpha1.DynamixCluster,
	machine *clusterv1.Machine,
	dynamixMachine *infrastructurev1alpha1.DynamixMachine,
) (ctrl.Result, error) {
	var err error

	if dynamixMachine.Status.FailureReason != nil || dynamixMachine.Status.FailureMessage != nil {
		logger.Info("DynamixMachine has failed, will not reconcile. See DynamixMachine status for details.")
		return ctrl.Result{}, nil
	}

	// If DynamixMachine is not under finalizer yet, set it now.
	if controllerutil.AddFinalizer(dynamixMachine, infrastructurev1alpha1.MachineFinalizer) {
		return ctrl.Result{}, nil
	}

	if !cluster.Status.InfrastructureReady {
		logger.Info("Waiting for Cluster infrastructure to become Ready")
		conditions.MarkFalse(
			dynamixMachine,
			infrastructurev1alpha1.VMReadyCondition,
			infrastructurev1alpha1.WaitingForClusterInfrastructureReason,
			clusterv1.ConditionSeverityInfo,
			"",
		)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap cloud-init secret reference is missing from Machine")
		conditions.MarkFalse(
			dynamixMachine,
			infrastructurev1alpha1.VMReadyCondition,
			infrastructurev1alpha1.WaitingForBootstrapScriptReason,
			clusterv1.ConditionSeverityInfo,
			"",
		)
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling DynamixMachine")

	vm, err := r.getOrCreateVM(ctx, dynamixCluster, machine, dynamixMachine)
	if err != nil {
		logger.Info("No VM can be found or created for Machine, see DynamixMachine status for details")
		conditions.MarkFalse(
			dynamixMachine,
			infrastructurev1alpha1.VMReadyCondition,
			infrastructurev1alpha1.VMErrorReason,
			clusterv1.ConditionSeverityError,
			"No VM can be found or created for Machine: %v",
			err,
		)
		return ctrl.Result{}, fmt.Errorf("find or create new VM for DynamixMachine: %w", err)
	}

	dynamixMachine.Spec.ID = strconv.FormatUint(vm.ID, 10)
	dynamixMachine.Spec.ProviderID = ProviderIDPrefix + dynamixMachine.Spec.ID

	// Node usually joins the cluster if the CSR generated by kubelet with the node name is approved.
	// The approval happens if the Machine InternalDNS matches the node name, so we add it here along with hostname.
	dynamixMachine.Status.Addresses = []clusterv1.MachineAddress{
		{
			Type:    clusterv1.MachineHostName,
			Address: machine.Name,
		},
		{
			Type:    clusterv1.MachineInternalDNS,
			Address: machine.Name,
		},
	}

	switch vm.TechStatus {
	case "STARTED":
		logger.Info("VM state is UP", "id", vm.ID)

		conditions.MarkTrue(dynamixMachine, infrastructurev1alpha1.VMReadyCondition)

		externalIPList, internalIPList, err := r.Dynamix.ComputeService.GetVMIPAddresses(vm)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get VM IP addresses: %w", err)
		}

		for _, externalIP := range externalIPList {
			dynamixMachine.Status.Addresses = append(dynamixMachine.Status.Addresses, clusterv1.MachineAddress{
				Type:    clusterv1.MachineExternalIP,
				Address: externalIP,
			})
		}

		for _, internalIP := range internalIPList {
			dynamixMachine.Status.Addresses = append(dynamixMachine.Status.Addresses, clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
				Address: internalIP,
			})
		}

		dynamixMachine.Status.Ready = true
	case "DOWN",
		"PAUSED":
		// If the machine has a NodeRef then it must have been working at some point,
		// so the error could be something temporary.
		// If not, it is more likely a configuration error, so we record failure and never retry.
		logger.Info("VM failed", "id", vm.ID, "state", vm.TechStatus)

		if machine.Status.NodeRef == nil {
			err = fmt.Errorf("VM state %q is unexpected", vm.TechStatus)
			dynamixMachine.Status.FailureReason = pointer.String(string(capierrors.UpdateMachineError))
			dynamixMachine.Status.FailureMessage = pointer.String(err.Error())
		}

		conditions.MarkFalse(
			dynamixMachine,
			infrastructurev1alpha1.VMReadyCondition,
			infrastructurev1alpha1.VMInFailedStateReason,
			clusterv1.ConditionSeverityError,
			"",
		)

		return ctrl.Result{}, nil
	case "STOPPED":
		logger.Info("VM is STOPPED, starting it", "id", vm.ID)

		err := r.Dynamix.ComputeService.StartVM(ctx, vm.ID)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to start VM %q : %w", vm.ID, err)
		}
	default:
		// The other states are normal (for example, migration or shutoff) but we don't want to proceed until it's up
		// due to potential conflict or unexpected actions
		logger.Info("Waiting for VM to become STARTED", "id", vm.ID, "status", vm.TechStatus)

		conditions.MarkUnknown(
			dynamixMachine,
			infrastructurev1alpha1.VMReadyCondition,
			infrastructurev1alpha1.VMNotReadyReason,
			"VM is not ready, state is %s",
			vm.TechStatus,
		)

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("Reconciled DynamixMachine successfully")

	return ctrl.Result{}, nil
}

func (r *DynamixMachineReconciler) getOrCreateVM(
	ctx context.Context,
	dynamixCluster *infrastructurev1alpha1.DynamixCluster,
	machine *clusterv1.Machine,
	dynamixMachine *infrastructurev1alpha1.DynamixMachine,
) (*compute.ItemCompute, error) {
	vm, foundVM, err := r.findVMForMachine(ctx, machine)
	if err != nil {
		return nil, fmt.Errorf("lookup VM by name of Machine: %w", err)
	}

	if foundVM {
		return vm, nil
	}

	return r.createVM(ctx, dynamixCluster, machine, dynamixMachine)
}

func (r *DynamixMachineReconciler) createVM(
	ctx context.Context,
	dynamixCluster *infrastructurev1alpha1.DynamixCluster,
	machine *clusterv1.Machine,
	dynamixMachine *infrastructurev1alpha1.DynamixMachine,
) (*compute.ItemCompute, error) {
	bootstrapDataSecret := &corev1.Secret{}
	err := r.Client.Get(
		ctx,
		client.ObjectKey{
			Namespace: machine.GetNamespace(),
			Name:      *machine.Spec.Bootstrap.DataSecretName,
		},
		bootstrapDataSecret,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get cloud-init data secret: %w", err)
	}

	cloudInitScript, hasBootstrapScript := bootstrapDataSecret.Data["value"]
	if !hasBootstrapScript {
		return nil, fmt.Errorf("expected to find a cloud-init script in secret %s/%s", bootstrapDataSecret.Namespace, bootstrapDataSecret.Name)
	}

	vmConfig, err := r.vmConfigFromDynamixMachineSpec(ctx, &dynamixCluster.Spec, &dynamixMachine.Spec, machine.Name, cloudInitScript)
	if err != nil {
		return nil, err
	}

	vm, err := r.Dynamix.ComputeService.CreateVM(
		ctx,
		*vmConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	return vm, nil
}

func (r *DynamixMachineReconciler) reconcileDelete(
	ctx context.Context,
	logger logr.Logger,
	machine *clusterv1.Machine,
	dynamixMachine *infrastructurev1alpha1.DynamixMachine,
) (ctrl.Result, error) {
	logger.Info("Reconciling Machine delete")

	vm, vmFound, err := r.findVMForMachine(ctx, machine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find Dynamix VM for Machine: %w", err)
	}
	if vmFound {
		err := r.Dynamix.ComputeService.DeleteVM(ctx, vm.ID, true)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete Dynamix VM: %w", err)
		}
	} else {
		logger.Info("VM not found in Dynamix, nothing to do")
	}

	controllerutil.RemoveFinalizer(dynamixMachine, infrastructurev1alpha1.MachineFinalizer)

	logger.Info("Reconciled Machine delete successfully")

	return ctrl.Result{}, nil
}

func (r *DynamixMachineReconciler) findVMForMachine(
	ctx context.Context,
	machine *clusterv1.Machine,
) (*compute.ItemCompute, bool, error) {
	vm, err := r.Dynamix.ComputeService.GetVMByName(ctx, machine.Name)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get VM by name: %w", err)
	}

	return vm, true, nil
}

func (r *DynamixMachineReconciler) vmConfigFromDynamixMachineSpec(
	ctx context.Context,
	clusterSpec *infrastructurev1alpha1.DynamixClusterSpec,
	machineSpec *infrastructurev1alpha1.DynamixMachineSpec,
	name string,
	cloudInitScript []byte,
) (*api.VMConfig, error) {
	resourceGroup, err := r.Dynamix.ResourceGroupService.GetResourceGroup(ctx, clusterSpec.ResourceGroup)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource group: %w", err)
	}

	image, err := r.Dynamix.ImageService.GetImageByName(ctx, machineSpec.ImageName)
	if err != nil {
		return nil, fmt.Errorf("failed get VM image %q: %w", machineSpec.ImageName, err)
	}

	externalNetworkID, err := r.Dynamix.ExternalNetworkService.GetExternalNetworkID(ctx, clusterSpec.ExternalNetwork)
	if err != nil {
		return nil, fmt.Errorf("failed to get external network ID: %w", err)
	}

	internalNetworkID, err := r.Dynamix.InternalNetworkService.GetInternalNetworkID(ctx, resourceGroup.ID, clusterSpec.InternalNetwork)
	if err != nil {
		return nil, fmt.Errorf("failed to get internal network ID: %w", err)
	}

	return &api.VMConfig{
		ResourceGroupID: resourceGroup.ID,
		Name:            name,
		ImageID:         image.ID,
		CPU:             uint64(machineSpec.CPU),
		RAM:             uint64(machineSpec.Memory),
		Userdata:        string(cloudInitScript),
		BootDiskSize:    uint64(machineSpec.RootDiskSize),
		Interfaces: []kvmx86.Interface{
			{
				NetType: "EXTNET",
				NetID:   externalNetworkID,
			},
			{
				NetType: "VINS",
				NetID:   internalNetworkID,
			},
		},
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DynamixMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.DynamixMachine{}).
		Complete(r)
}
