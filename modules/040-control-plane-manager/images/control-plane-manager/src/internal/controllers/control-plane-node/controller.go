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
	"sort"
	"strings"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	maxConcurrentReconciles     = 1
	cacheSyncTimeout            = 3 * time.Minute
	requeueInterval             = 5 * time.Minute
	maxTerminalCPOsPerComponent = 5
)

var errStatusConflict = errors.New("status update conflict")

type Reconciler struct {
	client  client.Client
	log     *log.Logger
	metrics *metrics
}

func Register(mgr manager.Manager, metricsStorage metricsstorage.Storage) error {
	nodeName := os.Getenv(constants.NodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("environment variable %s is not set", constants.NodeNameEnvVar)
	}

	metricHandlers, err := newMetrics(metricsStorage)
	if err != nil {
		return fmt.Errorf("init metrics: %w", err)
	}

	r := &Reconciler{
		client:  mgr.GetClient(),
		log:     log.Default().With(slog.String("controller", constants.CpnControllerName)),
		metrics: metricHandlers,
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
			r.metrics.deleteMaintenanceModeMetrics(nodeName)
			logger.Info("ControlPlaneNode not found, skipping")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	defer r.metrics.syncMaintenanceModeMetrics(cpn)

	ops := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, ops, client.MatchingLabels{
		constants.ControlPlaneNodeNameLabelKey: nodeName,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list operations for node %s: %w", nodeName, err)
	}
	// Use only operations owned by the current CPN object (UID) for prevents status reconstruction from stale operations after CPN recreation.
	currentOps := filterOperationsOwnedByCPN(ops.Items, cpn)

	// Apply completed operation results to CPN status so that drift detection uses up-to-date checksums.
	states, err := r.updateStatusFromOperations(ctx, cpn, currentOps)
	if err != nil {
		if errors.Is(err, errStatusConflict) {
			return reconcile.Result{RequeueAfter: requeueInterval}, nil
		}
		return reconcile.Result{}, err
	}

	if isMaintenanceMode(cpn) {
		logger.Info("Maintenance mode is enabled for ControlPlaneNode, skipping operation planning")
		return reconcile.Result{}, nil
	}

	currentOps, err = r.ensureOperationsExist(ctx, cpn, states, currentOps, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	currentOps, err = r.ensureCertRenewalExists(ctx, cpn, states, currentOps, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.ensureObserveOperations(ctx, cpn, currentOps, logger); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func filterOperationsOwnedByCPN(ops []controlplanev1alpha1.ControlPlaneOperation, cpn *controlplanev1alpha1.ControlPlaneNode) []controlplanev1alpha1.ControlPlaneOperation {
	filtered := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(ops))
	for i := range ops {
		op := ops[i]
		if isOperationOwnedByCPN(&op, cpn) {
			filtered = append(filtered, op)
		}
	}
	return filtered
}

func isOperationOwnedByCPN(op *controlplanev1alpha1.ControlPlaneOperation, cpn *controlplanev1alpha1.ControlPlaneNode) bool {
	for i := range op.OwnerReferences {
		ref := op.OwnerReferences[i]
		if ref.APIVersion != controlplanev1alpha1.GroupVersion.String() || ref.Kind != "ControlPlaneNode" {
			continue
		}
		if ref.Name != cpn.Name || ref.UID != cpn.UID {
			continue
		}
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

// componentState holds spec and status checksums for a single component.
type componentState struct {
	component     controlplanev1alpha1.OperationComponent
	conditionType string
	spec          controlplanev1alpha1.Checksums // Config, PKI from component; CA absent (global)
	status        controlplanev1alpha1.Checksums // Config, PKI, CA from component status
	specCA        string                         // global spec.CAChecksum
	hasPKI        bool
}

func buildComponentStates(cpn *controlplanev1alpha1.ControlPlaneNode) []componentState {
	all := []componentState{
		{
			component:     controlplanev1alpha1.OperationComponentEtcd,
			conditionType: controlplanev1alpha1.CPNConditionEtcdReady,
			spec:          cpn.Spec.Components.Etcd.Checksums,
			status:        cpn.Status.Components.Etcd.Checksums,
			specCA:        cpn.Spec.CAChecksum,
			hasPKI:        true,
		},
		{
			component:     controlplanev1alpha1.OperationComponentKubeAPIServer,
			conditionType: controlplanev1alpha1.CPNConditionAPIServerReady,
			spec:          cpn.Spec.Components.KubeAPIServer.Checksums,
			status:        cpn.Status.Components.KubeAPIServer.Checksums,
			specCA:        cpn.Spec.CAChecksum,
			hasPKI:        true,
		},
		{
			component:     controlplanev1alpha1.OperationComponentKubeControllerManager,
			conditionType: controlplanev1alpha1.CPNConditionControllerManagerReady,
			spec:          cpn.Spec.Components.KubeControllerManager.Checksums,
			status:        cpn.Status.Components.KubeControllerManager.Checksums,
			specCA:        cpn.Spec.CAChecksum,
		},
		{
			component:     controlplanev1alpha1.OperationComponentKubeScheduler,
			conditionType: controlplanev1alpha1.CPNConditionSchedulerReady,
			spec:          cpn.Spec.Components.KubeScheduler.Checksums,
			status:        cpn.Status.Components.KubeScheduler.Checksums,
			specCA:        cpn.Spec.CAChecksum,
		},
	}

	// Filter components with empty spec checksums (etcd-arbiter nodes only have Etcd + CA)
	result := make([]componentState, 0, len(all))
	for _, s := range all {
		if s.component.IsStaticPodComponent() {
			if s.spec.Config == "" && s.spec.PKI == "" {
				continue
			}
		} else if s.spec.Config == "" && s.spec.PKI == "" && s.specCA == "" {
			continue
		}
		result = append(result, s)
	}
	return result
}

// componentStateInSync reports whether a component state is in sync with the desired spec/status checksums.
func componentStateInSync(state componentState) bool {
	return state.spec.Config == state.status.Config &&
		(!state.hasPKI || state.spec.PKI == state.status.PKI) &&
		state.specCA == state.status.CA
}

// ensureOperationsExist creates operations (CPOs) for components where spec != status.
func (r *Reconciler) ensureOperationsExist(
	ctx context.Context,
	cpn *controlplanev1alpha1.ControlPlaneNode,
	states []componentState,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	logger *log.Logger,
) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	for _, state := range states {
		configChanged := state.spec.Config != state.status.Config
		pkiChanged := state.hasPKI && state.spec.PKI != state.status.PKI
		caChanged := state.specCA != state.status.CA

		if !configChanged && !pkiChanged && !caChanged {
			continue
		}

		// Skip creating duplicates while an active operation with the same desired checksums exists.
		if existing := findActiveOperation(ops, func(op *controlplanev1alpha1.ControlPlaneOperation) bool {
			return op.Spec.Component == state.component && matchesDesiredChecksums(op, state)
		}); existing != nil {
			logger.Debug("active operation with same desired checksums exists, waiting",
				slog.String("operation", existing.Name),
				slog.String("component", string(state.component)))
			continue
		}

		steps := determineSteps(state, pkiChanged, caChanged)
		op := operationBase(cpn, state.component, steps)
		op.ObjectMeta.GenerateName = operationGenerateNamePrefix(state)
		op.Spec.DesiredConfigChecksum = state.spec.Config
		op.Spec.DesiredPKIChecksum = state.spec.PKI
		op.Spec.DesiredCAChecksum = state.specCA

		if err := r.client.Create(ctx, op); err != nil {
			return nil, fmt.Errorf("create CPO for %s: %w", state.component, err)
		}
		ops = append(ops, *op)
		logger.Info("ControlPlaneOperation created",
			slog.String("operation", op.Name),
			slog.String("component", string(state.component)),
			slog.Any("steps", steps))

		// Keep only 5 terminal operations per component.
		// Active operations are never deleted
		rotatedOps, err := r.rotateComponentOperations(ctx, ops, state.component, maxTerminalCPOsPerComponent, logger)
		if err != nil {
			return nil, fmt.Errorf("rotate CPOs for %s: %w", state.component, err)
		}
		ops = rotatedOps
	}

	return ops, nil
}

func (r *Reconciler) rotateComponentOperations(
	ctx context.Context,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	component controlplanev1alpha1.OperationComponent,
	limit int,
	logger *log.Logger,
) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	if limit <= 0 {
		return ops, nil
	}

	terminalOps := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(ops))
	for i := range ops {
		if ops[i].Spec.Component == component && ops[i].IsTerminal() {
			terminalOps = append(terminalOps, ops[i])
		}
	}

	excess := len(terminalOps) - limit
	if excess <= 0 {
		return ops, nil
	}

	sort.SliceStable(terminalOps, func(i, j int) bool {
		ti := terminalOps[i].CreationTimestamp.Time
		tj := terminalOps[j].CreationTimestamp.Time
		if ti.Equal(tj) {
			return terminalOps[i].Name < terminalOps[j].Name
		}
		return ti.Before(tj)
	})

	deletedNames := make(map[string]struct{}, excess)

	for i := 0; i < excess; i++ {
		op := terminalOps[i]
		if err := r.client.Delete(ctx, &controlplanev1alpha1.ControlPlaneOperation{
			ObjectMeta: metav1.ObjectMeta{Name: op.Name},
		}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		deletedNames[op.Name] = struct{}{}
		logger.Info("ControlPlaneOperation rotated out",
			slog.String("operation", op.Name),
			slog.String("component", string(component)))
	}

	if len(deletedNames) == 0 {
		return ops, nil
	}

	filtered := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(ops)-len(deletedNames))
	for i := range ops {
		if _, deleted := deletedNames[ops[i].Name]; deleted {
			continue
		}
		filtered = append(filtered, ops[i])
	}

	return filtered, nil
}

func findLatestAppliedOperationForComponent(ops []controlplanev1alpha1.ControlPlaneOperation, component controlplanev1alpha1.OperationComponent) *controlplanev1alpha1.ControlPlaneOperation {
	var latest *controlplanev1alpha1.ControlPlaneOperation
	for i := range ops {
		op := &ops[i]
		if op.Spec.Component != component {
			continue
		}
		if !op.IsTerminal() || (!op.IsCompleted() && !hasCommitPoint(op)) {
			continue
		}
		if latest == nil || op.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = op
		}
	}
	return latest
}

// determineSteps returns the list of steps to execute based on what changed and the component type.
func determineSteps(state componentState, pkiChanged, caChanged bool) []controlplanev1alpha1.StepName {
	switch state.component {
	case controlplanev1alpha1.OperationComponentEtcd:
		// Etcd has no kubeconfigs (admin.conf is handled by ensureAdminKubeconfig inside JoinEtcdCluster).
		steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}
		if caChanged || pkiChanged {
			steps = append(steps,
				controlplanev1alpha1.StepSyncCA,
				controlplanev1alpha1.StepRenewPKICerts,
			)
		}
		steps = append(steps, controlplanev1alpha1.StepJoinEtcdCluster)
		// Join (empty status): SyncManifests is skipped in pipeline for JoinEtcdCluster only.
		// In this mode JoinEtcdCluster must ensure manifest convergence itself, including the case when status is empty but the member is already in etcd (needsJoin=false).
		// Update (non-empty status): SyncManifests overwrites manifest itself.
		isJoin := state.status.Config == "" && state.status.PKI == ""
		if !isJoin {
			steps = append(steps, controlplanev1alpha1.StepSyncManifests)
		}
		steps = append(steps,
			controlplanev1alpha1.StepWaitPodReady,
			controlplanev1alpha1.StepCertObserve,
		)
		return steps
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}
		if caChanged || pkiChanged {
			steps = append(steps,
				controlplanev1alpha1.StepSyncCA,
				controlplanev1alpha1.StepRenewPKICerts,
				controlplanev1alpha1.StepRenewKubeconfigs,
			)
		}
		steps = append(steps,
			controlplanev1alpha1.StepSyncManifests,
			controlplanev1alpha1.StepWaitPodReady,
			controlplanev1alpha1.StepCertObserve,
		)
		return steps
	default:
		// KCM, Scheduler: no leaf PKI certs
		steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}
		if caChanged {
			steps = append(steps,
				controlplanev1alpha1.StepSyncCA,
				controlplanev1alpha1.StepRenewKubeconfigs,
			)
		}
		steps = append(steps,
			controlplanev1alpha1.StepSyncManifests,
			controlplanev1alpha1.StepWaitPodReady,
			controlplanev1alpha1.StepCertObserve,
		)
		return steps
	}
}

