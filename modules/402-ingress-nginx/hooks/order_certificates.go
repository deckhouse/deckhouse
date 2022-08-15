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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const namespace = "d8-ingress-nginx"

type CertificateInfo struct {
	ControllerName string                          `json:"controllerName,omitempty"`
	IngressClass   string                          `json:"ingressClass,omitempty"`
	Data           tls_certificate.CertificateInfo `json:"data,omitempty"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "42 4 * * *"},
	},
}, dependency.WithExternalDependencies(orderCertificate))

func getSecret(namespace, name string, dc dependency.Container) (*tls_certificate.CertificateSecret, error) {
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

	cc := tls_certificate.ParseSecret(secret)

	return cc, nil
}

func orderCertificate(input *go_hook.HookInput, dc dependency.Container) error {
	if input.Values.Exists("ingressNginx.internal.ingressControllers") {
		var certificates []CertificateInfo
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
			if secret != nil && len(secret.Crt) > 0 && len(secret.Key) > 0 {
				shouldGenerateNewCert, err := certificate.IsCertificateExpiringSoon(secret.Crt, time.Hour*24*7)
				if err != nil {
					return err
				}

				if !shouldGenerateNewCert {
					certificates = append(certificates, CertificateInfo{
						ControllerName: controller.Name,
						IngressClass:   ingressClass,
						Data: tls_certificate.CertificateInfo{
							Certificate: string(secret.Crt),
							Key:         string(secret.Key),
						},
					})

					continue
				}
			}

			request := tls_certificate.OrderCertificateRequest{
				Namespace:  namespace,
				SecretName: secretName,
				CommonName: fmt.Sprintf("nginx-ingress:%s", controller.Name),
				Groups:     []string{"ingress-nginx:auth"},
				ModuleName: "ingressNginx",
			}

			info, err := tls_certificate.IssueCertificate(input, dc, request)
			if err != nil {
				return err
			}

			certificates = append(certificates, CertificateInfo{
				ControllerName: controller.Name,
				IngressClass:   ingressClass,
				Data:           *info,
			})
		}

		// Sort slice to prevent triggering helm on elements order change.
		sort.Slice(certificates, func(i, j int) bool {
			return certificates[i].ControllerName < certificates[j].ControllerName
		})

		input.Values.Set("ingressNginx.internal.nginxAuthTLS", certificates)
	}

	return nil
}
