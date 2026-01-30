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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	mutatingWebhookConfigurationName = "cluster-objects-grants-defaulting"

	certCAValuesPath = "multitenancyManager.internal.clusterObjectsControllerWebhookCert.ca"
)

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
}, dependency.WithExternalDependencies(configureDefaultingWebhook))

func configureDefaultingWebhook(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
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
		caBundle := input.Values.Get(certCAValuesPath).String()
		if caBundle == "" {
			return errors.New("webhook certificate is not issued yet")
		}

		whConfigExists = false
		whConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: v1.ObjectMeta{Name: mutatingWebhookConfigurationName},
			Webhooks: []admissionregistrationv1.MutatingWebhook{
				{
					Name: fmt.Sprintf("%s.projects.deckhouse.io", mutatingWebhookConfigurationName),
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "cluster-objects-controller",
							Namespace: "d8-multitenancy-manager",
							Path:      ptr.To("/defaults"),
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
		return fmt.Errorf("read MutatingWebhookConfiguration: %w", err)
	}

	whRules := make([]admissionregistrationv1.RuleWithOperations, 0)
	snaps := input.Snapshots.Get("policies")
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
			apiVersion := ref.(map[string]any)["apiVersion"].(string)
			res := ref.(map[string]any)["resource"].(string)

			gv, err := schema.ParseGroupVersion(apiVersion)
			if err != nil {
				input.Logger.InfoContext(
					ctx,
					"Skipping usageReference with invalid apiVersion field",
					"apiVersion", apiVersion,
					"policy", policy.GetName(),
				)
				continue
			}

			whRules = append(whRules, admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{gv.Group},
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
		return fmt.Errorf("apply update MutatingWebhookConfiguration: %w", err)
	}

	return nil
}
