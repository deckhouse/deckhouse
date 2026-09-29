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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

// The kube-rbac-proxy sidecar in front of Kiali used to issue its own serving
// certificate at pod startup, so in-cluster clients had nothing to verify it
// against and had to skip verification. This hook issues the serving
// certificate from the cluster-wide kube-rbac-proxy CA instead: that CA is
// published as the kube-rbac-proxy-ca.crt ConfigMap in module namespaces, so
// any client of the sidecar can verify it the usual way.
const (
	kialiRBACProxySecretName = "kiali-kube-rbac-proxy-tls"

	// Common name and SANs the clients address the sidecar by — the Service in
	// the module namespace.
	kialiRBACProxyCN = "kiali.d8-istio.svc"

	kialiRBACProxyCertPath = "istio.internal.kialiRBACProxyTLS.crt"
	kialiRBACProxyKeyPath  = "istio.internal.kialiRBACProxyTLS.key"
	kialiRBACProxyCAPath   = "istio.internal.kialiRBACProxyTLS.ca"

	globalRBACProxyCACertPath = "global.internal.modules.kubeRBACProxyCA.cert"
	globalRBACProxyCAKeyPath  = "global.internal.modules.kubeRBACProxyCA.key"

	// Certificates are long-lived, as elsewhere in the platform; the renewal
	// margin only matters for clusters older than the certificate lifetime.
	kialiRBACProxyCertExpiry     = 87600 * time.Hour
	kialiRBACProxyCertRenewAfter = 30 * 24 * time.Hour
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/istio/gen-kiali-rbac-proxy-cert",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "kiali_rbac_proxy_tls_secret",
			ApiVersion:        "v1",
			Kind:              "Secret",
			FilterFunc:        applyKialiRBACProxyTLSFilter,
			NameSelector:      &types.NameSelector{MatchNames: []string{kialiRBACProxySecretName}},
			NamespaceSelector: lib.NsSelector(),
		},
	},
}, generateKialiRBACProxyCert)

func applyKialiRBACProxyTLSFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert kiali kube-rbac-proxy secret to structured secret: %v", err)
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func generateKialiRBACProxyCert(_ context.Context, input *go_hook.HookInput) error {
	ca := certificate.Authority{
		Cert: input.Values.Get(globalRBACProxyCACertPath).String(),
		Key:  input.Values.Get(globalRBACProxyCAKeyPath).String(),
	}
	if ca.Cert == "" || ca.Key == "" {
		return fmt.Errorf("cluster-wide kube-rbac-proxy CA is not generated yet")
	}

	if cert, ok := reusableKialiRBACProxyCert(input, ca.Cert); ok {
		input.Values.Set(kialiRBACProxyCertPath, cert.Cert)
		input.Values.Set(kialiRBACProxyKeyPath, cert.Key)
		input.Values.Set(kialiRBACProxyCAPath, cert.CA)

		return nil
	}

	tls, err := certificate.GenerateSelfSignedCert(input.Logger,
		kialiRBACProxyCN,
		ca,
		certificate.WithSANs(kialiRBACProxyCN, "kiali.d8-istio", "kiali"),
		certificate.WithSigningDefaultExpiry(kialiRBACProxyCertExpiry),
	)
	if err != nil {
		return fmt.Errorf("generate kiali kube-rbac-proxy certificate: %w", err)
	}

	input.Values.Set(kialiRBACProxyCertPath, tls.Cert)
	input.Values.Set(kialiRBACProxyKeyPath, tls.Key)
	// The signer is the cluster-wide CA, and clients verify against its
	// ConfigMap copy — store that CA, not the one cfssl echoes back.
	input.Values.Set(kialiRBACProxyCAPath, ca.Cert)

	return nil
}

// reusableKialiRBACProxyCert reports the certificate from the cluster when it
// still fits: issued by the current CA and not about to expire. Reissuing on
// every run would rotate the sidecar certificate for no reason; keeping a
// certificate of a rotated CA would leave clients unable to verify it.
func reusableKialiRBACProxyCert(input *go_hook.HookInput, caCert string) (certificate.Certificate, bool) {
	snapshots := input.Snapshots.Get("kiali_rbac_proxy_tls_secret")
	if len(snapshots) == 0 {
		return certificate.Certificate{}, false
	}

	var cert certificate.Certificate
	if err := snapshots[0].UnmarshalTo(&cert); err != nil {
		input.Logger.Warn("cannot read kiali kube-rbac-proxy secret, reissuing the certificate", "error", err)
		return certificate.Certificate{}, false
	}

	if cert.Cert == "" || cert.Key == "" || cert.CA != caCert {
		return certificate.Certificate{}, false
	}

	expiring, err := certificate.IsCertificateExpiringSoon([]byte(cert.Cert), kialiRBACProxyCertRenewAfter)
	if err != nil {
		input.Logger.Warn("cannot parse kiali kube-rbac-proxy certificate, reissuing it", "error", err)
		return certificate.Certificate{}, false
	}

	return cert, !expiring
}
