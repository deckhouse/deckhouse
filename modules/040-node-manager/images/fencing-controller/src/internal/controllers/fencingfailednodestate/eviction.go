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
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
	"fencing-controller/internal/domain/fsm"
)

const (
	// outOfServiceValue is the value of the out-of-service taint for a node
	// that is shut down.
	outOfServiceValue = "nodeshutdown"

	// evictionTimeout is the time after the start of Evicting. When the
	// eviction is not complete after it, the incident gets the condition
	// Progressing with the status False.
	evictionTimeout = 60 * time.Second

	// recheckAfter is how often the controller examines a Node again while it
	// waits: for the end of a condition that prevents the taint, or for the
	// end of the eviction.
	recheckAfter = 5 * time.Second

	// podNodeNameField selects the pods of one Node.
	podNodeNameField = "spec.nodeName"

	// podDeletionWorkers is how many pods of one Node the controller deletes
	// at the same time.
	podDeletionWorkers = 10

	// The condition DisruptionTarget of a pod that the controller deletes.
	// The format is the same as the format of PodGC.
	disruptionReason  = "DeletionByFencing"
	disruptionMessage = "Fencing: node is out of service"

	// The fencing agent sets this condition on its Node. The status True tells
	// that the watchdog of the node is armed.
	fencingConditionType = "Fencing"
)

// maintenanceAnnotations tell that the Node is in maintenance.
var maintenanceAnnotations = []string{
	"node-manager.deckhouse.io/fencing-disable",
	"update.node.deckhouse.io/approved",
	"update.node.deckhouse.io/disruption-approved",
}

var outOfServiceTaint = corev1.Taint{
	Key:    corev1.TaintNodeOutOfService,
	Value:  outOfServiceValue,
	Effect: corev1.TaintEffectNoExecute,
}

// evict moves a proven incident through the eviction. First the controller
// writes Proven. The write checks the version of the object that the decision
// used. Only after this write the controller sets the taint and writes
// Evicting. Then the controller deletes the pods of the Node and waits until
// the Node has no VolumeAttachments. The controller deletes pods only when the
// Node has the taint.
func (r *Reconciler) evict(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	node *corev1.Node,
	phase v1alpha1.FencingFailedNodeStatePhase,
	seen fsm.Observations,
	params fsm.Params,
	conditions []metav1.Condition,
	now time.Time,
) (ctrl.Result, error) {
	if err := r.writeStatus(ctx, incident, phase, proofOf(incident, seen, params, now), conditions...); err != nil {
		return ctrl.Result{}, err
	}

	wait, after, err := r.taint(ctx, incident, node, seen, params, now)
	if err != nil {
		return ctrl.Result{}, err
	}

	if wait != "" {
		logf.FromContext(ctx).Info("out-of-service taint waits", "node", incident.Name, "phase", string(phase), "reason", wait)

		return ctrl.Result{RequeueAfter: after}, nil
	}

	if err := r.writeStatus(ctx, incident, v1alpha1.PhaseEvicting, "", conditions...); err != nil {
		return ctrl.Result{}, err
	}

	return r.deletePods(ctx, incident, conditions, now)
}

