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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/webhooks/validationwebhook"
)

const fencingModeWebhookName = "node-manager-fencing-mode"

var _ = Describe("Module :: node-manager :: helm template :: the webhook of the fencing mode", func() {
	f := SetupHelmConfig(``)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	render := func(edition string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.deckhouseEdition", edition)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		f.ValuesSetFromYaml("nodeManager.internal.fencing", `nodeGroups: {master: {mode: Disabled}, worker: {mode: Disabled}}`)
		f.ApplyOpenAPIDefaults()
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	webhook := func() object_store.KubeObject {
		object := f.KubernetesGlobalResource("ValidationWebhook", fencingModeWebhookName)
		Expect(object.Exists()).To(BeTrue())
		return object
	}

	Context("a commercial edition", func() {
		BeforeEach(func() { render("EE") })

		It("runs the hook of webhooks/validating", func() {
			Expect(validationwebhook.HandlerMatchesFiles(moduleDir, webhook().Field("handler.python").String(),
				"webhooks/validating/fencing_mode.py")).To(Succeed())
		})

		It("renders only the fields the CRD and the operator template accept", func() {
			undeclared, err := validationwebhook.UndeclaredFields(webhook())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(undeclared).To(BeEmpty())
			Expect(validationwebhook.LiteralHazards(webhook())).To(BeEmpty())
		})

		It("reviews the updates of NodeGroups and fails closed", func() {
			Expect(webhook().Field("validationObject.rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"],
				"apiVersions": ["v1"],
				"operations": ["UPDATE"],
				"resources": ["nodegroups"],
				"scope": "Cluster"
			}]`))
			Expect(webhook().Field("validationObject.failurePolicy").String()).To(Equal("Fail"))
		})

		// The match conditions are evaluated by the API server: a request they do not match never
		// waits for the webhook-handler.
		DescribeTable("sends a request to the hook only when the value of the mode changes",
			func(oldMode, mode string, want bool) {
				var conditions []admissionregistrationv1.MatchCondition
				Expect(yaml.Unmarshal([]byte(webhook().Field("validationObject.matchConditions").String()), &conditions)).To(Succeed())
				policy := admissionregistrationv1.ValidatingAdmissionPolicy{
					Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
						MatchConditions: conditions,
						Validations:     []admissionregistrationv1.Validation{{Expression: "false"}},
					},
				}
				rendered, err := yaml.Marshal(policy)
				Expect(err).ShouldNot(HaveOccurred())
				gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
				matched := compilePolicy(string(rendered), gvk, "nodegroups").
					matches(&user.DefaultInfo{Name: "admin@example.com"}, admission.Update, testNodeGroup(oldMode), testNodeGroup(mode))
				Expect(matched).To(Equal(want))
			},
			Entry("Normal to Slow", "Normal", "Slow", true),
			Entry("no fencing to Disabled", "", "Disabled", true),
			Entry("Watchdog to Slow", "Watchdog", "Slow", true),
			Entry("Normal to no fencing", "Normal", "", true),
			Entry("the same mode", "Normal", "Normal", false),
			Entry("no fencing on both sides", "", "", false),
		)
	})

	It("sets Community Edition in the hook, where the Critical mode works as Normal", func() {
		render("CE")
		handler := strings.TrimRight(webhook().Field("handler.python").String(), "\n")
		code, last, found := strings.Cut(handler, "\nCOMMUNITY_EDITION = True")
		Expect(found).To(BeTrue())
		Expect(last).To(BeEmpty(), "the line goes after the code of the hook and sets its constant")
		Expect(validationwebhook.HandlerMatchesFiles(moduleDir, code, "webhooks/validating/fencing_mode.py")).To(Succeed())
	})
})
