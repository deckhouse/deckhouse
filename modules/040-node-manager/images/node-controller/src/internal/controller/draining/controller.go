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
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kubedrain "github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-draining", &corev1.Node{}, &Reconciler{})
}

// spotTerminationLabel is set by the cloud provider when the cloud announces that the spot
// instance behind this node is about to be reclaimed
// (modules/030-cloud-provider-aws/hooks/add_termination_metadata.go:37).
const spotTerminationLabel = "node.deckhouse.io/termination-in-progress"

type Reconciler struct {
	register.Base
	kubeClient kubernetes.Interface
}

func (r *Reconciler) Setup(_ context.Context, mgr ctrl.Manager) error {
	var err error
	r.kubeClient, err = kubernetes.NewForConfig(mgr.GetConfig())
	return err
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, hasGroup := obj.GetLabels()[nodecommon.NodeGroupLabel]
		return hasGroup
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			clearDrainMetric(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Backward compatibility: treat empty annotation value as "bashible" (original hook behavior).
	var drainingSource, drainedSource string
	if source, ok := node.Annotations[nodecommon.DrainingAnnotation]; ok {
		if source == "" {
			drainingSource = "bashible"
		} else {
			drainingSource = source
		}
	}
	if source, ok := node.Annotations[nodecommon.DrainedAnnotation]; ok {
		if source == "" {
			drainedSource = "bashible"
		} else {
			drainedSource = source
		}
	}

	// A spot node whose drain is over has no reason to live: deleting its Instance releases the
	// VM. A drain still in flight wins, so a drained annotation left over from an earlier bashible
	// run cannot release the VM before the workloads have been evicted for this termination.
	if drainingSource == "" && drainedSource != "" && node.Labels[spotTerminationLabel] == "true" {
		return ctrl.Result{}, r.deleteInstance(ctx, node.Name)
	}

	if drainingSource == "" {
		clearDrainMetric(node.Name)

		if drainedSource == "user" && !node.Spec.Unschedulable {
			logger.Info("removing stale drained=user annotation from schedulable node", "node", node.Name)
			return ctrl.Result{}, r.patchAnnotations(ctx, node.Name, map[string]interface{}{
				nodecommon.DrainedAnnotation: nil,
			})
		}

		logger.V(1).Info("skipping: no draining annotation", "node", node.Name, "drainedSource", drainedSource)
		return ctrl.Result{}, nil
	}

	logger.Info("node drain requested", "node", node.Name, "source", drainingSource, "nodeGroup", node.Labels[nodecommon.NodeGroupLabel])

	if drainedSource == "user" {
		logger.Info("removing existing drained=user annotation before new drain", "node", node.Name)
		if err := r.patchAnnotations(ctx, node.Name, map[string]interface{}{
			nodecommon.DrainedAnnotation: nil,
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	drainTimeout := nodecommon.DrainTimeout(ctx, r.Client, node.Labels[nodecommon.NodeGroupLabel])
	logger.V(1).Info("drain timeout resolved", "node", node.Name, "timeout", drainTimeout)

	if node.Spec.Unschedulable {
		logger.V(1).Info("node already cordoned", "node", node.Name)
	} else {
		logger.Info("cordoning node", "node", node.Name)
	}
	if err := r.cordonNode(ctx, node); err != nil {
		logger.Error(err, "failed to cordon node", "node", node.Name)
		return ctrl.Result{}, err
	}

	logger.Info("draining node pods", "node", node.Name, "timeout", drainTimeout)
	drainCtx, cancel := context.WithTimeout(ctx, drainTimeout)
	defer cancel()

	err := r.drainNode(drainCtx, node.Name)
	if err != nil {
		logger.Error(err, "node drain failed", "node", node.Name)
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "DrainFailed", "drain failed: %v", err)
		nodeDrainingGauge.WithLabelValues(node.Name, err.Error()).Set(1)

		if drainCtx.Err() != nil {
			logger.Info("drain timed out, marking as drained anyway", "node", node.Name, "timeout", drainTimeout)
		} else {
			return ctrl.Result{}, err
		}
	} else {
		clearDrainMetric(node.Name)
	}

	if err := r.patchAnnotations(ctx, node.Name, map[string]interface{}{
		nodecommon.DrainingAnnotation: nil,
		nodecommon.DrainedAnnotation:  drainingSource,
	}); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("drain completed, annotations updated",
		"node", node.Name,
		"source", drainingSource,
	)
	r.Recorder.Eventf(node, corev1.EventTypeNormal, "DrainSucceeded", "node %q drained successfully", node.Name)

	return ctrl.Result{}, nil
}

// deleteInstance removes the Instance of a reclaimed spot node. The Instance is read first so the
// steady state of an already-terminating node costs a cached read instead of a delete call on every
// kubelet heartbeat.
func (r *Reconciler) deleteInstance(ctx context.Context, name string) error {
	logger := log.FromContext(ctx)

	instance := &deckhousev1alpha2.Instance{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get Instance %s: %w", name, err)
	}
	if instance.DeletionTimestamp != nil {
		logger.V(1).Info("skipping: Instance of the spot node is already being deleted", "instance", name)
		return nil
	}

	logger.Info("spot node drained, deleting its Instance", "instance", name)
	if err := r.Client.Delete(ctx, instance, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete Instance %s: %w", name, err)
	}

	return nil
}

func (r *Reconciler) cordonNode(ctx context.Context, node *corev1.Node) error {
	if node.Spec.Unschedulable {
		return nil
	}

	patch, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"unschedulable": true,
		},
	})
	if err != nil {
		return err
	}

	return r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patch))
}

func (r *Reconciler) drainNode(ctx context.Context, nodeName string) error {
	timeout := time.Duration(0) // 0 means infinite; actual timeout is controlled by ctx
	drainer := kubedrain.NewDrainer(kubedrain.HelperConfig{
		Client:  r.kubeClient,
		Timeout: &timeout,
	})
	drainer.Ctx = ctx

	return kubedrain.RunNodeDrain(drainer, nodeName)
}

func (r *Reconciler) patchAnnotations(ctx context.Context, nodeName string, annotations map[string]interface{}) error {
	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	})
	if err != nil {
		return err
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	return r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patch))
}
