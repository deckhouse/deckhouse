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

// Package hostipchange recreates the bashible-apiserver Pod when the host IP of
// its node changes.
//
// bashible-apiserver serves node bootstrap data and its serving certificate is
// pinned to the host IP it was scheduled on. On first sight the controller
// records the current host IP into the node.deckhouse.io/initial-host-ip
// annotation. If the node later comes back with a different host IP (for
// example a reboot with a new DHCP lease), the recorded value diverges from the
// live status.hostIP and the Pod is deleted so the Deployment recreates it with
// a certificate valid for the new address.
//
// Taken over from the node-manager change_host_ip hook (the shared
// go_lib/hooks/change_host_address library instantiated for
// app=bashible-apiserver in d8-cloud-instance-manager). Other modules still use
// that library for their own components; only the bashible-apiserver instance
// moves here.
package hostipchange

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	appLabelKey             = "app"
	appLabelValue           = "bashible-apiserver"
	initialHostIPAnnotation = "node.deckhouse.io/initial-host-ip"
)

func init() {
	register.RegisterController("bashible-apiserver-host-ip", &corev1.Pod{}, &Reconciler{})
}

type Reconciler struct {
	register.Base
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == nodecommon.MachineNamespace &&
			obj.GetLabels()[appLabelKey] == appLabelValue
	}))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	pod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	hostIP := pod.Status.HostIP
	if hostIP == "" {
		// Pod is not scheduled yet; nothing to record.
		return ctrl.Result{}, nil
	}

	initialHostIP := pod.Annotations[initialHostIPAnnotation]

	if initialHostIP == "" {
		patch := client.MergeFrom(pod.DeepCopy())
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[initialHostIPAnnotation] = hostIP
		if err := r.Client.Patch(ctx, pod, patch); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("recorded initial host IP", "pod", pod.Name, "hostIP", hostIP)
		return ctrl.Result{}, nil
	}

	if initialHostIP != hostIP {
		if err := r.Client.Delete(ctx, pod); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		logger.Info("host IP changed, deleted pod for recreation", "pod", pod.Name, "initialHostIP", initialHostIP, "hostIP", hostIP)
	}

	return ctrl.Result{}, nil
}
