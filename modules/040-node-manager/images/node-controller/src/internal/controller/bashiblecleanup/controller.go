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

package bashiblecleanup

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/deckhouse/node-controller/internal/register"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

const (
	bashibleFirstRunFinishedLabel = "node.deckhouse.io/bashible-first-run-finished"
	bashibleUninitializedTaintKey = "node.deckhouse.io/bashible-uninitialized"
)

func init() {
	register.RegisterController(register.BashibleCleanup, &corev1.Node{}, &Reconciler{})
}

type Reconciler struct {
	dynctrl.Base
}

func (r *Reconciler) SetupWatches(_ dynctrl.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if _, hasLabel := node.Labels[bashibleFirstRunFinishedLabel]; !hasLabel {
		return ctrl.Result{}, nil
	}

	base := node.DeepCopy()

	delete(node.Labels, bashibleFirstRunFinishedLabel)

	hasTaint := false
	for _, t := range node.Spec.Taints {
		if t.Key == bashibleUninitializedTaintKey {
			hasTaint = true
			break
		}
	}

	if hasTaint {
		taints := make([]corev1.Taint, 0, len(node.Spec.Taints))
		for _, t := range node.Spec.Taints {
			if t.Key != bashibleUninitializedTaintKey {
				taints = append(taints, t)
			}
		}
		if len(taints) == 0 {
			node.Spec.Taints = nil
		} else {
			node.Spec.Taints = taints
		}
	}

	if err := r.Client.Patch(ctx, node, client.MergeFrom(base)); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("cleaned up bashible artifacts", "node", node.Name, "removedTaint", hasTaint)
	return ctrl.Result{}, nil
}

var _ dynctrl.Reconciler = (*Reconciler)(nil)
