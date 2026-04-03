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

package controlplanenode

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client client.Client
	log    *log.Logger
}

func Register(mgr manager.Manager) error {
	nodeName := os.Getenv(constants.NodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("environment variable %s is not set", constants.NodeNameEnvVar)
	}

	r := &Reconciler{
		client: mgr.GetClient(),
		log:    log.Default(),
	}

	nodeLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneNodeNameLabelKey: nodeName,
		},
	})
	if err != nil {
		return fmt.Errorf("create node label predicate: %w", err)
	}

	// Ignore Create, Delete.
	// React only to Update when Status changed (ignore spec-only updates).
	operationPredicate := predicate.And(
		nodeLabelPredicate,
		predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
				newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
				if !okOld || !okNew {
					return false
				}
				return !reflect.DeepEqual(oldOp.Status, newOp.Status)
			},
			DeleteFunc:  func(event.DeleteEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(false),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constants.CpnControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneOperation{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &controlplanev1alpha1.ControlPlaneNode{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(operationPredicate),
		).
		Watches(
			&controlplanev1alpha1.ControlPlaneNode{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(nodeLabelPredicate, predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	nodeName := req.Name
	logger := r.log.With(slog.String("node", nodeName))
	logger.Info("Reconcile started for ControlPlaneNode")

	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: nodeName}, cpn); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ControlPlaneNode not found, skipping")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	ops := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, ops, client.MatchingLabels{
		constants.ControlPlaneNodeNameLabelKey: nodeName,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list operations for node %s: %w", nodeName, err)
	}

	states := buildComponentStates(cpn)

	if err := r.ensureOperationsExist(ctx, cpn, states, ops.Items, logger); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.ensureObserveExists(ctx, cpn, ops.Items, logger); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusFromOperations(ctx, cpn, states, ops.Items); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

// componentState holds spec and status checksums for a single component.
type componentState struct {
	component            controlplanev1alpha1.OperationComponent
	conditionType        string
	specConfigChecksum   string
	statusConfigChecksum string
	specPKIChecksum      string
	statusPKIChecksum    string
	specCAChecksum       string
	statusCAChecksum     string
	hasPKI               bool
}

func buildComponentStates(cpn *controlplanev1alpha1.ControlPlaneNode) []componentState {
	all := []componentState{
		{
			component:            controlplanev1alpha1.OperationComponentEtcd,
			conditionType:        constants.ConditionEtcdReady,
			specConfigChecksum:   cpn.Spec.Components.Etcd.ConfigChecksum,
			statusConfigChecksum: cpn.Status.Components.Etcd.ConfigChecksum,
			specPKIChecksum:      cpn.Spec.Components.Etcd.PKIChecksum,
			statusPKIChecksum:    cpn.Status.Components.Etcd.PKIChecksum,
			specCAChecksum:       cpn.Spec.CAChecksum,
			statusCAChecksum:     cpn.Status.Components.Etcd.CAChecksum,
			hasPKI:               true,
		},
		{
			component:            controlplanev1alpha1.OperationComponentKubeAPIServer,
			conditionType:        constants.ConditionAPIServerReady,
			specConfigChecksum:   cpn.Spec.Components.KubeAPIServer.ConfigChecksum,
			statusConfigChecksum: cpn.Status.Components.KubeAPIServer.ConfigChecksum,
			specPKIChecksum:      cpn.Spec.Components.KubeAPIServer.PKIChecksum,
			statusPKIChecksum:    cpn.Status.Components.KubeAPIServer.PKIChecksum,
			specCAChecksum:       cpn.Spec.CAChecksum,
			statusCAChecksum:     cpn.Status.Components.KubeAPIServer.CAChecksum,
			hasPKI:               true,
		},
		{
			component:            controlplanev1alpha1.OperationComponentKubeControllerManager,
			conditionType:        constants.ConditionControllerManagerReady,
			specConfigChecksum:   cpn.Spec.Components.KubeControllerManager.ConfigChecksum,
			statusConfigChecksum: cpn.Status.Components.KubeControllerManager.ConfigChecksum,
			specCAChecksum:       cpn.Spec.CAChecksum,
			statusCAChecksum:     cpn.Status.Components.KubeControllerManager.CAChecksum,
			hasPKI:               false,
		},
		{
			component:            controlplanev1alpha1.OperationComponentKubeScheduler,
			conditionType:        constants.ConditionSchedulerReady,
			specConfigChecksum:   cpn.Spec.Components.KubeScheduler.ConfigChecksum,
			statusConfigChecksum: cpn.Status.Components.KubeScheduler.ConfigChecksum,
			specCAChecksum:       cpn.Spec.CAChecksum,
			statusCAChecksum:     cpn.Status.Components.KubeScheduler.CAChecksum,
			hasPKI:               false,
		},
		{
			component:            controlplanev1alpha1.OperationComponentHotReload,
			conditionType:        constants.ConditionHotReloadSynced,
			specConfigChecksum:   cpn.Spec.HotReloadChecksum,
			statusConfigChecksum: cpn.Status.HotReloadChecksum,
			hasPKI:               false,
		},
		{
			component:        controlplanev1alpha1.OperationComponentCA,
			conditionType:    constants.ConditionCASynced,
			specCAChecksum:   cpn.Spec.CAChecksum,
			statusCAChecksum: cpn.Status.CAChecksum,
		},
	}

	// Filter components with empty spec checksums (etcd-arbiter nodes only have Etcd + CA)
	result := make([]componentState, 0, len(all))
	for _, s := range all {
		if s.specConfigChecksum == "" && s.specPKIChecksum == "" && s.specCAChecksum == "" {
			continue
		}
		result = append(result, s)
	}
	return result
}

// ensureOperationsExist creates ControlPlaneOperation resources for components where spec != status.
func (r *Reconciler) ensureOperationsExist(
	ctx context.Context,
	cpn *controlplanev1alpha1.ControlPlaneNode,
	states []componentState,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	logger *log.Logger,
) error {
	existingOwners := make(map[string]types.UID, len(ops))
	for i := range ops {
		for _, ref := range ops[i].OwnerReferences {
			if ref.Controller != nil && *ref.Controller {
				existingOwners[ops[i].Name] = ref.UID
				break
			}
		}
	}

	for _, state := range states {
		configChanged := state.specConfigChecksum != state.statusConfigChecksum
		pkiChanged := state.hasPKI && state.specPKIChecksum != state.statusPKIChecksum
		caChanged := state.specCAChecksum != state.statusCAChecksum

		if !configChanged && !pkiChanged && !caChanged {
			continue
		}

		commands := determineCommands(state, configChanged, pkiChanged, caChanged)
		operationName := operationNameForNode(cpn.Name, state)

		if ownerUID, exists := existingOwners[operationName]; exists && ownerUID == cpn.UID {
			logger.Debug("ControlPlaneOperation already exists, skipping",
				slog.String("operation", operationName),
				slog.String("component", string(state.component)))
			continue
		}

		op := newControlPlaneOperation(cpn, operationName, state, commands)
		if err := r.client.Create(ctx, op); err != nil {
			if apierrors.IsAlreadyExists(err) {
				logger.Debug("ControlPlaneOperation already exists (race), skipping",
					slog.String("operation", operationName))
				continue
			}
			return fmt.Errorf("create ControlPlaneOperation %s: %w", operationName, err)
		}
		logger.Info("ControlPlaneOperation created",
			slog.String("operation", operationName),
			slog.String("component", string(state.component)),
			slog.Any("commands", commands))
	}

	return nil
}

// determineCommands returns the list of commands to execute based on what changed and the component type.
func determineCommands(state componentState, configChanged, pkiChanged, caChanged bool) []controlplanev1alpha1.CommandName {
	switch state.component {
	case controlplanev1alpha1.OperationComponentCA:
		return []controlplanev1alpha1.CommandName{controlplanev1alpha1.CommandSyncCA}
	case controlplanev1alpha1.OperationComponentHotReload:
		return []controlplanev1alpha1.CommandName{controlplanev1alpha1.CommandSyncHotReload}
	case controlplanev1alpha1.OperationComponentEtcd:
		// Etcd has no kubeconfigs (admin.conf is handled by ensureAdminKubeconfig inside JoinEtcdCluster).
		var commands []controlplanev1alpha1.CommandName
		if caChanged || pkiChanged {
			commands = append(commands,
				controlplanev1alpha1.CommandSyncCA,
				controlplanev1alpha1.CommandRenewPKICerts,
			)
		}
		commands = append(commands, controlplanev1alpha1.CommandJoinEtcdCluster)
		// Join (empty status): JoinEtcdCluster writes manifest with correct --initial-cluster.
		// Update (non-empty status): SyncManifests overwrites manifest from template.
		isJoin := state.statusConfigChecksum == "" && state.statusPKIChecksum == ""
		if !isJoin {
			commands = append(commands, controlplanev1alpha1.CommandSyncManifests)
		}
		commands = append(commands, controlplanev1alpha1.CommandWaitPodReady)
		return commands
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		var commands []controlplanev1alpha1.CommandName
		if caChanged || pkiChanged {
			commands = append(commands,
				controlplanev1alpha1.CommandSyncCA,
				controlplanev1alpha1.CommandRenewPKICerts,
				controlplanev1alpha1.CommandRenewKubeconfigs,
			)
		}
		commands = append(commands,
			controlplanev1alpha1.CommandSyncManifests,
			controlplanev1alpha1.CommandWaitPodReady,
		)
		return commands
	default:
		// KCM, Scheduler: no leaf PKI certs
		var commands []controlplanev1alpha1.CommandName
		if caChanged {
			commands = append(commands,
				controlplanev1alpha1.CommandSyncCA,
				controlplanev1alpha1.CommandRenewKubeconfigs,
			)
		}
		commands = append(commands,
			controlplanev1alpha1.CommandSyncManifests,
			controlplanev1alpha1.CommandWaitPodReady,
		)
		return commands
	}
}

// operationNameForNode returns a deterministic k8s-compatible name for a ControlPlaneOperation.
// The name encodes all non-empty checksums so that a new operation is created when any checksum changes.
func operationNameForNode(nodeName string, state componentState) string {
	sanitized := strings.ReplaceAll(nodeName, ".", "-")
	compName := strings.ToLower(string(state.component))

	var parts []string
	if state.specConfigChecksum != "" {
		parts = append(parts, short(state.specConfigChecksum))
	}
	if state.specPKIChecksum != "" {
		parts = append(parts, short(state.specPKIChecksum))
	}
	if state.specCAChecksum != "" {
		parts = append(parts, short(state.specCAChecksum))
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%s-%s", sanitized, compName)
	}
	return fmt.Sprintf("%s-%s-%s", sanitized, compName, strings.Join(parts, "-"))
}

func short(s string) string {
	if len(s) > 6 {
		return s[:6]
	}
	return s
}

func newControlPlaneOperation(
	cpn *controlplanev1alpha1.ControlPlaneNode,
	name string,
	state componentState,
	commands []controlplanev1alpha1.CommandName,
) *controlplanev1alpha1.ControlPlaneOperation {
	spec := controlplanev1alpha1.ControlPlaneOperationSpec{
		ConfigVersion:         cpn.Spec.ConfigVersion,
		NodeName:              cpn.Name,
		Component:             state.component,
		Commands:              commands,
		DesiredConfigChecksum: state.specConfigChecksum,
		DesiredPKIChecksum:    state.specPKIChecksum,
		DesiredCAChecksum:     state.specCAChecksum,
		Approved:              false,
	}

	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  cpn.Name,
				constants.ControlPlaneComponentLabelKey: string(state.component),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         controlplanev1alpha1.GroupVersion.String(),
					Kind:               "ControlPlaneNode",
					Name:               cpn.Name,
					UID:                cpn.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				},
			},
		},
		Spec: spec,
	}
}

