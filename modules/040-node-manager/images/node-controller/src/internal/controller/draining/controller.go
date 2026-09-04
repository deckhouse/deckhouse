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
	"sigs.k8s.io/controller-runtime/pkg/event"
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

func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.WithEventFilter(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return inNodeGroup(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return inNodeGroup(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return inNodeGroup(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !inNodeGroup(e.ObjectNew) {
				return false
			}

			oldNode, ok := e.ObjectOld.(*corev1.Node)
			if !ok {
				return false
			}

			newNode, ok := e.ObjectNew.(*corev1.Node)
			if !ok {
				return false
			}

			old := stateFromNode(oldNode)
			new := stateFromNode(newNode)
			return !new.equal(old)
		},
	})

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

	state := stateFromNode(node)

	if state.requestedBy == "" {
		if err := r.cancelDrainIfExist(ctx, logger, node); err != nil {
			return ctrl.Result{}, err
		}
		if state.recordedFor == userSource && !state.unschedulable {
			logger.Info("removing a stale drained=user annotation")
			delete(node.Annotations, nodecommon.DrainedAnnotation)
		}
		return ctrl.Result{}, nil
	}

	// A hand drain's marker is cleared before a new drain starts. Left there it
	// would read as the new drain's own result, and a second hand drain would
	// never overwrite it — finishDrain records nothing for the user source.
	if state.recordedFor == userSource {
		logger.Info("removing an existing drained=user annotation before a new drain")
		delete(node.Annotations, nodecommon.DrainedAnnotation)
		return ctrl.Result{}, nil
	}

	// The drain's outcome decides what is written, so it is collected first.
	if finished, drainErr := r.drains.result(node.Name); finished {
		return ctrl.Result{}, r.finishDrain(ctx, logger, node, state.requestedBy, drainErr)
	}

	return ctrl.Result{}, r.startDrain(ctx, logger, node, state.requestedBy)
}

// cleanupDeletedNode stops a drain running for an object nobody can see.
func (r *Reconciler) cleanupDeletedNode(ctx context.Context, nodeName string) error {
	clearDrainMetric(nodeName)
	_, err := r.drains.cancel(ctx, nodeName)
	return err
}

// cancelDrainIfExist stops the drain when its request disappears, so a drain
// nobody asked for any more does not run to completion and record a result.
func (r *Reconciler) cancelDrainIfExist(ctx context.Context, logger logr.Logger, node *corev1.Node) error {
	clearDrainMetric(node.Name)

	cancelled, err := r.drains.cancel(ctx, node.Name)
	if err != nil {
		return err
	}
	if !cancelled {
		return nil
	}

	logger.Info("drain canceled")
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "DrainCancelled",
		"drain of node %q was cancelled", node.Name)
	return nil
}

// finishDrain writes down how the drain ended. The request is consumed
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
		logger.Info("drain timed out, recording it as done anyway", "error", drainErr.Error())
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "DrainFailed", "drain failed: %v", drainErr)
		nodeDrainingGauge.WithLabelValues(node.Name, drainErr.Error()).Set(1)

	default:
		logger.Error(drainErr, "drain failed")
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "DrainFailed", "drain failed: %v", drainErr)
		nodeDrainingGauge.WithLabelValues(node.Name, drainErr.Error()).Set(1)
		// The request stays, so the requeue starts a fresh drain.
		return drainErr
	}

	delete(node.Annotations, nodecommon.DrainingAnnotation)
	node.Annotations[nodecommon.DrainedAnnotation] = source
	logger.Info("drain finished")
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "DrainSucceeded", "node %q drained successfully", node.Name)
	return nil
}

// startDrain cordons the node, then starts the drain on the pass that sees
// the cordon. Not both at once: this reconcile's patch is still unsent, and
// pods must stop arriving before anything empties the node. The cordon's own
// event brings us back.
func (r *Reconciler) startDrain(ctx context.Context, logger logr.Logger, node *corev1.Node, source string) error {
	if !node.Spec.Unschedulable {
		logger.Info("cordoning node", "source", source)
		node.Spec.Unschedulable = true

		// The drain starts on the pass this reconcile's own patch brings
		// about: writing spec.unschedulable is one of the changes the watch
		// admits, so nothing has to be requeued to come back.
		return nil
	}

	timeout := nodecommon.DrainTimeout(ctx, r.Client, node.Labels[nodecommon.NodeGroupLabel])

	return r.drains.start(logger, node.Name, timeout)
}

// drainSource reads the drain request off a node, empty when nobody asked. A
// bare annotation means bashible: that is how the original hook behaved, and old
// scripts still set it.
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

func inNodeGroup(obj client.Object) bool {
	_, hasGroup := obj.GetLabels()[nodecommon.NodeGroupLabel]
	return hasGroup
}

func stateFromNode(node *corev1.Node) state {
	return state{
		requestedBy:   drainSource(node, nodecommon.DrainingAnnotation),
		recordedFor:   drainSource(node, nodecommon.DrainedAnnotation),
		unschedulable: node.Spec.Unschedulable,
	}
}

type state struct {
	requestedBy   string
	recordedFor   string
	unschedulable bool
}

func (s state) equal(other state) bool {
	return s == other
}
