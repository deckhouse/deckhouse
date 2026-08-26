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

package draining

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-draining", &corev1.Node{}, &Reconciler{})
}

const (
	bashibleSource = "bashible"
	userSource     = "user"
)

// Reconciler empties a node when somebody annotates it, and answers with a
// second annotation naming whoever asked. The requesters — bashible, cloud
// hooks, updateapproval, an operator — cannot evict pods themselves.
type Reconciler struct {
	register.Base

	drains *drainer
}

func (r *Reconciler) Setup(ctx context.Context, mgr ctrl.Manager) error {
	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("create the kubernetes client: %w", err)
	}

	r.drains = newDrainer(ctx, kubeClient)
	return nil
}

// ForPredicates admits only nodes in a NodeGroup. Deliberately not
// WithEventFilter, which applies to every source and would drop the name-only
// Node a finished eviction sends down the wake channel.
func (r *Reconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			_, hasGroup := obj.GetLabels()[nodecommon.NodeGroupLabel]
			return hasGroup
		}),
	}
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.WatchesRawSource(r.drains.wakeSource())
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.cleanupDeletedNode(ctx, req.Name)
		}
		return ctrl.Result{}, err
	}

	return r.reconcileNode(ctx, node)
}

func (r *Reconciler) reconcileNode(ctx context.Context, node *corev1.Node) (_ ctrl.Result, resErr error) {
	logger := log.FromContext(ctx).WithValues("node", node.Name)

	patchHelper, err := patch.NewHelper(node, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	defer func() {
		if err := patchHelper.Patch(ctx, node); err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("failed to patch Node %s: %w", node.Name, err))
		}
	}()

	requestedBy := drainSource(node, nodecommon.DrainingAnnotation)
	recordedFor := drainSource(node, nodecommon.DrainedAnnotation)

	// drained=user is never written any more, so it can only be a leftover: an
	// upgraded cluster, or somebody setting it by hand.
	if recordedFor == userSource {
		logger.Info("removing an orphan drain result")
		delete(node.Annotations, nodecommon.DrainedAnnotation)
		return ctrl.Result{}, nil
	}

	// The request is gone: whoever asked has changed their mind.
	if requestedBy == "" {
		return ctrl.Result{}, r.cancelDrain(ctx, logger, node)
	}

	// The eviction's outcome decides what is written, so it is collected first.
	if finished, drainErr := r.drains.result(node.Name); finished {
		return ctrl.Result{}, r.finishDrain(ctx, logger, node, requestedBy, drainErr)
	}

	return ctrl.Result{}, r.startDrain(ctx, logger, node, requestedBy)
}

// drainSource reads one drain annotation, empty when the node does not carry it.
func drainSource(node *corev1.Node, annotation string) string {
	source, ok := node.Annotations[annotation]
	if !ok {
		return ""
	}
	if source == "" {
		return bashibleSource
	}

	return source
}

// cleanupDeletedNode stops an eviction running for an object nobody can see.
func (r *Reconciler) cleanupDeletedNode(ctx context.Context, nodeName string) error {
	clearDrainMetric(nodeName)
	_, err := r.drains.cancel(ctx, nodeName)
	return err
}

// cancelDrain gives the node back when its request disappears.
//
// Only a running eviction is undone. A drain that succeeds consumes its own
// request, so every node passes through here — uncordoning them all would
// return half-updated nodes to service, and past that point the cordon belongs
// to updateapproval. A restart forgets the eviction, and such a node keeps its
// cordon until someone runs kubectl uncordon.
func (r *Reconciler) cancelDrain(ctx context.Context, logger logr.Logger, node *corev1.Node) error {
	clearDrainMetric(node.Name)

	cancelled, err := r.drains.cancel(ctx, node.Name)
	if err != nil {
		return err
	}
	if !cancelled {
		return nil
	}

	logger.Info("request withdrawn, node going back into service")
	node.Spec.Unschedulable = false
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "DrainCancelled",
		"drain of node %q was cancelled, node is schedulable again", node.Name)
	return nil
}

// finishDrain writes down how the eviction ended. The request is consumed
// either way; only a source that polls for a result gets one.
func (r *Reconciler) finishDrain(_ context.Context, logger logr.Logger, node *corev1.Node, source string, drainErr error) error {
	logger = logger.WithValues("source", source)

	switch {
	case drainErr == nil:
		clearDrainMetric(node.Name)

	case errors.Is(drainErr, errDrainDeadline):
		// Recorded as drained anyway: the cause is durable, a retry gets no
		// further, and the node's update must not wedge. The gauge stays at 1
		// for NodeStuckInDraining.
		logger.Info("eviction timed out, recording it as done anyway", "error", drainErr.Error())
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "DrainFailed", "drain failed: %v", drainErr)
		nodeDrainingGauge.WithLabelValues(node.Name, drainErr.Error()).Set(1)

	default:
		logger.Error(drainErr, "eviction failed")
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "DrainFailed", "drain failed: %v", drainErr)
		nodeDrainingGauge.WithLabelValues(node.Name, drainErr.Error()).Set(1)
		// The request stays, so the requeue starts a fresh eviction.
		return drainErr
	}

	delete(node.Annotations, nodecommon.DrainingAnnotation)
	if source != userSource {
		node.Annotations[nodecommon.DrainedAnnotation] = source
	}

	logger.Info("drain finished")
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "DrainSucceeded", "node %q drained successfully", node.Name)
	return nil
}

// startDrain cordons the node, then starts the eviction on the pass that sees
// the cordon. Not both at once: this reconcile's patch is still unsent, and
// pods must stop arriving before anything empties the node. The cordon's own
// event brings us back.
func (r *Reconciler) startDrain(ctx context.Context, logger logr.Logger, node *corev1.Node, source string) error {
	timeout := nodecommon.DrainTimeout(ctx, r.Client, node.Labels[nodecommon.NodeGroupLabel])

	if !node.Spec.Unschedulable {
		logger.Info("cordoning node", "source", source)
		node.Spec.Unschedulable = true
		return nil
	}

	return r.drains.start(logger, node.Name, timeout)
}
