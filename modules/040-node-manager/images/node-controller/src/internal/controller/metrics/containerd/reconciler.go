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

package containerd

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	nodeGroupLabel               = "node.deckhouse.io/group"
	containerdV2UnsupportedLabel = "node.deckhouse.io/containerd-v2-unsupported"
	cgroupLabel                  = "node.deckhouse.io/cgroup"
	cgroupV2Value                = "cgroup2fs"
)

var (
	cntrdV2Unsupported = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_nodes_cntrd_v2_unsupported",
		Help: "Set to 1 if the node has the containerd-v2-unsupported label.",
	}, []string{"node", "node_group", "cgroup_version"})

	cgroupV2Unsupported = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_node_cgroup_v2_unsupported",
		Help: "Set to 1 if the node cgroup version is not cgroup2fs.",
	}, []string{"node", "node_group", "cgroup_version"})
)

func init() {
	prometheus.MustRegister(cntrdV2Unsupported, cgroupV2Unsupported)
	dynr.RegisterReconciler(rcname.MetricsContainerd, &corev1.Node{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler exports Prometheus metrics about containerd v2 and cgroup version support per Node.
// It mirrors the logic from the cntrd_v2_support.go addon-operator hook.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, ok := obj.GetLabels()[nodeGroupLabel]
		return ok
	})}
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Node deleted - remove its metrics.
			// We cannot know the exact label values, so we reset and rebuild.
			return r.reconcileAll(ctx)
		}
		return ctrl.Result{}, fmt.Errorf("get Node %s: %w", req.Name, err)
	}

	ngName, ok := node.Labels[nodeGroupLabel]
	if !ok {
		return ctrl.Result{}, nil
	}

	_, hasUnsupportedLabel := node.Labels[containerdV2UnsupportedLabel]
	cgroupVersion := node.Labels[cgroupLabel]

	labels := prometheus.Labels{
		"node":           node.Name,
		"node_group":     ngName,
		"cgroup_version": cgroupVersion,
	}

	if hasUnsupportedLabel {
		cntrdV2Unsupported.With(labels).Set(1.0)
	} else {
		cntrdV2Unsupported.Delete(labels)
	}

	if cgroupVersion != cgroupV2Value {
		cgroupV2Unsupported.With(labels).Set(1.0)
	} else {
		cgroupV2Unsupported.Delete(labels)
	}

	log.V(1).Info("updated containerd metrics", "node", node.Name, "nodeGroup", ngName)
	return ctrl.Result{}, nil
}

// reconcileAll rebuilds all metrics by listing all matching Nodes.
// Used when a deletion makes it impossible to remove a single metric series.
func (r *Reconciler) reconcileAll(ctx context.Context) (ctrl.Result, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.HasLabels{nodeGroupLabel}); err != nil {
		return ctrl.Result{}, fmt.Errorf("list Nodes: %w", err)
	}

	cntrdV2Unsupported.Reset()
	cgroupV2Unsupported.Reset()

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		ngName := node.Labels[nodeGroupLabel]
		_, hasUnsupportedLabel := node.Labels[containerdV2UnsupportedLabel]
		cgroupVersion := node.Labels[cgroupLabel]

		labels := prometheus.Labels{
			"node":           node.Name,
			"node_group":     ngName,
			"cgroup_version": cgroupVersion,
		}

		if hasUnsupportedLabel {
			cntrdV2Unsupported.With(labels).Set(1.0)
		}

		if cgroupVersion != cgroupV2Value {
			cgroupV2Unsupported.With(labels).Set(1.0)
		}
	}

	return ctrl.Result{}, nil
}
