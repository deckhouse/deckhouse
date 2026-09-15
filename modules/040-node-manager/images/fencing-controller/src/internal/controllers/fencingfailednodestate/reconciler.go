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

package fencingfailednodestate

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
	"fencing-controller/internal/domain/fsm"
	"fencing-controller/internal/usecase/profile"
)

// Profiles resolves the timings an incident is processed under.
type Profiles interface {
	// Resolve returns the timings of the incident, keeping them stable for as
	// long as the incident lives.
	Resolve(ctx context.Context, incident *v1alpha1.FencingFailedNodeState) (fsm.Params, error)
	// Forget drops the timings of a Node whose incident is over.
	Forget(node string)
}

// Reconciler drives the fencing state machine of every FencingFailedNodeState.
//
// Of the object it writes phase and conditions only; the blockers it reports
// there are also published as events and as a metric. The machine holds every
// transition the ADR describes, but this reconciler drives the timing ones and
// stops at ReadyToEvict: deleting the pods of a fenced Node and validating the
// reference from the object to its Node are not implemented yet, so the states
// past ReadyToEvict are unreachable until they land.
type Reconciler struct {
	client   client.Client
	profiles Profiles
	recorder record.EventRecorder
	now      func() time.Time
}

func New(c client.Client, profiles Profiles, recorder record.EventRecorder) *Reconciler {
	return &Reconciler{client: c, profiles: profiles, recorder: recorder, now: time.Now}
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// FencingSLAProfile is deliberately not watched: an edited profile must not
	// move the deadlines of an incident that is already being processed.
	return ctrl.NewControllerManagedBy(mgr).
		Named(common.ControllerName).
		For(&v1alpha1.FencingFailedNodeState{}).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var incident v1alpha1.FencingFailedNodeState

	if err := r.client.Get(ctx, req.NamespacedName, &incident); err != nil {
		if apierrors.IsNotFound(err) {
			// A missing CR means the Node has no active fencing signal: either a
			// recovered Node deleted it, or it was collected with its Node.
			r.profiles.Forget(req.Name)
			clearConfigurationError(req.Name)

			logger.Info("fencingfailednodestate is gone, node has no active fencing signal", "node", req.Name)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get fencingfailednodestate %q: %w", req.Name, err)
	}

	logger.Info("observed fencingfailednodestate", observedFields(&incident)...)

	machine, err := fsm.NewFSMFromCR(&incident)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("restore fencing state machine of %q: %w", incident.Name, err)
	}

	// One reconcile decides against one moment. Reading the clock a second time
	// for the requeue could put the deadline on the other side of it, and the
	// incident would then be left in a waiting state with no timer to leave it.
	now := r.now()

	params, err := r.profiles.Resolve(ctx, &incident)
	if err != nil {
		return r.reportUnusableProfile(ctx, &incident, machine, err, now)
	}

	if fired := machine.Advance(&incident, params, now); len(fired) > 0 {
		logger.Info("fencing state machine advanced",
			"node", incident.Name,
			"events", eventNames(fired),
			"phase", string(machine.State()),
			"fallback_ttl", params.FallbackTTL.String(),
			"evacuation_delay", params.EvacuationDelay.String(),
		)
	}

	if err := r.writeStatus(ctx, &incident, machine.State(), profileResolved(&incident, now)); err != nil {
		return ctrl.Result{}, err
	}

	r.announceProfileIsBack(&incident)
	clearConfigurationError(incident.Name)

	return ctrl.Result{RequeueAfter: machine.RequeueAfter(&incident, params, now)}, nil
}

