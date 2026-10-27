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
	"github.com/tidwall/gjson"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// ruleVerbs returns the verbs the ClusterRole grants on resource in apiGroup, merged across all its rules.
func ruleVerbs(rules gjson.Result, apiGroup, resource string) []string {
	var verbs []string
	for _, rule := range rules.Array() {
		// A rule limited to named objects does not grant the resource as a whole.
		if len(rule.Get("resourceNames").Array()) > 0 {
			continue
		}
		groupMatches := false
		for _, g := range rule.Get("apiGroups").Array() {
			if g.String() == apiGroup || g.String() == "*" {
				groupMatches = true
			}
		}
		if !groupMatches {
			continue
		}
		for _, r := range rule.Get("resources").Array() {
			if r.String() == resource || r.String() == "*" {
				for _, v := range rule.Get("verbs").Array() {
					verbs = append(verbs, v.String())
				}
			}
		}
	}
	return verbs
}

var _ = Describe("Module :: admissionPolicyEngine :: gatekeeper RBAC", func() {
	f := SetupHelmConfig(`
admissionPolicyEngine:
  podSecurityStandards: {}
  internal:
    bootstrapped: true
    ratify:
      webhook:
        ca: test-ca-placeholder
        crt: test-crt-placeholder
        key: test-key-placeholder
    podSecurityStandards:
      enforcementActions:
        - deny
    trackedConstraintResources: []
    trackedMutateResources: []
    webhook:
      ca: test-ca-placeholder
      crt: test-crt-placeholder
      key: test-key-placeholder
`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
	})

	// The mutatorstatus controller (audit pod, --operation=mutation-status) aggregates MutatorPodStatus
	// objects into status.byPod with Status().Update on the mutator, which needs the /status subresource.
	It("must let Gatekeeper update the status of every mutator kind", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		role := f.KubernetesGlobalResource("ClusterRole", "d8:admission-policy-engine:gatekeeper")
		Expect(role.Exists()).To(BeTrue())
		rules := role.Field("rules").Result

		for _, kind := range []string{"assign", "assignimage", "assignmetadata", "modifyset"} {
			Expect(ruleVerbs(rules, "mutations.gatekeeper.sh", kind+"/status")).
				To(ContainElement("update"), "missing %s/status", kind)
		}
		Expect(ruleVerbs(rules, "status.gatekeeper.sh", "mutatorpodstatuses")).
			To(ContainElements("create", "get", "list", "watch", "update", "delete"))
	})

	// The connectionstatus controller (audit pod, --operation=status) writes status.byPod of a Connection
	// with Status().Update.
	It("must let Gatekeeper update the status of connections", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		role := f.KubernetesGlobalResource("ClusterRole", "d8:admission-policy-engine:gatekeeper")
		Expect(role.Exists()).To(BeTrue())
		Expect(ruleVerbs(role.Field("rules").Result, "connection.gatekeeper.sh", "connections/status")).
			To(ContainElement("update"))
	})

	// The externaldatastatus controller (audit pod, --operation=status) writes status.byPod of a Provider
	// with Status().Update, for example of the ratify Provider of image signature verification.
	It("must let Gatekeeper update the status of external data providers", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		role := f.KubernetesGlobalResource("ClusterRole", "d8:admission-policy-engine:gatekeeper")
		Expect(role.Exists()).To(BeTrue())
		Expect(ruleVerbs(role.Field("rules").Result, "externaldata.gatekeeper.sh", "providers/status")).
			To(ContainElement("update"))
	})
})
