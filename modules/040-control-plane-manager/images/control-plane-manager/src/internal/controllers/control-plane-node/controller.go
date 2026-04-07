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
	"errors"
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

var errStatusConflict = errors.New("status update conflict")

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

	if err := r.ensureCertObserverExists(ctx, cpn, ops.Items, logger); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.ensureCertRenewalExists(ctx, cpn, ops.Items, logger); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusFromOperations(ctx, cpn, states, ops.Items); err != nil {
		// Using update - may be status conflicts, requeue to try again.
		if errors.Is(err, errStatusConflict) {
			return reconcile.Result{Requeue: true}, nil
		}
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
		if s.component.IsStaticPodComponent() {
			if s.specConfigChecksum == "" && s.specPKIChecksum == "" {
				continue
			}
		} else if s.specConfigChecksum == "" && s.specPKIChecksum == "" && s.specCAChecksum == "" {
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
		commands = append(commands,
			controlplanev1alpha1.CommandWaitPodReady,
			controlplanev1alpha1.CommandCertObserve,
		)
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
			controlplanev1alpha1.CommandCertObserve,
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
			controlplanev1alpha1.CommandCertObserve,
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

	// Apply cert expiration dates from any completed CPO that has ObservedState.
	// This covers standalone CertObserver and regular operations with CertObserve as last command.
	for i := range ops {
		op := &ops[i]
		if isCompleted(op) && op.Status.ObservedState != nil {
			applyCertDates(cpn, op.Status.ObservedState)
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

	// CertsRenewal condition
	meta.SetStatusCondition(&cpn.Status.Conditions, renewalCondition(cpn, ops))

	if reflect.DeepEqual(original.Status, cpn.Status) {
		return nil
	}
	if err := r.client.Status().Update(ctx, cpn); err != nil {
		if apierrors.IsConflict(err) {
			return errStatusConflict
		}
		return err
	}
	return nil
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

// ensureCertObserverExists creates a CertObserver CPO if needed (weekly CertObserver operation).
func (r *Reconciler) ensureCertObserverExists(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation, logger *log.Logger) error {
	if cpn.Status.LastObservedAt == nil {
		return nil
	}
	if time.Since(cpn.Status.LastObservedAt.Time) <= constants.CertObserverInterval {
		return nil
	}

	for i := range ops {
		if ops[i].Spec.Component == controlplanev1alpha1.OperationComponentCertObserver &&
			!isCompleted(&ops[i]) && !isFailed(&ops[i]) {
			return nil
		}
	}

	opName := fmt.Sprintf("%s-certobserve-%s", cpn.Name, short(cpn.Spec.ConfigVersion))

	op := &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: opName,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  cpn.Name,
				constants.ControlPlaneComponentLabelKey: string(controlplanev1alpha1.OperationComponentCertObserver),
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
			Component:     controlplanev1alpha1.OperationComponentCertObserver,
			Commands:      []controlplanev1alpha1.CommandName{controlplanev1alpha1.CommandCertObserve},
			Approved:      false,
		},
	}

	if err := r.client.Create(ctx, op); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create CertObserver operation %s: %w", opName, err)
	}
	logger.Info("CertObserver operation created", slog.String("operation", opName))
	return nil
}

// applyCertDates copies certificate expiration dates from ObservedState into CPN status.
func applyCertDates(cpn *controlplanev1alpha1.ControlPlaneNode, observedState map[string]controlplanev1alpha1.ObservedComponentState) {
	if observed, ok := observedState["etcd"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.Etcd.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}
	if observed, ok := observedState["kube-apiserver"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.KubeAPIServer.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}
	if observed, ok := observedState["kube-controller-manager"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.KubeControllerManager.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}
	if observed, ok := observedState["kube-scheduler"]; ok && len(observed.CertificatesExpirationDate) > 0 {
		cpn.Status.Components.KubeScheduler.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}

	now := metav1.Now()
	cpn.Status.LastObservedAt = &now
}

// ensureCertRenewalExists creates CertRenew CPOs for components whose certs expire within CertRenewalThreshold.
func (r *Reconciler) ensureCertRenewalExists(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation, logger *log.Logger) error {
	for component := range controlplanev1alpha1.ComponentRegistry() {
		dates := certDatesForComponent(cpn, component)
		if len(dates) == 0 {
			continue
		}

		minExpiry := minExpirationDate(dates)
		if minExpiry.IsZero() || time.Until(minExpiry) >= constants.CertRenewalThreshold {
			continue
		}

		if hasPendingCertRenewal(ops, component) {
			continue
		}

		opName := fmt.Sprintf("%s-%s-certrenewal-%s",
			cpn.Name,
			strings.ToLower(string(component)),
			time.Now().Format("20060102"))

		if operationExists(ops, opName) {
			continue
		}

		commands := certRenewalCommands(component)
		op := newCertRenewalOperation(cpn, opName, component, commands)
		if err := r.client.Create(ctx, op); err != nil {
			if apierrors.IsAlreadyExists(err) {
				continue
			}
			return fmt.Errorf("create cert renewal %s: %w", opName, err)
		}
		logger.Info("cert renewal created",
			slog.String("op", opName),
			slog.String("component", string(component)),
			slog.String("minExpiry", minExpiry.Format(time.RFC3339)))
	}
	return nil
}

// certRenewalCommands returns the command pipeline for a cert renewal operation.
func certRenewalCommands(component controlplanev1alpha1.OperationComponent) []controlplanev1alpha1.CommandName {
	switch component {
	case controlplanev1alpha1.OperationComponentEtcd:
		return []controlplanev1alpha1.CommandName{
			controlplanev1alpha1.CommandRenewPKICerts,
			controlplanev1alpha1.CommandSyncManifests,
			controlplanev1alpha1.CommandWaitPodReady,
			controlplanev1alpha1.CommandCertObserve,
		}
	default:
		return []controlplanev1alpha1.CommandName{
			controlplanev1alpha1.CommandRenewPKICerts,
			controlplanev1alpha1.CommandRenewKubeconfigs,
			controlplanev1alpha1.CommandSyncManifests,
			controlplanev1alpha1.CommandWaitPodReady,
			controlplanev1alpha1.CommandCertObserve,
		}
	}
}

func newCertRenewalOperation(
	cpn *controlplanev1alpha1.ControlPlaneNode,
	name string,
	component controlplanev1alpha1.OperationComponent,
	commands []controlplanev1alpha1.CommandName,
) *controlplanev1alpha1.ControlPlaneOperation {
	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  cpn.Name,
				constants.ControlPlaneComponentLabelKey: string(component),
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
			Component:     component,
			Commands:      commands,
			Approved:      false,
		},
	}
}

