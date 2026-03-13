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

package hooks

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

type kubeRbacProxyTLSSnapshot struct {
	Cert string
	Key  string
}

const (
	lokiKubeRbacProxyTLSQueue        = "/modules/loki/kube_rbac_proxy_tls"
	lokiKubeRbacProxyTLSSecretName   = "loki-kube-rbac-proxy-tls"
	lokiKubeRbacProxyTLSSecretNS     = "d8-monitoring"
	lokiKubeRbacProxyTLSSecretSnap   = "kube_rbac_proxy_tls_secret"
	lokiKubeRbacProxyCertCN          = "loki.d8-monitoring"
	lokiKubeRbacProxyRotationCron    = "0 5 1 * *"
	lokiKubeRbacProxyRotateIfExpires = 60 * 24 * time.Hour // 2 months
)

func kubeRbacProxyTLSSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert loki kube-rbac-proxy tls secret to Secret: %w", err)
	}

	crt, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, fmt.Errorf("'tls.crt' field not found")
	}
	key, ok := secret.Data["tls.key"]
	if !ok {
		return nil, fmt.Errorf("'tls.key' field not found")
	}

	return kubeRbacProxyTLSSnapshot{Cert: string(crt), Key: string(key)}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "lokiKubeRbacProxyTLSRotation", Crontab: lokiKubeRbacProxyRotationCron},
	},
	Queue: lokiKubeRbacProxyTLSQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       lokiKubeRbacProxyTLSSecretSnap,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{MatchNames: []string{
				lokiKubeRbacProxyTLSSecretName,
			}},
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{
				lokiKubeRbacProxyTLSSecretNS,
			}}},
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			FilterFunc:                   kubeRbacProxyTLSSecretFilter,
		},
	},
}, generateOrRotateLokiKubeRbacProxyServerCert)

func generateOrRotateLokiKubeRbacProxyServerCert(_ context.Context, input *go_hook.HookInput) error {
	clusterDomain := input.Values.Get("global.discovery.clusterDomain").String()
	if clusterDomain == "" {
		return fmt.Errorf("global.discovery.clusterDomain is empty")
	}

	requiredSANs := []string{
		"loki",
		"loki.d8-monitoring",
		"loki.d8-monitoring.svc",
		fmt.Sprintf("loki.d8-monitoring.svc.%s", clusterDomain),
	}

	caAuthority := certificate.Authority{
		Key:  input.Values.Get("global.internal.modules.kubeRBACProxyCA.key").String(),
		Cert: input.Values.Get("global.internal.modules.kubeRBACProxyCA.cert").String(),
	}
	if caAuthority.Key == "" || caAuthority.Cert == "" {
		return fmt.Errorf("global.internal.modules.kubeRBACProxyCA.{key,cert} must be set")
	}

	// If existing Secret already contains a valid, not-expiring certificate with the required SANs, reuse it.
	snapshots := input.Snapshots.Get(lokiKubeRbacProxyTLSSecretSnap)
	for existing, err := range sdkobjectpatch.SnapshotIter[kubeRbacProxyTLSSnapshot](snapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %w", lokiKubeRbacProxyTLSSecretSnap, err)
		}

		if existing.Cert == "" || existing.Key == "" {
			break
		}

		parsed, err := helpers.ParseCertificatePEM([]byte(existing.Cert))
		if err != nil {
			break
		}

		if !stringSlicesSetEqual(parsed.DNSNames, requiredSANs) {
			break
		}

		expiringSoon, err := certificate.IsCertificateExpiringSoon([]byte(existing.Cert), lokiKubeRbacProxyRotateIfExpires)
		if err != nil {
			return fmt.Errorf("check loki kube-rbac-proxy certificate expiration: %w", err)
		}
		if expiringSoon {
			break
		}

		input.Values.Set("loki.internal.kubeRbacProxyTLS", map[string]string{"cert": existing.Cert, "key": existing.Key, "ca": caAuthority.Cert})
		return nil
	}

	issued, err := certificate.GenerateSelfSignedCert(
		input.Logger,
		lokiKubeRbacProxyCertCN,
		caAuthority,
		certificate.WithSANs(requiredSANs...),
		certificate.WithSigningDefaultExpiry(10*365*24*time.Hour),
		certificate.WithSigningDefaultUsage([]string{"signing", "key encipherment", "server auth"}),
	)
	if err != nil {
		return fmt.Errorf("generate loki kube-rbac-proxy server certificate: %w", err)
	}

	input.Values.Set("loki.internal.kubeRbacProxyTLS", map[string]string{"cert": issued.Cert, "key": issued.Key, "ca": caAuthority.Cert})

	return nil
}

func stringSlicesSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aCopy := append([]string(nil), a...)
	bCopy := append([]string(nil), b...)

	sort.Strings(aCopy)
	sort.Strings(bCopy)

	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}