// updateStatusFromOperations reads CPO statuses and updates CPN conditions and applied checksums.
func (r *Reconciler) updateStatusFromOperations(
	ctx context.Context,
	cpn *controlplanev1alpha1.ControlPlaneNode,
	states []componentState,
	ops []controlplanev1alpha1.ControlPlaneOperation,
) error {
	original := cpn.DeepCopy()

	for _, state := range states {
		op := findOperationForState(ops, state, cpn.Spec.ConfigVersion)
		cond := r.conditionForState(state, op, cpn)
		meta.SetStatusCondition(&cpn.Status.Conditions, cond)

		if op != nil && isCompleted(op) {
			applyOperationResult(cpn, op)
		}
	}

	// Consume completed Observe operation and delete it for free the name (for next periodic run)
	for i := range ops {
		op := &ops[i]
		if op.Spec.Component == controlplanev1alpha1.OperationComponentObserver && isCompleted(op) {
			applyObserveResult(cpn, op)
			if err := r.client.Delete(ctx, op); err != nil {
				return fmt.Errorf("delete completed Observe operation %s: %w", op.Name, err)
			}
			break
		}
	}

	// Derive global status.CAChecksum - set when ALL static pod components have per component CAChecksum matching spec.
	if cpn.Spec.CAChecksum != "" && cpn.Status.CAChecksum != cpn.Spec.CAChecksum {
		allMatch := true
		for _, state := range states {
			if state.component == controlplanev1alpha1.OperationComponentCA ||
				state.component == controlplanev1alpha1.OperationComponentHotReload {
				continue
			}
			if state.specCAChecksum == "" {
				continue
			}
			switch state.component {
			case controlplanev1alpha1.OperationComponentEtcd:
				if cpn.Status.Components.Etcd.CAChecksum != cpn.Spec.CAChecksum {
					allMatch = false
				}
			case controlplanev1alpha1.OperationComponentKubeAPIServer:
				if cpn.Status.Components.KubeAPIServer.CAChecksum != cpn.Spec.CAChecksum {
					allMatch = false
				}
			case controlplanev1alpha1.OperationComponentKubeControllerManager:
				if cpn.Status.Components.KubeControllerManager.CAChecksum != cpn.Spec.CAChecksum {
					allMatch = false
				}
			case controlplanev1alpha1.OperationComponentKubeScheduler:
				if cpn.Status.Components.KubeScheduler.CAChecksum != cpn.Spec.CAChecksum {
					allMatch = false
				}
			}
		}
		if allMatch {
			cpn.Status.CAChecksum = cpn.Spec.CAChecksum
		}
	}

	if reflect.DeepEqual(original.Status, cpn.Status) {
		return nil
	}
	return r.client.Status().Update(ctx, cpn)
}

