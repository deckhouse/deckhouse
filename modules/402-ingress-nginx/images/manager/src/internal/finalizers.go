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

package internal

import (
	"context"
	"fmt"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "manager/src/api/v1"
	"manager/src/internal/helper"
)

const (
	controllerNamespace      = "d8-ingress-nginx"
	controllerFinalizer      = "finalizer.ingress-nginx.deckhouse.io"
	admissionWebhookName     = "d8-ingress-nginx-admission"
	webhookNamePattern       = "%s.validate.d8-ingress-nginx"
	deckhouseWebhookPattern  = "%s.validate.d8-ingress-nginx-deckhouse"
	finalizerRequeueInterval = 5 * time.Second
)

func (r *IngressNginxController) reconcileFinalizer(
	ctx context.Context,
	ic *v1.IngressNginxController,
) (ctrl.Result, bool, error) {
	r.ensureServices()

	hasChildren, err := r.hasChildResources(ctx, ic)
	if err != nil {
		return ctrl.Result{}, true, err
	}

	hasFinalizer := controllerutil.ContainsFinalizer(ic, controllerFinalizer)

	if !ic.GetDeletionTimestamp().IsZero() {
		if !hasFinalizer {
			return ctrl.Result{}, true, nil
		}

		if hasChildren {
			return ctrl.Result{RequeueAfter: finalizerRequeueInterval}, true, nil
		}

		if err := r.patchFinalizer(ctx, ic, false); err != nil {
			return ctrl.Result{}, true, err
		}

		return ctrl.Result{}, true, nil
	}

	if hasChildren == hasFinalizer {
		return ctrl.Result{}, false, nil
	}

	if err := r.patchFinalizer(ctx, ic, hasChildren); err != nil {
		return ctrl.Result{}, true, err
	}

	return ctrl.Result{Requeue: true}, true, nil
}

func (r *IngressNginxController) hasChildResources(
	ctx context.Context,
	ic *v1.IngressNginxController,
) (bool, error) {
	serviceNames := []string{
		ic.Name + "-load-balancer",
		ic.Name + "-admission",
		fmt.Sprintf("controller-%s-failover", ic.Name),
	}

	for _, serviceName := range serviceNames {
		found, err := r.hasService(ctx, serviceName)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}

	workloadTracks := []map[string]string{
		helper.WorkloadLabels("controller", ic.Name),
		helper.WorkloadLabels("controller", ic.Name+"-failover"),
		helper.WorkloadLabels("proxy-failover", ic.Name),
	}

	for _, labels := range workloadTracks {
		found, err := r.hasWorkloadTrack(ctx, labels)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}

	return r.hasAdmissionWebhook(ctx, ic.Name)
}

func (r *IngressNginxController) hasService(
	ctx context.Context,
	name string,
) (bool, error) {
	var service corev1.Service
	err := r.Get(ctx, types.NamespacedName{Namespace: controllerNamespace, Name: name}, &service)
	if apierrors.IsNotFound(err) {
		return false, nil
	}

	return err == nil, err
}

func (r *IngressNginxController) hasWorkloadTrack(
	ctx context.Context,
	labels map[string]string,
) (bool, error) {
	workloads, err := r.Workloads.ListByLabels(ctx, controllerNamespace, labels)
	if err != nil {
		return false, err
	}

	return len(workloads) > 0, nil
}

func (r *IngressNginxController) hasAdmissionWebhook(
	ctx context.Context,
	controllerName string,
) (bool, error) {
	var webhook admissionregistrationv1.ValidatingWebhookConfiguration
	err := r.Get(ctx, client.ObjectKey{Name: admissionWebhookName}, &webhook)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	for _, item := range webhook.Webhooks {
		if item.Name == fmt.Sprintf(webhookNamePattern, controllerName) ||
			item.Name == fmt.Sprintf(deckhouseWebhookPattern, controllerName) {
			return true, nil
		}
	}

	return false, nil
}

func (r *IngressNginxController) patchFinalizer(
	ctx context.Context,
	ic *v1.IngressNginxController,
	add bool,
) error {
	base := ic.DeepCopy()

	if add {
		controllerutil.AddFinalizer(ic, controllerFinalizer)
	} else {
		controllerutil.RemoveFinalizer(ic, controllerFinalizer)
	}

	return r.Patch(ctx, ic, client.MergeFrom(base))
}
