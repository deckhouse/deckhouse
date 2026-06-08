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
	"errors"
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const validatingWebhookConfigurationName = "cluster-objects-grants-validator"

// projectNamespaceSelector restricts the webhook to namespaces managed by the
// multitenancy-manager (i.e. project namespaces). Without it, with
// failurePolicy: Fail, an unavailable controller would block create/update of
// the configured resources in every namespace — including system ones — which
// can deadlock the cluster. The in-handler IsSystem check remains as
// defense-in-depth.
var projectNamespaceSelector = &v1.LabelSelector{
	MatchLabels: map[string]string{"heritage": "multitenancy-manager"},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "registrations",
			ApiVersion: "multitenancy.deckhouse.io/v1alpha1",
			Kind:       "ClusterGrantableResource",
			FilterFunc: filterRegistrations,
		},
	},
}, dependency.WithExternalDependencies(configureGrantValidationWebhook))

func configureGrantValidationWebhook(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kube, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	admissionClient := kube.AdmissionregistrationV1().
		ValidatingWebhookConfigurations()

	whConfigExists := true
	whConfig, err := admissionClient.Get(
		ctx, validatingWebhookConfigurationName, v1.GetOptions{},
	)
	switch {
	case k8serrors.IsNotFound(err):
		caBundle := input.Values.Get("multitenancyManager.internal.clusterObjectsControllerWebhookCert.ca").String()
		if caBundle == "" {
			return errors.New("webhook certificate is not issued yet")
		}

		whConfigExists = false
		whConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: v1.ObjectMeta{Name: validatingWebhookConfigurationName},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: fmt.Sprintf("%s.multitenancy.deckhouse.io", validatingWebhookConfigurationName),
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "cluster-objects-controller",
							Namespace: "d8-multitenancy-manager",
							Path:      ptr.To("/is-granted"),
							Port:      ptr.To(int32(9443)),
						},
						CABundle: []byte(caBundle),
					},
					NamespaceSelector:       projectNamespaceSelector,
					SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
					AdmissionReviewVersions: []string{"v1"},
					FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
					TimeoutSeconds:          ptr.To(int32(10)),
					Rules:                   []admissionregistrationv1.RuleWithOperations{},
				},
			},
		}
	case err != nil:
		return fmt.Errorf("read ValidatingWebhookConfiguration: %w", err)
	}

	whConfig.Webhooks[0].Rules = grantableWebhookRules(input)
	// Reconcile selector/timeout on existing configurations too (e.g. upgrades).
	whConfig.Webhooks[0].NamespaceSelector = projectNamespaceSelector
	whConfig.Webhooks[0].TimeoutSeconds = ptr.To(int32(10))
	if whConfigExists {
		_, err = admissionClient.Update(ctx, whConfig, v1.UpdateOptions{})
	} else {
		_, err = admissionClient.Create(ctx, whConfig, v1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("apply update ValidatingWebhookConfiguration: %w", err)
	}

	return nil
}