// findOperationForState finds the single current CPO for a given component state.
// At most one such operation exists per component - guaranteed by name deduplication in ensureOperationsExist.
func findOperationForState(ops []controlplanev1alpha1.ControlPlaneOperation, state componentState, configVersion string) *controlplanev1alpha1.ControlPlaneOperation {
	for i := range ops {
		op := &ops[i]
		if string(op.Spec.Component) != string(state.component) {
			continue
		}
		if op.Spec.ConfigVersion != configVersion {
			continue
		}
		if matchesDesiredChecksums(op, state) {
			return op
		}
	}
	return nil
}

// matchesDesiredChecksums returns true if the operation targets the current spec checksums.
func matchesDesiredChecksums(op *controlplanev1alpha1.ControlPlaneOperation, state componentState) bool {
	return op.Spec.DesiredConfigChecksum == state.specConfigChecksum &&
		op.Spec.DesiredPKIChecksum == state.specPKIChecksum &&
		op.Spec.DesiredCAChecksum == state.specCAChecksum
}

func (r *Reconciler) conditionForState(
	state componentState,
	op *controlplanev1alpha1.ControlPlaneOperation,
	cpn *controlplanev1alpha1.ControlPlaneNode,
) metav1.Condition {
	gen := cpn.Generation

	if op == nil {
		// No operation — either synced or unknown
		if state.specConfigChecksum == state.statusConfigChecksum &&
			(!state.hasPKI || state.specPKIChecksum == state.statusPKIChecksum) &&
			state.specCAChecksum == state.statusCAChecksum {
			return metav1.Condition{
				Type:               state.conditionType,
				Status:             metav1.ConditionTrue,
				Reason:             constants.ReasonSynced,
				ObservedGeneration: gen,
			}
		}
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionUnknown,
			Reason:             constants.ReasonUnknown,
			ObservedGeneration: gen,
		}
	}

	if isCompleted(op) {
		// this condition reflects aggregate state: synced only when status.caChecksum matches (all pods restarted with new CA)
		if state.component == controlplanev1alpha1.OperationComponentCA &&
			state.specCAChecksum != state.statusCAChecksum {
			return metav1.Condition{
				Type:               state.conditionType,
				Status:             metav1.ConditionFalse,
				Reason:             constants.ReasonWaitingForComponents,
				Message:            "CA files installed, waiting for all components to restart with new CA",
				ObservedGeneration: gen,
			}
		}
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionTrue,
			Reason:             constants.ReasonSynced,
			ObservedGeneration: gen,
		}
	}

	if isFailed(op) {
		msg := failureMessage(op)
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionFalse,
			Reason:             constants.ReasonUpdateFailed,
			Message:            msg,
			ObservedGeneration: gen,
		}
	}

	if op.Spec.Approved {
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionFalse,
			Reason:             constants.ReasonUpdating,
			Message:            fmt.Sprintf("operation %s in progress", op.Name),
			ObservedGeneration: gen,
		}
	}

	return metav1.Condition{
		Type:               state.conditionType,
		Status:             metav1.ConditionFalse,
		Reason:             constants.ReasonPendingUpdate,
		ObservedGeneration: gen,
	}
}