func operationGenerateNamePrefix(state componentState) string {
	compName := strings.ToLower(string(state.component))

	var parts []string
	if state.spec.Config != "" {
		parts = append(parts, checksum.ShortChecksum(state.spec.Config))
	}
	if state.spec.PKI != "" {
		parts = append(parts, checksum.ShortChecksum(state.spec.PKI))
	}
	if state.specCA != "" {
		parts = append(parts, checksum.ShortChecksum(state.specCA))
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%s-", compName)
	}
	return fmt.Sprintf("%s-%s-", compName, strings.Join(parts, "-"))
}

// operationBase creates a CPO with the standard ObjectMeta and base Spec.
// Uses GenerateName for unique naming, caller must set DesiredChecksums.
func operationBase(
	cpn *controlplanev1alpha1.ControlPlaneNode,
	component controlplanev1alpha1.OperationComponent,
	steps []controlplanev1alpha1.StepName,
) *controlplanev1alpha1.ControlPlaneOperation {
	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", strings.ToLower(string(component))),
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  cpn.Name,
				constants.ControlPlaneComponentLabelKey: component.LabelValue(),
				constants.HeritageLabelKey:              constants.HeritageLabelValue,
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
			NodeName:  cpn.Name,
			Component: component,
			Steps:     steps,
			Approved:  false,
		},
	}
}

