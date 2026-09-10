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
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	metricsScraperNamespace = "kube-system"
	metricsScraperSAName    = "d8-metrics-scraper"

	metricsTokenTTL        = 24 * time.Hour
	metricsTokenRegenBelow = 6 * time.Hour
)

func metricsTokenSecretName(vcpName string) string {
	return constants.VirtualResourceName(constants.VirtualMetricsTokenSecretName, vcpName)
}

// reconcileMetricsToken keeps a scrape token for the tenant control plane in the parent namespace.
//
// The Secret is deliberately Opaque rather than kubernetes.io/service-account-token:
// prometheus-operator runs with --secret-field-selector excluding that type, so a classic SA Secret
// would be invisible to it and the scrape config would silently never be generated.
func (r *reconciler) reconcileMetricsToken(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) error {
	name := metricsTokenSecretName(vcp.Name)

	current, err := r.getSecret(ctx, vcp.Namespace, name)
	notFound := apierrors.IsNotFound(err)
	if err != nil && !notFound {
		return fmt.Errorf("get metrics token Secret: %w", err)
	}
	if !notFound && !tokenNeedsRenewal(current, metricsTokenRegenBelow) {
		return nil
	}

	ts, _, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return err
	}

	expSecs := int64(metricsTokenTTL / time.Second)
	tr, err := ts.CoreV1().ServiceAccounts(metricsScraperNamespace).CreateToken(
		ctx,
		metricsScraperSAName,
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: &expSecs},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("TokenRequest metrics scraper: %w", err)
	}

	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: vcp.Namespace,
			Labels: map[string]string{
				constants.HeritageLabelKey:                 constants.HeritageLabelValue,
				constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
			},
			Annotations: map[string]string{
				tokenExpiresAtKey: tr.Status.ExpirationTimestamp.UTC().Format(time.RFC3339),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"token": []byte(tr.Status.Token)},
	}
	if err := setVCPControllerReference(vcp, target, r.scheme); err != nil {
		return err
	}

	if notFound {
		return r.createSecret(ctx, target)
	}

	base := current.DeepCopy()
	current.Data = target.Data
	if current.Annotations == nil {
		current.Annotations = map[string]string{}
	}
	current.Annotations[tokenExpiresAtKey] = target.Annotations[tokenExpiresAtKey]
	syncOwnerReferences(current, target)

	return r.patchSecret(ctx, base, current)
}
