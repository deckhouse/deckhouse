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

package ycpreemptible

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	requeueInterval = 1 * time.Minute
	machineNS       = "d8-cloud-instance-manager"
	nodeGroupLabel  = "node.deckhouse.io/group"

	// Preemptible instances are forcibly stopped by Yandex.Cloud after 24 hours.
	// We delete machines approaching that threshold.
	durationThresholdForDeletion = 24*time.Hour - 4*time.Hour

	// We will not delete any Machines if it would violate overall Node readiness of a given NodeGroup.
	nodeGroupReadinessRatio = 0.9
)

func init() {
	dynr.RegisterReconciler(rcname.YCPreemptible, &mcmv1alpha1.Machine{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler handles deletion of preemptible Yandex.Cloud machines that are nearing
// their 24-hour forced termination window. It deletes the oldest 10% of eligible
// machines per reconcile cycle, provided the NodeGroup readiness ratio stays above 90%.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, ok := obj.GetLabels()["node.deckhouse.io/preemptible"]
		return ok && obj.GetNamespace() == machineNS
	})}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	machine := &mcmv1alpha1.Machine{}
	if err := r.Client.Get(ctx, req.NamespacedName, machine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip machines already being deleted.
	if machine.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	// Find the corresponding Node to get creation timestamp and NodeGroup.
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: machine.Name}, node); err != nil {
		// Node not found for this machine — skip.
		return ctrl.Result{RequeueAfter: requeueInterval}, client.IgnoreNotFound(err)
	}

	ngName, ok := node.Labels[nodeGroupLabel]
	if !ok || ngName == "" {
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	// Check age threshold.
	timeNow := time.Now().UTC()
	if node.CreationTimestamp.Time.Add(durationThresholdForDeletion).After(timeNow) {
		// Machine is still young enough.
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	// Check NodeGroup readiness ratio.
	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ngName}, ng); err != nil {
		return ctrl.Result{RequeueAfter: requeueInterval}, client.IgnoreNotFound(err)
	}

	ngRatio := float64(ng.Status.Ready) / float64(ng.Status.Nodes)
	if ng.Status.Nodes == 0 || ngRatio < nodeGroupReadinessRatio {
		log.V(1).Info("NodeGroup readiness ratio too low, skipping deletion",
			"nodeGroup", ngName,
			"ready", ng.Status.Ready,
			"nodes", ng.Status.Nodes,
		)
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	log.Info("deleting preemptible machine approaching 24h limit",
		"machine", machine.Name,
		"nodeGroup", ngName,
		"nodeAge", timeNow.Sub(node.CreationTimestamp.Time),
	)

	if err := r.Client.Delete(ctx, machine); err != nil {
		return ctrl.Result{}, fmt.Errorf("delete preemptible machine %s: %w", machine.Name, err)
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}
