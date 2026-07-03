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

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func (r *reconciler) tenantClientset(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (kubernetes.Interface, error) {
	ns := constants.VirtualControlPlaneNamespacePrefix + vcp.Name
	sec, err := r.getSecret(ctx, ns, constants.VirtualAdminKubeconfigSecretName)
	if err != nil {
		return nil, fmt.Errorf("get admin kubeconfig secret: %w", err)
	}

	raw, ok := sec.Data[string(kubeconfig.SuperAdmin)]
	if !ok {
		return nil, fmt.Errorf("admin kubeconfig secret missing %q", string(kubeconfig.SuperAdmin))
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build tenant clientset: %w", err)
	}
	return cs, nil
}
