/*
Copyright 2023 Flant JSC

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
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha1"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/client"
	"caps-controller-manager/internal/pool"
	"caps-controller-manager/internal/scope"
)

const (
	DefaultStaticInstanceBootstrapTimeout = 60 * time.Minute
	DefaultStaticInstanceCleanupTimeout   = 30 * time.Minute
)

// StaticMachineReconciler reconciles a StaticMachine object
type StaticMachineReconciler struct {
	k8sClient.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	HostClient *client.Client
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines/status,verbs=get;update;patch

//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StaticMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *StaticMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := ctrl.LoggerFrom(ctx)

	logger.Info("Reconciling StaticMachine")

	// Fetch the StaticMachine.
	staticMachine := &infrav1.StaticMachine{}
	err = r.Get(ctx, req.NamespacedName, staticMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	machineScope, ok, err := NewMachineScope(ctx, r.Client, r.Config, staticMachine)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create machine scope")
	}
	if !ok {
		return ctrl.Result{}, nil
	}
	defer func() {
		err := machineScope.Close(ctx)
		if err != nil {
			logger.Error(err, "failed to close machine scope")
		}
	}()

	instanceScope, err := r.fetchStaticInstanceByStaticMachineUID(ctx, machineScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to fetch static instance by static machine uid")
	}

	// Return early if the object or Cluster is paused
	if annotations.IsPaused(machineScope.ClusterScope.Cluster, staticMachine) {
		machineScope.Logger.Info("StaticMachine or linked Cluster is marked as paused. Won't reconcile")

		if instanceScope != nil {
			err := r.setPausedConditionForStaticInstance(ctx, instanceScope, true)
			if err != nil {
				machineScope.Logger.Error(err, "cannot set paused annotation for static instance")
			}
		}

		conditions.MarkFalse(staticMachine, infrav1.StaticMachineStaticInstanceReadyCondition, infrav1.ClusterOrResourcePausedReason, clusterv1.ConditionSeverityInfo, "")

		return ctrl.Result{}, nil
	}

	if instanceScope != nil {
		err := r.setPausedConditionForStaticInstance(ctx, instanceScope, false)
		if err != nil {
			machineScope.Logger.Error(err, "cannot remove paused annotation for static instance")
		}
	}

	// If the StaticMachine is in an error state, return early.
	if machineScope.HasFailed() {
		machineScope.Logger.Info("Not reconciling StaticMachine in failed state. See staticMachine.status.failureReason, staticMachine.status.failureMessage, or previously logged error for details")

		return ctrl.Result{}, nil
	}

	// Handle deleted machines
	if !staticMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		machineScope.Logger.Info("Reconciling delete StaticMachine")

		return r.reconcileDelete(ctx, machineScope, instanceScope)
	}

	return r.reconcileNormal(ctx, machineScope, instanceScope)
}

func (r *StaticMachineReconciler) reconcileNormal(
	ctx context.Context,
	machineScope *scope.MachineScope,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	// If the StaticMachine doesn't have finalizer, add it.
	if controllerutil.AddFinalizer(machineScope.StaticMachine, infrav1.MachineFinalizer) {
		err := machineScope.Patch(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	if !machineScope.ClusterScope.Cluster.Status.InfrastructureReady {
		machineScope.Logger.Info("Cluster infrastructure is not ready yet")

		conditions.MarkFalse(machineScope.StaticMachine, infrav1.StaticMachineStaticInstanceReadyCondition, infrav1.StaticMachineWaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")

		return ctrl.Result{}, nil
	}

	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Logger.Info("Bootstrap Data Secret not available yet")

		conditions.MarkFalse(machineScope.StaticMachine, infrav1.StaticMachineStaticInstanceReadyCondition, infrav1.StaticMachineWaitingForBootstrapDataSecretReason, clusterv1.ConditionSeverityInfo, "")

		return ctrl.Result{}, nil
	}

	// If there is not yet a StaticInstance for this StaticMachine,
	// then pick one from the static instance pool
	if instanceScope == nil {
		instanceScope, ok, err := pool.NewStaticInstancePool(r.Client, r.Config).PickStaticInstance(ctx, machineScope)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to pick StaticInstance")
		}
		if !ok {
			machineScope.Logger.Info("No pending StaticInstance available, waiting...")

			conditions.MarkFalse(machineScope.StaticMachine, infrav1.StaticMachineStaticInstanceReadyCondition, infrav1.StaticMachineStaticInstancesUnavailableReason, clusterv1.ConditionSeverityInfo, "")

			return ctrl.Result{}, nil
		}

		err = r.HostClient.Bootstrap(ctx, instanceScope)
		if err != nil {
			instanceScope.Logger.Error(err, "failed to bootstrap StaticInstance")
		}

		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	return r.reconcileStaticInstancePhase(ctx, instanceScope)
}

func (r *StaticMachineReconciler) setPausedConditionForStaticInstance(ctx context.Context, instanceScope *scope.InstanceScope, isPaused bool) error {
	if isPaused {
		desired := map[string]string{
			clusterv1.PausedAnnotation: "",
		}
		annotations.AddAnnotations(instanceScope.Instance, desired)
	} else {
		delete(instanceScope.Instance.Annotations, clusterv1.PausedAnnotation)
	}

	return instanceScope.Patch(ctx)
}

func (r *StaticMachineReconciler) reconcileDelete(
	ctx context.Context,
	machineScope *scope.MachineScope,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	if instanceScope != nil {
		result, err := r.cleanup(ctx, instanceScope)
		if err != nil {
			return result, errors.Wrap(err, "failed to cleanup StaticInstance")
		}

		if !result.IsZero() {
			return result, nil
		}
	}

	controllerutil.RemoveFinalizer(machineScope.StaticMachine, infrav1.MachineFinalizer)

	return ctrl.Result{}, nil
}

func (r *StaticMachineReconciler) cleanup(
	ctx context.Context,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	result, err := r.reconcileStaticInstancePhase(ctx, instanceScope)
	if err != nil {
		return result, errors.Wrap(err, "failed to reconcile StaticInstance")
	}

	if !result.IsZero() {
		return result, nil
	}

	if instanceScope.GetPhase() != deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning {
		return ctrl.Result{}, nil
	}

	instanceScope.MachineScope.SetNotReady()

	patchHelper, err := patch.NewHelper(instanceScope.MachineScope.Machine, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}

	// Cluster API controller is a raceful service. We must fix bug https://github.com/kubernetes-sigs/cluster-api/issues/7237.
	if instanceScope.MachineScope.Machine.Status.NodeRef == nil {
		instanceScope.MachineScope.Machine.Status.NodeRef = &corev1.ObjectReference{
			APIVersion: instanceScope.Instance.Status.NodeRef.APIVersion,
			Kind:       instanceScope.Instance.Status.NodeRef.Kind,
			Name:       instanceScope.Instance.Status.NodeRef.Name,
			UID:        instanceScope.Instance.Status.NodeRef.UID,
		}
	}

	if instanceScope.MachineScope.Machine.Annotations == nil {
		instanceScope.MachineScope.Machine.Annotations = make(map[string]string)
	}

	if instanceScope.MachineScope.Machine.Annotations[clusterv1.PreTerminateDeleteHookAnnotationPrefix] != "true" {
		instanceScope.MachineScope.Machine.Annotations[clusterv1.PreTerminateDeleteHookAnnotationPrefix] = "true"
	}

	cond := conditions.Get(instanceScope.MachineScope.Machine, clusterv1.PreTerminateDeleteHookSucceededCondition)
	if cond != nil && cond.Status == corev1.ConditionFalse {
		err = r.HostClient.Cleanup(ctx, instanceScope)
		if err != nil {
			instanceScope.Logger.Error(err, "failed to clean up StaticInstance")
		}

		delete(instanceScope.MachineScope.Machine.Annotations, clusterv1.PreTerminateDeleteHookAnnotationPrefix)
	}

	err = patchHelper.Patch(ctx, instanceScope.MachineScope.Machine)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch Machine with NodeRef")
	}

	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *StaticMachineReconciler) reconcileStaticInstancePhase(
	ctx context.Context,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	switch instanceScope.GetPhase() {
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping:
		instanceScope.MachineScope.SetNotReady()

		instanceScope.Logger.Info("StaticInstance is bootstrapping")

		estimated := DefaultStaticInstanceBootstrapTimeout - time.Now().Sub(instanceScope.Instance.Status.CurrentStatus.LastUpdateTime.Time)

		if estimated < (10 * time.Second) {
			instanceScope.MachineScope.Fail(capierrors.CreateMachineError, errors.New("timed out waiting for StaticInstance to bootstrap"))

			err := instanceScope.MachineScope.Patch(ctx)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to set StaticMachine error status")
			}

			return ctrl.Result{}, errors.New("timed out waiting to bootstrap StaticInstance")
		}

		err := r.HostClient.Bootstrap(ctx, instanceScope)
		if err != nil {
			instanceScope.Logger.Error(err, "failed to bootstrap StaticInstance")
		}

		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		instanceScope.MachineScope.SetReady()

		instanceScope.Logger.Info("StaticInstance is running")
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		instanceScope.MachineScope.SetNotReady()

		instanceScope.Logger.Info("StaticInstance is cleaning")

		estimated := DefaultStaticInstanceCleanupTimeout - time.Now().Sub(instanceScope.Instance.Status.CurrentStatus.LastUpdateTime.Time)

		if estimated < (10 * time.Second) {
			instanceScope.MachineScope.Fail(capierrors.DeleteMachineError, errors.New("timed out waiting for StaticInstance to clean up"))

			err := instanceScope.MachineScope.Patch(ctx)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to set StaticMachine error status")
			}

			return ctrl.Result{}, errors.New("timed out waiting to clean up StaticInstance")
		}

		err := r.HostClient.Cleanup(ctx, instanceScope)
		if err != nil {
			instanceScope.Logger.Error(err, "failed to clean up StaticInstance")
		}

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *StaticMachineReconciler) fetchStaticInstanceByStaticMachineUID(
	ctx context.Context,
	machineScope *scope.MachineScope,
) (*scope.InstanceScope, error) {
	instances := &deckhousev1.StaticInstanceList{}
	uidSelector := fields.OneTermEqualSelector("status.machineRef.uid", string(machineScope.StaticMachine.UID))

	err := r.List(
		ctx,
		instances,
		k8sClient.MatchingFieldsSelector{Selector: uidSelector},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find StaticInstance by static machine uid '%s'", machineScope.StaticMachine.UID)
	}

	if len(instances.Items) == 0 {
		return nil, nil
	}

	staticInstance := &instances.Items[0]

	newScope, err := scope.NewScope(r.Client, r.Config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a scope")
	}

	instanceScope, err := scope.NewInstanceScope(newScope, staticInstance)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create an instance scope")
	}

	instanceScope.MachineScope = machineScope

	credentials := &deckhousev1.SSHCredentials{}
	credentialsKey := k8sClient.ObjectKey{
		Namespace: staticInstance.Namespace,
		Name:      staticInstance.Spec.CredentialsRef.Name,
	}

	err = r.Client.Get(ctx, credentialsKey, credentials)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get static instance credentials")
	}

	instanceScope.Credentials = credentials

	return instanceScope, nil
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
		return errors.Wrap(err, "failed to setup StaticInstance field 'status.machineRef.uid' indexer")
	}

	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Node{},
		"spec.providerID",
		func(rawObj k8sClient.Object) []string {
			node := rawObj.(*corev1.Node)

			return []string{node.Spec.ProviderID}
		})
	if err != nil {
		return errors.Wrap(err, "failed to setup Node field 'spec.providerID' indexer")
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