// taint makes sure that the Node has the out-of-service taint. If the Node has
// no taint, the controller sets it only when its own observations prove the
// failure now. The phase in the object is not a proof: the node can be alive
// again, or this leader did not see the attestation for the full time yet.
//
// read is the Node that the reconcile read, or nil. The patch of the taint
// checks the version of the Node, so after a change of the Node, for example
// AgentStopping, the patch gets a conflict, and taint reads the Node again.
//
// taint returns the reason why the taint must wait and the time after which
// to look again. Zero time means that only a new observation can change the
// result. The reason is empty when the Node has the taint.
func (r *Reconciler) taint(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	read *corev1.Node,
	seen fsm.Observations,
	params fsm.Params,
	now time.Time,
) (string, time.Duration, error) {
	var (
		wait  string
		after time.Duration
	)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node := read
		read = nil

		if node == nil {
			var err error
			if node, err = r.getNode(ctx, incident.Name); err != nil {
				return fmt.Errorf("get node %q: %w", incident.Name, err)
			}
		}

		wait, after = "", 0

		for i := range node.Spec.Taints {
			if node.Spec.Taints[i].MatchTaint(&outOfServiceTaint) {
				return nil
			}
		}

		left, provable := fsm.ProvenAfter(seen, params, now)

		switch {
		case !provable:
			wait = "the controller has no proof: the node is not armed"

			return nil
		case left > 0:
			wait, after = fmt.Sprintf("the observations of the controller prove the failure in %s", left), left

			return nil
		}

		if wait = blocker(node); wait != "" {
			after = recheckAfter

			return nil
		}

		taint := outOfServiceTaint
		taint.TimeAdded = &metav1.Time{Time: now}

		tainted := node.DeepCopy()
		tainted.Spec.Taints = append(tainted.Spec.Taints, taint)

		// The taints are one list. The version check keeps the taints that
		// other writers add at the same time.
		patch := client.MergeFromWithOptions(node, client.MergeFromWithOptimisticLock{})
		if err := callAPI(ctx, func(ctx context.Context) error {
			return r.client.Patch(ctx, tainted, patch)
		}); err != nil {
			return fmt.Errorf("set out-of-service taint on node %q: %w", incident.Name, err)
		}

		logf.FromContext(ctx).Info("out-of-service taint is set", append([]any{
			"profile", string(params.Profile),
			"attested_node_taint_after", params.AttestedNodeTaintAfter.String(),
			"silent_node_taint_after", params.SilentNodeTaintAfter.String(),
			"incident_seen_at", formatSeenAt(seen.Incident),
			"attestation_seen_at", formatSeenAt(seen.Attestation),
			"heartbeat_seen_at", formatSeenAt(seen.Heartbeat),
		}, observedFields(incident)...)...)

		r.announceTaintSet(incident)

		return nil
	})

	return wait, after, err
}

// proofOf tells what proves the failure of the node at now and the delay that
// the controller waited: the attestation of a witness or the silence of the
// node.
func proofOf(
	incident *v1alpha1.FencingFailedNodeState,
	seen fsm.Observations,
	params fsm.Params,
	now time.Time,
) string {
	if left, ok := fsm.AttestedAfter(seen, params, now); ok && left <= 0 && incident.Status.Attestation != nil {
		return fmt.Sprintf("The witness %s attested that the node stopped the feed of its watchdog, "+
			"and %s (watchdog.timeout + watchdog.margin) passed after the controller saw the attestation.",
			incident.Status.Attestation.Witness, params.AttestedNodeTaintAfter)
	}

	return fmt.Sprintf("The node is silent: %s (controlPlaneLostAfter + 0.2s + watchdog.timeout + watchdog.margin + 0.9s) "+
		"passed after its last sign of life that the controller saw.", params.SilentNodeTaintAfter)
}

// blocker returns why the controller must not set the taint on the Node now: a
// maintenance annotation, or the agent of the Node is not armed. It returns an
// empty string when nothing prevents the taint. The state of the pod of the
// agent is not a reason.
func blocker(node *corev1.Node) string {
	for _, key := range maintenanceAnnotations {
		if _, ok := node.Annotations[key]; ok {
			return "node has the maintenance annotation " + key
		}
	}

	return notArmed(node)
}

// notArmed returns why the agent of the Node does not tell that its watchdog
// is armed: the condition Fencing of the Node is not True. It returns an empty
// string when the condition is True. The agent writes the condition. When the
// node stops, the condition keeps the last value that the agent wrote.
func notArmed(node *corev1.Node) string {
	if node == nil {
		return "the node is not found"
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type != fencingConditionType {
			continue
		}

		if condition.Status == corev1.ConditionTrue {
			return ""
		}

		return fmt.Sprintf("the condition %s of the node is %s with the reason %q", fencingConditionType, condition.Status, condition.Reason)
	}

	return "the node has no condition " + fencingConditionType
}

