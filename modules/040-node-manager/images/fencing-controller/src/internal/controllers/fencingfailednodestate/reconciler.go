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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
	"fencing-controller/internal/domain/fsm"
	"fencing-controller/internal/usecase/profile"
)

const (
	// conflictRetryAfter is how soon the controller decides again after a
	// write fails with a conflict.
	conflictRetryAfter = time.Second

	// maxConcurrentReconciles is how many incidents the controller processes
	// at the same time. The deletion of the pods of one Node takes time. It
	// must not delay the decision on the incident of another Node.
	maxConcurrentReconciles = 10

	// requestTimeout is the time limit of one attempt of a request to the API
	// server on the path of an incident. Under load the API
	// server answers such a request in less than 1 s. When a member of etcd
	// stops, the API server can hold a request for approximately 10 s. In most
	// cases, the next attempt goes to a different member of etcd.
	requestTimeout = 2 * time.Second

	// requestAttempts is the maximum number of attempts of such a request.
	// The last attempt has no time limit, so a slow API server also gives an
	// answer.
	requestAttempts = 3

	// The delays of the retries after an error grow from minRetryDelay to
	// maxRetryDelay. A dead Node gives no events, so after an error, for
	// example a profile that is not usable, only the retry decides again.
	minRetryDelay = 5 * time.Millisecond
	maxRetryDelay = time.Minute
)

// Profiles gives the timings of an incident.
type Profiles interface {
	// Resolve returns the timings of the incident.
	Resolve(ctx context.Context, incident *v1alpha1.FencingFailedNodeState) (fsm.Params, error)
}

// NodeReference tells if an incident belongs to the Node that has its name now.
type NodeReference interface {
	// Identifies tells if node owns the object. node is the Node with the
	// name of the object, or nil when there is no such Node.
	Identifies(incident *v1alpha1.FencingFailedNodeState, node *corev1.Node) bool
}

// Reconciler sets the phase of each FencingFailedNodeState and evicts the Node
// of a proven incident.
//
// It writes the phase and the conditions of the object. It also publishes the
// problems of an incident as events and metrics. For a proven incident it sets
// the out-of-service taint on the Node and deletes the pods of the Node.
//
// The controller processes different incidents at the same time, but one
// incident only in one reconcile at a time.
//
// Only the leader runs the reconciler. The deadlines start at the moments when
// this process saw the evidence. These moments are kept in memory only. A new
// leader starts again from what it sees, so a change of the leader can delay a
// decision but cannot make it earlier.
type Reconciler struct {
	client client.Client
	// reader reads Nodes, pods and VolumeAttachments without the cache. The
	// controller needs them only for the Node of a proven incident.
	reader client.Reader
	// namespace is the namespace of the controller. It is also the namespace
	// of the machines and of the machine health checks.
	namespace string
	nodes     NodeReference
	profiles  Profiles
	recorder  record.EventRecorder
	now       func() time.Time

	mu   sync.Mutex
	seen map[string]sighting
}

// sighting is what the controller saw of one object.
type sighting struct {
	// uid identifies the object. A recreated object is a new incident.
	uid types.UID
	// heartbeat is the last value of status.fallback.lastHeartbeatAt. The
	// controller uses it only to find a new heartbeat.
	heartbeat metav1.MicroTime
	// attestation is the last value of status.attestation. The controller
	// uses it only to find a new attestation.
	attestation v1alpha1.FencingFailedNodeStateAttestation
	// evicting is when the controller saw the incident in Evicting first.
	evicting time.Time
	at       fsm.Observations
}

func New(
	c client.Client,
	reader client.Reader,
	namespace string,
	nodes NodeReference,
	profiles Profiles,
	recorder record.EventRecorder,
) *Reconciler {
	return &Reconciler{
		client:    c,
		reader:    reader,
		namespace: namespace,
		nodes:     nodes,
		profiles:  profiles,
		recorder:  recorder,
		now:       time.Now,
		seen:      make(map[string]sighting),
	}
}

// SetupWithManager adds the reconciler to the manager. The controller runs
// only on the leader, because leader election is on by default for a
// controller.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// A change of a Node starts the reconcile of the incident of the Node. Only
	// a change of the Node tells that the node came back after a reboot. The
	// cache of the Nodes has no status, but each update of a Node comes as an
	// event.
	return ctrl.NewControllerManagedBy(mgr).
		Named(common.ControllerName).
		For(&v1alpha1.FencingFailedNodeState{}).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.incidentOfNode)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			RateLimiter:             workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](minRetryDelay, maxRetryDelay),
		}).
		Complete(r)
}

