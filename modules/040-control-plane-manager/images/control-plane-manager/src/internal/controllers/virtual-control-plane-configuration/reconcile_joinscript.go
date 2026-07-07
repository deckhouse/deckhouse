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
	"encoding/base64"
	"fmt"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const registryPackagesProxyTokenNamespace = "d8-cloud-instance-manager"

func (r *reconciler) reconcileJoinScript(
	ctx context.Context,
	vcp *controlplanev1alpha1.VirtualControlPlane,
	pkiSecret *corev1.Secret,
	configSecret *corev1.Secret,
	joinToken string,
) (reconcile.Result, error) {
	host, port := externalAPIEndpoint(vcp)
	if joinToken == "" {
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	vip, err := r.albVIP(ctx, vcp)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("resolve ALB VIP: %w", err)
	}
	if vip == "" {
		// ALB LoadBalancer address not assigned yet, waiting — the join.sh /etc/hosts entry needs it.
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}
	endpoint := fmt.Sprintf("https://%s:%d", host, port)

	caPEM := pkiSecret.Data["ca.crt"]
	bootstrapKubeconfig, err := kubeconfig.BuildBootstrapKubeletConfig(caPEM, endpoint, joinToken)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("build bootstrap kubeconfig: %w", err)
	}

	table, err := parseImagesTable(configSecret.Data)
	if err != nil {
		return reconcile.Result{}, err
	}
	rp, ok := table.RegistryPackages.Versioned[vcp.Spec.KubernetesVersion]
	if !ok || rp.Kubelet == "" {
		// registrypackages digests not available (e.g. partial build) — skip, retry later.
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	rppToken, err := r.registryPackagesProxyToken(ctx)
	if err != nil {
		return reconcile.Result{RequeueAfter: requeueIntervalOnReadingClusterIP}, nil
	}

	tpl, ok := configSecret.Data["join.sh.tpl"]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("config secret missing join.sh.tpl")
	}

	replacer := strings.NewReplacer(
		"${VCP_RPP_TOKEN}", rppToken,
		"${VCP_MINGET_B64}", string(configSecret.Data["minget"]),
		"${VCP_CLUSTER_UUID}", string(configSecret.Data["cluster-uuid"]),
		"${VCP_RPP_GET_DIGEST}", table.RegistryPackages.Fixed.RppGet,
		"${VCP_CONTAINERD_DIGEST}", table.RegistryPackages.Fixed.Containerd,
		"${VCP_CRICTL_DIGEST}", rp.Crictl,
		"${VCP_KUBELET_DIGEST}", rp.Kubelet,
		"${VCP_CA_CRT_B64}", base64.StdEncoding.EncodeToString(caPEM),
		"${VCP_BOOTSTRAP_KUBECONFIG}", string(bootstrapKubeconfig),
		"${VCP_CLUSTER_DOMAIN}", constants.DefaultTenantClusterDomain,
		"${VCP_CLUSTER_DNS}", "10.96.0.10",
		"${VCP_ALB_VIP}", vip,
		"${VCP_API_HOST}", host,
		"${VCP_KONN_HOST}", konnExposeHost(vcp),
		"${VCP_PKG_HOST}", packagesExposeHost(vcp),
	)
	rendered := replacer.Replace(string(tpl))

	ns := constants.VirtualControlPlaneNamespacePrefix + vcp.Name
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.VirtualJoinScriptSecretName,
			Namespace: ns,
			Labels:    map[string]string{constants.HeritageLabelKey: constants.HeritageLabelValue},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"join.sh": []byte(rendered)},
	}

	current, err := r.getSecret(ctx, ns, target.Name)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, r.createSecret(ctx, target)
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get join-script secret: %w", err)
	}
	base := current.DeepCopy()
	current.Data = target.Data
	return reconcile.Result{}, r.patchSecret(ctx, base, current)
}

// registryPackagesProxyToken returns the RPP bearer token from the parent cluster
func (r *reconciler) registryPackagesProxyToken(ctx context.Context) (string, error) {
	sec, err := r.getSecret(ctx, registryPackagesProxyTokenNamespace, "registry-packages-proxy-token")
	if err != nil {
		return "", fmt.Errorf("get rpp token: %w", err)
	}
	return string(sec.Data["token"]), nil
}