// updateStatusFromOperations reads CPO statuses and updates CPN conditions and applied checksums.
// returns component states built from the updated CPN object.
func (r *Reconciler) updateStatusFromOperations(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) ([]componentState, error) {
	original := cpn.DeepCopy()
	states := buildComponentStates(cpn)

	for _, state := range states {
		op := findOperationForState(ops, state)
		cond := r.conditionForState(state, op, cpn)
		meta.SetStatusCondition(&cpn.Status.Conditions, cond)

		if latestOp := findLatestAppliedOperationForComponent(ops, state.component); latestOp != nil {
			applyOperationResult(cpn, latestOp)
		}
	}

	// Apply cert expiration dates from completed CPOs with CertObserve.
	// Apply in monotonic observedAt order to avoid rollback to stale cert dates due to list ordering.
	type observedStateSnapshot struct {
		observedAt metav1.Time
		component  controlplanev1alpha1.OperationComponent
		state      controlplanev1alpha1.ObservedComponentState
	}
	snapshots := make([]observedStateSnapshot, 0, len(ops))
	for i := range ops {
		op := &ops[i]
		if op.IsCompleted() && op.Status.ObservedState != nil && op.HasStep(controlplanev1alpha1.StepCertObserve) {
			observedAt := operationObservedAt(op)
			if observedAt.IsZero() {
				continue
			}
			snapshots = append(snapshots, observedStateSnapshot{
				observedAt: observedAt,
				component:  op.Spec.Component,
				state:      *op.Status.ObservedState,
			})
		}
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].observedAt.Before(&snapshots[j].observedAt)
	})
	for i := range snapshots {
		applyCertDatesAndTimestamp(cpn, snapshots[i].component, snapshots[i].state, snapshots[i].observedAt)
	}

	// Derive global status.CAChecksum - set when ALL static pod components have per component CAChecksum matching spec.
	if cpn.Spec.CAChecksum != "" && cpn.Status.CAChecksum != cpn.Spec.CAChecksum {
		allMatch := true
		for _, state := range states {
			if !state.component.IsStaticPodComponent() {
				continue
			}
			compStatus := cpn.Status.Components.Component(state.component)
			if compStatus == nil {
				continue
			}
			if compStatus.Checksums.CA != cpn.Spec.CAChecksum {
				allMatch = false
				break
			}
		}
		if allMatch {
			cpn.Status.CAChecksum = cpn.Spec.CAChecksum
		}
	}

	// CertificatesHealthy condition
	meta.SetStatusCondition(&cpn.Status.Conditions, certificatesHealthyCondition(cpn))

	updatedStates := buildComponentStates(cpn)
	if reflect.DeepEqual(original.Status, cpn.Status) {
		return updatedStates, nil
	}
	if err := r.client.Status().Update(ctx, cpn); err != nil {
		if apierrors.IsConflict(err) {
			return nil, errStatusConflict
		}
		return nil, err
	}
	return updatedStates, nil
}

