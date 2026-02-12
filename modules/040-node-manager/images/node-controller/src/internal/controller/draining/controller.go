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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubedrain "github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

const (
	defaultDrainTimeout = 10 * time.Minute
)

type Reconciler struct {
	dynctrl.Base
	kubeClient kubernetes.Interface
}

func (r *Reconciler) Setup(mgr ctrl.Manager) error {
	var err error
	r.kubeClient, err = kubernetes.NewForConfig(mgr.GetConfig())
	return err
}

func (r *Reconciler) SetupWatches(_ dynctrl.Watcher) {}

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

	if drainingSource == "" {
		clearDrainMetric(node.Name)

		if drainedSource == "" {
			logger.V(1).Info("skipping: no draining/drained annotations", "node", node.Name)
			return ctrl.Result{}, nil
		}

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

	// If the node is marked for draining while it has been drained by user, remove 'drained'
	if drainedSource == "user" {
		logger.Info("removing existing drained=user annotation before new drain", "node", node.Name)
		if err := r.patchAnnotations(ctx, node.Name, map[string]interface{}{
			nodecommon.DrainedAnnotation: nil,
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	drainTimeout := r.getDrainTimeout(ctx, node.Labels[nodecommon.NodeGroupLabel])
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

	// Run drain
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

	logger.Info("drain completed, updating annotations",
		"node", node.Name,
		"removingAnnotation", nodecommon.DrainingAnnotation,
		"settingAnnotation", nodecommon.DrainedAnnotation,
		"value", drainingSource,
	)

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

func (r *Reconciler) getDrainTimeout(ctx context.Context, ngName string) time.Duration {
	if ngName == "" {
		return defaultDrainTimeout
	}

	ng, err := nodecommon.GetNodeGroup(ctx, r.Client, ngName)
	if err != nil {
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

var _ dynctrl.Reconciler = (*Reconciler)(nil)
