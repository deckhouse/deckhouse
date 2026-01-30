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

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const validatingWebhookConfigurationName = "cluster-objects-grants"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "policies",
			ApiVersion: "projects.deckhouse.io/v1alpha1",
			Kind:       "ClusterObjectGrantPolicy",
			FilterFunc: filterPolicies,
		},
	},
}, dependency.WithExternalDependencies(configureGrantValidationWebhook))

func filterPolicies(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

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
		whConfigExists = false
		whConfig = &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: v1.ObjectMeta{Name: validatingWebhookConfigurationName},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: fmt.Sprintf("%s.projects.deckhouse.io", validatingWebhookConfigurationName),
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "cluster-objects-controller",
							Namespace: "d8-multitenancy-manager",
							Path:      ptr.To("/is-granted"),
							Port:      ptr.To(int32(9443)),
						},
						CABundle: []byte(caBundle),
					},
					SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
					AdmissionReviewVersions: []string{"v1"},
					FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
					Rules:                   []admissionregistrationv1.RuleWithOperations{},
				},
			},
		}
	case err != nil:
		return fmt.Errorf("read ValidatingWebhookConfiguration: %w", err)
	}

	whRules := make([]admissionregistrationv1.RuleWithOperations, 0)
	snaps := input.Snapshots.Get("policies")
	input.Logger.Info("Got snaps: %+v", snaps)
	for _, snap := range snaps {
		policy := &unstructured.Unstructured{}
		if err = snap.UnmarshalTo(policy); err != nil {
			return fmt.Errorf("unmarshal snapshot: %w", err)
		}

		// Error is ignored as openapi gurantees its a slice
		usageRefs, found, _ := unstructured.NestedSlice(policy.Object, "spec", "usageReferences")
		if !found {
			continue
		}

		for _, ref := range usageRefs {
			apiGroup := ref.(map[string]any)["apiGroup"].(string)
			res := ref.(map[string]any)["resource"].(string)

			whRules = append(whRules, admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{apiGroup},
					APIVersions: []string{"*"},
					Resources:   []string{res},
					Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
				},
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create, admissionregistrationv1.Update,
				},
			})

		}
	}

	whConfig.Webhooks[0].Rules = whRules
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