// incidentOfNode returns the request for the incident of the Node, if the
// Node has an incident. The incident has the name of its Node.
func (r *Reconciler) incidentOfNode(ctx context.Context, node client.Object) []reconcile.Request {
	var incident v1alpha1.FencingFailedNodeState
	if err := r.client.Get(ctx, types.NamespacedName{Name: node.GetName()}, &incident); err != nil {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: incident.Name}}}
}

// Reconcile decides the phase of the incident and acts on it.
//
// The writes of the controller check the version of the object that it read.
// A conflict tells that another writer, for example a witness, changed the
// object or the Node after the read. This is a usual result, not an error. The
// controller wrote nothing, and it decides again on the new version soon.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res, err := r.reconcile(ctx, req)
	if apierrors.IsConflict(err) {
		logf.FromContext(ctx).Info("object changed after the read, the controller decides again on the new version",
			"node", req.Name, "conflict", err.Error())

		return ctrl.Result{RequeueAfter: conflictRetryAfter}, nil
	}

	return res, err
}

func (r *Reconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var incident v1alpha1.FencingFailedNodeState

	if err := r.client.Get(ctx, req.NamespacedName, &incident); err != nil {
		if apierrors.IsNotFound(err) {
			// The Node has no fencing signal. The recovered Node deleted the
			// object, or the garbage collector deleted it with its Node.
			r.dropTraceOf(req.Name)
			clearPhase(req.Name)

			logger.Info("fencingfailednodestate is gone, node has no active fencing signal", "node", req.Name)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get fencingfailednodestate %q: %w", req.Name, err)
	}

	logger.Info("observed fencingfailednodestate", observedFields(&incident)...)

	// An object that is being deleted is the same as an object that is gone.
	// The controller does not write to it and does not set a timer for it.
	if incident.DeletionTimestamp != nil {
		r.dropTraceOf(incident.Name)
		clearPhase(incident.Name)

		logger.Info("fencingfailednodestate is being deleted, node has no active fencing signal", "node", incident.Name)

		return ctrl.Result{}, nil
	}

	// Each object that exists has the phase series, also when the reconcile
	// below does not write the object or fails.
	reportPhase(&incident)
	reportProgress(&incident)

	// One reconcile uses one moment for all decisions. A second read of the
	// clock can put a deadline between the two reads. Then the incident can
	// stay in a phase with no timer to change it.
	now := r.now()

	// The reconcile reads the Node one time. The checks of the reference, of
	// the return and of the taint use this read. A write to the Node checks
	// its version, so a change of the Node after the read gives a conflict,
	// and the controller reads the Node again.
	node, err := r.getNode(ctx, incident.Name)
	if apierrors.IsNotFound(err) {
		node, err = nil, nil
	}

	if err != nil {
		return ctrl.Result{}, fmt.Errorf("find node of %q: %w", incident.Name, err)
	}

	// A Node with the same name can be a new Node, and the deletion of a Node
	// can leave its object for a short time. The controller does nothing for
	// an object that does not belong to the Node with its name now. The garbage
	// collector deletes such an object.
	if !r.nodes.Identifies(&incident, node) {
		r.dropTraceOf(incident.Name)

		logger.Info("fencingfailednodestate does not belong to a live node, the controller does nothing for it", "node", incident.Name)

		return ctrl.Result{}, nil
	}

	// The rule of the return is the same for all incidents: in each phase,
	// armed or not, and also when the profile is not usable.
	if back, err := r.releaseReturnedNode(ctx, &incident, node); err != nil || back {
		return ctrl.Result{}, err
	}

	seen := r.observe(&incident, now)

	// A heartbeat after the attestation stops the taint. The series tells only
	// about a heartbeat that the controller saw in a later reconcile than the
	// attestation: a heartbeat seen in the same reconcile can be older.
	reportAliveAfterAttestation(incident.Name, !seen.Attestation.IsZero() && seen.Heartbeat.After(seen.Attestation))

	params, err := r.profiles.Resolve(ctx, &incident)
	if err != nil {
		return r.reportUnusableProfile(ctx, &incident, params.Profile, err, now)
	}

	phase, requeueAfter := fsm.Next(incident.Status.Phase, seen, params, now)

	// Proven means the taint now. The agent tells through the condition
	// Fencing of its Node whether the watchdog is armed. A node without an
	// armed watchdog can be alive, so the incident stays Suspected until the
	// condition is True.
	if phase == v1alpha1.PhaseProven && incident.Status.Phase != v1alpha1.PhaseProven {
		if why := notArmed(node); why != "" {
			logger.Info("failure is proven, but the controller waits for the armed agent", "node", incident.Name, "reason", why)

			phase, requeueAfter = v1alpha1.PhaseSuspected, recheckAfter
		}
	}

	if phase != incident.Status.Phase {
		logger.Info("phase of fencingfailednodestate is decided",
			"node", incident.Name,
			"from", string(incident.Status.Phase),
			"to", string(phase),
			"profile", string(params.Profile),
			"heartbeat_ttl", params.HeartbeatTTL.String(),
			"heartbeat_seen_at", formatSeenAt(seen.Heartbeat),
			"attested_node_taint_after", params.AttestedNodeTaintAfter.String(),
			"silent_node_taint_after", params.SilentNodeTaintAfter.String(),
			"incident_seen_at", formatSeenAt(seen.Incident),
			"attestation_seen_at", formatSeenAt(seen.Attestation),
			"alive_after_attestation", seen.AliveAfterAttestation(),
		)
	}

	conditions := []metav1.Condition{profileResolved(&incident, params.Profile, now)}

	res := ctrl.Result{RequeueAfter: requeueAfter}

	switch phase {
	case v1alpha1.PhaseProven, v1alpha1.PhaseEvicting:
		res, err = r.evict(ctx, &incident, node, phase, seen, params, conditions, now)
	case v1alpha1.PhaseEvicted:
		if err = r.writeStatus(ctx, &incident, phase, "", conditions...); err == nil {
			res, err = r.replace(ctx, incident.Name)
		}
	default:
		err = r.writeStatus(ctx, &incident, phase, "", conditions...)
	}

	if err != nil {
		return ctrl.Result{}, err
	}

	clearConfigurationError(incident.Name)

	return res, nil
}

// callAPI sends a request to the API server on the path of an incident: the
// decision, the taint and the eviction. When an attempt gets no answer in
// requestTimeout, callAPI sends the request again immediately.
//
// A write that timed out can be complete on the server. The next attempt of
// such a write changes nothing:
//   - A write with the version of the object that the controller read fails
//     with a conflict. This is the write of the phase and the patch of the
//     taint. In this case the controller does not write the event of the
//     change.
//   - The patch of the phase Failed of a pod gives the same result again. The
//     deletion of a pod has the UID of the pod, and the controller accepts
//     NotFound and Conflict as a result.
func callAPI(ctx context.Context, send func(context.Context) error) error {
	for attempt := 1; attempt < requestAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		err := send(attemptCtx)
		timedOut := errors.Is(attemptCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil

		cancel()

		if err == nil || !timedOut {
			return err
		}

		logf.FromContext(ctx).Info("request to the API server timed out, the controller sends it again",
			"attempt", attempt, "timeout", requestTimeout.String(), "error", err.Error())
	}

	return send(ctx)
}

// getNode reads the Node from the API server. The cache has no status of a
// Node, and it can be late.
func (r *Reconciler) getNode(ctx context.Context, name string) (*corev1.Node, error) {
	var node corev1.Node

	err := callAPI(ctx, func(ctx context.Context) error {
		return r.reader.Get(ctx, types.NamespacedName{Name: name}, &node)
	})
	if err != nil {
		return nil, err
	}

	return &node, nil
}

// observe records the evidence in the object. It returns the moments when the
// controller saw the object first and when it saw the evidence. A heartbeat or
// an attestation is new when the object has a value that the controller did not
// see before. The controller uses the time of the reconcile, not the time in
// the object.
func (r *Reconciler) observe(incident *v1alpha1.FencingFailedNodeState, now time.Time) fsm.Observations {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := r.seen[incident.Name]
	if seen.uid != incident.UID {
		seen = sighting{uid: incident.UID, at: fsm.Observations{Incident: now}}
	}

	if fallback := incident.Status.Fallback; fallback != nil && fallback.LastHeartbeatAt != nil &&
		!fallback.LastHeartbeatAt.IsZero() && !fallback.LastHeartbeatAt.Equal(&seen.heartbeat) {
		seen.heartbeat = *fallback.LastHeartbeatAt
		seen.at.Heartbeat = now
	}

	if attestation := incident.Status.Attestation; attestation != nil &&
		(attestation.Witness != seen.attestation.Witness || !attestation.At.Equal(&seen.attestation.At)) {
		seen.attestation = *attestation
		seen.at.Attestation = now
	}

	r.seen[incident.Name] = seen

	return seen.at
}

// dropTraceOf removes what the controller keeps about a Node outside its
// object.
func (r *Reconciler) dropTraceOf(node string) {
	r.mu.Lock()
	delete(r.seen, node)
	r.mu.Unlock()

	clearConfigurationError(node)
	clearAliveAfterAttestation(node)
	clearProgress(node)
}

// reportUnusableProfile records a configuration error and does not change the
// phase, because the deadlines of the incident are not known. The name is the
// profile that the resolver found, or empty if it found none.
func (r *Reconciler) reportUnusableProfile(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	name v1alpha1.ProfileName,
	cause error,
	now time.Time,
) (ctrl.Result, error) {
	if !errors.Is(cause, profile.ErrConfiguration) {
		return ctrl.Result{}, fmt.Errorf("resolve SLA profile of %q: %w", incident.Name, cause)
	}

	reportConfigurationError(incident.Name, name)

	// Read the old conditions before the write below changes them.
	isNew := !blockedOnProfile(incident)

	if err := r.writeStatus(ctx, incident, incident.Status.Phase, "", configurationError(incident, cause, now)); err != nil {
		return ctrl.Result{}, err
	}

	// The event marks the moment when fencing stopped. The retries below do
	// not repeat it.
	if isNew {
		r.recorder.Event(incident, corev1.EventTypeWarning, common.ReasonProfileUnavailable, eventPrefix+cause.Error())
	}

	// The module ships the built-in profiles, so a missing or broken profile
	// is expected to come back. The error requeues the incident with backoff,
	// at least once in maxRetryDelay.
	return ctrl.Result{}, fmt.Errorf("fencing of node %q is blocked: %w", incident.Name, cause)
}

func blockedOnProfile(incident *v1alpha1.FencingFailedNodeState) bool {
	return meta.IsStatusConditionTrue(incident.Status.Conditions, common.ConditionTypeConfigurationError)
}

func configurationError(incident *v1alpha1.FencingFailedNodeState, cause error, now time.Time) metav1.Condition {
	return condition(incident, common.ConditionTypeConfigurationError, metav1.ConditionTrue,
		common.ReasonProfileUnavailable, cause.Error(), now)
}

// profileResolved tells that the controller has the timings of the incident.
// An unarmed node needs no SLA profile.
func profileResolved(incident *v1alpha1.FencingFailedNodeState, profile v1alpha1.ProfileName, now time.Time) metav1.Condition {
	message := fmt.Sprintf("The SLA profile %q is in force for this incident.", profile)
	if incident.Spec.ProfileRef == nil {
		message = "The node is not armed, so the incident needs no SLA profile."
	}

	return condition(incident, common.ConditionTypeConfigurationError, metav1.ConditionFalse,
		common.ReasonProfileResolved, message, now)
}

func condition(
	incident *v1alpha1.FencingFailedNodeState,
	conditionType string,
	status metav1.ConditionStatus,
	reason, message string,
	now time.Time,
) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: incident.Generation,
		LastTransitionTime: metav1.NewTime(now),
		Reason:             reason,
		Message:            message,
	}
}

