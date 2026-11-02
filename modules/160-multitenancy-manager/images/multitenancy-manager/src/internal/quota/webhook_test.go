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
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// renderedWebhook is the webhook of the ledger as Helm renders it, with the fields the API server
// defaults filled in.
func renderedWebhook() admissionregistrationv1.ValidatingWebhook {
	return admissionregistrationv1.ValidatingWebhook{
		Name: LedgerWebhookEntryName,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{
			Name: LedgerServiceName, Namespace: ConfigNamespace, Path: ptr.To(LedgerWebhookPath), Port: ptr.To[int32](443)}},
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
			Rule: admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"resourcequotas/status"},
				Scope: ptr.To(admissionregistrationv1.NamespacedScope)},
		}},
		NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: NamespaceLabel, Operator: metav1.LabelSelectorOpIn, Values: []string{LabelPending, LabelEnforced, LabelCanary}}}},
		ObjectSelector:  &metav1.LabelSelector{},
		MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "the-quota-plugin-writes-the-project-quota-or-the-canary", Expression: LedgerWebhookMatchCondition}},
		FailurePolicy:   ptr.To(admissionregistrationv1.Fail),
		MatchPolicy:     ptr.To(admissionregistrationv1.Equivalent),
		SideEffects:     ptr.To(admissionregistrationv1.SideEffectClassNone),
		TimeoutSeconds:  ptr.To[int32](1),
	}
}

func TestWebhookDrift(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*admissionregistrationv1.ValidatingWebhook)
		drift string
	}{
		{name: "as rendered and defaulted", edit: func(*admissionregistrationv1.ValidatingWebhook) {}},
		{name: "failure policy Ignore", drift: "failurePolicy", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.FailurePolicy = ptr.To(admissionregistrationv1.Ignore)
		}},
		{name: "a rule of another subresource", drift: "rules", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.Rules[0].Resources = []string{"resourcequotas"}
		}},
		{name: "a rule for creates only", drift: "rules", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.Rules[0].Operations = []admissionregistrationv1.OperationType{admissionregistrationv1.Create}
		}},
		{name: "an object selector", drift: "objectSelector", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.ObjectSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"x": "y"}}
		}},
		{name: "the canary namespace only", drift: "namespaceSelector", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.NamespaceSelector.MatchExpressions[0].Values = []string{LabelCanary}
		}},
		{name: "another label value", drift: "namespaceSelector", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.NamespaceSelector.MatchExpressions[0].Values = []string{"enforced-x", LabelCanary}
		}},
		{name: "a match condition for the canary only", drift: "matchConditions", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.MatchConditions[0].Expression = `request.name == "quota-ledger-canary"`
		}},
		{name: "another service", drift: "clientConfig.service", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.ClientConfig.Service.Name = "elsewhere"
		}},
		{name: "a URL", drift: "clientConfig.service", edit: func(w *admissionregistrationv1.ValidatingWebhook) {
			w.ClientConfig.Service, w.ClientConfig.URL = nil, ptr.To("https://example.com")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhook := renderedWebhook()
			tt.edit(&webhook)
			assert.Equal(t, tt.drift, webhookDrift(&webhook))
		})
	}
}

// The webhook the module renders is the one the drift check expects.
func TestTheRenderedWebhookIsTheExpectedOne(t *testing.T) {
	template, err := os.ReadFile("../../../../../templates/project-quota/ledger.yaml")
	require.NoError(t, err)
	text := string(template)
	for _, want := range []string{
		"- name: " + LedgerWebhookEntryName,
		"expression: '" + LedgerWebhookMatchCondition + "'",
		`values: ["pending", "enforced", "canary"]`,
		`resources: ["resourcequotas/status"]`,
		"failurePolicy: Fail",
		"path: " + LedgerWebhookPath,
	} {
		assert.True(t, strings.Contains(text, want), "ledger.yaml has %q", want)
	}
}

func TestMetricMode(t *testing.T) {
	managed := Config{ControlPlaneManaged: true}
	assert.Equal(t, "Enforce", managed.MetricMode())
	assert.Equal(t, "NotSupported", Config{}.MetricMode())
	assert.Equal(t, "Suspended", Config{ControlPlaneManaged: true, Suspended: true}.MetricMode())
	assert.Equal(t, "WebhookMissing", Config{ControlPlaneManaged: true, WebhookMissing: true}.MetricMode())
	assert.Equal(t, "WebhookMisconfigured", Config{ControlPlaneManaged: true, WebhookMisconfigured: "rules"}.MetricMode())
	assert.Equal(t, "Enforce", Config{ControlPlaneManaged: true, WebhookNotCalledBy: []string{"a"}}.MetricMode(),
		"one API server not calling the webhook keeps the ledger alerts of Enforce")
}
