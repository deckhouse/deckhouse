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

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/client"
	"caps-controller-manager/internal/event"
	"caps-controller-manager/internal/pool"
)

const (
	DefaultStaticInstanceBootstrapTimeout = 20 * time.Minute
	DefaultStaticInstanceCleanupTimeout   = 10 * time.Minute
	DefaultStaticInstanceAdoptTimeout     = 5 * time.Minute
	RequeueForStaticInstancePending       = 10 * time.Second
	RequeueForStaticInstanceCleaning      = 30 * time.Second
	RequeueForStaticMachineDeleting       = 5 * time.Second
)

var StaticInstanceCleanupTimedOut = errors.New("timed out waiting for StaticInstance to clean up")
var StaticMachineBootstrapTimedOut = errors.New("timed out waiting for StaticInstance to bootstrap")
var StaticMachineAdoptTimedOut = errors.New("timed out waiting for StaticInstance to adopt")

// StaticMachineReconciler reconciles a StaticMachine object
type StaticMachineReconciler struct {
	k8sClient.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	HostClient *client.Client
	Recorder   *event.Recorder
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// +kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StaticMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *StaticMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, resErr error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Reconciling StaticMachine")

	staticMachine := &infrav1.StaticMachine{}
	if err := r.Get(ctx, req.NamespacedName, staticMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get StaticMachine: %w", err)
	}

	defer func() {
		staticMachinePatchHelper, err := patch.NewHelper(staticMachine, r.Client)
		if err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to create staticMachine patch helper: %w", err))
			return
		}

		if err = patchStaticMachine(ctx, staticMachinePatchHelper, staticMachine); err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to patch staticMachine: %w", err))
		}
	}()

	machine, err := util.GetOwnerMachine(ctx, r.Client, staticMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get Machine: &w", err)
	}
	if machine == nil {
		logger.Info("Machine Controller has not yet set OwnerRef. Won't reconcile")
		return ctrl.Result{}, nil
	}

	defer func() {
		machinePatchHelper, err := patch.NewHelper(machine, r.Client)
		if err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to create Machine patch helper: %w", err))
			return
		}

		if err = machinePatchHelper.Patch(ctx, machine); err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to patch Machine: %w", err))
		}
	}()

	logger = logger.WithValues("machine", machine.Name)
	ctx = ctrl.LoggerInto(ctx, logger)

	nodeGroupLabel, ok := machine.Labels["node-group"]
	if !ok {
		machine.Labels["node-group"] = staticMachine.Labels["node-group"]
	} else if nodeGroupLabel != staticMachine.Labels["node-group"] {
		logger.Info("'node-group' label in StaticMachine and Machine are different. Won't reconcile")
		return ctrl.Result{}, nil
	}

	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, staticMachine.ObjectMeta)
	if err != nil {
		logger.Info("Machine is missing cluster label or cluster does not exist. Won't reconcile")
		return ctrl.Result{}, nil
	}

	instances := &deckhousev1.StaticInstanceList{}
	uidSelector := fields.OneTermEqualSelector("status.machineRef.uid", string(staticMachine.UID))
	if err = r.List(
		ctx,
		instances,
		k8sClient.MatchingFieldsSelector{Selector: uidSelector},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find StaticInstance by static machine uid %s: %w", string(staticMachine.UID), err)
	}

	var staticInstance *deckhousev1.StaticInstance
	if len(instances.Items) != 0 {
		staticInstance = &instances.Items[0]
		logger = logger.WithValues("staticInstance", staticInstance.Name)
		ctx = ctrl.LoggerInto(ctx, logger)
	}

	defer func() {
		if staticInstance != nil {
			staticInstancePatchHelper, err := patch.NewHelper(staticInstance, r.Client)
			if err != nil {
				resErr = errors.Join(resErr, fmt.Errorf("failed to create StaticInstance patch helper: %w", err))
				return
			}

			if err = patchStaticInstance(ctx, staticInstancePatchHelper, staticInstance); err != nil {
				resErr = errors.Join(resErr, fmt.Errorf("failed to patch StaticInstance: %w", err))
			}
		}
	}()

	// Return early if the object or Cluster is paused
	if annotations.IsPaused(cluster, staticMachine) {
		logger.Info("StaticMachine or linked Cluster is marked as paused. Won't reconcile")

		if staticInstance != nil {
			// set paused annotation
			desired := map[string]string{
				clusterv1.PausedAnnotation: "",
			}
			annotations.AddAnnotations(staticInstance, desired)
		}

		conditions.Set(staticMachine, metav1.Condition{
			Type:               infrav1.StaticMachineStaticInstanceReadyCondition,
			Reason:             infrav1.ClusterOrResourcePausedReason,
			Status:             metav1.ConditionFalse,
			Message:            "StaticMachine is paused",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{}, nil
	}

	if staticInstance != nil {
		delete(staticInstance.Annotations, clusterv1.PausedAnnotation)
	}

	// Handle deleted machines
	if !staticMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Reconciling delete StaticMachine")
		return r.reconcileDelete(ctx, machine, staticMachine, staticInstance)
	}

	return r.reconcileNormal(ctx, cluster, machine, staticMachine, staticInstance)
}