func observedFields(incident *v1alpha1.FencingFailedNodeState) []any {
	fields := []any{
		"node", incident.Name,
		"source", string(incident.Spec.Source),
		"detected_at", formatMicroTime(&incident.Spec.DetectedAt),
		"detected_by", incident.Spec.DetectedBy,
		"armed", incident.Spec.Armed,
		"profile_ref", string(profileName(incident)),
		"phase", string(incident.Status.Phase),
		"generation", incident.Generation,
		"resource_version", incident.ResourceVersion,
	}

	if fallback := incident.Status.Fallback; fallback != nil {
		fields = append(fields,
			"fallback_last_heartbeat_at", formatMicroTime(fallback.LastHeartbeatAt),
		)
	}

	if attestation := incident.Status.Attestation; attestation != nil {
		fields = append(fields,
			"attestation_witness", attestation.Witness,
			"attestation_at", formatMicroTime(&attestation.At),
		)
	}

	if contact := incident.Status.Contact; contact != nil {
		fields = append(fields,
			"contact_reported_by", contact.ReportedBy,
			"contact_at", formatMicroTime(&contact.At),
		)
	}

	if deletedAt := incident.DeletionTimestamp; deletedAt != nil {
		fields = append(fields, "deletion_timestamp", formatTime(deletedAt))
	}

	return fields
}

// profileName returns spec.profileRef.name. It returns an empty name for an
// incident without spec.profileRef.
func profileName(incident *v1alpha1.FencingFailedNodeState) v1alpha1.ProfileName {
	if incident.Spec.ProfileRef == nil {
		return ""
	}

	return incident.Spec.ProfileRef.Name
}

// formatSeenAt returns an observation moment, or an empty string if the
// controller did not observe the evidence.
func formatSeenAt(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format(metav1.RFC3339Micro)
}

func formatTime(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}

func formatMicroTime(t *metav1.MicroTime) string {
	if t == nil || t.IsZero() {
		return ""
	}

	return t.UTC().Format(metav1.RFC3339Micro)
}
