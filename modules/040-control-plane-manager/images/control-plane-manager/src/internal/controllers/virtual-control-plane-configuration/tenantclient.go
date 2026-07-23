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
	"crypto/sha256"
	"fmt"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	tenantClientTimeout = 10 * time.Second
	tenantClientQPS     = 5
	tenantClientBurst   = 10
)

// tenantClientSet caches the clients built for one tenant cluster.
// kubeconfigHash invalidates the entry when the admin kubeconfig rotates.
type tenantClientSet struct {
	kubeconfigHash [sha256.Size]byte
	clientset      kubernetes.Interface
	client         client.Client
}

func (r *reconciler) tenantKubeconfigRaw(ctx context.Context, namespace string) ([]byte, error) {
	sec, err := r.getSecret(ctx, namespace, constants.VirtualAdminKubeconfigSecretName)
	if err != nil {
		return nil, fmt.Errorf("get admin kubeconfig secret: %w", err)
	}

	raw, ok := sec.Data[string(kubeconfig.SuperAdmin)]
	if !ok {
		return nil, fmt.Errorf("admin kubeconfig secret missing %q", string(kubeconfig.SuperAdmin))
	}
	return raw, nil
}

// tenantClients returns clients for the tenant cluster, cached per VirtualControlPlane:
// - typed clientset (for bootstrap-token management)
// - controller-runtime client (for applying arbitrary/unstructured addon manifests)
// Building client.New triggers discovery against the tenant API server, so cached
// entries are reused until the admin kubeconfig content changes (e.g. cert rotation).
func (r *reconciler) tenantClients(ctx context.Context, vcp *controlplanev1alpha1.VirtualControlPlane) (kubernetes.Interface, client.Client, error) {
	raw, err := r.tenantKubeconfigRaw(ctx, vcp.Namespace)
	if err != nil {
		return nil, nil, err
	}
	hash := sha256.Sum256(raw)

	if v, ok := r.tenantClientSets.Load(tenantClientCacheKey(vcp.Namespace, vcp.Name)); ok {
		if cached := v.(*tenantClientSet); cached.kubeconfigHash == hash {
			return cached.clientset, cached.client, nil
		}
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("build rest config: %w", err)
	}
	cfg.Timeout = tenantClientTimeout
	cfg.QPS = tenantClientQPS
	cfg.Burst = tenantClientBurst

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build tenant clientset: %w", err)
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, nil, fmt.Errorf("build tenant client: %w", err)
	}

	r.tenantClientSets.Store(tenantClientCacheKey(vcp.Namespace, vcp.Name), &tenantClientSet{
		kubeconfigHash: hash,
		clientset:      cs,
		client:         c,
	})
	return cs, c, nil
}

// forgetTenantClients drops cached tenant clients for a deleted VirtualControlPlane.
func (r *reconciler) forgetTenantClients(namespace, name string) {
	r.tenantClientSets.Delete(tenantClientCacheKey(namespace, name))
}

func tenantClientCacheKey(namespace, name string) string {
	return namespace + "/" + name
}
