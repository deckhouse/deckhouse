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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const validatingWebhookConfigurationName = "cluster-objects-grants-validator"

// projectNamespaceSelector restricts the webhook to namespaces managed by the
// multitenancy-manager (i.e. project namespaces). Without it, with
// failurePolicy: Fail, an unavailable controller would block create/update of
// the configured resources in every namespace — including system ones — which
// can deadlock the cluster. The in-handler IsSystem check remains as
// defense-in-depth.
//
// Auto-wrapped (managed-by-namespace) projects are EXCLUDED: they are plain orphan
// namespaces wrapped only for accounting and must behave like ordinary namespaces
// (allowNamespacesWithoutProjects, card-16). Enforcing the grant allow-list there
// breaks the legacy access-level RoleBindings (d8:user-authz:*) that the user-authz
// Helm release emits for CARs/AuthorizationRules targeting such a namespace: the
// release then fails on this webhook and retries forever, deadlocking the user-authz
// module ('main' queue). This mirrors the exclusion the d8-multitenancy-manager
// ValidatingAdmissionPolicy already applies to managed-by-namespace namespaces.
var projectNamespaceSelector = &v1.LabelSelector{
	MatchLabels: map[string]string{"heritage": "multitenancy-manager"},
	MatchExpressions: []v1.LabelSelectorRequirement{
		{
			Key:      "multitenancy.deckhouse.io/project-managed-by-namespace",
			Operator: v1.LabelSelectorOpDoesNotExist,
		},
	},
}

// systemWriterMatchConditions make the apiserver SKIP this webhook entirely for system / module
// writers — evaluated locally (CEL), BEFORE any network call to the webhook backend. This is the
// anti-deadlock guarantee: the grant allow-list exists to police PROJECT USERS, but every module's
// resources land in project namespaces via that module's Helm release applied by the
// deckhouse-controller (system:serviceaccount:d8-system:deckhouse). With failurePolicy: Fail, if the
// webhook is denied OR merely unreachable/slow, that server-side apply fails and addon-operator
// retries it forever, locking the module's queue (observed: user-authz emitting
// "RoleBinding/...:d8:user-authz:*:user" into every project namespace -> "webhook retry timed out
// after 2m0s"). Excluding system writers at the apiserver level removes the lock unconditionally —
// it holds even when the webhook backend is completely down, because the apiserver never calls it for
// these requests. Project users are still policed (and get a fast, terminal denial). The in-handler
// isSystemRequest bypass mirrors this as defense-in-depth.
var systemWriterMatchConditions = []admissionregistrationv1.MatchCondition{
	{
		Name:       "exclude-apiserver",
		Expression: `request.userInfo.username != "system:apiserver"`,
	},
	{
		Name:       "exclude-deckhouse-controller",
		Expression: `request.userInfo.username != "system:serviceaccount:d8-system:deckhouse"`,
	},
	{
		Name:       "exclude-multitenancy-manager",
		Expression: `request.userInfo.username != "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"`,
	},
	{
		Name:       "exclude-system-serviceaccounts",
		Expression: `!request.userInfo.groups.exists(g, g == "system:serviceaccounts:d8-system" || g == "system:serviceaccounts:kube-system")`,
	},
	{
		Name:       "exclude-cluster-admins-and-nodes",
		Expression: `!request.userInfo.groups.exists(g, g == "system:masters" || g == "system:nodes")`,
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
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
		caBundle := input.Values.Get("multitenancyManager.internal.admissionWebhookCert.ca").String()
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
							Name:      "multitenancy-manager",
							Namespace: "d8-multitenancy-manager",
							Path:      ptr.To("/is-granted"),
							Port:      ptr.To(int32(9443)),
						},
						CABundle: []byte(caBundle),
					},
					NamespaceSelector:       projectNamespaceSelector,
					MatchConditions:         systemWriterMatchConditions,
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
	// Reconcile selector/match-conditions/timeout on existing configurations too (e.g. upgrades), so a
	// cluster that already has the webhook without the system-writer exclusion is healed in place.
	whConfig.Webhooks[0].NamespaceSelector = projectNamespaceSelector
	whConfig.Webhooks[0].MatchConditions = systemWriterMatchConditions
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