// deletePods deletes the pods of the Node, also the pods that tolerate the
// out-of-service taint. It does not delete the pods of a DaemonSet and the
// mirror pods of the static pods. A pod that stays in the API because of a
// finalizer does not stop the eviction. When the Node has no
// VolumeAttachments, the volumes of the pods are free for other nodes. Then
// the incident is Evicted, and after this write the controller sends the
// deletion that gets a new machine, if the cloud replaces the Node.
//
// The incident stays in Evicting until the eviction is complete. The
// controller examines the Node again and again. When the eviction is not
// complete after evictionTimeout, the condition Progressing with the status
// False tells what stops it now.
func (r *Reconciler) deletePods(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	conditions []metav1.Condition,
	now time.Time,
) (ctrl.Result, error) {
	deletion, reason, message, err := r.evacuate(ctx, incident, now)
	if err != nil {
		reason, message = common.ReasonPodsRemaining, err.Error()
	}

	if reason == "" {
		// The deletion of the Node also deletes the object, so Evicted comes
		// first. When the write fails, for example on a conflict, the
		// controller deletes nothing.
		if err := r.writeStatus(ctx, incident, v1alpha1.PhaseEvicted, "", conditions...); err != nil || deletion == nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, deletion(ctx)
	}

	logf.FromContext(ctx).Info("eviction of the node is not complete", "node", incident.Name, "reason", reason, "message", message)

	// A new leader starts the time again.
	previous := meta.FindStatusCondition(incident.Status.Conditions, common.ConditionTypeProgressing)
	left := r.evictingSince(incident, now).Add(evictionTimeout).Sub(now)
	late := left <= 0

	if late {
		conditions = append(conditions, condition(incident, common.ConditionTypeProgressing, metav1.ConditionFalse, reason, message, now))
	}

	if err := r.writeStatus(ctx, incident, v1alpha1.PhaseEvicting, "", conditions...); err != nil {
		return ctrl.Result{}, err
	}

	if late && (previous == nil || previous.Reason != reason) {
		r.announceNotProgressing(incident, reason, message)
	}

	// The cause, also an error, is in the log and in the condition. The
	// controller examines the Node again after recheckAfter, not with the
	// backoff of an error.
	if left > 0 {
		return ctrl.Result{RequeueAfter: min(recheckAfter, left)}, nil
	}

	return ctrl.Result{RequeueAfter: recheckAfter}, nil
}

// evacuate deletes the pods of the Node, podDeletionWorkers pods at the same
// time. It returns the reason and the message of what stops the eviction now.
// The reason is empty when the eviction is complete. Then evacuate also returns
// the deletion that gets a new machine, or nil when the Node is not replaced.
func (r *Reconciler) evacuate(
	ctx context.Context,
	incident *v1alpha1.FencingFailedNodeState,
	now time.Time,
) (func(context.Context) error, string, string, error) {
	pods, err := r.podsOn(ctx, incident.Name)
	if err != nil {
		return nil, "", "", err
	}

	var (
		deletions errgroup.Group
		mu        sync.Mutex
		failures  []string
	)

	deletions.SetLimit(podDeletionWorkers)

	for i := range pods {
		pod := &pods[i]
		if staysOnNode(pod) {
			continue
		}

		// A failure of one pod does not stop the deletion of the other pods.
		deletions.Go(func() error {
			if err := r.failAndDelete(ctx, incident.Name, pod, now); err != nil {
				mu.Lock()
				failures = append(failures, err.Error())
				mu.Unlock()
			}

			return nil
		})
	}

	_ = deletions.Wait()

	if len(failures) > 0 {
		// The order of the deletions changes. The sort keeps the same
		// message for the same failures.
		slices.Sort(failures)

		return nil, common.ReasonPodsRemaining, "The controller could not delete pods of the node: " + strings.Join(failures, "; "), nil
	}

	attachments, err := r.attachmentsOf(ctx, incident.Name)
	if err != nil {
		return nil, "", "", err
	}

	if len(attachments) > 0 {
		return nil, common.ReasonVolumesAttached, fmt.Sprintf("Volume attachments of the node remain: %s.", strings.Join(attachments, ", ")), nil
	}

	deletion, refused, err := r.replacement(ctx, incident.Name)
	if err != nil || refused == "" {
		return deletion, "", "", err
	}

	return nil, common.ReasonTooManyUnhealthyMachines, refused, nil
}

