/*
Copyright 2023 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const podIstiodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: istiod
    istio.io/rev: {{ .Revision }}
  name: istiod-{{ .Revision }}-some-pod-hash
  namespace: d8-istio
spec: {}
status:
  phase: {{ .Phase }}
`

const istioSidecarInjectorGlobalWebhook = `
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: d8-istio-sidecar-injector-global
  labels:
    module: istio
    istio.deckhouse.io/full-version: 1.88.55
webhooks: []
`

const validationWebHook = `
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: d8-istio-validator-global
webhooks: []
`

type PodIstiodTemplateParams struct {
	Revision string
	Phase    string
}

func podIstiodYaml(podParams PodIstiodTemplateParams) string {
	return lib.TemplateToYAML(podIstiodTemplate, podParams)
}

var _ = Describe("Istio hooks :: discovery istiod health ::", func() {
	f := HookExecutionConfigInit(`
{"istio":
  {"internal":
    { "versionMap":
     {
        "1.33": {
          "revision": "v1x33",
          "fullVersion": "1.13.55"
       },
       "1.88": {
          "revision": "v1x88",
          "fullVersion": "1.88.55"
        }
      }
    }
  }
}`, "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Empty cluster and minimal settings", func() {
		BeforeEach(func() {
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Without istiod pods", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.88")
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
		})
	})

	Context("Without istiod pods but webhook exists", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.88")
			f.BindingContexts.Set(f.KubeStateSet(validationWebHook))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-istio-validator-global").Exists()).To(BeFalse())
		})
	})

	Context("Istiod pods in `Failed` phase", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.88")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Failed",
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
			Expect(f.ValuesGet(versionMapPath).String()).To(MatchJSON(`{"1.33":{"fullVersion":"1.13.55","revision":"v1x33","imageSuffix":"","isReady":false},"1.88":{"fullVersion":"1.88.55","revision":"v1x88","imageSuffix":"","isReady":false}}`))
		})
	})

	Context("Istiod pods in `Running` phase and injector webhook with actual full version", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.88")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Running",
			}) + "---" + istioSidecarInjectorGlobalWebhook))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			versionMap := f.ValuesGet(versionMapPath).Map()
			Expect(versionMap["1.33"]).To(MatchJSON(`{"fullVersion": "1.13.55","revision": "v1x33","imageSuffix": "","isReady": false}`))
			Expect(versionMap["1.88"]).To(MatchJSON(`{"fullVersion": "1.88.55","revision": "v1x88","imageSuffix": "","isReady": true}`))
		})
	})

	Context("Istiod pods in `Running` phase and injector webhook with old full version", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.33")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x33",
				Phase:    "Running",
			}) + "---" + istioSidecarInjectorGlobalWebhook))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			versionMap := f.ValuesGet(versionMapPath).Map()
			Expect(versionMap["1.33"]).To(MatchJSON(`{"fullVersion": "1.13.55","revision": "v1x33","imageSuffix": "","isReady": false}`))
			Expect(versionMap["1.88"]).To(MatchJSON(`{"fullVersion": "1.88.55","revision": "v1x88","imageSuffix": "","isReady": false}`))
		})
	})

	Context("Both istiod pods with different revisions in `Running` phase", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.88")
			f.BindingContexts.Set(f.KubeStateSet(
				podIstiodYaml(PodIstiodTemplateParams{
					Revision: "v1x88",
					Phase:    "Running",
				}) + "---" +
					podIstiodYaml(PodIstiodTemplateParams{
						Revision: "v1x33",
						Phase:    "Running",
					})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			versionMap := f.ValuesGet(versionMapPath).Map()
			Expect(versionMap["1.33"]).To(MatchJSON(`{"fullVersion": "1.13.55","revision": "v1x33","imageSuffix": "","isReady": true}`))
			Expect(versionMap["1.88"]).To(MatchJSON(`{"fullVersion": "1.88.55","revision": "v1x88","imageSuffix": "","isReady": false}`))
		})
	})

	Context("Istiod pods with `Running` phase and validation webhook exists", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.88")
			f.BindingContexts.Set(f.KubeStateSet(validationWebHook + podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Running",
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-istio-validator-global").Exists()).To(BeTrue())
		})
	})

	Context("Istiod pods with `Running` phase but with different revision", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.33")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Running",
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeFalse())
		})
	})

})
