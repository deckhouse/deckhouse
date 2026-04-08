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

package bashiblelock

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	bashibleNamespace = "d8-cloud-instance-manager"
	bashibleName      = "bashible-apiserver"
	secretName        = "bashible-apiserver-context"
	lockedAnnotation  = "node.deckhouse.io/bashible-locked"
)

func init() {
	dynr.RegisterReconciler(rcname.BashibleLock, &appsv1.Deployment{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler watches the bashible-apiserver Deployment and manages the
// "node.deckhouse.io/bashible-locked" annotation on the bashible-apiserver-context Secret.
//
// When the Deployment image digest/tag does not match the expected digest (from the
// "node.deckhouse.io/bashible-apiserver-image-digest" annotation on the Deployment),
// the Secret is locked. When the Deployment is fully rolled out with the correct image,
// the lock is removed.
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == bashibleName && obj.GetNamespace() == bashibleNamespace
	})}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	dep := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, dep); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Extract the current image digest or tag from the running container.
	currentDigestOrTag := extractImageDigestOrTag(dep)

	// Get the expected digest from the Deployment annotation.
	expectedDigest := dep.Annotations["node.deckhouse.io/bashible-apiserver-image-digest"]

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: bashibleNamespace, Name: secretName}

	if err := r.Client.Get(ctx, secretKey, secret); err != nil {
		// Secret may not exist yet — that is fine.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if currentDigestOrTag != expectedDigest {
		// Image mismatch — lock the Secret.
		log.Info("bashible-apiserver image mismatch, locking",
			"current", currentDigestOrTag,
			"expected", expectedDigest,
		)
		return ctrl.Result{}, r.setLockAnnotation(ctx, secret, true)
	}

	// Image matches — check if rollout is complete.
	if dep.Status.Replicas != dep.Status.UpdatedReplicas {
		log.V(1).Info("bashible-apiserver rollout not complete yet",
			"desired", dep.Status.Replicas,
			"updated", dep.Status.UpdatedReplicas,
		)
		return ctrl.Result{}, nil
	}

	// Fully rolled out with correct image — unlock.
	log.Info("bashible-apiserver fully rolled out, unlocking")
	return ctrl.Result{}, r.setLockAnnotation(ctx, secret, false)
}

// setLockAnnotation sets or removes the bashible-locked annotation on the Secret.
func (r *Reconciler) setLockAnnotation(ctx context.Context, secret *corev1.Secret, lock bool) error {
	patch := client.MergeFrom(secret.DeepCopy())

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}

	if lock {
		secret.Annotations[lockedAnnotation] = "true"
	} else {
		delete(secret.Annotations, lockedAnnotation)
	}

	if err := r.Client.Patch(ctx, secret, patch); err != nil {
		return fmt.Errorf("patch Secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

// extractImageDigestOrTag extracts the digest or tag from the bashible-apiserver container image.
func extractImageDigestOrTag(dep *appsv1.Deployment) string {
	for _, c := range dep.Spec.Template.Spec.Containers {
		if c.Name != bashibleName {
			continue
		}

		if idx := strings.LastIndex(c.Image, "@sha256:"); idx != -1 {
			return c.Image[idx+1:] // returns "sha256:..."
		}
		if idx := strings.LastIndex(c.Image, ":"); idx != -1 {
			return c.Image[idx+1:]
		}
	}
	return ""
}
