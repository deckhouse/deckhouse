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

package quota

import (
	"slices"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// LedgerWebhookEntryName is the webhook of the ledger inside its ValidatingWebhookConfiguration.
	LedgerWebhookEntryName = "quota-ledger.multitenancy-manager.deckhouse.io"
	// LedgerWebhookMatchCondition is the expression of the match condition of the webhook of the
	// ledger: the quota plugin writes d8-project-quota, or the canary writes the canary quota.
	LedgerWebhookMatchCondition = `(request.userInfo.username == "system:apiserver" && request.name == "d8-project-quota") || ` +
		`(request.userInfo.username == "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager" && request.name == "quota-ledger-canary")`
	// LabelCanary marks the namespace of the module, where the canary quota is.
	LabelCanary = "canary"

	// LedgerServiceName and LedgerWebhookPath are where the API servers call the ledger.
	LedgerServiceName = "multitenancy-manager-quota-ledger"
	LedgerWebhookPath = "/validate/quota-ledger"
)

// webhookDrift names what of the webhook of the ledger differs from what Helm renders in a way that
// keeps writes of d8-project-quota from reaching the ledger, empty when nothing does. The canary sees
// only the canary namespace, so a selector or a match condition that leaves the quota namespaces out
// keeps it green. The fields the API server defaults are compared only where a value leaves writes
// out, so a defaulted object shows no drift.
func webhookDrift(webhook *admissionregistrationv1.ValidatingWebhook) string {
	if webhook.FailurePolicy != nil && *webhook.FailurePolicy != admissionregistrationv1.Fail {
		return "failurePolicy"
	}
	if !coversQuotaStatus(webhook.Rules) {
		return "rules"
	}
	if webhook.ObjectSelector != nil && (len(webhook.ObjectSelector.MatchLabels) > 0 || len(webhook.ObjectSelector.MatchExpressions) > 0) {
		return "objectSelector"
	}
	selector, err := metav1.LabelSelectorAsSelector(webhook.NamespaceSelector)
	if webhook.NamespaceSelector == nil {
		selector, err = labels.Everything(), nil
	}
	if err != nil {
		return "namespaceSelector"
	}
	for _, value := range []string{LabelPending, LabelEnforced, LabelCanary} {
		if !selector.Matches(labels.Set{NamespaceLabel: value}) {
			return "namespaceSelector"
		}
	}
	if len(webhook.MatchConditions) != 1 || webhook.MatchConditions[0].Expression != LedgerWebhookMatchCondition {
		return "matchConditions"
	}
	service := webhook.ClientConfig.Service
	if service == nil || service.Name != LedgerServiceName || service.Namespace != ConfigNamespace ||
		service.Path == nil || *service.Path != LedgerWebhookPath || (service.Port != nil && *service.Port != 443) {
		return "clientConfig.service"
	}
	return ""
}

// coversQuotaStatus reports whether a rule sends the updates of the status of a namespaced core
// ResourceQuota to the webhook.
func coversQuotaStatus(rules []admissionregistrationv1.RuleWithOperations) bool {
	has := func(values []string, want string) bool {
		return slices.Contains(values, "*") || slices.Contains(values, want)
	}
	for _, rule := range rules {
		operations := make([]string, 0, len(rule.Operations))
		for _, operation := range rule.Operations {
			operations = append(operations, string(operation))
		}
		scopeOK := rule.Scope == nil || *rule.Scope == admissionregistrationv1.NamespacedScope || *rule.Scope == admissionregistrationv1.AllScopes
		if has(operations, string(admissionregistrationv1.Update)) && has(rule.APIGroups, "") && has(rule.APIVersions, "v1") &&
			(has(rule.Resources, "resourcequotas/status") || slices.Contains(rule.Resources, "*/*") || slices.Contains(rule.Resources, "*/status")) && scopeOK {
			return true
		}
	}
	return false
}

// ledgerWebhook returns the webhook of the ledger in its configuration, nil when it is not there.
func ledgerWebhook(configuration *admissionregistrationv1.ValidatingWebhookConfiguration) *admissionregistrationv1.ValidatingWebhook {
	for i := range configuration.Webhooks {
		if configuration.Webhooks[i].Name == LedgerWebhookEntryName {
			return &configuration.Webhooks[i]
		}
	}
	return nil
}
