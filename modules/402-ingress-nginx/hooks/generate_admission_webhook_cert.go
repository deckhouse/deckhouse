/*
Copyright 2021 Flant JSC

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

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cert-secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"ingress-admission-certificate"},
			},
			NamespaceSelector:            internal.NsSelector(),
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterAdmissionSecret,
		},
	},
}, generateValidateWebhookCert)

func filterAdmissionSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec v1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(sec.Data["ca.crt"]),
		Cert: string(sec.Data["tls.crt"]),
		Key:  string(sec.Data["tls.key"]),
	}, nil
}

func generateValidateWebhookCert(_ context.Context, input *go_hook.HookInput) error {
	snap := input.Snapshots.Get("cert-secret")

	if len(snap) > 0 {
		var adm certificate.Certificate
		if err := snap[0].UnmarshalTo(&adm); err != nil {
			return fmt.Errorf("failed to unmarshal 'cert-secret' snapshots: %w", err)
		}

		input.Values.Set("ingressNginx.internal.admissionCertificate.cert", adm.Cert)
		input.Values.Set("ingressNginx.internal.admissionCertificate.key", adm.Key)
		input.Values.Set("ingressNginx.internal.admissionCertificate.ca", adm.CA)

		return nil
	}

	const cn = "ingress-nginx-validation.webhook.ca"
	ca, err := certificate.GenerateCA(input.Logger, cn,
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithSANs("ingress-nginx-validation.webhook.ca"),
		certificate.WithGroups("ingress-nginx.d8-ingress-nginx"),
	)
	if err != nil {
		return errors.Wrap(err, "generate CA failed")
	}

	tls, err := certificate.GenerateSelfSignedCert(input.Logger,
		cn,
		ca,
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithSANs("*.d8-ingress-nginx", "*.d8-ingress-nginx.svc"),
		certificate.WithGroups("ingress-nginx.d8-ingress-nginx"),
		certificate.WithSigningDefaultExpiry(87600*time.Hour),
	)
	if err != nil {
		return errors.Wrap(err, "generate Cert failed")
	}

	input.Values.Set("ingressNginx.internal.admissionCertificate.cert", tls.Cert)
	input.Values.Set("ingressNginx.internal.admissionCertificate.key", tls.Key)
	input.Values.Set("ingressNginx.internal.admissionCertificate.ca", tls.CA)

	return nil
}
