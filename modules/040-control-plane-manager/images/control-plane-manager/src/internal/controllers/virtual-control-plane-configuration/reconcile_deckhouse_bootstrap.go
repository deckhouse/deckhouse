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

package virtualcontrolplaneconfiguration

import (
	"context"
	"fmt"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	deckhouseBootstrapManifestKey     = "deckhouse-bootstrap.yaml.tpl"
	deckhouseModuleConfigsManifestKey = "deckhouse-moduleconfigs.yaml.tpl"
)

func (r *reconciler) reconcileDeckhouseBootstrap(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	configSecret *corev1.Secret,
) (reconcile.Result, error) {
	_, tc, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	if err := applyTenantManifests(ctx, tc, configSecret, deckhouseBootstrapManifestKey); err != nil {
		return reconcile.Result{}, fmt.Errorf("apply tenant %s: %w", deckhouseBootstrapManifestKey, err)
	}

	if err := applyTenantManifests(ctx, tc, configSecret, deckhouseModuleConfigsManifestKey); err != nil {
		if isMissingModuleConfigCRD(err) {
			return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
		}

		return reconcile.Result{}, fmt.Errorf("apply tenant %s: %w", deckhouseModuleConfigsManifestKey, err)
	}

	lock := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deckhouse-bootstrap-lock",
			Namespace: "d8-system",
		},
	}
	if err := tc.Delete(ctx, lock); err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("delete deckhouse bootstrap lock: %w", err)
	}

	return reconcile.Result{}, nil
}

func isMissingModuleConfigCRD(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no matches for kind \"ModuleConfig\"") ||
		strings.Contains(msg, "the server could not find the requested resource")
}
