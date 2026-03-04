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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

	var needToGenerate = false
	if len(certs) == 0 {
		needToGenerate = true
	} else {
		cert, err := certificate.ParseCertificate(certs[0].Cert)
		if err != nil {
			return fmt.Errorf("cannot parse kubernetes-api-proxy-discovery-cert from snapshots: %v", err)
		}

		if time.Until(cert.NotAfter) < certOutdatedDuration {
			needToGenerate = true
		}
	}

	if !needToGenerate {
		return nil
	}

	cert, err := tls_certificate.IssueCertificate(input, dc, tls_certificate.OrderCertificateRequest{
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

	input.Values.Set("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.crt", cert.Certificate)
	input.Values.Set("nodeManager.internal.kubernetesAPIProxyDiscoveryCert.key", cert.Key)

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
