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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const mutatingWebhookConfigurationName = "cluster-objects-grants-defaulting"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		admissionWebhookCertWatch(),
		{
			Name:       "registrations",
			ApiVersion: "multitenancy.deckhouse.io/v1alpha1",
			Kind:       "GrantableClusterResourceDefinition",
			FilterFunc: filterRegistrations,
		},
		{
			Name:       "references",
			ApiVersion: "multitenancy.deckhouse.io/v1alpha1",
			Kind:       "GrantableClusterResourceReference",
			FilterFunc: filterReferences,
		},
	},
}, dependency.WithExternalDependencies(configureDefaultingWebhook))

func newGrantDefaultingWebhook() admissionregistrationv1.MutatingWebhook {
	return admissionregistrationv1.MutatingWebhook{
		Name: fmt.Sprintf("%s.multitenancy.deckhouse.io", mutatingWebhookConfigurationName),
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Name:      "multitenancy-manager",
				Namespace: "d8-multitenancy-manager",
				Path:      ptr.To("/defaults"),
				Port:      ptr.To(int32(9443)),
			},
		},
		NamespaceSelector:       projectNamespaceSelector,
		MatchConditions:         systemWriterMatchConditions,
		SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
		AdmissionReviewVersions: []string{"v1"},
		FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
		TimeoutSeconds:          ptr.To(int32(10)),
		Rules:                   []admissionregistrationv1.RuleWithOperations{},
	}
}

func configureDefaultingWebhook(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	caBundle, err := admissionWebhookCABundle(input)
	if err != nil {
		return err
	}

	kube, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	admissionClient := kube.AdmissionregistrationV1().MutatingWebhookConfigurations()

	whConfigExists := true
	whConfig, err := admissionClient.Get(
		ctx, mutatingWebhookConfigurationName, v1.GetOptions{},
	)
	switch {
	case k8serrors.IsNotFound(err):
		if caBundle == "" {
			return errWebhookCertNotIssued
		}
		whConfigExists = false
		whConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: v1.ObjectMeta{Name: mutatingWebhookConfigurationName},
			Webhooks:   []admissionregistrationv1.MutatingWebhook{newGrantDefaultingWebhook()},
		}
	case err != nil:
		return fmt.Errorf("read MutatingWebhookConfiguration: %w", err)
	}

	if len(whConfig.Webhooks) == 0 {
		whConfig.Webhooks = []admissionregistrationv1.MutatingWebhook{newGrantDefaultingWebhook()}
	}

	whConfig.Webhooks[0].Rules = grantableWebhookRules(input)
	// Reconcile CA/selector/match-conditions/timeout on existing configurations too (e.g. upgrades
	// and TLS rotation). The serving cert is regenerated when SANs change; leaving caBundle at the
	// create-time value makes the apiserver reject the webhook with x509 ECDSA verification failure.
	// Skip the CA write when it is not issued yet: rules/matchConditions must still reconcile, or
	// the hook fails the queue the way #20700 was written to prevent.
	if caBundle != "" {
		whConfig.Webhooks[0].ClientConfig.CABundle = []byte(caBundle)
	}
	whConfig.Webhooks[0].NamespaceSelector = projectNamespaceSelector
	whConfig.Webhooks[0].MatchConditions = systemWriterMatchConditions
	whConfig.Webhooks[0].TimeoutSeconds = ptr.To(int32(10))
	if whConfigExists {
		_, err = admissionClient.Update(ctx, whConfig, v1.UpdateOptions{})
	} else {
		_, err = admissionClient.Create(ctx, whConfig, v1.CreateOptions{})
	}
	if err != nil {
		return fmt.Errorf("apply update MutatingWebhookConfiguration: %w", err)
	}

	return nil
}