// failAndDelete sets the phase Failed and the condition DisruptionTarget of
// the pod, and then deletes the pod with no grace period, as PodGC does. The
// Node is dead, so the kubelet cannot do this. The owners of the finalizers,
// for example the Job controller, remove them when the pod is Failed. The
// controller does not remove the finalizers.
func (r *Reconciler) failAndDelete(ctx context.Context, node string, pod *corev1.Pod, now time.Time) error {
	if !terminal(pod) {
		failed := pod.DeepCopy()
		failed.Status.Phase = corev1.PodFailed
		setDisruptionTarget(&failed.Status, now)

		// Only the patched object has the UID, so the patch has it. The UID
		// keeps a new pod with the same name.
		from := pod.DeepCopy()
		from.UID = ""

		err := callAPI(ctx, func(ctx context.Context) error {
			return r.client.Status().Patch(ctx, failed, client.StrategicMergeFrom(from))
		})

		switch {
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return fmt.Errorf("set phase Failed of pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}
	}

	// A deletion with no grace period is already sent. The pod waits for
	// its finalizers. A deletion with a grace period, for example from the
	// taint, waits for the kubelet of the dead Node, so the controller sends
	// its own.
	if pod.DeletionTimestamp != nil && pod.DeletionGracePeriodSeconds != nil && *pod.DeletionGracePeriodSeconds == 0 {
		return nil
	}

	// The UID keeps a new pod with the same name.
	uid := pod.UID

	err := callAPI(ctx, func(ctx context.Context) error {
		return r.client.Delete(ctx, pod, client.GracePeriodSeconds(0), client.Preconditions{UID: &uid})
	})

	switch {
	case apierrors.IsNotFound(err), apierrors.IsConflict(err):
	case err != nil:
		return fmt.Errorf("delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
	default:
		owner := ""
		if ref := metav1.GetControllerOf(pod); ref != nil {
			owner = ref.Kind + "/" + ref.Name
		}

		logf.FromContext(ctx).Info("pod of the dead node is Failed and deleted",
			"node", node, "pod", pod.Namespace+"/"+pod.Name, "owner", owner,
			"tolerates_out_of_service", toleratesOutOfService(pod), "finalizers", pod.Finalizers)
	}

	return nil
}

// setDisruptionTarget sets the condition DisruptionTarget of a pod that the
// controller deletes.
func setDisruptionTarget(status *corev1.PodStatus, now time.Time) {
	target := corev1.PodCondition{
		Type:               corev1.DisruptionTarget,
		Status:             corev1.ConditionTrue,
		Reason:             disruptionReason,
		Message:            disruptionMessage,
		LastTransitionTime: metav1.NewTime(now),
	}

	for i := range status.Conditions {
		if status.Conditions[i].Type == corev1.DisruptionTarget {
			status.Conditions[i] = target

			return
		}
	}

	status.Conditions = append(status.Conditions, target)
}

// evictingSince returns when the controller saw the incident in Evicting first.
// A new leader starts this time again, so it can only delay the condition
// Progressing.
func (r *Reconciler) evictingSince(incident *v1alpha1.FencingFailedNodeState, now time.Time) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := r.seen[incident.Name]
	if seen.evicting.IsZero() {
		seen.evicting = now
		r.seen[incident.Name] = seen
	}

	return seen.evicting
}

// podsOn reads the pods of the Node from the API server. The field selector
// keeps the controller from reading the pods of all Nodes.
func (r *Reconciler) podsOn(ctx context.Context, node string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := callAPI(ctx, func(ctx context.Context) error {
		return r.reader.List(ctx, &pods, client.MatchingFields{podNodeNameField: node})
	}); err != nil {
		return nil, fmt.Errorf("list pods on node %q: %w", node, err)
	}

	return pods.Items, nil
}

// attachmentsOf returns the names of the VolumeAttachments of the Node. It
// reads them from the API server.
func (r *Reconciler) attachmentsOf(ctx context.Context, node string) ([]string, error) {
	var attachments storagev1.VolumeAttachmentList
	if err := callAPI(ctx, func(ctx context.Context) error {
		return r.reader.List(ctx, &attachments)
	}); err != nil {
		return nil, fmt.Errorf("list volumeattachments of node %q: %w", node, err)
	}

	var names []string

	for i := range attachments.Items {
		if attachments.Items[i].Spec.NodeName == node {
			names = append(names, attachments.Items[i].Name)
		}
	}

	return names, nil
}

// staysOnNode tells if the eviction leaves the pod on the Node: a pod of a
// DaemonSet or the mirror pod of a static pod.
func staysOnNode(pod *corev1.Pod) bool {
	if _, ok := pod.Annotations[corev1.MirrorPodAnnotationKey]; ok {
		return true
	}

	owner := metav1.GetControllerOf(pod)

	return owner != nil && owner.Kind == "DaemonSet"
}

// terminal tells if the pod is in the phase Succeeded or Failed.
func terminal(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}

func toleratesOutOfService(pod *corev1.Pod) bool {
	for i := range pod.Spec.Tolerations {
		if pod.Spec.Tolerations[i].ToleratesTaint(&outOfServiceTaint) {
			return true
		}
	}

	return false
}
