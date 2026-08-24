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
	"github.com/pkg/errors"
	certificatesv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

type AuthCertificate struct {
	Cert string `json:"crt"`
	Key  string `json:"key"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kubernetes-api-proxy-discovery-cert",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes-api-proxy-discovery-cert"},
			},
			FilterFunc: apiserverProxyCertFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "certificateCheck",
			Crontab: "42 4 * * *",
		},
	},
}, dependency.WithExternalDependencies(createRBACForKubeAPIServerProxy))

// issueKubeAPIProxyCertificate is a seam over tls_certificate.IssueCertificate so the
// certificate-generation path can be exercised in unit tests without a CSR signer.
var issueKubeAPIProxyCertificate = tls_certificate.IssueCertificate

func createRBACForKubeAPIServerProxy(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	const (
		roleName             = "node-manager:kubernetes-api-proxy"
		userName             = "kubernetes-api-proxy"
		certOutdatedDuration = (24 * time.Hour) * 365 / 2
	)

	var (
		certExpirationSec = int32((time.Hour * 24 * 365 * 10).Seconds()) // 10 years
	)

	certs, err := sdkobjectpatch.UnmarshalToStruct[AuthCertificate](input.Snapshots, "kubernetes-api-proxy-discovery-cert")
	if err != nil {
		return fmt.Errorf("cannot unmarshal kubernetes-api-proxy-discovery-cert from snapshots: %v", err)
	}

	if len(certs) > 0 {
		cert, err := certificate.ParseCertificate(certs[0].Cert)
		if err != nil {
			return fmt.Errorf("cannot parse kubernetes-api-proxy-discovery-cert from snapshots: %v", err)
		}

		if time.Until(cert.NotAfter) >= certOutdatedDuration {
			// The certificate already exists and is still valid. Always republish it into the
			// module values so apiserverProxyCerts stays present in the bashible context and the
			// configuration checksum stays stable, then stop.
			setKubeAPIProxyDiscoveryCertValues(input, certs[0].Cert, certs[0].Key)
			return nil
		}
	}

	// There is no certificate yet, or the existing one is about to expire: issue a new one.
	cert, err := issueKubeAPIProxyCertificate(input, dc, tls_certificate.OrderCertificateRequest{
		CommonName: userName,
		Groups: []string{
			roleName,
		},
		Usages: []certificatesv1.KeyUsage{
			certificatesv1.UsageClientAuth,
		},
		ExpirationSeconds: &certExpirationSec,
	})
	if err != nil {
		return errors.Wrap(err, "failed to issue certificate")
	}

	setKubeAPIProxyDiscoveryCertValues(input, cert.Certificate, cert.Key)

	// Persist the certificate into a Secret so the next reconcile finds it in the snapshot and
	// does not regenerate it. Without this the snapshot is always empty, a fresh certificate is
	// minted on every run, apiserverProxyCerts changes every time, and the bashible configuration
	// checksum flaps — forcing endless node rollouts.
	if err := saveKubeAPIProxyDiscoveryCertSecret(input, cert.Certificate, cert.Key); err != nil {
		return err
	}

	return nil
}

func setKubeAPIProxyDiscoveryCertValues(input *go_hook.HookInput, crt, key string) {
	input.Values.Set("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.crt", crt)
	input.Values.Set("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.key", key)
}

func saveKubeAPIProxyDiscoveryCertSecret(input *go_hook.HookInput, crt, key string) error {
	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-api-proxy-discovery-cert",
			Namespace: "kube-system",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "node-manager",
			},
		},
		Data: map[string][]byte{
			"crt": []byte(crt),
			"key": []byte(key),
		},
		Type: v1.SecretTypeOpaque,
	}

	secretUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		return errors.Wrap(err, "failed to convert kubernetes-api-proxy-discovery-cert secret to unstructured")
	}

	input.PatchCollector.CreateOrUpdate(secretUnstructured)

	return nil
}

func apiserverProxyCertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}

	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}

	return AuthCertificate{
		Cert: string(secret.Data["crt"]),
		Key:  string(secret.Data["key"]),
	}, nil
}
