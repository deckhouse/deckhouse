/*
Copyright 2025 Flant JSC

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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/register"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

const (
	drainingAnnotationKey = "update.node.deckhouse.io/draining"
	drainedAnnotationKey  = "update.node.deckhouse.io/drained"
	nodeGroupLabel        = "node.deckhouse.io/group"
	defaultDrainTimeout   = 10 * time.Minute
)

func init() {
	register.RegisterController(register.NodeDraining, &corev1.Node{}, &Reconciler{})
}

type Reconciler struct {
	dynctrl.Base
	kubeClient kubernetes.Interface
}

func (r *Reconciler) SetupWatches(w dynctrl.Watcher) {
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, hasGroup := obj.GetLabels()[nodeGroupLabel]
		return hasGroup
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	drainingSource := node.Annotations[drainingAnnotationKey]
	drainedSource := node.Annotations[drainedAnnotationKey]

	if drainingSource == "" && drainedSource == "" {
		return ctrl.Result{}, nil
	}

	// If the node became schedulable but 'drained' annotation is still present (user source), remove it
	if drainingSource == "" && drainedSource == "user" && !node.Spec.Unschedulable {
		return ctrl.Result{}, r.patchAnnotations(ctx, node.Name, map[string]interface{}{
			drainedAnnotationKey: nil,
		})
	}

	if drainingSource == "" {
		return ctrl.Result{}, nil
	}

	// If the node is marked for draining while it has been drained by user, remove 'drained'
	if drainedSource == "user" {
		if err := r.patchAnnotations(ctx, node.Name, map[string]interface{}{
			drainedAnnotationKey: nil,
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Ensure kubeClient is initialized for drain operations
	if err := r.ensureKubeClient(); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Get drain timeout from NodeGroup
	drainTimeout := r.getDrainTimeout(ctx, node.Labels[nodeGroupLabel])

	// Cordon the node
	if err := r.cordonNode(ctx, node); err != nil {
		logger.Error(err, "failed to cordon node", "node", node.Name)
		return ctrl.Result{}, err
	}

	// Run drain
	logger.Info("node draining started", "node", node.Name)
	drainCtx, cancel := context.WithTimeout(ctx, drainTimeout)
	defer cancel()

	err := r.drainNode(drainCtx, node.Name)
	if err != nil {
		logger.Error(err, "node drain failed", "node", node.Name)
		r.Recorder.Eventf(node, corev1.EventTypeWarning, "DrainFailed", "drain failed: %v", err)

		// On timeout, still mark as drained (matching original hook behavior)
		if drainCtx.Err() != nil {
			logger.Info("node drain timeout, marking as drained anyway", "node", node.Name)
		} else {
			return ctrl.Result{}, err
		}
	}

	logger.Info("node draining finished", "node", node.Name)

	// Remove draining, set drained
	return ctrl.Result{}, r.patchAnnotations(ctx, node.Name, map[string]interface{}{
		drainingAnnotationKey: nil,
		drainedAnnotationKey:  drainingSource,
	})
}

func (r *Reconciler) ensureKubeClient() error {
	if r.kubeClient != nil {
		return nil
	}

	config := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	r.kubeClient = clientset
	return nil
}

func (r *Reconciler) getDrainTimeout(ctx context.Context, ngName string) time.Duration {
	if ngName == "" {
		return defaultDrainTimeout
	}

	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ngName}, ng); err != nil {
		return defaultDrainTimeout
	}

	if ng.Spec.NodeDrainTimeoutSecond != nil {
		return time.Duration(*ng.Spec.NodeDrainTimeoutSecond) * time.Second
	}

	return defaultDrainTimeout
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
	return drainNodePods(ctx, r.kubeClient, nodeName)
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

var _ dynctrl.Reconciler = (*Reconciler)(nil)