// findOperationForState finds the current CPO for a given component state by matching desired checksums.
// Priority: active (non-terminal) CPO, then completed CPO, then terminal CPO.
func findOperationForState(ops []controlplanev1alpha1.ControlPlaneOperation, state componentState) *controlplanev1alpha1.ControlPlaneOperation {
	var latestActive *controlplanev1alpha1.ControlPlaneOperation
	var latestCompleted *controlplanev1alpha1.ControlPlaneOperation
	var latestTerminal *controlplanev1alpha1.ControlPlaneOperation

	for i := range ops {
		op := &ops[i]
		if op.Spec.Component != state.component {
			continue
		}
		if !matchesDesiredChecksums(op, state) {
			continue
		}
		if !op.IsTerminal() {
			if latestActive == nil || op.CreationTimestamp.After(latestActive.CreationTimestamp.Time) {
				latestActive = op
			}
			continue
		}
		if latestTerminal == nil || op.CreationTimestamp.After(latestTerminal.CreationTimestamp.Time) {
			latestTerminal = op
		}
		if op.IsCompleted() && (latestCompleted == nil || op.CreationTimestamp.After(latestCompleted.CreationTimestamp.Time)) {
			latestCompleted = op
		}
	}

	if latestActive != nil {
		return latestActive
	}
	if latestCompleted != nil {
		return latestCompleted
	}
	return latestTerminal
}

