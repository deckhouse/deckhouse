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
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha1"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	controller "caps-controller-manager/internal/controller/infrastructure"
	"caps-controller-manager/internal/event"
	"caps-controller-manager/internal/providerid"
	"caps-controller-manager/internal/scope"
)

// StaticInstanceReconciler reconciles a StaticInstance object
type StaticInstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Config   *rest.Config
	Recorder *event.Recorder
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=staticinstances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=staticinstances/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=deckhouse.io,resources=sshcredentials,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StaticInstance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *StaticInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling StaticInstance")

	staticInstance := &deckhousev1.StaticInstance{}
	err := r.Get(ctx, req.NamespacedName, staticInstance)
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

	err = instanceScope.LoadSSHCredentials(ctx, r.Recorder)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to load SSHCredentials")
	}

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
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, instanceScope)
}

const skipBootstrapPhaseAnnotation = "static.node.deckhouse.io/skip-bootstrap-phase"

func (r *StaticInstanceReconciler) reconcileNormal(
	ctx context.Context,
	instanceScope *scope.InstanceScope,
) (ctrl.Result, error) {
	if _, shouldSkipBootstrap := instanceScope.Instance.Annotations[skipBootstrapPhaseAnnotation]; shouldSkipBootstrap {
		return r.adoptBootstrappedStaticInstance(ctx, instanceScope)
	}

	if instanceScope.Instance.Status.CurrentStatus == nil || instanceScope.Instance.Status.CurrentStatus.Phase == "" {
		conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceAddedToNodeGroupCondition)

		instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhasePending)

		err := instanceScope.Patch(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to set StaticInstance phase to Pending")
		}

		instanceScope.Logger.Info("StaticInstance is pending")
	}

	if instanceScope.MachineScope != nil {
		instances := &deckhousev1.StaticInstanceList{}

		labelSelector, err := instanceScope.MachineScope.LabelSelector()
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to get label selector")
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
			instanceScope.Logger.Info("Labels on StaticInstance have changed and StaticInstance has left the StaticMachine.spec.labelSelector, trying to clean up StaticInstance (transfer Node to another NodeGroup)")

			err := r.Client.Delete(ctx, instanceScope.MachineScope.Machine)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "failed to delete Machine")
			}

			r.Recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "StaticInstanceNodeGroupLeaved", fmt.Sprintf("StaticInstance has left the StaticMachine.spec.labelSelector in NodeGroup '%s'", instanceScope.MachineScope.StaticMachine.Labels["node-group"]))

			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *StaticInstanceReconciler) adoptBootstrappedStaticInstance(ctx context.Context, instanceScope *scope.InstanceScope) (ctrl.Result, error) {
	instanceScope.Logger.Info(
		fmt.Sprintf("adopting node for StaticInstance with %s annotation", skipBootstrapPhaseAnnotation),
	)

	staticMachines := &infrav1.StaticMachineList{}
	if err := r.Client.List(ctx, staticMachines, client.InNamespace("d8-cloud-instance-manager")); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to list StaticMachines")
	}

	var machine *infrav1.StaticMachine
	for _, m := range staticMachines.Items {
		if m.Status.Ready {
			continue
		}
		if m.Spec.ProviderID != "" {
			continue
		}
		if conditions.IsTrue(&m, infrav1.StaticMachineStaticInstanceReadyCondition) ||
			conditions.IsTrue(&m, infrav1.StaticMachineWaitingForBootstrapDataSecretReason) {
			continue
		}
		if len(m.Status.Addresses) > 0 {
			continue
		}
		if m.Status.FailureMessage != nil || m.Status.FailureReason != nil {
			continue
		}

		machine = &m
		break
	}
	if machine == nil {
		return ctrl.Result{}, errors.Errorf("no valid StaticMachine found for adoption")
	}

	instanceScope.Instance.Status.MachineRef = &corev1.ObjectReference{
		Kind:       machine.Kind,
		Namespace:  machine.Namespace,
		Name:       machine.Name,
		UID:        machine.UID,
		APIVersion: machine.APIVersion,
	}

	delete(instanceScope.Instance.Annotations, skipBootstrapPhaseAnnotation)
	conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceBootstrapSucceededCondition)
	conditions.MarkTrue(instanceScope.Instance, infrav1.StaticInstanceAddedToNodeGroupCondition)
	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning)
	err := instanceScope.Patch(ctx)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to set StaticInstance phase to Running")
	}

	originalMachine := machine.DeepCopy()
	machine.Spec.ProviderID = providerid.GenerateProviderID(instanceScope.Instance.Name)
	machine.Status.Ready = true
	conditions.MarkTrue(machine, infrav1.StaticMachineStaticInstanceReadyCondition)
	if err = r.Client.Patch(ctx, machine, client.MergeFrom(originalMachine)); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch StaticMachine's provider ID")
	}

	return ctrl.Result{}, nil
}

func (r *StaticInstanceReconciler) getStaticMachine(
	ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
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

	machineScope, ok, err := controller.NewMachineScope(ctx, r.Client, r.Config, staticMachine)
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
		For(&deckhousev1.StaticInstance{}).
		Complete(r)
}
