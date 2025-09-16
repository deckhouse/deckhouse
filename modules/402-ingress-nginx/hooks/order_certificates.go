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
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

type CertificateInfo struct {
	ControllerName string                  `json:"controllerName,omitempty"`
	IngressClass   string                  `json:"ingressClass,omitempty"`
	Data           certificate.Certificate `json:"data,omitempty"`
}

type certificateData struct {
	CertificateData *certificate.Certificate `json:"certificateData,omitempty"`
	SecretName      string                   `json:"name,omitempty"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// this hook should be run after get_ingress_controllers hook, which has order: 10
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "certificates_data",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			FilterFunc:                   applyIngressSecretFilter,
			NamespaceSelector:            internal.NsSelector(),
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "42 4 * * *"},
	},
}, orderCertificate)

func applyIngressSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert tls secret to Secret: %v", err)
	}

	return certificateData{
		SecretName: secret.GetName(),
		CertificateData: &certificate.Certificate{
			Cert: string(secret.Data["client.crt"]),
			Key:  string(secret.Data["client.key"]),
		},
	}, nil
}

func orderCertificate(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Exists("ingressNginx.internal.ingressControllers") {
		return nil
	}

	caAuthority := certificate.Authority{
		Key:  input.Values.Get("global.internal.modules.kubeRBACProxyCA.key").String(),
		Cert: input.Values.Get("global.internal.modules.kubeRBACProxyCA.cert").String(),
	}

	certificates := make([]CertificateInfo, 0)
	controllersValues := input.Values.Get("ingressNginx.internal.ingressControllers").Array()

	certificatesSecretMap := make(map[string]*certificate.Certificate)
	for certificateData, err := range sdkobjectpatch.SnapshotIter[certificateData](input.Snapshots.Get("certificates_data")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'certificates_data' snapshots: %w", err)
		}

		certificatesSecretMap[certificateData.SecretName] = certificateData.CertificateData
	}

	for _, c := range controllersValues {
		var controller Controller
		err := json.Unmarshal([]byte(c.Raw), &controller)
		if err != nil {
			return fmt.Errorf("cannot unmarshal: %v", err)
		}

		ingressClass, _, err := unstructured.NestedString(controller.Spec, "ingressClass")
		if err != nil {
			return fmt.Errorf("cannot get ingressClass from ingress controller spec: %v", err)
		}

		secretName := fmt.Sprintf("ingress-nginx-%s-auth-tls", controller.Name)

		// If existing Certificate expires in more than 365 days — use it.
		if certData, ok := certificatesSecretMap[secretName]; ok {
			if certData != nil && len(certData.Cert) > 0 && len(certData.Key) > 0 {
				shouldGenerateNewCert, err := certificate.IsCertificateExpiringSoon([]byte(certData.Cert), time.Hour*24*365) // 1 year
				if err != nil {
					return err
				}

				if !shouldGenerateNewCert {
					certificates = append(certificates, CertificateInfo{
						ControllerName: controller.Name,
						IngressClass:   ingressClass,
						Data:           *certData,
					})

					continue
				}
			}
		}

		info, err := certificate.GenerateSelfSignedCert(input.Logger,
			fmt.Sprintf("nginx-ingress:%s", controller.Name),
			caAuthority,
			certificate.WithGroups("ingress-nginx:auth"),
			certificate.WithSigningDefaultExpiry(10*365*24*time.Hour), // 10 years
			certificate.WithSigningDefaultUsage([]string{
				"signing",
				"key encipherment",
				"client auth",
			}),
		)

		if err != nil {
			return err
		}

		certificates = append(certificates, CertificateInfo{
			ControllerName: controller.Name,
			IngressClass:   ingressClass,
			Data:           info,
		})
	}

	// Sort slice to prevent triggering helm on elements order change.
	sort.Slice(certificates, func(i, j int) bool {
		return certificates[i].ControllerName < certificates[j].ControllerName
	})

	input.Values.Set("ingressNginx.internal.nginxAuthTLS", certificates)

	return nil
}
