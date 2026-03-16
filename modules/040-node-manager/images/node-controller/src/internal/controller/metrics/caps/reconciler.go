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

package caps

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

var (
	capsReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_replicas",
		Help: "Total number of non-terminated machines targeted by this MachineDeployment.",
	}, []string{"machine_deployment_name"})

	capsDesired = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_desired",
		Help: "Desired number of machines for this MachineDeployment.",
	}, []string{"machine_deployment_name"})

	capsReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_ready",
		Help: "Total number of ready machines targeted by this MachineDeployment.",
	}, []string{"machine_deployment_name"})

	capsUnavailable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_unavailable",
		Help: "Total number of unavailable machines targeted by this MachineDeployment.",
	}, []string{"machine_deployment_name"})

	capsPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_phase",
		Help: "Current phase of the MachineDeployment (1=Running,2=ScalingUp,3=ScalingDown,4=Failed,5=Unknown).",
	}, []string{"machine_deployment_name"})
)

func init() {
	prometheus.MustRegister(capsReplicas, capsDesired, capsReady, capsUnavailable, capsPhase)

	dynr.RegisterReconciler(rcname.MetricsCAPS, &mcmv1alpha1.MachineDeployment{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler exports Prometheus metrics for CAPS MachineDeployments.
// It mirrors the logic from the machine_deployments_caps_metrics.go addon-operator hook.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()["app"] == "caps-controller-manager"
	})}
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	md := &mcmv1alpha1.MachineDeployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, md); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Object deleted - remove its metrics.
			labels := prometheus.Labels{"machine_deployment_name": req.Name}
			capsReplicas.Delete(labels)
			capsDesired.Delete(labels)
			capsReady.Delete(labels)
			capsUnavailable.Delete(labels)
			capsPhase.Delete(labels)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get MachineDeployment %s: %w", req.NamespacedName, err)
	}

	// Re-check label in case predicate was bypassed.
	if md.Labels["app"] != "caps-controller-manager" {
		return ctrl.Result{}, nil
	}

	labels := prometheus.Labels{"machine_deployment_name": md.Name}

	capsReplicas.With(labels).Set(float64(md.Status.Replicas))
	capsDesired.With(labels).Set(float64(md.Spec.Replicas))
	capsReady.With(labels).Set(float64(md.Status.ReadyReplicas))
	capsUnavailable.With(labels).Set(float64(md.Status.UnavailableReplicas))
	capsPhase.With(labels).Set(phaseToFloat(md))

	log.V(1).Info("updated CAPS MD metrics", "machineDeployment", md.Name)
	return ctrl.Result{}, nil
}

// phaseToFloat derives a numeric phase from MachineDeployment conditions.
// Mapping: 1=Running, 2=ScalingUp, 3=ScalingDown, 4=Failed, 5=Unknown.
func phaseToFloat(md *mcmv1alpha1.MachineDeployment) float64 {
	for _, c := range md.Status.Conditions {
		if c.Type == mcmv1alpha1.MachineDeploymentReplicaFailure && c.Status == mcmv1alpha1.ConditionTrue {
			return 4 // Failed
		}
	}

	desired := md.Spec.Replicas
	current := md.Status.Replicas

	switch {
	case current < desired:
		return 2 // ScalingUp
	case current > desired:
		return 3 // ScalingDown
	}

	// Check if available.
	for _, c := range md.Status.Conditions {
		if c.Type == mcmv1alpha1.MachineDeploymentAvailable && c.Status == mcmv1alpha1.ConditionTrue {
			return 1 // Running
		}
	}

	return 5 // Unknown
}
