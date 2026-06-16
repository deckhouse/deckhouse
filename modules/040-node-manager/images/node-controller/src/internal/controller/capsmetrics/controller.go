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

// Package capsmetrics exposes Prometheus gauges for caps-controller CAPI
// MachineDeployments. It is the controller-runtime port of the
// machine_deployments_caps_metrics hook.
package capsmetrics

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"

	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const capsControllerAppLabel = "caps-controller"

type reconciler struct {
	register.Base
}

var _ register.Reconciler = (*reconciler)(nil)

func (r *reconciler) SetupWatches(_ register.Watcher) {}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	md := ngcommon.NewUnstructured(ngcommon.CAPIMachineDeploymentGVK)
	if err := r.Client.Get(ctx, req.NamespacedName, md); err != nil {
		if apierrors.IsNotFound(err) {
			deleteMachineDeploymentMetrics(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if md.GetLabels()["app"] != capsControllerAppLabel {
		deleteMachineDeploymentMetrics(req.Name)
		return ctrl.Result{}, nil
	}

	desired, _, _ := unstructured.NestedInt64(md.Object, "spec", "replicas")
	replicas, _, _ := unstructured.NestedInt64(md.Object, "status", "replicas")
	ready, _, _ := unstructured.NestedInt64(md.Object, "status", "readyReplicas")
	unavailable, _, _ := unstructured.NestedInt64(md.Object, "status", "unavailableReplicas")
	phase, _, _ := unstructured.NestedString(md.Object, "status", "phase")

	setMachineDeploymentMetrics(req.Name, float64(replicas), float64(desired), float64(ready), float64(unavailable), phaseValue(phase))
	return ctrl.Result{}, nil
}

func init() {
	register.RegisterController("caps-machine-deployment-metrics", ngcommon.NewUnstructured(ngcommon.CAPIMachineDeploymentGVK), &reconciler{})
}
