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

package nodegroupconfigurations

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

var (
	ngConfigTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_node_group_configurations_total",
		Help: "Number of NodeGroupConfigurations per NodeGroup.",
	}, []string{"node_group"})
)

func init() {
	prometheus.MustRegister(ngConfigTotal)
	dynr.RegisterReconciler(rcname.MetricsNGConfig, &deckhousev1.NodeGroupConfiguration{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler exports Prometheus metrics counting NodeGroupConfigurations per NodeGroup.
// It mirrors the logic from the metrics_node_group_configurations.go addon-operator hook.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// List all NodeGroupConfigurations and rebuild the full metric set.
	ngcList := &deckhousev1.NodeGroupConfigurationList{}
	if err := r.Client.List(ctx, ngcList); err != nil {
		return ctrl.Result{}, fmt.Errorf("list NodeGroupConfigurations: %w", err)
	}

	// Reset metrics before recalculating to avoid stale data from deleted objects.
	ngConfigTotal.Reset()

	if len(ngcList.Items) == 0 {
		log.V(1).Info("no NodeGroupConfigurations found")
		return ctrl.Result{}, nil
	}

	countByNodeGroup := make(map[string]float64)
	for i := range ngcList.Items {
		ngc := &ngcList.Items[i]
		nodeGroups := ngc.Spec.NodeGroups
		if len(nodeGroups) == 0 {
			// NodeGroupConfiguration applies to all groups when nodeGroups is empty.
			nodeGroups = []string{"*"}
		}
		for _, ng := range nodeGroups {
			countByNodeGroup[ng]++
		}
	}

	for ng, count := range countByNodeGroup {
		ngConfigTotal.With(prometheus.Labels{"node_group": ng}).Set(count)
	}

	log.V(1).Info("updated NodeGroupConfiguration metrics", "groups", len(countByNodeGroup))
	return ctrl.Result{}, nil
}
