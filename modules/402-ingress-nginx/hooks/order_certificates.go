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

	"github.com/cloudflare/cfssl/helpers"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const namespace = "d8-ingress-nginx"

type CertificateInfo struct {
	ControllerName string                  `json:"controllerName,omitempty"`
	IngressClass   string                  `json:"ingressClass,omitempty"`
	Data           certificate.Certificate `json:"data,omitempty"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// this hook should be run after get_ingress_controllers hook, which has order: 10
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "42 4 * * *"},
	},
}, dependency.WithExternalDependencies(orderCertificate))

func getSecret(namespace, name string, dc dependency.Container) (*certificate.Certificate, error) {
	k8, err := dc.GetK8sClient()
	if err != nil {
		return nil, err
	}

	secret, err := k8.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &certificate.Certificate{
		Cert: string(secret.Data["client.crt"]),
		Key:  string(secret.Data["client.key"]),
	}, nil
}

func orderCertificate(input *go_hook.HookInput, dc dependency.Container) error {
	if !input.Values.Exists("ingressNginx.internal.ingressControllers") {
		return nil
	}

	caAuthority := certificate.Authority{
		Key:  input.Values.Get("global.internal.modules.kubeRBACProxyCA.key").String(),
		Cert: input.Values.Get("global.internal.modules.kubeRBACProxyCA.cert").String(),
	}

	certificates := make([]CertificateInfo, 0)
	controllersValues := input.Values.Get("ingressNginx.internal.ingressControllers").Array()

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
		secret, err := getSecret(namespace, secretName, dc)
		if err != nil {
			return fmt.Errorf("can't get Secret %s: %v", secretName, err)
		}

		// If existing Certificate expires in more than 7 days — use it.
		if secret != nil && len(secret.Cert) > 0 && len(secret.Key) > 0 {
			shouldGenerateNewCert, err := certificate.IsCertificateExpiringSoon([]byte(secret.Cert), time.Hour*24*7)
			if err != nil {
				return err
			}

			// migration 1.42: this branch could be deleted after 1.42 release
			if !shouldGenerateNewCert {
				shouldGenerateNewCert, err = shouldMigrateOldCertificate([]byte(secret.Cert))
				if err != nil {
					return err
				}
			}
			// end migration

			if !shouldGenerateNewCert {
				certificates = append(certificates, CertificateInfo{
					ControllerName: controller.Name,
					IngressClass:   ingressClass,
					Data: certificate.Certificate{
						Cert: secret.Cert,
						Key:  secret.Key,
					},
				})

				continue
			}
		}

		info, err := certificate.GenerateSelfSignedCert(input.LogEntry,
			fmt.Sprintf("nginx-ingress:%s", controller.Name),
			caAuthority,
			certificate.WithGroups("ingress-nginx:auth"),
			certificate.WithSigningDefaultExpiry(87600*time.Hour),
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

func shouldMigrateOldCertificate(cert []byte) (bool, error) {
	c, err := helpers.ParseCertificatePEM(cert)
	if err != nil {
		return false, err
	}
	return c.Issuer.CommonName == "kubernetes", nil
}