// certDatesForComponent returns cert expiration dates from CPN status for a given component.
func certDatesForComponent(cpn *controlplanev1alpha1.ControlPlaneNode, component controlplanev1alpha1.OperationComponent) map[string]metav1.Time {
	switch component {
	case controlplanev1alpha1.OperationComponentEtcd:
		return cpn.Status.Components.Etcd.CertificatesExpirationDate
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		return cpn.Status.Components.KubeAPIServer.CertificatesExpirationDate
	case controlplanev1alpha1.OperationComponentKubeControllerManager:
		return cpn.Status.Components.KubeControllerManager.CertificatesExpirationDate
	case controlplanev1alpha1.OperationComponentKubeScheduler:
		return cpn.Status.Components.KubeScheduler.CertificatesExpirationDate
	default:
		return nil
	}
}

// minExpirationDate returns the earliest expiration time from the given dates map.
func minExpirationDate(dates map[string]metav1.Time) time.Time {
	var min time.Time
	for _, t := range dates {
		if min.IsZero() || t.Time.Before(min) {
			min = t.Time
		}
	}
	return min
}

// hasPendingCertRenewal checks if there is an active (not completed, not failed) cert renewal CPO for the component.
func hasPendingCertRenewal(ops []controlplanev1alpha1.ControlPlaneOperation, component controlplanev1alpha1.OperationComponent) bool {
	for i := range ops {
		if ops[i].Spec.Component != component {
			continue
		}
		if !isRenewalOperation(&ops[i]) {
			continue
		}
		if !isCompleted(&ops[i]) && !isFailed(&ops[i]) {
			return true
		}
	}
	return false
}

// isRenewalOperation detects if CPO is a cert renewal operation by name
func isRenewalOperation(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return strings.Contains(op.Name, "-certrenewal-")
}

func operationExists(ops []controlplanev1alpha1.ControlPlaneOperation, name string) bool {
	for i := range ops {
		if ops[i].Name == name {
			return true
		}
	}
	return false
}

// renewalCondition computes the CertsRenewal condition for CPN status.
func renewalCondition(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) metav1.Condition {
	gen := cpn.Generation

	var latest *controlplanev1alpha1.ControlPlaneOperation
	for i := range ops {
		if !isRenewalOperation(&ops[i]) {
			continue
		}
		if latest == nil || ops[i].CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = &ops[i]
		}
	}

	if latest == nil {
		if msg := findExpiringCertsMessage(cpn); msg != "" {
			return metav1.Condition{
				Type:               constants.ConditionCertsRenewal,
				Status:             metav1.ConditionFalse,
				Reason:             constants.ReasonCertExpiring,
				Message:            msg,
				ObservedGeneration: gen,
			}
		}
		return metav1.Condition{
			Type:               constants.ConditionCertsRenewal,
			Status:             metav1.ConditionTrue,
			Reason:             constants.ReasonHealthy,
			ObservedGeneration: gen,
		}
	}

	switch {
	case isCompleted(latest):
		return metav1.Condition{
			Type:               constants.ConditionCertsRenewal,
			Status:             metav1.ConditionTrue,
			Reason:             constants.ReasonRenewed,
			Message:            "renewed by " + latest.Name,
			ObservedGeneration: gen,
		}
	case isFailed(latest):
		return metav1.Condition{
			Type:               constants.ConditionCertsRenewal,
			Status:             metav1.ConditionFalse,
			Reason:             constants.ReasonRenewalFailed,
			Message:            failureMessage(latest),
			ObservedGeneration: gen,
		}
	case latest.Spec.Approved:
		return metav1.Condition{
			Type:               constants.ConditionCertsRenewal,
			Status:             metav1.ConditionFalse,
			Reason:             constants.ReasonRenewing,
			Message:            latest.Name + " in progress",
			ObservedGeneration: gen,
		}
	default:
		return metav1.Condition{
			Type:               constants.ConditionCertsRenewal,
			Status:             metav1.ConditionFalse,
			Reason:             constants.ReasonCertExpiring,
			Message:            latest.Name + " pending approval",
			ObservedGeneration: gen,
		}
	}
}

// findExpiringCertsMessage checks all components for certs expiring within threshold.
func findExpiringCertsMessage(cpn *controlplanev1alpha1.ControlPlaneNode) string {
	for component := range controlplanev1alpha1.ComponentRegistry() {
		dates := certDatesForComponent(cpn, component)
		if len(dates) == 0 {
			continue
		}
		minExpiry := minExpirationDate(dates)
		if !minExpiry.IsZero() && time.Until(minExpiry) < constants.CertRenewalThreshold {
			return fmt.Sprintf("%s cert expires at %s", component, minExpiry.Format(time.RFC3339))
		}
	}
	return ""
}
