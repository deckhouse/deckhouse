package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	compute "github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "cluster-api-provider-yandex/api/v1alpha1"
	"cluster-api-provider-yandex/internal/yc"
)

type YandexMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	YC     *yc.Client
}

func (r *YandexMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)

	yandexMachine := &infrastructurev1alpha1.YandexMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, yandexMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get YandexMachine: %w", err)
	}

	machine, err := capiutil.GetOwnerMachine(ctx, r.Client, yandexMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	cluster, err := capiutil.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, yandexMachine) {
		logger.Info("YandexMachine or linked Cluster is marked as paused. Will not reconcile.")
		return ctrl.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(yandexMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := patchHelper.Patch(ctx, yandexMachine, patch.WithOwnedConditions{Conditions: []string{
			string(clusterv1.ReadyCondition),
			string(infrastructurev1alpha1.VMReadyCondition),
		}}); err != nil {
			result = ctrl.Result{}
			reterr = err
		}
	}()

	if !yandexMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, yandexMachine)
	}

	if controllerutil.AddFinalizer(yandexMachine, infrastructurev1alpha1.MachineFinalizer) {
		return ctrl.Result{}, nil
	}

	if !conditions.IsTrue(cluster, clusterv1.InfrastructureReadyCondition) {
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.WaitingForClusterInfrastructureReason,
			Message:            "Cluster infrastructure is not ready yet",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.WaitingForBootstrapScriptReason,
			Message:            "Bootstrap cloud-init secret is missing",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{}, nil
	}

	instance, err := r.getOrCreateInstance(ctx, yandexMachine, machine.Namespace, *machine.Spec.Bootstrap.DataSecretName)
	if err != nil {
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.VMErrorReason,
			Message:            fmt.Sprintf("Failed to reconcile instance: %v", err),
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{}, err
	}

	return r.syncMachineStatus(yandexMachine, instance)
}

func (r *YandexMachineReconciler) reconcileDelete(ctx context.Context, yandexMachine *infrastructurev1alpha1.YandexMachine) (ctrl.Result, error) {
	instanceID := providerIDToInstanceID(yandexMachine.Spec.ProviderID)
	if instanceID == "" {
		controllerutil.RemoveFinalizer(yandexMachine, infrastructurev1alpha1.MachineFinalizer)
		return ctrl.Result{}, nil
	}

	instance, err := r.YC.GetInstance(ctx, instanceID)
	if err != nil {
		if isNotFoundErr(err) {
			controllerutil.RemoveFinalizer(yandexMachine, infrastructurev1alpha1.MachineFinalizer)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if instance != nil {
		if err := r.YC.DeleteInstance(ctx, instanceID); err != nil && !isNotFoundErr(err) {
			conditions.Set(yandexMachine, metav1.Condition{
				Type:               string(infrastructurev1alpha1.VMReadyCondition),
				Status:             metav1.ConditionFalse,
				Reason:             infrastructurev1alpha1.VMDeletingReason,
				Message:            fmt.Sprintf("Failed to delete instance: %v", err),
				LastTransitionTime: metav1.Now(),
			})
			return ctrl.Result{}, err
		}
	}

	controllerutil.RemoveFinalizer(yandexMachine, infrastructurev1alpha1.MachineFinalizer)
	return ctrl.Result{}, nil
}

func (r *YandexMachineReconciler) getOrCreateInstance(
	ctx context.Context,
	yandexMachine *infrastructurev1alpha1.YandexMachine,
	namespace string,
	bootstrapSecretName string,
) (*compute.Instance, error) {
	instanceID := providerIDToInstanceID(yandexMachine.Spec.ProviderID)
	if instanceID != "" {
		instance, err := r.YC.GetInstance(ctx, instanceID)
		if err == nil {
			return instance, nil
		}
		if !isNotFoundErr(err) {
			return nil, err
		}
		yandexMachine.Spec.ProviderID = ""
	}

	instance, err := r.YC.FindInstanceByName(ctx, yandexMachine.Spec.FolderID, yandexMachine.Name)
	if err != nil {
		return nil, fmt.Errorf("find instance by name: %w", err)
	}
	if instance != nil {
		yandexMachine.Spec.ProviderID = infrastructurev1alpha1.ProviderIDPrefix + instance.GetId()
		return instance, nil
	}

	bootstrapSecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      bootstrapSecretName,
	}, bootstrapSecret); err != nil {
		return nil, fmt.Errorf("get bootstrap secret %s/%s: %w", namespace, bootstrapSecretName, err)
	}

	bootstrapData, ok := bootstrapSecret.Data["value"]
	if !ok || len(bootstrapData) == 0 {
		return nil, fmt.Errorf("bootstrap secret %s/%s has no data.value", namespace, bootstrapSecretName)
	}

	spec := yc.MachineSpec{
		Name:              yandexMachine.Name,
		FolderID:          yandexMachine.Spec.FolderID,
		Zone:              yandexMachine.Spec.Zone,
		PlatformID:        yandexMachine.Spec.PlatformID,
		Cores:             int64(yandexMachine.Spec.Resources.Cores),
		CoreFraction:      int64(yandexMachine.Spec.Resources.CoreFraction),
		MemoryBytes:       yandexMachine.Spec.Resources.MemoryMiB * 1024 * 1024,
		GPUs:              int64(yandexMachine.Spec.Resources.GPUs),
		BootDiskType:      yandexMachine.Spec.BootDisk.Type,
		BootDiskSizeBytes: int64(yandexMachine.Spec.BootDisk.SizeGiB) * 1024 * 1024 * 1024,
		BootDiskImageID:   yandexMachine.Spec.BootDisk.ImageID,
		Hostname:          yandexMachine.Name,
		NetworkType:       yandexMachine.Spec.NetworkType,
		Labels:            copyStringMap(yandexMachine.Spec.Labels),
		Metadata:          copyStringMap(yandexMachine.Spec.Metadata),
		NetworkInterfaces: make([]yc.NetworkInterfaceSpec, 0, len(yandexMachine.Spec.NetworkInterfaces)),
	}

	if yandexMachine.Spec.SchedulingPolicy != nil {
		spec.Preemptible = yandexMachine.Spec.SchedulingPolicy.Preemptible
	}

	for _, nic := range yandexMachine.Spec.NetworkInterfaces {
		spec.NetworkInterfaces = append(spec.NetworkInterfaces, yc.NetworkInterfaceSpec{
			SubnetID:              nic.SubnetID,
			AssignPublicIPAddress: nic.AssignPublicIPAddress,
		})
	}

	if spec.Metadata == nil {
		spec.Metadata = map[string]string{}
	}
	spec.Metadata["user-data"] = string(bootstrapData)

	instance, err = r.YC.CreateInstance(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}

	yandexMachine.Spec.ProviderID = infrastructurev1alpha1.ProviderIDPrefix + instance.GetId()
	return instance, nil
}

