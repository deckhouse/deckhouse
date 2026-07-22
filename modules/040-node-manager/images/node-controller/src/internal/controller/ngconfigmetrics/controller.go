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

// Package ngconfigmetrics exports the d8_node_group_configurations_total metric:
// the number of NodeGroupConfigurations aggregated by the NodeGroups they target.
//
// This replaces the shell-operator hook hooks/metrics_node_group_configurations.go.
// The hook watched NodeGroupConfiguration and, on every event, expired the metric
// group and re-emitted one series per node_group. The controller reconciles a
// NodeGroupConfiguration reactively and recomputes the whole aggregate with a full
// LIST, resetting the gauge before setting the current counts — identical behavior.
package ngconfigmetrics

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/deckhouse/node-controller/internal/register"
)

var ngConfigurationsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "d8_node_group_configurations_total",
	Help: "Number of NodeGroupConfigurations targeting a NodeGroup (\"*\" for all groups)",
}, []string{"node_group"})

var ngConfigurationGVK = schema.GroupVersionKind{
	Group: "deckhouse.io", Version: "v1alpha1", Kind: "NodeGroupConfiguration",
}

func newNodeGroupConfiguration() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(ngConfigurationGVK)
	return u
}

func init() {
	ctrlmetrics.Registry.MustRegister(ngConfigurationsTotal)
	register.RegisterController("node-group-configuration-metrics", newNodeGroupConfiguration(), &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(ngConfigurationGVK.GroupVersion().WithKind("NodeGroupConfigurationList"))
	if err := r.Client.List(ctx, list); err != nil {
		return ctrl.Result{}, err
	}

	countByNodeGroup := make(map[string]uint)
	for i := range list.Items {
		for _, ng := range targetNodeGroups(&list.Items[i]) {
			countByNodeGroup[ng]++
		}
	}

	// Reset before setting so series for deleted or retargeted configurations
	// disappear, mirroring the hook's MetricsCollector.Expire of the group.
	ngConfigurationsTotal.Reset()
	for ng, count := range countByNodeGroup {
		ngConfigurationsTotal.With(prometheus.Labels{"node_group": ng}).Set(float64(count))
	}

	log.FromContext(ctx).V(1).Info("updated NodeGroupConfiguration metrics", "nodeGroups", len(countByNodeGroup))
	return ctrl.Result{}, nil
}

// targetNodeGroups returns spec.nodeGroups, defaulting to ["*"] only when the
// field is absent — the same rule the hook's filter applied (an explicitly empty
// list yields no series for that configuration).
func targetNodeGroups(ngc *unstructured.Unstructured) []string {
	ngs, ok, err := unstructured.NestedStringSlice(ngc.Object, "spec", "nodeGroups")
	if err != nil || !ok {
		return []string{"*"}
	}
	return ngs
}