// reportUnusableProfile records a configuration error without touching the phase
// and leaves the incident where it is. The fast eviction path is not entered,
// because the evacuation delay and the fallback TTL it has to respect are
// unknown.
func (r *Reconciler) reportUnusableProfile(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	machine *fsm.FSM,
	cause error,
	now time.Time,
) (ctrl.Result, error) {
	if !errors.Is(cause, profile.ErrConfiguration) {
		return ctrl.Result{}, fmt.Errorf("resolve SLA profile of %q: %w", incident.Name, cause)
	}

	reportConfigurationError(incident.Name, incident.Spec.ProfileRef.Name)

	// Whether the blocker is new is read from the object as observed, before the
	// write below records it there.
	isNew := !blockedOnProfile(incident)

	if err := r.writeStatus(ctx, incident, machine.State(), configurationError(incident, cause, now)); err != nil {
		return ctrl.Result{}, err
	}

	// The event marks the moment fencing stopped being possible. Repeating it on
	// every retry would say nothing new, and the requeue below retries for as
	// long as the profile stays broken.
	if isNew {
		r.recorder.Event(incident, corev1.EventTypeWarning, common.ReasonProfileUnavailable, cause.Error())
	}

	// The module ships the built-in profiles, so a missing or broken one is
	// expected to come back: the error requeues the incident with backoff.
	return ctrl.Result{}, fmt.Errorf("fencing of node %q is blocked: %w", incident.Name, cause)
}

// announceProfileIsBack reports that an incident blocked on its profile can be
// processed again, so an operator watching the events of the object sees the
// fencing path reopen. The object as observed decides, which keeps an incident
// that was never blocked from producing an event on every pass.
func (r *Reconciler) announceProfileIsBack(incident *v1alpha1.FencingFailedNodeState) {
	if !blockedOnProfile(incident) {
		return
	}

	r.recorder.Eventf(incident, corev1.EventTypeNormal, common.ReasonProfileResolved,
		"SLA profile %q was resolved, fencing of the node continues.", incident.Spec.ProfileRef.Name)
}

func blockedOnProfile(incident *v1alpha1.FencingFailedNodeState) bool {
	return meta.IsStatusConditionTrue(incident.Status.Conditions, common.ConditionTypeConfigurationError)
}

func configurationError(incident *v1alpha1.FencingFailedNodeState, cause error, now time.Time) metav1.Condition {
	return condition(incident, metav1.ConditionTrue, common.ReasonProfileUnavailable, cause.Error(), now)
}

func profileResolved(incident *v1alpha1.FencingFailedNodeState, now time.Time) metav1.Condition {
	return condition(incident, metav1.ConditionFalse, common.ReasonProfileResolved,
		fmt.Sprintf("SLA profile %q is in force for this incident.", incident.Spec.ProfileRef.Name), now)
}

func condition(
	incident *v1alpha1.FencingFailedNodeState,
	status metav1.ConditionStatus,
	reason, message string,
	now time.Time,
) metav1.Condition {
	return metav1.Condition{
		Type:               common.ConditionTypeConfigurationError,
		Status:             status,
		ObservedGeneration: incident.Generation,
		LastTransitionTime: metav1.NewTime(now),
		Reason:             reason,
		Message:            message,
	}
}

func eventNames(events []fsm.Event) []string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, string(event))
	}

	return names
}

func observedFields(incident *v1alpha1.FencingFailedNodeState) []any {
	fields := []any{
		"node", incident.Name,
		"node_group", incident.Spec.NodeGroup,
		"profile", string(incident.Spec.ProfileRef.Name),
		"phase", string(incident.Status.Phase),
		"generation", incident.Generation,
		"observed_generation", incident.Status.ObservedGeneration,
		"resource_version", incident.ResourceVersion,
	}

	if failed := incident.Status.Failed; failed != nil {
		fields = append(fields,
			"failed_detected_at", formatTime(&failed.DetectedAt),
			"failed_detected_by", failed.DetectedBy,
			"failed_reason", string(failed.Reason),
			"failed_alive_count", failed.AliveCount,
			"failed_quorum_size", failed.QuorumSize,
		)
	}

	if fallback := incident.Status.Fallback; fallback != nil {
		fields = append(fields,
			"fallback_active", fallback.Active,
			"fallback_last_heartbeat_at", formatTime(fallback.LastHeartbeatAt),
			"fallback_quorum_lost_at", formatTime(fallback.QuorumLostAt),
			"fallback_heartbeat_interval_seconds", fallback.HeartbeatIntervalSeconds,
		)
	}

	if deletedAt := incident.DeletionTimestamp; deletedAt != nil {
		fields = append(fields, "deletion_timestamp", formatTime(deletedAt))
	}

	return fields
}

func formatTime(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}
