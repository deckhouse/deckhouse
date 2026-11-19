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

package template_tests

import (
	"github.com/google/cel-go/cel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// callsWebhookFor evaluates a match condition of a webhook for a request of the user the way the API
// server does; the webhook is called when it is true.
func callsWebhookFor(expression, username string, groups ...string) bool {
	env, err := cel.NewEnv(cel.Variable("request", cel.DynType))
	Expect(err).ShouldNot(HaveOccurred())
	ast, issues := env.Compile(expression)
	Expect(issues.Err()).ShouldNot(HaveOccurred(), expression)
	program, err := env.Program(ast)
	Expect(err).ShouldNot(HaveOccurred())
	userGroups := make([]any, 0, len(groups))
	for _, group := range groups {
		userGroups = append(userGroups, group)
	}
	out, _, err := program.Eval(map[string]any{
		"request": map[string]any{"userInfo": map[string]any{"username": username, "groups": userGroups}},
	})
	Expect(err).ShouldNot(HaveOccurred(), expression)
	return out.Value() == true
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: project authors ::", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderQuota(hec, true)
	})

	It("records the authors of a project through a mutating webhook that people go through and the platform does not", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())
		webhooks := hec.KubernetesGlobalResource("MutatingWebhookConfiguration", "d8-multitenancy-manager-webhook").Field("webhooks").Array()
		Expect(webhooks).To(HaveLen(1))
		webhook := webhooks[0]

		Expect(webhook.Get("name").String()).To(Equal("project-authors.multitenancy-webhook.deckhouse.io"))
		Expect(webhook.Get("rules.0.apiGroups").String()).To(MatchJSON(`["deckhouse.io"]`))
		Expect(webhook.Get("rules.0.apiVersions").String()).To(MatchJSON(`["v1alpha3"]`))
		Expect(webhook.Get("rules.0.resources").String()).To(MatchJSON(`["projects"]`), "the status is written by the controller alone")
		Expect(webhook.Get("rules.0.operations").String()).To(MatchJSON(`["CREATE","UPDATE"]`))
		Expect(webhook.Get("matchPolicy").String()).To(Equal("Equivalent"), "a write through v1alpha2 is converted to the rule version")
		Expect(webhook.Get("failurePolicy").String()).To(Equal("Ignore"), "a write by system:sudouser must pass while the backend is down")
		Expect(webhook.Get("reinvocationPolicy").String()).To(Equal("IfNeeded"))
		Expect(webhook.Get("sideEffects").String()).To(Equal("None"))
		Expect(webhook.Get("clientConfig.service.path").String()).To(Equal("/mutate/v1alpha3/projects/authors"))

		Expect(webhook.Get("matchConditions").Array()).To(HaveLen(1))
		expression := webhook.Get("matchConditions.0.expression").String()
		Expect(callsWebhookFor(expression, "alice@example.com", "developers")).To(BeTrue())
		Expect(callsWebhookFor(expression, "admin", "system:masters")).To(BeTrue(), "cluster administrators are recorded too")
		Expect(callsWebhookFor(expression, "system:sudouser")).To(BeTrue(), "a change made as system:sudouser is recorded")
		Expect(callsWebhookFor(expression, "system:serviceaccount:argocd:argocd-application-controller",
			"system:serviceaccounts", "system:serviceaccounts:argocd")).To(BeTrue(), "a GitOps tool is recorded")

		Expect(callsWebhookFor(expression, "system:apiserver")).To(BeFalse())
		Expect(callsWebhookFor(expression, "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager")).To(BeFalse())
		Expect(callsWebhookFor(expression, "system:serviceaccount:d8-system:deckhouse")).To(BeFalse())
		Expect(callsWebhookFor(expression, "system:serviceaccount:d8-user-authz:controller")).To(BeFalse())
		Expect(callsWebhookFor(expression, "system:serviceaccount:d8-system:other",
			"system:serviceaccounts", "system:serviceaccounts:d8-system")).To(BeFalse())
		Expect(callsWebhookFor(expression, "system:serviceaccount:kube-system:generic-garbage-collector",
			"system:serviceaccounts", "system:serviceaccounts:kube-system")).To(BeFalse())
		Expect(callsWebhookFor(expression, "system:node:worker-0", "system:nodes")).To(BeFalse())
	})
})