func (r *YandexMachineReconciler) syncMachineStatus(
	yandexMachine *infrastructurev1alpha1.YandexMachine,
	instance *compute.Instance,
) (ctrl.Result, error) {
	yandexMachine.Status.Addresses = buildMachineAddresses(yandexMachine.Name, instance)

	switch instance.GetStatus() {
	case compute.Instance_RUNNING:
		yandexMachine.Status.Ready = true
		provisioned := true
		yandexMachine.Status.Initialization.Provisioned = &provisioned
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionTrue,
			Reason:             clusterv1.ReadyCondition,
			Message:            "Instance is running",
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{}, nil
	case compute.Instance_PROVISIONING, compute.Instance_STARTING, compute.Instance_RESTARTING, compute.Instance_UPDATING:
		yandexMachine.Status.Ready = false
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.VMCreatingReason,
			Message:            fmt.Sprintf("Instance is in %s state", instance.GetStatus().String()),
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	case compute.Instance_STOPPED:
		yandexMachine.Status.Ready = false
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.VMInStoppedStateReason,
			Message:            "Instance is stopped",
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	case compute.Instance_ERROR, compute.Instance_CRASHED:
		yandexMachine.Status.Ready = false
		reason := infrastructurev1alpha1.VMInFailedStateReason
		message := fmt.Sprintf("Instance is in %s state", instance.GetStatus().String())
		yandexMachine.Status.FailureReason = &reason
		yandexMachine.Status.FailureMessage = &message
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.VMInFailedStateReason,
			Message:            message,
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{}, nil
	case compute.Instance_DELETING:
		yandexMachine.Status.Ready = false
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.VMDeletingReason,
			Message:            "Instance is deleting",
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	default:
		yandexMachine.Status.Ready = false
		conditions.Set(yandexMachine, metav1.Condition{
			Type:               string(infrastructurev1alpha1.VMReadyCondition),
			Status:             metav1.ConditionFalse,
			Reason:             infrastructurev1alpha1.VMNotReadyReason,
			Message:            fmt.Sprintf("Instance is in %s state", instance.GetStatus().String()),
			LastTransitionTime: metav1.Now(),
		})
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

func buildMachineAddresses(machineName string, instance *compute.Instance) []clusterv1.MachineAddress {
	addresses := []clusterv1.MachineAddress{
		{Type: clusterv1.MachineHostName, Address: machineName},
		{Type: clusterv1.MachineInternalDNS, Address: machineName},
	}

	if fqdn := strings.TrimSpace(instance.GetFqdn()); fqdn != "" {
		addresses = append(addresses, clusterv1.MachineAddress{Type: clusterv1.MachineInternalDNS, Address: fqdn})
	}

	for _, nic := range instance.GetNetworkInterfaces() {
		if primary := nic.GetPrimaryV4Address(); primary != nil {
			if ip := strings.TrimSpace(primary.GetAddress()); ip != "" {
				addresses = append(addresses, clusterv1.MachineAddress{Type: clusterv1.MachineInternalIP, Address: ip})
			}
			if nat := primary.GetOneToOneNat(); nat != nil {
				if ip := strings.TrimSpace(nat.GetAddress()); ip != "" {
					addresses = append(addresses, clusterv1.MachineAddress{Type: clusterv1.MachineExternalIP, Address: ip})
				}
			}
		}
	}

	return dedupMachineAddresses(addresses)
}

func dedupMachineAddresses(items []clusterv1.MachineAddress) []clusterv1.MachineAddress {
	result := make([]clusterv1.MachineAddress, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := string(item.Type) + "|" + item.Address
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func providerIDToInstanceID(providerID string) string {
	return strings.TrimPrefix(providerID, infrastructurev1alpha1.ProviderIDPrefix)
}

func copyStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func isNotFoundErr(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

func (r *YandexMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.YandexMachine{}).
		Named("yandexmachine").
		Complete(r)
}
