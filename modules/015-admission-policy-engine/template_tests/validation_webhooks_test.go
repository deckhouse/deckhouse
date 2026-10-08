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
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/webhooks/validationwebhook"
)

// The ValidationWebhooks of the module and the files of webhooks/validating their handler is made of.
var admissionPolicyEngineValidationWebhooks = map[string][]string{
	"admission-policy-engine-reserved-policy-names": {"webhooks/validating/reserved_policy_names.py"},
	"admission-policy-engine-security-policy":       {"webhooks/validating/security_policy.py"},
}

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: validation webhooks", func() {
	f := SetupHelmConfig(`
admissionPolicyEngine:
  internal:
    ratify:
      webhook:
        ca: test-ca-placeholder
        crt: test-crt-placeholder
        key: test-key-placeholder
    podSecurityStandards:
      enforcementActions:
        - deny
    bootstrapped: true
    webhook:
      ca: test-ca-placeholder
      crt: test-crt-placeholder
      key: test-key-placeholder
    trackedConstraintResources: []
    trackedMutateResources: []
  podSecurityStandards:
    defaultPolicy: Baseline
    enforcementAction: Deny
`)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	resource := func(name string) object_store.KubeObject {
		object := f.KubernetesGlobalResource("ValidationWebhook", name)
		Expect(object.Exists()).To(BeTrue(), name)
		return object
	}

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
	})

	It("Should check every ValidationWebhook the module declares", func() {
		declared, err := validationwebhook.ManifestNames(moduleDir)
		Expect(err).ShouldNot(HaveOccurred())
		checked := make([]string, 0, len(admissionPolicyEngineValidationWebhooks))
		for name := range admissionPolicyEngineValidationWebhooks {
			checked = append(checked, name)
		}
		Expect(checked).To(ConsistOf(declared))
	})

	It("Should render the handlers from webhooks/validating", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name, files := range admissionPolicyEngineValidationWebhooks {
			handler := resource(name).Field("handler.python").String()
			Expect(validationwebhook.HandlerMatchesFiles(moduleDir, handler, files...)).To(Succeed(), name)
		}
	})

	It("Should render only the fields the CRD and the operator template accept", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name := range admissionPolicyEngineValidationWebhooks {
			object := resource(name)
			undeclared, err := validationwebhook.UndeclaredFields(object)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(undeclared).To(BeEmpty(), name)
			Expect(validationwebhook.LiteralHazards(object)).To(BeEmpty(), name)
		}
	})

	It("Should keep the wrapper of the python test helper in sync with the operator template", func() {
		Expect(validationwebhook.TestHelperMatchesTemplate(moduleDir)).To(Succeed())
	})

	Context("security-policy", func() {
		policy := func(name string, references ...string) map[string]interface{} {
			signatures := make([]interface{}, 0, len(references))
			for _, reference := range references {
				signatures = append(signatures, map[string]interface{}{"reference": reference, "publicKeys": []interface{}{"test-public-key"}})
			}
			policies := map[string]interface{}{}
			if len(signatures) > 0 {
				policies["verifyImageSignatures"] = signatures
			}
			return map[string]interface{}{"metadata": map[string]interface{}{"name": name}, "spec": map[string]interface{}{"policies": policies}}
		}

		filter := func() string {
			for _, item := range resource("admission-policy-engine-security-policy").Field("context").Array() {
				if item.Get("name").String() == "policies" {
					return item.Get("kubernetes.jqFilter").String()
				}
			}
			Fail("no policies binding")
			return ""
		}

		It("Should collect the image references of every policy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			out, err := validationwebhook.RunJQ(filter(), policy("p", "registry.example.com/a/*", "registry.example.com/b/*"))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(out).To(Equal(map[string]interface{}{
				"name":       "p",
				"references": []interface{}{"registry.example.com/a/*", "registry.example.com/b/*"},
			}))

			out, err = validationwebhook.RunJQ(filter(), policy("p"))
			Expect(err).ShouldNot(HaveOccurred())
			Expect(out).To(Equal(map[string]interface{}{"name": "p", "references": []interface{}{}}))
		})
	})
})
