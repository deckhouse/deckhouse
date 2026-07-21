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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	konnectivityAgentCPSecretName       = "konnectivity-agent-cp"
	konnectivityAgentNamespace          = "kube-system"
	konnectivityAgentSAName             = "konnectivity-agent"
	konnectivityAudience                = "system:konnectivity-server"
	konnectivityAgentTokenTTL           = 24 * time.Hour
	konnectivityAgentTokenRegenBelow    = 6 * time.Hour
	konnectivityAgentCPPlaceholderToken = "placeholder"
	konnectivityAgentCPTokenExpiresAt   = "control-plane.deckhouse.io/token-expires-at"
)

// reconcileKonnectivityCPAgentSecret upgrades the parent secret with a real nested TokenRequest
// token once the nested API and konnectivity-agent SA are available.
func (r *reconciler) reconcileKonnectivityCPAgentSecret(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
) (reconcile.Result, error) {
	ns := vcpNamespace(vcp)

	if err := r.ensureKonnectivityCPAgentSecretBootstrap(ctx, vcp, pkiSecret.Data["ca.crt"]); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.ensureKonnectivitySA(ctx, vcp); err != nil {
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	token, exp, err := r.ensureKonnectivityCPAgentToken(ctx, vcp)
	if err != nil {
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	target := r.konnectivityCPAgentSecret(vcp, pkiSecret.Data["ca.crt"], token, exp)
	current, err := r.getSecret(ctx, ns, konnectivityAgentCPSecretName)
	if err != nil {
		return reconcile.Result{}, err
	}

	base := current.DeepCopy()
	current.Data = target.Data
	current.Labels = target.Labels
	current.Annotations = target.Annotations
	return reconcile.Result{}, r.patchSecret(ctx, base, current)
}

// ensureKonnectivityCPAgentSecretBootstrap creates the parent secret with ca.crt before the
// nested apiserver and konnectivity-agent SA exist, so the konnectivity-agent-cp volume can mount.
func (r *reconciler) ensureKonnectivityCPAgentSecretBootstrap(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	caPEM []byte,
) error {
	ns := vcpNamespace(vcp)

	current, err := r.getSecret(ctx, ns, konnectivityAgentCPSecretName)
	if apierrors.IsNotFound(err) {
		target := r.konnectivityCPAgentSecret(
			vcp,
			caPEM,
			konnectivityAgentCPPlaceholderToken,
			"",
		)
		if err := ctrl.SetControllerReference(vcp, target, r.scheme); err != nil {
			return err
		}
		return r.createSecret(ctx, target)
	}
	if err != nil {
		return err
	}

	if string(current.Data["ca.crt"]) == string(caPEM) {
		return nil
	}
	base := current.DeepCopy()
	if current.Data == nil {
		current.Data = map[string][]byte{}
	}
	current.Data["ca.crt"] = caPEM
	return r.patchSecret(ctx, base, current)
}

func (r *reconciler) ensureKonnectivitySA(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) error {
	ts, _, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return err
	}

	_, err = ts.CoreV1().ServiceAccounts(konnectivityAgentNamespace).Get(
		ctx, konnectivityAgentSAName, metav1.GetOptions{},
	)
	if apierrors.IsNotFound(err) {
		_, err = ts.CoreV1().ServiceAccounts(konnectivityAgentNamespace).Create(
			ctx,
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      konnectivityAgentSAName,
					Namespace: konnectivityAgentNamespace,
					Labels: map[string]string{
						constants.HeritageLabelKey: constants.HeritageLabelValue,
					},
				},
			},
			metav1.CreateOptions{},
		)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create nested konnectivity-agent SA: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get nested konnectivity-agent SA: %w", err)
	}
	return nil
}

func (r *reconciler) ensureKonnectivityCPAgentToken(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) (string, string, error) {
	ns := vcpNamespace(vcp)

	if current, err := r.getSecret(ctx, ns, konnectivityAgentCPSecretName); err == nil {
		token := string(current.Data["token"])
		if token != "" && token != konnectivityAgentCPPlaceholderToken {
			if expRaw := current.Annotations[konnectivityAgentCPTokenExpiresAt]; expRaw != "" {
				if exp, err := time.Parse(time.RFC3339, expRaw); err == nil {
					if time.Until(exp) > konnectivityAgentTokenRegenBelow {
						return token, expRaw, nil
					}
				}
			}
		}
	}

	ts, _, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return "", "", err
	}

	expSecs := int64(konnectivityAgentTokenTTL / time.Second)
	tr, err := ts.CoreV1().ServiceAccounts(konnectivityAgentNamespace).CreateToken(
		ctx,
		konnectivityAgentSAName,
		&authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				Audiences:         []string{konnectivityAudience},
				ExpirationSeconds: &expSecs,
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", "", fmt.Errorf("TokenRequest nested konnectivity-agent: %w", err)
	}

	return tr.Status.Token, tr.Status.ExpirationTimestamp.UTC().Format(time.RFC3339), nil
}

func (r *reconciler) konnectivityCPAgentSecret(
	vcp *controlplanev1alpha1.VirtualControlPlane,
	caPEM []byte,
	token, exp string,
) *corev1.Secret {
	ns := vcpNamespace(vcp)
	annotations := map[string]string{}
	if exp != "" {
		annotations[konnectivityAgentCPTokenExpiresAt] = exp
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        konnectivityAgentCPSecretName,
			Namespace:   ns,
			Annotations: annotations,
			Labels: map[string]string{
				constants.HeritageLabelKey:                 constants.HeritageLabelValue,
				constants.VirtualControlPlaneScopeLabelKey: vcp.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token":  []byte(token),
			"ca.crt": caPEM,
		},
	}
}