func (r *StaticMachineReconciler) reconcileNormal(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	machine *clusterv1.Machine,
	staticMachine *infrav1.StaticMachine,
	staticInstance *deckhousev1.StaticInstance,
) (res ctrl.Result, resErr error) {
	logger := ctrl.LoggerFrom(ctx)
	if staticMachine.Status.FailureReason != nil || staticMachine.Status.FailureMessage != nil {
		logger.Info("StaticMachine has failed, will not reconcile. See StaticMachine status for details.")
		return ctrl.Result{}, nil
	}

	// If the StaticMachine is not under finalizer yet, set it now.
	if controllerutil.AddFinalizer(staticMachine, infrav1.MachineFinalizer) {
		return ctrl.Result{}, nil
	}

	if !conditions.IsTrue(cluster, clusterv1.InfrastructureReadyCondition) {
		logger.Info("Cluster infrastructure is not ready yet, requeuing")
		conditions.Set(staticMachine, metav1.Condition{
			Type:               infrav1.StaticMachineStaticInstanceReadyCondition,
			Reason:             infrav1.StaticMachineWaitingForClusterInfrastructureReason,
			Status:             metav1.ConditionFalse,
			Message:            "Cluster infrastructure is not ready yet",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{RequeueAfter: RequeueForStaticInstancePending}, nil
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Bootstrap Data Secret not available yet")
		conditions.Set(staticMachine, metav1.Condition{
			Type:               infrav1.StaticMachineStaticInstanceReadyCondition,
			Reason:             infrav1.StaticMachineWaitingForBootstrapDataSecretReason,
			Status:             metav1.ConditionFalse,
			Message:            "Bootstrap Data Secret not available yet",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{}, nil
	}

	if staticInstance != nil {
		return r.reconcileStaticInstancePhase(ctx, machine, staticMachine, staticInstance)
	}

	// If there is not yet a StaticInstance for this StaticMachine,
	// then pick one from the static instance pool
	newStaticInstance, err := pool.NewStaticInstancePool(r.Client, r.Config, r.Recorder).PickStaticInstance(ctx, staticMachine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to pick StaticInstance: %w", err)
	}
	if newStaticInstance == nil {
		logger.Info("No pending StaticInstance available, requeuing")
		r.Recorder.SendWarningEvent(staticMachine, staticMachine.Labels["node-group"], "StaticInstanceSelectionFailed", "No available StaticInstance")
		conditions.Set(staticMachine, metav1.Condition{
			Type:               infrav1.StaticMachineStaticInstanceReadyCondition,
			Reason:             infrav1.StaticMachineStaticInstancesUnavailableReason,
			Status:             metav1.ConditionFalse,
			Message:            "No available StaticInstance",
			LastTransitionTime: metav1.Now(),
		})

		return ctrl.Result{RequeueAfter: RequeueForStaticInstancePending}, nil
	}

	logger = logger.WithValues("staticInstance", newStaticInstance.Name)
	ctx = ctrl.LoggerInto(ctx, logger)

	defer func() {
		staticInstancePatchHelper, err := patch.NewHelper(newStaticInstance, r.Client)
		if err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to create StaticInstance patch helper: %w", err))
			return
		}

		if err = patchStaticInstance(ctx, staticInstancePatchHelper, newStaticInstance); err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to patch StaticInstance: %w", err))
		}
	}()

	logger.Info("picked StaticInstance")
	r.Recorder.SendNormalEvent(newStaticInstance, staticMachine.Labels["node-group"], "StaticInstanceAttachSucceeded", fmt.Sprintf("Attached to StaticMachine %s", staticMachine.Name))
	r.Recorder.SendNormalEvent(staticMachine, staticMachine.Labels["node-group"], "StaticInstanceAttachSucceeded", fmt.Sprintf("Attached StaticInstance %s", newStaticInstance.Name))

	_, shouldSkipBootstrap := newStaticInstance.Annotations[deckhousev1.SkipBootstrapPhaseAnnotation]
	if shouldSkipBootstrap {
		result, err := r.HostClient.AdoptStaticInstance(ctx, newStaticInstance, staticMachine, machine)
		if err != nil {
			logger.Error(err, "failed to adopt StaticInstance")
		}

		return result, nil
	}

	result, err := r.HostClient.Bootstrap(ctx, newStaticInstance, staticMachine, machine)
	if err != nil {
		logger.Error(err, "failed to bootstrap StaticInstance")
	}

	return result, nil
}

func (r *StaticMachineReconciler) reconcileDelete(ctx context.Context,
	machine *clusterv1.Machine,
	staticMachine *infrav1.StaticMachine,
	staticInstance *deckhousev1.StaticInstance) (ctrl.Result, error) {
	if staticInstance != nil {
		result, err := r.cleanup(ctx, machine, staticMachine, staticInstance)
		if err != nil {
			return result, fmt.Errorf("failed to cleanup StaticInstance: %w", err)
		}

		// if requeued
		if !result.IsZero() {
			return result, nil
		}
	}

	controllerutil.RemoveFinalizer(staticMachine, infrav1.MachineFinalizer)
	return ctrl.Result{}, nil
}

func (r *StaticMachineReconciler) cleanup(ctx context.Context, machine *clusterv1.Machine, staticMachine *infrav1.StaticMachine, staticInstance *deckhousev1.StaticInstance) (ctrl.Result, error) {
	phase := staticInstance.GetPhase()

	logger := ctrl.LoggerFrom(ctx)
	logger.Info("StaticInstance is cleaning", "phase", phase)

	// Delete flow might observe an inconsistent state where phase is Pending (or empty),
	// but refs are still set. Normalize it and allow StaticMachine deletion to proceed.
	if phase == deckhousev1.StaticInstanceStatusCurrentStatusPhasePending {
		if staticInstance.Status.MachineRef != nil || staticInstance.Status.NodeRef != nil || staticInstance.Status.CurrentStatus != nil {
			staticInstance.ToPending()
		}
		return ctrl.Result{}, nil
	}

	if phase != deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning && staticInstance.Status.NodeRef != nil {
		staticMachine.Status.Ready = false
		staticMachine.Status.Initialization.Provisioned = ptr.To(false)

		// Cluster API controller is a raceful service. We must fix bug https://github.com/kubernetes-sigs/cluster-api/issues/7237.
		if machine.Status.NodeRef.Name == "" {
			machine.Status.NodeRef.Name = staticInstance.Status.NodeRef.Name
		}

		if machine.Annotations == nil {
			machine.Annotations = make(map[string]string)
		}

		if machine.Annotations[clusterv1.PreTerminateDeleteHookAnnotationPrefix] != "true" {
			machine.Annotations[clusterv1.PreTerminateDeleteHookAnnotationPrefix] = "true"
		}

		cond := conditions.Get(machine, clusterv1.DeletingCondition)
		if cond != nil && cond.Status == metav1.ConditionTrue {
			err := r.HostClient.Cleanup(ctx, staticInstance, staticMachine, machine)
			if err != nil {
				// don't return here
				logger.Error(err, "failed to clean up StaticInstance")
			}

			delete(machine.Annotations, clusterv1.PreTerminateDeleteHookAnnotationPrefix)
		}

		return ctrl.Result{RequeueAfter: RequeueForStaticMachineDeleting}, nil
	}

	if phase == deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning && time.Since(staticInstance.Status.CurrentStatus.LastUpdateTime.Time) > DefaultStaticInstanceCleanupTimeout {
		logger.Error(StaticInstanceCleanupTimedOut, "")
		r.Recorder.SendWarningEvent(staticInstance, staticMachine.Labels["node-group"], "StaticInstanceCleanupTimeoutReached", "Timed out waiting for StaticInstance to clean up")

		staticMachine.Status.FailureReason = ptr.To("DeleteError")
		staticMachine.Status.FailureMessage = ptr.To(StaticInstanceCleanupTimedOut.Error())

		staticInstance.ToPending()
		return ctrl.Result{}, nil
	}

	err := r.HostClient.Cleanup(ctx, staticInstance, staticMachine, machine)
	if err != nil {
		// don't return here
		logger.Error(err, "failed to clean up StaticInstance")
	}

	return ctrl.Result{RequeueAfter: RequeueForStaticInstanceCleaning}, nil
}

func (r *StaticMachineReconciler) reconcileStaticInstancePhase(ctx context.Context,
	machine *clusterv1.Machine,
	staticMachine *infrav1.StaticMachine,
	staticInstance *deckhousev1.StaticInstance) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	switch staticInstance.GetPhase() {
	case deckhousev1.StaticInstanceStatusCurrentStatusPhasePending:
		_, shouldSkipBootstrap := staticInstance.Annotations[deckhousev1.SkipBootstrapPhaseAnnotation]
		if !shouldSkipBootstrap {
			return ctrl.Result{}, nil
		}

		logger.Info("StaticInstance is adopting")

		staticMachine.Status.Ready = false
		staticMachine.Status.Initialization.Provisioned = ptr.To(false)

		estimated := DefaultStaticInstanceAdoptTimeout - time.Since(staticMachine.CreationTimestamp.Time)
		if estimated < (10 * time.Second) {
			staticMachine.Status.FailureReason = ptr.To("UpdateError")
			staticMachine.Status.FailureMessage = ptr.To(StaticMachineAdoptTimedOut.Error())
			r.Recorder.SendWarningEvent(staticInstance, staticMachine.Labels["node-group"], "StaticInstanceAdoptTimeoutReached", "Timed out waiting for StaticInstance to adopt")
			return ctrl.Result{}, StaticMachineAdoptTimedOut
		}

		result, err := r.HostClient.AdoptStaticInstance(ctx, staticInstance, staticMachine, machine)
		if err != nil {
			logger.Error(err, "failed to adopt StaticInstance")
		}

		return result, nil
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping:
		logger.Info("StaticInstance is bootstrapping")

		staticMachine.Status.Ready = false
		staticMachine.Status.Initialization.Provisioned = ptr.To(false)

		estimated := DefaultStaticInstanceBootstrapTimeout - time.Since(staticInstance.Status.CurrentStatus.LastUpdateTime.Time)
		if estimated < (10 * time.Second) {
			staticMachine.Status.FailureReason = ptr.To("CreateError")
			staticMachine.Status.FailureMessage = ptr.To(StaticMachineBootstrapTimedOut.Error())
			r.Recorder.SendWarningEvent(staticInstance, staticMachine.Labels["node-group"], "StaticInstanceBootstrapTimeoutReached", "Timed out waiting for StaticInstance to bootstrap")
			return ctrl.Result{}, StaticMachineBootstrapTimedOut
		}

		result, err := r.HostClient.Bootstrap(ctx, staticInstance, staticMachine, machine)
		if err != nil {
			logger.Error(err, "failed to bootstrap StaticInstance")
		}

		return result, nil
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		staticMachine.Status.Ready = true
		staticMachine.Status.Initialization.Provisioned = ptr.To(true)
		logger.Info("StaticInstance is running")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StaticMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&deckhousev1.StaticInstance{},
		"status.machineRef.uid",
		func(rawObj k8sClient.Object) []string {
			staticInstance := rawObj.(*deckhousev1.StaticInstance)

			if staticInstance.Status.MachineRef == nil {
				return nil
			}

			return []string{string(staticInstance.Status.MachineRef.UID)}
		})
	if err != nil {
		return fmt.Errorf("failed to setup StaticInstance field 'status.machineRef.uid' indexer: %w", err)
	}

	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Node{},
		"spec.providerID",
		func(rawObj k8sClient.Object) []string {
			node := rawObj.(*corev1.Node)

			if node.Spec.ProviderID == "static://" {
				providerID := node.Annotations["node.deckhouse.io/provider-id"]

				if providerID != "" {
					node.Spec.ProviderID = providerID
				}
			}

			return []string{node.Spec.ProviderID}
		})
	if err != nil {
		return fmt.Errorf("failed to setup Node field 'spec.providerID' indexer: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.StaticMachine{}).
		Watches(
			&deckhousev1.StaticInstance{},
			handler.EnqueueRequestsFromMapFunc(r.StaticInstanceToStaticMachineMapFunc(infrav1.GroupVersion.WithKind("StaticMachine"))),
		).
		Complete(r)
}

// StaticInstanceToStaticMachineMapFunc returns a handler.ToRequestsFunc that watches for
// StaticInstance events and returns reconciliation requests for an infrastructure provider object.
func (r *StaticMachineReconciler) StaticInstanceToStaticMachineMapFunc(gvk schema.GroupVersionKind) handler.MapFunc {
	return func(ctx context.Context, object k8sClient.Object) []reconcile.Request {
		logger := ctrl.LoggerFrom(ctx)

		staticInstance, ok := object.(*deckhousev1.StaticInstance)
		if !ok {
			return nil
		}
		if staticInstance.Status.CurrentStatus != nil && staticInstance.Status.CurrentStatus.Phase == deckhousev1.StaticInstanceStatusCurrentStatusPhasePending {
			machines := &infrav1.StaticMachineList{}

			err := r.List(
				ctx,
				machines,
			)
			if err != nil {
				logger.Error(err, "failed to get StaticMachineList")

				return nil
			}

			if len(machines.Items) == 0 {
				return nil
			}

			requests := make([]reconcile.Request, 0, len(machines.Items))

			for _, machine := range machines.Items {
				if machine.Status.Ready {
					continue
				}

				if machine.Status.Initialization.Provisioned != nil && *machine.Status.Initialization.Provisioned {
					continue
				}

				requests = append(requests, reconcile.Request{
					NamespacedName: k8sClient.ObjectKey{
						Namespace: machine.Namespace,
						Name:      machine.Name,
					},
				})
			}

			return requests
		}

		if staticInstance.Status.MachineRef == nil {
			return nil
		}

		// Return early if the GroupKind doesn't match what we expect
		if gvk.GroupKind() != staticInstance.Status.MachineRef.GroupVersionKind().GroupKind() {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: k8sClient.ObjectKey{
					Namespace: staticInstance.Status.MachineRef.Namespace,
					Name:      staticInstance.Status.MachineRef.Name,
				},
			},
		}
	}
}

func patchStaticInstance(ctx context.Context, patchHelper *patch.Helper, staticInstance *deckhousev1.StaticInstance, options ...patch.Option) error {
	// No SetSummary in v1beta2; individual conditions should be updated with conditions.Set() elsewhere.
	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav1.StaticInstanceAddedToNodeGroupCondition,
			infrav1.StaticInstanceBootstrapSucceededCondition,
		}},
	)

	return patchHelper.Patch(ctx, staticInstance, options...)
}

func patchStaticMachine(ctx context.Context, patchHelper *patch.Helper, staticMachine *infrav1.StaticMachine, options ...patch.Option) error {
	// No SetSummary in v1beta2; individual conditions should be updated with conditions.Set() elsewhere.
	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []string{
			clusterv1.ReadyCondition,
			infrav1.StaticMachineStaticInstanceReadyCondition,
		}},
	)

	return patchHelper.Patch(ctx, staticMachine, options...)
}
