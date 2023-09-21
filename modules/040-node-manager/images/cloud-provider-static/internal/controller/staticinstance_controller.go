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
	infrav1 "cloud-provider-static/api/v1alpha1"
	"cloud-provider-static/internal/scope"
	"cloud-provider-static/internal/util"
	"context"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// StaticInstanceReconciler reconciles a StaticInstance object
type StaticInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=deckhouse.io,resources=staticinstances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deckhouse.io,resources=staticinstances/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deckhouse.io,resources=staticinstances/finalizers,verbs=update

//+kubebuilder:rbac:groups=deckhouse.io,resources=sshcredentials,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StaticInstance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *StaticInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	defer func() {
		result.Requeue = true
		result.RequeueAfter = 60 * time.Second
	}()

	logger := log.FromContext(ctx)

	logger.Info("Reconciling StaticInstance")

	staticInstance := &infrav1.StaticInstance{}
	err = r.Get(ctx, req.NamespacedName, staticInstance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	newScope, err := scope.NewScope(r.Client, r.Config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create scope")
	}

	instanceScope, err := scope.NewInstanceScope(newScope, staticInstance)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create instance scope")
	}
	defer func() {
		err := instanceScope.Close(ctx)
		if err != nil {
			logger.Error(err, "failed to close instance scope")
		}
	}()

	if staticInstance.Labels == nil ||
		staticInstance.Labels["node-group"] == "" {
		conditions.MarkFalse(staticInstance, infrav1.StaticInstanceAddedToNodeGroupCondition, infrav1.StaticInstanceWaitingForNodeGroupReason, clusterv1.ConditionSeverityInfo, "")

		return ctrl.Result{Requeue: true}, nil
	}

	nodeGroupRef := &corev1.ObjectReference{
		APIVersion: "deckhouse.io/v1",
		Kind:       "NodeGroup",
		Name:       staticInstance.Labels["node-group"],
	}

	nodeGroup, err := util.Get(ctx, r.Client, nodeGroupRef, "")
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get node group")
	}

	_, found, err := unstructured.NestedMap(nodeGroup.Object, "spec", "staticInstances")
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to find node type")
	}
	if !found {
		return ctrl.Result{}, errors.New("NodeGroup does not have staticInstances field")
	}

	credentials := &infrav1.SSHCredentials{}
	credentialsKey := client.ObjectKey{
		Namespace: staticInstance.Namespace,
		Name:      staticInstance.Spec.CredentialsRef.Name,
	}

	err = r.Client.Get(ctx, credentialsKey, credentials)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditions.MarkFalse(staticInstance, infrav1.StaticInstanceAddedToNodeGroupCondition, infrav1.StaticInstanceWaitingForCredentialsRefReason, clusterv1.ConditionSeverityInfo, "")
		}

		return ctrl.Result{}, errors.Wrap(err, "failed to get SSHCredentials")
	}

	instanceScope.Credentials = credentials

	machineScope, err := r.getStaticMachine(ctx, staticInstance)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get StaticMachine")
	}

	instanceScope.MachineScope = machineScope

	if machineScope != nil {
		// Return early if the object or Cluster is paused
		if annotations.IsPaused(machineScope.ClusterScope.Cluster, staticInstance) {
			logger.Info("StaticInstance or linked Cluster is marked as paused. Won't reconcile")

			return ctrl.Result{}, nil
		}
	} else {
		// Return early if the object is paused
		if annotations.HasPaused(staticInstance) {
			logger.Info("StaticInstance is marked as paused. Won't reconcile")

			return ctrl.Result{}, nil
		}
	}

	// Handle deleted static instance
	if !staticInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		instanceScope.Logger.Info("Reconciling delete StaticInstance")

		return r.reconcileDelete(ctx, instanceScope)
	}

	return r.reconcileNormal(ctx, instanceScope)
}

func (r *StaticInstanceReconciler) reconcileNormal(
	ctx context.Context,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	// If the StaticInstance doesn't have finalizer, add it.
	if controllerutil.AddFinalizer(instanceScope.Instance, infrav1.InstanceFinalizer) {
		err := instanceScope.Patch(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to add finalizer")
		}
	}

	if instanceScope.Instance.Status.CurrentStatus == nil || instanceScope.Instance.Status.CurrentStatus.Phase == "" {
		instanceScope.SetPhase(infrav1.StaticInstanceStatusCurrentStatusPhasePending)

		conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceAddedToNodeGroupCondition)

		err := instanceScope.Patch(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to set StaticInstance phase to Pending")
		}

		instanceScope.Logger.Info("StaticInstance is pending")
	}

	if instanceScope.MachineScope != nil {
		instances := &infrav1.StaticInstanceList{}

		labelSelector, err := metav1.LabelSelectorAsSelector(&instanceScope.MachineScope.StaticMachine.Spec.LabelSelector)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "unable to convert StaticMachine label selector")
		}

		uidSelector := fields.OneTermEqualSelector("status.machineRef.uid", string(instanceScope.MachineScope.StaticMachine.UID))

		err = r.List(
			ctx,
			instances,
			client.MatchingLabelsSelector{Selector: labelSelector},
			client.MatchingFieldsSelector{Selector: uidSelector},
		)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to find StaticInstance by static machine uid '%s'", instanceScope.MachineScope.StaticMachine.UID)
		}

		if len(instances.Items) == 0 {
			instanceScope.Logger.Info("Labels on StaticInstance have changed and StaticInstance has left the MachineDeployment selector, trying to clean up StaticInstance (transfer Node to another NodeGroup)")

			err := r.Client.Delete(ctx, instanceScope.MachineScope.Machine)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to delete Machine")
			}

			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *StaticInstanceReconciler) reconcileDelete(
	ctx context.Context,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	switch instanceScope.GetPhase() {
	case "":
	case infrav1.StaticInstanceStatusCurrentStatusPhasePending:
	case infrav1.StaticInstanceStatusCurrentStatusPhaseBootstrapping:
		return ctrl.Result{Requeue: true}, nil
	case infrav1.StaticInstanceStatusCurrentStatusPhaseRunning:
		if instanceScope.MachineScope != nil {
			err := r.Client.Delete(ctx, instanceScope.MachineScope.Machine)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to delete Machine")
			}

			return ctrl.Result{Requeue: true}, nil
		}
	case infrav1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		return ctrl.Result{Requeue: true}, nil
	}

	if controllerutil.RemoveFinalizer(instanceScope.Instance, infrav1.InstanceFinalizer) {
		err := instanceScope.Patch(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to remove finalizer")
		}
	}

	return ctrl.Result{}, nil
}

func (r *StaticInstanceReconciler) getStaticMachine(
	ctx context.Context,
	staticInstance *infrav1.StaticInstance,
) (*scope.MachineScope, error) {
	logger := log.FromContext(ctx)

	if staticInstance.Status.MachineRef == nil {
		return nil, nil
	}

	staticMachine := &infrav1.StaticMachine{}
	staticMachineNamespacedName := client.ObjectKey{
		Namespace: staticInstance.Status.MachineRef.Namespace,
		Name:      staticInstance.Status.MachineRef.Name,
	}

	// Fetch the static machine.
	err := r.Get(ctx, staticMachineNamespacedName, staticMachine)
	if err != nil {
		logger.Info("No StaticMachine is associated with StaticInstance")

		return nil, nil
	}

	var ok bool

	machineScope, ok, err := NewMachineScope(ctx, r.Client, r.Config, staticMachine)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a machine scope")
	}
	if !ok {
		return nil, nil
	}

	return machineScope, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StaticInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.StaticInstance{}).
		Complete(r)
}