// findActiveOperation returns the latest non-terminal operation matching the predicate.
func findActiveOperation(ops []controlplanev1alpha1.ControlPlaneOperation, match func(*controlplanev1alpha1.ControlPlaneOperation) bool) *controlplanev1alpha1.ControlPlaneOperation {
	var latest *controlplanev1alpha1.ControlPlaneOperation
	for i := range ops {
		op := &ops[i]
		if op.IsTerminal() {
			continue
		}
		if !match(op) {
			continue
		}
		if latest == nil || op.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = op
		}
	}
	return latest
}

// hasCommitPoint returns true if the operation has completed a step that writes to disk.
func hasCommitPoint(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.IsStepCompleted(controlplanev1alpha1.StepSyncManifests) ||
		op.IsStepCompleted(controlplanev1alpha1.StepJoinEtcdCluster)
}

// matchesDesiredChecksums returns true if the operation targets the current spec checksums.
func matchesDesiredChecksums(op *controlplanev1alpha1.ControlPlaneOperation, state componentState) bool {
	return op.Spec.DesiredConfigChecksum == state.spec.Config &&
		op.Spec.DesiredPKIChecksum == state.spec.PKI &&
		op.Spec.DesiredCAChecksum == state.specCA
}

func (r *Reconciler) conditionForState(
	state componentState,
	op *controlplanev1alpha1.ControlPlaneOperation,
	cpn *controlplanev1alpha1.ControlPlaneNode,
) metav1.Condition {
	gen := cpn.Generation

	if op == nil {
		// No operation — either synced or unknown
		if state.spec.Config == state.status.Config &&
			(!state.hasPKI || state.spec.PKI == state.status.PKI) &&
			state.specCA == state.status.CA {
			return metav1.Condition{
				Type:               state.conditionType,
				Status:             metav1.ConditionTrue,
				Reason:             controlplanev1alpha1.CPNReasonReady,
				ObservedGeneration: gen,
			}
		}
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionUnknown,
			Reason:             controlplanev1alpha1.CPNReasonNotReady,
			ObservedGeneration: gen,
		}
	}

	if op.IsCompleted() {
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionTrue,
			Reason:             controlplanev1alpha1.CPNReasonReady,
			ObservedGeneration: gen,
		}
	}

	if op.IsFailed() {
		msg := op.FailureMessage()
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionFalse,
			Reason:             controlplanev1alpha1.CPNReasonNotReady,
			Message:            msg,
			ObservedGeneration: gen,
		}
	}

	if op.Spec.Approved {
		return metav1.Condition{
			Type:               state.conditionType,
			Status:             metav1.ConditionFalse,
			Reason:             controlplanev1alpha1.CPNReasonNotReady,
			Message:            fmt.Sprintf("operation %s in progress", op.Name),
			ObservedGeneration: gen,
		}
	}

	return metav1.Condition{
		Type:               state.conditionType,
		Status:             metav1.ConditionFalse,
		Reason:             controlplanev1alpha1.CPNReasonNotReady,
		ObservedGeneration: gen,
	}
}

