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
	"fmt"
	"time"

	"github.com/cloudflare/cfssl/csr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

const (
	commonName      = "kube-api-admission"
	snapshotName    = "admission-webhook-client-key-pair-secrets"
	secretName      = "admission-webhook-client-key-pair"
	secretNamespace = "d8-system"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       snapshotName,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{secretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{secretNamespace},
				},
			},
			FilterFunc: filterAdmissionSecret,
		},
	},
}, generateValidateWebhookCert)

func filterAdmissionSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}
	return certificate.Certificate{
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func generateValidateWebhookCert(input *go_hook.HookInput) error {
	snapshots := input.NewSnapshots.Get(snapshotName)

	// Try reuse certificate from Secret
	if len(snapshots) > 0 {
		secret := certificate.Certificate{}
		err := snapshots[0].UnmarshalTo(&secret)
		if err != nil {
			input.Logger.Error(fmt.Sprintf("Failed to unmarshal certificate: %v", err))
			return err
		}
		input.Values.Set("controlPlaneManager.internal.admissionWebhookClient.cert", secret.Cert)
		input.Values.Set("controlPlaneManager.internal.admissionWebhookClient.key", secret.Key)
		return nil
	}
	input.Logger.Debug(fmt.Sprintf("Certificate not found in secret %s/%s and will be generated", secretNamespace, secretName))

	// Get CA certificate from global values
	caCert := input.Values.Get("global.internal.modules.admissionWebhookClientCA.cert").String()
	caKey := input.Values.Get("global.internal.modules.admissionWebhookClientCA.key").String()
	if caCert == "" || caKey == "" {
		return fmt.Errorf("admission webhook client CA certificate or key not found")
	}
	CA := certificate.Authority{
		Cert: caCert,
		Key:  caKey,
	}

	// Generate self signed certificate
	tls, err := certificate.GenerateSelfSignedCert(input.Logger,
		commonName,
		CA,
		certificate.WithKeyRequest(&csr.KeyRequest{
			A: "rsa",
			S: 2048,
		}),
		certificate.WithSigningDefaultExpiry(87600*time.Hour),
	)
	if err != nil {
		return fmt.Errorf("failed generate validate webhook client certificate: %s", err)
	}

	input.Values.Set("controlPlaneManager.internal.admissionWebhookClient.cert", tls.Cert)
	input.Values.Set("controlPlaneManager.internal.admissionWebhookClient.key", tls.Key)

	return nil
}
