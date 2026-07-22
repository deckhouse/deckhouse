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

// Package spottermination deletes the Instance of a drained spot/preemptible node
// whose cloud VM is being reclaimed.
//
// A reclaimed spot instance is marked by the provider with the
// node.deckhouse.io/termination-in-progress label. Once the node has also been
// drained (update.node.deckhouse.io/drained annotation), the matching Instance CR
// must be deleted so machine-controller-manager tears the machine and VM down.
//
// This replaces the shell-operator hook hooks/handle_spot_instance_deletion.go
// with the same trigger (label + drained annotation) and effect (delete Instance
// named after the node). Unlike the hook, which hardcoded the deckhouse.io/v1alpha1
// Instance apiVersion and silently broke once Instance graduated to v1alpha2
// (v1alpha1 served:false since PR #18795, 2026-05-07), this controller deletes via
// the typed v1alpha2 client, so it targets the served version through the scheme.
package spottermination

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("node-spot-termination", &corev1.Node{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(_ register.Watcher) {}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	node := &corev1.Node{}
	if err := r.Client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if node.Labels[nodecommon.TerminationInProgressLabel] != "true" {
		return ctrl.Result{}, nil
	}
	if _, drained := node.Annotations[nodecommon.DrainedAnnotation]; !drained {
		return ctrl.Result{}, nil
	}

	instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: node.Name}}
	if err := r.Client.Delete(ctx, instance, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to delete Instance for spot-terminated node", "node", node.Name)
		return ctrl.Result{}, err
	}

	logger.Info("deleted Instance for drained spot-terminated node", "node", node.Name)
	return ctrl.Result{}, nil
}