// applyOperationResult updates CPN status checksums based on a completed operation.
// All non-empty desired checksums are applied - no need to switch on step type.
func applyOperationResult(cpn *controlplanev1alpha1.ControlPlaneNode, op *controlplanev1alpha1.ControlPlaneOperation) {
	compStatus := cpn.Status.Components.Component(op.Spec.Component)
	if compStatus == nil {
		return
	}
	if op.Spec.DesiredConfigChecksum != "" {
		compStatus.Checksums.Config = op.Spec.DesiredConfigChecksum
	}
	if op.Spec.DesiredPKIChecksum != "" {
		compStatus.Checksums.PKI = op.Spec.DesiredPKIChecksum
	}
	if op.Spec.DesiredCAChecksum != "" {
		compStatus.Checksums.CA = op.Spec.DesiredCAChecksum
	}
}

// ensureObserveOperations creates observe-only CPOs per component when observation is due.
func (r *Reconciler) ensureObserveOperations(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation, logger *log.Logger) error {
	for component := range controlplanev1alpha1.ComponentRegistry() {
		compStatus := cpn.Status.Components.Component(component)
		if compStatus == nil {
			continue
		}

		// Component is not deployed(empty status) yet.
		if compStatus.Checksums.Config == "" {
			continue
		}

		lastObservedAt := compStatus.LastObservedAt
		if !lastObservedAt.IsZero() && time.Since(lastObservedAt.Time) <= constants.CertObserveInterval {
			continue
		}

		if existing := findActiveOperation(ops, func(op *controlplanev1alpha1.ControlPlaneOperation) bool {
			return op.Spec.Component == component && op.IsObserveOnlyOperation()
		}); existing != nil {
			logger.Debug("active observe-only operation exists, waiting",
				slog.String("operation", existing.Name),
				slog.String("component", string(component)))
			continue
		}

		op := operationBase(cpn, component, []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve})
		op.Spec.Approved = true

		if err := r.client.Create(ctx, op); err != nil {
			return fmt.Errorf("create observe-only operation for %s: %w", component, err)
		}
		logger.Info("observe-only operation created",
			slog.String("operation", op.Name),
			slog.String("component", string(component)))
	}

	return nil
}

