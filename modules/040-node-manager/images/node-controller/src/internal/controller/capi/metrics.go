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

package capi

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	"github.com/deckhouse/node-controller/internal/register"
)

var (
	mdReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_replicas",
		Help: "Current replica count from MachineDeployment status",
	}, []string{"machine_deployment_name"})

	mdDesired = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_desired",
		Help: "Desired replica count from MachineDeployment spec",
	}, []string{"machine_deployment_name"})

	mdReady = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_ready",
		Help: "Ready replica count from MachineDeployment status",
	}, []string{"machine_deployment_name"})

	mdUnavailable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_unavailable",
		Help: "Unavailable replica count from MachineDeployment status",
	}, []string{"machine_deployment_name"})

	mdPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "d8_caps_md_phase",
		Help: "MachineDeployment phase (1=Running, 2=ScalingUp, 3=ScalingDown, 4=Failed, 5=Unknown)",
	}, []string{"machine_deployment_name"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(mdReplicas, mdDesired, mdReady, mdUnavailable, mdPhase)
	register.RegisterController("capi-md-metrics", &capiv1beta2.MachineDeployment{}, &MetricsReconciler{})
}

// MetricsReconciler exports MachineDeployment metrics to Prometheus.
type MetricsReconciler struct {
	register.Base
}

func (r *MetricsReconciler) SetupWatches(_ register.Watcher) {}

func (r *MetricsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	md := &capiv1beta2.MachineDeployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, md); err != nil {
		if client.IgnoreNotFound(err) == nil {
			clearMetrics(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get MachineDeployment: %w", err)
	}

	name := md.Name
	l := prometheus.Labels{"machine_deployment_name": name}

	var specReplicas int32
	if md.Spec.Replicas != nil {
		specReplicas = *md.Spec.Replicas
	}

	var statusReplicas, readyReplicas, availableReplicas int32
	if md.Status.Replicas != nil {
		statusReplicas = *md.Status.Replicas
	}
	if md.Status.ReadyReplicas != nil {
		readyReplicas = *md.Status.ReadyReplicas
	}
	if md.Status.AvailableReplicas != nil {
		availableReplicas = *md.Status.AvailableReplicas
	}

	var unavailable int32
	if statusReplicas > availableReplicas {
		unavailable = statusReplicas - availableReplicas
	}

	mdReplicas.With(l).Set(float64(statusReplicas))
	mdDesired.With(l).Set(float64(specReplicas))
	mdReady.With(l).Set(float64(readyReplicas))
	mdUnavailable.With(l).Set(float64(unavailable))
	mdPhase.With(l).Set(phaseToFloat(md.Status.Phase))

	logger.V(1).Info("updated metrics", "machineDeployment", name)
	return ctrl.Result{}, nil
}

func phaseToFloat(phase string) float64 {
	switch phase {
	case "Running":
		return 1
	case "ScalingUp":
		return 2
	case "ScalingDown":
		return 3
	case "Failed":
		return 4
	default:
		return 5
	}
}

func clearMetrics(name string) {
	l := prometheus.Labels{"machine_deployment_name": name}
	mdReplicas.Delete(l)
	mdDesired.Delete(l)
	mdReady.Delete(l)
	mdUnavailable.Delete(l)
	mdPhase.Delete(l)
}