// applyOperationResult updates CPN status checksums based on a completed operation.
// All non-empty desired checksums are applied - no need to switch on command type.
func applyOperationResult(cpn *controlplanev1alpha1.ControlPlaneNode, op *controlplanev1alpha1.ControlPlaneOperation) {
	if op.Spec.DesiredConfigChecksum != "" {
		setConfigChecksum(cpn, op.Spec.Component, op.Spec.DesiredConfigChecksum)
	}
	if op.Spec.DesiredPKIChecksum != "" {
		setPKIChecksum(cpn, op.Spec.Component, op.Spec.DesiredPKIChecksum)
	}
	if op.Spec.DesiredCAChecksum != "" {
		setCAChecksum(cpn, op.Spec.Component, op.Spec.DesiredCAChecksum)
	}
}

func setConfigChecksum(cpn *controlplanev1alpha1.ControlPlaneNode, component controlplanev1alpha1.OperationComponent, checksum string) {
	switch component {
	case controlplanev1alpha1.OperationComponentEtcd:
		cpn.Status.Components.Etcd.ConfigChecksum = checksum
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		cpn.Status.Components.KubeAPIServer.ConfigChecksum = checksum
	case controlplanev1alpha1.OperationComponentKubeControllerManager:
		cpn.Status.Components.KubeControllerManager.ConfigChecksum = checksum
	case controlplanev1alpha1.OperationComponentKubeScheduler:
		cpn.Status.Components.KubeScheduler.ConfigChecksum = checksum
	case controlplanev1alpha1.OperationComponentHotReload:
		cpn.Status.HotReloadChecksum = checksum
	}
}