// applyCertDatesAndTimestamp copies certificate expiration dates from ObservedState into CPN status and updates per-component LastObservedAt.
func applyCertDatesAndTimestamp(cpn *controlplanev1alpha1.ControlPlaneNode, component controlplanev1alpha1.OperationComponent, observed controlplanev1alpha1.ObservedComponentState, observedAt metav1.Time) {
	compStatus := cpn.Status.Components.Component(component)
	if compStatus == nil {
		return
	}
	if len(observed.CertificatesExpirationDate) > 0 {
		compStatus.CertificatesExpirationDate = observed.CertificatesExpirationDate
	}
	if compStatus.LastObservedAt.IsZero() || observedAt.Time.After(compStatus.LastObservedAt.Time) {
		compStatus.LastObservedAt = observedAt
	}
}

func operationObservedAt(op *controlplanev1alpha1.ControlPlaneOperation) metav1.Time {
	ready := meta.FindStatusCondition(op.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
	if ready != nil && ready.Status == metav1.ConditionTrue && !ready.LastTransitionTime.IsZero() {
		return ready.LastTransitionTime
	}

	if !op.CreationTimestamp.IsZero() {
		return metav1.NewTime(op.CreationTimestamp.Time)
	}

	return metav1.Time{}
}

// ensureCertRenewalExists creates cert-renewal CPOs for in-sync components whose certs expire within CertRenewalThreshold.
func (r *Reconciler) ensureCertRenewalExists(
	ctx context.Context,
	cpn *controlplanev1alpha1.ControlPlaneNode,
	states []componentState,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	logger *log.Logger,
) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	for _, state := range states {
		if !componentStateInSync(state) {
			continue
		}

		dates := certDatesForComponent(cpn, state.component)
		if len(dates) == 0 {
			continue
		}

		minExpiry := minExpirationDate(dates)
		if minExpiry.IsZero() || time.Until(minExpiry) >= constants.CertRenewalThreshold {
			continue
		}

		if existing := findActiveOperation(ops, func(op *controlplanev1alpha1.ControlPlaneOperation) bool {
			return op.Spec.Component == state.component
		}); existing != nil {
			logger.Debug("active operation exists for component, skip cert renewal creation",
				slog.String("operation", existing.Name),
				slog.String("component", string(state.component)))
			continue
		}

		op := operationBase(cpn, state.component, certRenewalSteps(state.component))
		op.ObjectMeta.GenerateName = operationGenerateNamePrefix(state)
		op.Spec.DesiredConfigChecksum = state.spec.Config
		op.Spec.DesiredPKIChecksum = state.spec.PKI
		op.Spec.DesiredCAChecksum = state.specCA

		if err := r.client.Create(ctx, op); err != nil {
			return nil, fmt.Errorf("create cert renewal for %s: %w", state.component, err)
		}
		ops = append(ops, *op)
		logger.Info("cert renewal created",
			slog.String("operation", op.Name),
			slog.String("component", string(state.component)),
			slog.String("minExpiry", minExpiry.Format(time.RFC3339)))
	}

	return ops, nil
}

// certRenewalSteps returns the step pipeline for a cert renewal operation.
func certRenewalSteps(component controlplanev1alpha1.OperationComponent) []controlplanev1alpha1.StepName {
	switch component {
	case controlplanev1alpha1.OperationComponentEtcd:
		return []controlplanev1alpha1.StepName{
			controlplanev1alpha1.StepBackup,
			controlplanev1alpha1.StepSyncCA,
			controlplanev1alpha1.StepRenewPKICerts,
			controlplanev1alpha1.StepSyncManifests,
			controlplanev1alpha1.StepWaitPodReady,
			controlplanev1alpha1.StepCertObserve,
		}
	default:
		return []controlplanev1alpha1.StepName{
			controlplanev1alpha1.StepBackup,
			controlplanev1alpha1.StepSyncCA,
			controlplanev1alpha1.StepRenewPKICerts,
			controlplanev1alpha1.StepRenewKubeconfigs,
			controlplanev1alpha1.StepSyncManifests,
			controlplanev1alpha1.StepWaitPodReady,
			controlplanev1alpha1.StepCertObserve,
		}
	}
}

