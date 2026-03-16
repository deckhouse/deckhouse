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

package chaosmonkey

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const defaultChaosPeriod = 6 * time.Hour

func init() {
	dynr.RegisterReconciler(rcname.ChaosMonkey, &deckhousev1.NodeGroup{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler implements the chaos monkey logic: for each NodeGroup with
// Chaos.Mode == "DrainAndDelete", it periodically picks a random Machine
// and deletes it.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		ng, ok := obj.(*deckhousev1.NodeGroup)
		if !ok {
			return false
		}
		return ng.Spec.Chaos != nil && ng.Spec.Chaos.Mode == deckhousev1.ChaosModeDrainAndDelete
	})}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Safety check: the predicate filters for DrainAndDelete mode, but the object
	// may have changed between the event and this reconcile.
	if ng.Spec.Chaos == nil || ng.Spec.Chaos.Mode != deckhousev1.ChaosModeDrainAndDelete {
		return ctrl.Result{}, nil
	}

	chaosPeriod := defaultChaosPeriod
	if ng.Spec.Chaos.Period != "" {
		parsed, err := time.ParseDuration(ng.Spec.Chaos.Period)
		if err != nil {
			log.Error(err, "invalid chaos period, using default", "period", ng.Spec.Chaos.Period)
		} else {
			chaosPeriod = parsed
		}
	}

	// Check readiness: the NodeGroup must have enough ready nodes.
	if !isReadyForChaos(ng) {
		log.V(1).Info("NodeGroup is not ready for chaos", "nodeGroup", ng.Name)
		return ctrl.Result{RequeueAfter: chaosPeriod}, nil
	}

	selector := &victimSelector{client: r.Client}

	// If there is already an existing victim, skip this round.
	hasVictim, err := selector.hasExistingVictim(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("check existing victim: %w", err)
	}
	if hasVictim {
		log.V(1).Info("existing chaos monkey victim found, skipping")
		return ctrl.Result{RequeueAfter: chaosPeriod}, nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	victim, err := selector.selectVictim(ctx, rng, ng.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("select victim: %w", err)
	}
	if victim == nil {
		log.V(1).Info("no suitable victim found", "nodeGroup", ng.Name)
		return ctrl.Result{RequeueAfter: chaosPeriod}, nil
	}

	// Mark victim with chaos-monkey-victim annotation and delete it.
	log.Info("deleting chaos monkey victim", "machine", victim.Name, "nodeGroup", ng.Name)

	patch := client.MergeFrom(victim.DeepCopy())
	if victim.Annotations == nil {
		victim.Annotations = make(map[string]string)
	}
	victim.Annotations["node.deckhouse.io/chaos-monkey-victim"] = ""
	if err := r.Client.Patch(ctx, victim, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("annotate victim machine %s: %w", victim.Name, err)
	}

	if err := r.Client.Delete(ctx, victim); err != nil {
		return ctrl.Result{}, fmt.Errorf("delete victim machine %s: %w", victim.Name, err)
	}

	return ctrl.Result{RequeueAfter: chaosPeriod}, nil
}

// isReadyForChaos checks whether the NodeGroup has enough ready nodes for chaos operations.
func isReadyForChaos(ng *deckhousev1.NodeGroup) bool {
	if ng.Spec.NodeType == deckhousev1.NodeTypeCloudEphemeral {
		return ng.Status.Desired > 1 && ng.Status.Desired == ng.Status.Ready
	}
	return ng.Status.Nodes > 1 && ng.Status.Nodes == ng.Status.Ready
}
