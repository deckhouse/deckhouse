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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/event"
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
//
//nolint:nonamedreturns
func (r *StaticInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Reconciling StaticInstance")

	staticInstance := &deckhousev1.StaticInstance{}
	err := r.Get(ctx, req.NamespacedName, staticInstance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get StaticInstance: %w", err)
	}

	// Return early if the object is paused
	if annotations.HasPaused(staticInstance) {
		logger.Info("StaticInstance is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	staticMachine, err := r.getStaticMachine(ctx, staticInstance)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get StaticMachine: %w", err)
	}
	if staticMachine == nil {
		logger.Info("No StaticMachine is associated with StaticInstance")
	}

	if staticMachine != nil {
		cluster, err := util.GetClusterFromMetadata(ctx, r.Client, staticMachine.ObjectMeta)
		if err != nil {
			logger.Info("StaticMachine is missing cluster label or cluster does not exist. Won't reconcile")
			return ctrl.Result{}, nil
		}

		// Return early if the Cluster is paused
		if annotations.HasPaused(cluster) {
			logger.Info("linked Cluster is marked as paused. Won't reconcile")
			return ctrl.Result{}, nil
		}
	}

	// Handle deleted static instance
	if !staticInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, staticInstance, staticMachine)
}

//nolint:nonamedreturns
func (r *StaticInstanceReconciler) reconcileNormal(ctx context.Context, staticInstance *deckhousev1.StaticInstance, staticMachine *infrav1.StaticMachine) (res ctrl.Result, resErr error) {
	logger := ctrl.LoggerFrom(ctx)

	staticInstancePatchHelper, err := patch.NewHelper(staticInstance, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create staticInstance patch helper: %w", err)
	}

	defer func() {
		if err := patchStaticInstance(ctx, staticInstancePatchHelper, staticInstance); err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to patch staticInstance: %w", err))
		}
	}()

	credentials := &deckhousev1.SSHCredentials{}
	if err := r.Get(ctx, client.ObjectKey{Name: staticInstance.Spec.CredentialsRef.Name}, credentials); err != nil {
		logger.Error(err, "failed to load SSHCredentials")

		// TODO: StaticInstanceBootstrapSucceededCondition type?
		conditions.Set(staticInstance, metav1.Condition{
			Type:               infrav1.StaticInstanceWaitingForCredentialsRefReason,
			Reason:             infrav1.StaticInstanceWaitingForCredentialsRefReason,
			Status:             metav1.ConditionFalse,
			Message:            err.Error(),
			LastTransitionTime: metav1.Now(),
		})

		if staticInstance.GetPhase() == "" {
			staticInstance.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseError)
		}

		var nodeGroup string
		if staticMachine != nil {
			nodeGroup = staticMachine.Labels["node-group"]
		}
		r.Recorder.SendWarningEvent(staticInstance, nodeGroup, "StaticInstanceCredentialsUnavailable", err.Error())
		return ctrl.Result{}, fmt.Errorf("failed to load SSHCredentials: %w", err)
	}

	conditions.Set(staticInstance, metav1.Condition{
		// TODO: StaticInstanceBootstrapSucceededCondition type?
		Type:               infrav1.StaticInstanceWaitingForCredentialsRefReason,
		Reason:             infrav1.StaticInstanceWaitingForCredentialsRefReason,
		Status:             metav1.ConditionTrue,
		Message:            "SSHCredentials are available",
		LastTransitionTime: metav1.Now(),
	})

	if phase := staticInstance.GetPhase(); phase == "" || phase == deckhousev1.StaticInstanceStatusCurrentStatusPhaseError {
		// TODO: StaticInstanceBootstrapSucceededCondition type?
		conditions.Set(staticInstance, metav1.Condition{
			Type:               infrav1.StaticInstanceAddedToNodeGroupCondition,
			Reason:             infrav1.StaticInstanceAddedToNodeGroupCondition,
			Status:             metav1.ConditionTrue,
			Message:            "StaticInstance is added to NodeGroup",
			LastTransitionTime: metav1.Now(),
		})

		logger.Info("StaticInstance is pending")
		staticInstance.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhasePending)
	}

	if staticMachine == nil {
		return ctrl.Result{}, nil
	}

	labelSelector, err := staticMachineLabelSelector(staticMachine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get label selector: %w", err)
	}

	instances := &deckhousev1.StaticInstanceList{}
	uidSelector := fields.OneTermEqualSelector("status.machineRef.uid", string(staticMachine.UID))
	if err = r.List(ctx, instances, client.MatchingLabelsSelector{Selector: labelSelector}, client.MatchingFieldsSelector{Selector: uidSelector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find StaticInstance by static machine uid '%s': %w", staticMachine.UID, err)
	}

	if len(instances.Items) == 0 {
		logger.Info("Labels on StaticInstance have changed and StaticInstance has left the StaticMachine.spec.labelSelector, " +
			"trying to clean up StaticInstance (transfer Node to another NodeGroup)")

		machine, err := util.GetOwnerMachine(ctx, r.Client, staticMachine.ObjectMeta)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get Machine: %w", err)
		}
		if machine == nil {
			logger.Info("StaticMachine has no Machine OwnerRef, nothing to delete")
			return ctrl.Result{}, nil
		}

		if err = r.Client.Delete(ctx, machine); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete Machine: %w", err)
		}
		r.Recorder.SendNormalEvent(staticInstance, staticMachine.Labels["node-group"], "StaticInstanceNodeGroupLeaved", fmt.Sprintf("StaticInstance has left the StaticMachine.spec.labelSelector in NodeGroup '%s'", staticMachine.Labels["node-group"]))
	}

	return ctrl.Result{}, nil
}

func (r *StaticInstanceReconciler) getStaticMachine(ctx context.Context, staticInstance *deckhousev1.StaticInstance) (*infrav1.StaticMachine, error) {
	if staticInstance.Status.MachineRef == nil {
		return nil, nil
	}

	staticMachine := &infrav1.StaticMachine{}
	staticMachineNamespacedName := client.ObjectKey{
		Namespace: staticInstance.Status.MachineRef.Namespace,
		Name:      staticInstance.Status.MachineRef.Name,
	}

	err := r.Get(ctx, staticMachineNamespacedName, staticMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return staticMachine, nil
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

func staticMachineLabelSelector(staticMachine *infrav1.StaticMachine) (labels.Selector, error) {
	allowBootstrapRequirement, err := labels.NewRequirement("node.deckhouse.io/allow-bootstrap", selection.NotIn, []string{"false"})
	if err != nil {
		panic(err.Error())
	}

	if staticMachine.Spec.LabelSelector == nil {
		return labels.NewSelector().Add(*allowBootstrapRequirement), nil
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(staticMachine.Spec.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to convert StaticMachine label selector: %w", err)
	}

	requirements, _ := labelSelector.Requirements()

	for _, requirement := range requirements {
		if requirement.Key() == allowBootstrapRequirement.Key() {
			return nil, errors.New("label selector requirement for the 'node.deckhouse.io/allow-bootstrap' key can't be added manually")
		}
	}

	return labelSelector.Add(*allowBootstrapRequirement), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StaticInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhousev1.StaticInstance{}).
		Complete(r)
}
