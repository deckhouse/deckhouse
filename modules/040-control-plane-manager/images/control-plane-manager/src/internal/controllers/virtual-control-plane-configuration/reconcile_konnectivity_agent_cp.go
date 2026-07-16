package virtualcontrolplaneconfiguration

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"fmt"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	konnectivityAgentCPSecretName    = "konnectivity-agent-cp"
	konnectivityAgentNamespace       = "kube-system"
	konnectivityAgentSAName          = "konnectivity-agent"
	konnectivityAudience             = "system:konnectivity-server"
	konnectivityAgentTokenTTL        = 24 * time.Hour
	konnectivityAgentTokenRegenBelow = 6 * time.Hour
)

func (r *reconciler) reconcileKonnectivityCPAgentSecret(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
) (reconcile.Result, error) {
	ns := constants.VirtualControlPlaneNamespacePrefix + vcp.Name
	caPEM := pkiSecret.Data["ca.crt"]
	if len(caPEM) == 0 {
		return reconcile.Result{}, fmt.Errorf("pki secret missing ca.crt")
	}

	ts, _, err := r.tenantClients(ctx, vcp)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("tenant clients: %w", err)
	}

	_, err = ts.CoreV1().ServiceAccounts(konnectivityAgentNamespace).Get(ctx, konnectivityAgentSAName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get nested konnectivity-agent SA: %w", err)
	}

	token, exp, err := r.ensureKonnectivityCPAgentToken(ctx, vcp)
	if err != nil {
		return reconcile.Result{}, err
	}

	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      konnectivityAgentCPSecretName,
			Namespace: ns,
			Annotations: map[string]string{
				"control-plane.deckhouse.io/token-expires-at": exp,
			},
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

	current, err := r.getSecret(ctx, ns, konnectivityAgentCPSecretName)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, r.createSecret(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	base := current.DeepCopy()
	current.Data = target.Data
	current.Labels = target.Labels
	current.Annotations = target.Annotations
	return reconcile.Result{}, r.patchSecret(ctx, base, current)
}

func (r *reconciler) ensureKonnectivityCPAgentToken(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
) (string, string, error) {
	ns := constants.VirtualControlPlaneNamespacePrefix + vcp.Name

	if current, err := r.getSecret(ctx, ns, konnectivityAgentCPSecretName); err == nil {
		if token := string(current.Data["token"]); token != "" {
			if expRaw := current.Annotations["control-plane.deckhouse.io/token-expires-at"]; expRaw != "" {
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
