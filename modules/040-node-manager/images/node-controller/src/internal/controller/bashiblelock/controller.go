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

// Package bashiblelock locks the bashible-apiserver context while its Deployment
// rolls out to a new image. It replaces the OnBeforeHelm hook lock_bashible_apiserver.
//
// The bashible-apiserver reads the annotation node.deckhouse.io/bashible-locked on the
// Secret bashible-apiserver-context (see images/bashible-apiserver .../template/context.go
// secretEventHandler.lockApplied): when "true" it stops publishing updated context to
// nodes. This prevents old apiserver Pods from serving context that references step
// templates / image digests they do not yet have.
//
// The hook compared the live Deployment image to the helm values digest (the target
// image) OnBeforeHelm, so it could lock before helm applied. nc has no access to the
// values digest and only sees the Deployment after helm patched it, so it locks on the
// Deployment rollout status instead. The window difference is milliseconds and the race
// the lock guards against is still closed.
package bashiblelock

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/register"
)

const (
	bashibleNamespace      = "d8-cloud-instance-manager"
	bashibleDeploymentName = "bashible-apiserver"
	contextSecretName      = "bashible-apiserver-context"
	lockAnnotation         = "node.deckhouse.io/bashible-locked"
)

// bashibleLocked mirrors the hook metric d8_bashible_apiserver_locked. The alert
// D8BashibleApiserverLocked (monitoring/prometheus-rules/node-update.yaml) fires when it
// stays at 1 for 15m.
var bashibleLocked = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "d8_bashible_apiserver_locked",
	Help: "1 while the bashible-apiserver context is locked during a Deployment rollout, 0 otherwise",
})

func init() {
	ctrlmetrics.Registry.MustRegister(bashibleLocked)
	register.RegisterController("bashible-apiserver-lock", &appsv1.Deployment{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// Only react to the bashible-apiserver Deployment; ignore every other Deployment.
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == bashibleDeploymentName && obj.GetNamespace() == bashibleNamespace
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Name != bashibleDeploymentName || req.Namespace != bashibleNamespace {
		return ctrl.Result{}, nil
	}

	dep := &appsv1.Deployment{}
	err := r.Client.Get(ctx, req.NamespacedName, dep)
	if apierrors.IsNotFound(err) {
		// No Deployment to observe: nothing to lock or unlock on.
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting Deployment %s/%s: %w", bashibleNamespace, bashibleDeploymentName, err)
	}

	return ctrl.Result{}, r.setLocked(ctx, !rolloutComplete(dep))
}

// rolloutComplete reports whether every replica is on the current template and available.
// The ObservedGeneration guard rejects the stale-status window right after helm bumps the
// image, where the replica counts still reflect the previous generation and would falsely
// read "complete".
func rolloutComplete(dep *appsv1.Deployment) bool {
	if dep.Status.ObservedGeneration < dep.Generation {
		return false
	}
	return dep.Status.UpdatedReplicas == dep.Status.Replicas &&
		dep.Status.AvailableReplicas == dep.Status.Replicas
}

func (r *Reconciler) setLocked(ctx context.Context, locked bool) error {
	logger := log.FromContext(ctx)

	if locked {
		bashibleLocked.Set(1)
	} else {
		bashibleLocked.Set(0)
	}

	// Merge-patch the annotation on the context Secret. A nil value removes it (unlock).
	// The Secret may not exist yet on a fresh cluster; ignore that (the hook used
	// WithIgnoreMissingObject).
	var patch []byte
	if locked {
		patch = []byte(`{"metadata":{"annotations":{"` + lockAnnotation + `":"true"}}}`)
	} else {
		patch = []byte(`{"metadata":{"annotations":{"` + lockAnnotation + `":null}}}`)
	}

	secret := &corev1.Secret{}
	secret.SetName(contextSecretName)
	secret.SetNamespace(bashibleNamespace)
	if err := r.Client.Patch(ctx, secret, client.RawPatch(types.MergePatchType, patch)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("patching Secret %s/%s: %w", bashibleNamespace, contextSecretName, err)
	}

	logger.V(1).Info("bashible-apiserver context lock", "locked", locked)
	return nil
}
