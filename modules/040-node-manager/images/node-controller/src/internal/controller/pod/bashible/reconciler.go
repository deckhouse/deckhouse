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

package bashible

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const (
	initialHostIPAnnotation = "node.deckhouse.io/initial-host-ip"
	bashibleNamespace       = "d8-cloud-instance-manager"
	bashibleAppLabel        = "bashible-apiserver"
)

func init() {
	dynr.RegisterReconciler(rcname.BashiblePod, &corev1.Pod{}, &Reconciler{})
}

var _ dynr.Reconciler = (*Reconciler)(nil)

// Reconciler watches bashible-apiserver Pods in the d8-cloud-instance-manager
// namespace. It records the initial host IP via annotation and deletes the Pod
// if the host IP changes (indicating the Pod was rescheduled to a different node).
type Reconciler struct {
	dynr.Base
}

func (r *Reconciler) SetupWatches(_ dynr.Watcher) {}

func (r *Reconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != bashibleNamespace {
			return false
		}
		labels := obj.GetLabels()
		return labels != nil && labels["app"] == bashibleAppLabel
	})}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Get the Pod.
	pod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get pod %s: %w", req.NamespacedName, err)
	}

	// 2. Filter: only process bashible-apiserver pods in the correct namespace.
	if pod.Namespace != bashibleNamespace {
		return ctrl.Result{}, nil
	}
	if pod.Labels == nil || pod.Labels["app"] != bashibleAppLabel {
		return ctrl.Result{}, nil
	}

	hostIP := pod.Status.HostIP
	if hostIP == "" {
		// Pod not yet scheduled, nothing to do.
		return ctrl.Result{}, nil
	}

	annotations := pod.GetAnnotations()
	initialHost := ""
	if annotations != nil {
		initialHost = annotations[initialHostIPAnnotation]
	}

	// 3. If no initial host IP annotation, set it.
	if initialHost == "" {
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}
		pod.Annotations[initialHostIPAnnotation] = hostIP
		if err := r.Client.Patch(ctx, pod, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch pod %s with initial host IP annotation: %w", req.NamespacedName, err)
		}
		log.Info("set initial host IP annotation", "pod", req.NamespacedName, "hostIP", hostIP)
		return ctrl.Result{}, nil
	}

	// 4. If host IP changed, delete the Pod so it gets recreated on the correct node.
	if initialHost != hostIP {
		log.Info("host IP changed, deleting pod",
			"pod", req.NamespacedName,
			"initialHostIP", initialHost,
			"currentHostIP", hostIP,
		)
		if err := r.Client.Delete(ctx, pod); err != nil {
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("delete pod %s: %w", req.NamespacedName, err)
		}
	}

	return ctrl.Result{}, nil
}