func setPKIChecksum(cpn *controlplanev1alpha1.ControlPlaneNode, component controlplanev1alpha1.OperationComponent, checksum string) {
	switch component {
	case controlplanev1alpha1.OperationComponentEtcd:
		cpn.Status.Components.Etcd.PKIChecksum = checksum
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		cpn.Status.Components.KubeAPIServer.PKIChecksum = checksum
	}
}

func setCAChecksum(cpn *controlplanev1alpha1.ControlPlaneNode, component controlplanev1alpha1.OperationComponent, checksum string) {
	switch component {
	case controlplanev1alpha1.OperationComponentEtcd:
		cpn.Status.Components.Etcd.CAChecksum = checksum
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		cpn.Status.Components.KubeAPIServer.CAChecksum = checksum
	case controlplanev1alpha1.OperationComponentKubeControllerManager:
		cpn.Status.Components.KubeControllerManager.CAChecksum = checksum
	case controlplanev1alpha1.OperationComponentKubeScheduler:
		cpn.Status.Components.KubeScheduler.CAChecksum = checksum
	}
}

func isCompleted(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, constants.ConditionReady)
}

func isFailed(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, constants.ConditionFailed)
}

func failureMessage(op *controlplanev1alpha1.ControlPlaneOperation) string {
	for _, cond := range op.Status.Conditions {
		if cond.Type == constants.ConditionFailed && cond.Status == metav1.ConditionTrue {
			return cond.Message
		}
	}
	return ""
}

