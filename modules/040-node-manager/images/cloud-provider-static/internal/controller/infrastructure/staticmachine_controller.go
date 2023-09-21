/*
Copyright 2023.

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
	deckhousev1 "cloud-provider-static/api/deckhouse.io/v1alpha1"
	infrav1 "cloud-provider-static/api/infrastructure/v1alpha1"
	"cloud-provider-static/internal/bootstrap"
	"cloud-provider-static/internal/cleanup"
	"cloud-provider-static/internal/pool"
	"cloud-provider-static/internal/scope"
	"context"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultStaticInstanceBootstrapTimeout = 20 * time.Minute
	DefaultStaticInstanceCleanupTimeout   = 10 * time.Minute
)

// StaticMachineReconciler reconciles a StaticMachine object
type StaticMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticmachines/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines/status,verbs=get;list;watch

//+kubebuilder:rbac:groups=core,resources=secrets,verbs=create;get;list;watch
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch

//+kubebuilder:rbac:groups=deckhouse.io,resources=nodegroups,verbs=get

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
	defer func() {
		result.RequeueAfter = 60 * time.Second
	}()

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
	// If the StaticMachine is in an error state, return early.
	if machineScope.HasFailed() {
		machineScope.Logger.Info("Not reconciling StaticMachine in failed state. See staticMachine.status.failureReason, staticMachine.status.failureMessage, or previously logged error for details")

		return ctrl.Result{}, nil
	}

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

		err = bootstrap.Bootstrap(ctx, instanceScope)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to bootstrap StaticInstance")
		}

		return ctrl.Result{}, nil
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
		result, err := r.reconcileStaticInstancePhase(ctx, instanceScope)
		if err != nil {
			return result, errors.Wrap(err, "failed to reconcile StaticInstance")
		}

		if result.Requeue {
			return result, nil
		}
	}

	if controllerutil.RemoveFinalizer(machineScope.StaticMachine, infrav1.MachineFinalizer) {
		err := machineScope.Patch(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to remove finalizer")
		}
	}

	return ctrl.Result{}, nil
}

func (r *StaticMachineReconciler) reconcileStaticInstancePhase(
	ctx context.Context,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	switch instanceScope.GetPhase() {
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping:
		instanceScope.Logger.Info("StaticInstance is bootstrapping")

		estimated := DefaultStaticInstanceBootstrapTimeout - time.Now().Sub(instanceScope.Instance.Status.CurrentStatus.LastUpdateTime.Time)

		if estimated < (10 * time.Second) {
			instanceScope.MachineScope.Fail(capierrors.CreateMachineError, errors.New("timed out waiting for static instance to bootstrap"))

			err := instanceScope.MachineScope.Patch(ctx)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to set StaticMachine error status")
			}

			return ctrl.Result{}, errors.New("timed out waiting to bootstrap StaticInstance")
		}

		err := bootstrap.FinishBootstrapping(ctx, instanceScope)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to finish bootstrapping")
		}

		return ctrl.Result{}, nil
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		instanceScope.Logger.Info("StaticInstance is running")

		if !instanceScope.MachineScope.StaticMachine.ObjectMeta.DeletionTimestamp.IsZero() {
			err := cleanup.Cleanup(ctx, instanceScope)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to clean up StaticInstance")
			}

			return ctrl.Result{Requeue: true}, nil
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		instanceScope.Logger.Info("StaticInstance is cleaning")

		estimated := DefaultStaticInstanceCleanupTimeout - time.Now().Sub(instanceScope.Instance.Status.CurrentStatus.LastUpdateTime.Time)

		if estimated < (10 * time.Second) {
			instanceScope.MachineScope.Fail(capierrors.DeleteMachineError, errors.New("timed out waiting for static instance to bootstrap"))

			err := instanceScope.MachineScope.Patch(ctx)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to set StaticMachine error status")
			}

			return ctrl.Result{}, errors.New("timed out waiting to clean up StaticInstance")
		}

		err := cleanup.FinishCleaning(ctx, instanceScope)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to finish cleaning")
		}

		return ctrl.Result{}, nil
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
		//client.InNamespace(machineScope.Namespace()),
		client.MatchingFieldsSelector{Selector: uidSelector},
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
	credentialsKey := client.ObjectKey{
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
		func(rawObj client.Object) []string {
			staticInstance := rawObj.(*deckhousev1.StaticInstance)

			if staticInstance.Status.MachineRef == nil {
				return nil
			}

			return []string{string(staticInstance.Status.MachineRef.UID)}
		})
	if err != nil {
		return errors.Wrap(err, "failed to setup StaticInstance field 'status.currentStatus.phase' indexer")
	}

	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&deckhousev1.StaticInstance{},
		"status.currentStatus.phase",
		func(rawObj client.Object) []string {
			staticInstance := rawObj.(*deckhousev1.StaticInstance)

			if staticInstance.Status.CurrentStatus == nil {
				return []string{""}
			}

			return []string{string(staticInstance.Status.CurrentStatus.Phase)}
		})
	if err != nil {
		return errors.Wrap(err, "failed to setup StaticInstance field 'status.currentStatus.phase' indexer")
	}

	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.Node{},
		"spec.providerID",
		func(rawObj client.Object) []string {
			node := rawObj.(*corev1.Node)

			return []string{node.Spec.ProviderID}
		})
	if err != nil {
		return errors.Wrap(err, "failed to setup Node field 'status.nodeInfo.machineID' indexer")
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
// Machine events and returns reconciliation requests for an infrastructure provider object
func (r *StaticMachineReconciler) StaticInstanceToStaticMachineMapFunc(gvk schema.GroupVersionKind) handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		staticInstance, ok := object.(*deckhousev1.StaticInstance)
		if !ok {
			return nil
		}
		if staticInstance.Status.MachineRef == nil {
			// TODO, we can enqueue the static machine which providerID is nil to get better performance than requeue
			return nil
		}

		// Return early if the GroupKind doesn't match what we expect
		if gvk.GroupKind() != staticInstance.Status.MachineRef.GroupVersionKind().GroupKind() {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: staticInstance.Status.MachineRef.Namespace,
					Name:      staticInstance.Status.MachineRef.Name,
				},
			},
		}
	}
}