// certDatesForComponent returns cert expiration dates from CPN status for a given component.
func certDatesForComponent(cpn *controlplanev1alpha1.ControlPlaneNode, component controlplanev1alpha1.OperationComponent) map[string]metav1.Time {
	compStatus := cpn.Status.Components.Component(component)
	if compStatus == nil {
		return nil
	}
	return compStatus.CertificatesExpirationDate
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

// certificatesHealthyCondition computes CertificatesHealthy from per-component CA sync and cert expiration data.
func certificatesHealthyCondition(cpn *controlplanev1alpha1.ControlPlaneNode) metav1.Condition {
	gen := cpn.Generation
	states := buildComponentStates(cpn)
	if len(states) == 0 {
		return metav1.Condition{
			Type:               controlplanev1alpha1.CPNConditionCertificatesHealthy,
			Status:             metav1.ConditionUnknown,
			Reason:             controlplanev1alpha1.CPNReasonUnknown,
			Message:            "no components to evaluate",
			ObservedGeneration: gen,
		}
	}

	caOutOfSyncComponents := make([]string, 0, len(states))
	expiringComponents := make([]string, 0, len(states))
	for _, state := range states {
		if state.specCA != "" && state.status.CA != state.specCA {
			caOutOfSyncComponents = append(caOutOfSyncComponents, string(state.component))
			continue
		}

		dates := certDatesForComponent(cpn, state.component)
		if len(dates) == 0 {
			return metav1.Condition{
				Type:               controlplanev1alpha1.CPNConditionCertificatesHealthy,
				Status:             metav1.ConditionUnknown,
				Reason:             controlplanev1alpha1.CPNReasonUnknown,
				Message:            string(state.component),
				ObservedGeneration: gen,
			}
		}
		minExpiry := minExpirationDate(dates)
		if minExpiry.IsZero() {
			return metav1.Condition{
				Type:               controlplanev1alpha1.CPNConditionCertificatesHealthy,
				Status:             metav1.ConditionUnknown,
				Reason:             controlplanev1alpha1.CPNReasonUnknown,
				Message:            string(state.component),
				ObservedGeneration: gen,
			}
		}
		if time.Until(minExpiry) < constants.CertRenewalThreshold {
			expiringComponents = append(expiringComponents, string(state.component))
		}
	}

	if len(caOutOfSyncComponents) > 0 {
		sort.Strings(caOutOfSyncComponents)
		return metav1.Condition{
			Type:               controlplanev1alpha1.CPNConditionCertificatesHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             controlplanev1alpha1.CPNReasonWaitingForComponents,
			Message:            strings.Join(caOutOfSyncComponents, ", "),
			ObservedGeneration: gen,
		}
	}

	if len(expiringComponents) > 0 {
		sort.Strings(expiringComponents)
		return metav1.Condition{
			Type:               controlplanev1alpha1.CPNConditionCertificatesHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             controlplanev1alpha1.CPNReasonExpiredSoon,
			Message:            strings.Join(expiringComponents, ", "),
			ObservedGeneration: gen,
		}
	}

	return metav1.Condition{
		Type:               controlplanev1alpha1.CPNConditionCertificatesHealthy,
		Status:             metav1.ConditionTrue,
		Reason:             controlplanev1alpha1.CPNReasonHealthy,
		ObservedGeneration: gen,
	}
}

func isMaintenanceMode(cpn *controlplanev1alpha1.ControlPlaneNode) bool {
	_, exists := cpn.Labels[constants.MaintenanceModeLabelKey]
	return exists
}