// ensureObserveExists creates an Observe CPO if needed (bootstrap or periodic).
func (r *Reconciler) ensureObserveExists(
	ctx context.Context,
	cpn *controlplanev1alpha1.ControlPlaneNode,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	logger *log.Logger,
) error {
	for i := range ops {
		if ops[i].Spec.Component == controlplanev1alpha1.OperationComponentObserver && !isCompleted(&ops[i]) {
			return nil
		}
	}

	for i := range ops {
		if ops[i].Spec.Component != controlplanev1alpha1.OperationComponentObserver &&
			ops[i].Spec.Approved && !isCompleted(&ops[i]) && !isFailed(&ops[i]) {
			return nil
		}
	}

	needsObserve := false

	if cpn.Status.LastObservedAt == nil {
		needsObserve = true
	}

	if cpn.Status.LastObservedAt != nil && time.Since(cpn.Status.LastObservedAt.Time) > constants.ObserveInterval {
		needsObserve = true
	}

	if !needsObserve {
		return nil
	}

	opName := fmt.Sprintf("%s-observer", cpn.Name)

	for i := range ops {
		if ops[i].Name == opName {
			return nil
		}
	}

	op := &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: opName,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  cpn.Name,
				constants.ControlPlaneComponentLabelKey: string(controlplanev1alpha1.OperationComponentObserver),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         controlplanev1alpha1.GroupVersion.String(),
					Kind:               "ControlPlaneNode",
					Name:               cpn.Name,
					UID:                cpn.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(false),
				},
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			ConfigVersion: cpn.Spec.ConfigVersion,
			NodeName:      cpn.Name,
			Component:     controlplanev1alpha1.OperationComponentObserver,
			Commands:      []controlplanev1alpha1.CommandName{controlplanev1alpha1.CommandObserve},
			Approved:      false,
		},
	}

	if err := r.client.Create(ctx, op); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create Observe operation %s: %w", opName, err)
	}
	logger.Info("Observe operation created", slog.String("operation", opName))
	return nil
}

// applyObserveResult copies certificate expiration from completed Observe CPO into CPN status.
func applyObserveResult(cpn *controlplanev1alpha1.ControlPlaneNode, op *controlplanev1alpha1.ControlPlaneOperation) {
	if op.Status.ObservedState == nil {
		return
	}

	if observed, ok := op.Status.ObservedState["etcd"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.Etcd.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}
	if observed, ok := op.Status.ObservedState["kube-apiserver"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.KubeAPIServer.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}
	if observed, ok := op.Status.ObservedState["kube-controller-manager"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.KubeControllerManager.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}
	if observed, ok := op.Status.ObservedState["kube-scheduler"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.KubeScheduler.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}

	now := metav1.Now()
	cpn.Status.LastObservedAt = &now
}
