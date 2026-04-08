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

package cloudconditions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	configMapName      = "d8-cloud-provider-conditions"
	configMapNamespace = "kube-system"
)

// CloudCondition represents a single cloud provider condition entry.
type CloudCondition struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
}

var (
	unmetCloudConditionsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "d8_node_manager",
			Name:      "unmet_cloud_conditions",
			Help:      "Indicates whether there are unmet cloud provider conditions (1 = unmet conditions exist, 0 = all conditions met).",
		},
		[]string{},
	)

	cloudConditionStatusGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "d8_node_manager",
			Name:      "cloud_condition_status",
			Help:      "Status of individual cloud provider conditions (1 = ok, 0 = not ok).",
		},
		[]string{"name", "message"},
	)
)

func init() {
	dynr.RegisterReconciler(rcname.MetricsCloudConditions, &corev1.ConfigMap{}, &Reconciler{})

	metrics.Registry.MustRegister(unmetCloudConditionsGauge, cloudConditionStatusGauge)
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler reads conditions from the d8-cloud-provider-conditions ConfigMap
// and exports them as Prometheus gauge metrics.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == configMapName && obj.GetNamespace() == configMapNamespace
	})}
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      configMapName,
		Namespace: configMapNamespace,
	}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap does not exist — no unmet conditions.
			unmetCloudConditionsGauge.Reset()
			unmetCloudConditionsGauge.With(prometheus.Labels{}).Set(0)
			cloudConditionStatusGauge.Reset()
			log.V(1).Info("cloud provider conditions ConfigMap not found, resetting metrics")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ConfigMap %s/%s: %w", configMapNamespace, configMapName, err)
	}

	conditionsData, ok := cm.Data["conditions"]
	if !ok || conditionsData == "" {
		unmetCloudConditionsGauge.Reset()
		unmetCloudConditionsGauge.With(prometheus.Labels{}).Set(0)
		cloudConditionStatusGauge.Reset()
		log.V(1).Info("no conditions data in ConfigMap")
		return ctrl.Result{}, nil
	}

	var conditions []CloudCondition
	if err := json.Unmarshal([]byte(conditionsData), &conditions); err != nil {
		return ctrl.Result{}, fmt.Errorf("unmarshal conditions from ConfigMap %s/%s: %w", configMapNamespace, configMapName, err)
	}

	// Reset per-condition gauge to remove stale series.
	cloudConditionStatusGauge.Reset()

	var hasUnmet bool
	for i := range conditions {
		status := float64(1)
		if !conditions[i].Ok {
			hasUnmet = true
			status = 0
		}
		cloudConditionStatusGauge.With(prometheus.Labels{
			"name":    conditions[i].Name,
			"message": conditions[i].Message,
		}).Set(status)
	}

	unmetCloudConditionsGauge.Reset()
	if hasUnmet {
		unmetCloudConditionsGauge.With(prometheus.Labels{}).Set(1)
	} else {
		unmetCloudConditionsGauge.With(prometheus.Labels{}).Set(0)
	}

	log.V(1).Info("updated cloud conditions metrics", "conditionsCount", len(conditions), "hasUnmet", hasUnmet)

	return ctrl.Result{}, nil
}
