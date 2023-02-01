/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
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
	return internal.TemplateToYAML(podIstiodTemplate, podParams)
}

var _ = Describe("Istio hooks :: discovery istiod health ::", func() {
	f := HookExecutionConfigInit(`
{"istio":
  {"internal":
    { "versionMap":
     {
        "1.33": {
          "revision": "v1x33"
       },
       "1.88": {
          "revision": "v1x88"
        }
      }
    }
  }
}`, "")

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
			Expect(f.ValuesGet(versionMapPath).String()).To(MatchJSON(`{"1.33":{"fullVersion":"","revision":"v1x33","imageSuffix":"","isReady":false},"1.88":{"fullVersion":"","revision":"v1x88","imageSuffix":"","isReady":false}}`))
		})
	})

	Context("Istiod pods in `Running` phase", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalVersion", "1.88")
			f.BindingContexts.Set(f.KubeStateSet(podIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Running",
			})))
			f.RunHook()
		})
		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(isGlobalVersionIstiodReadyPath).Bool()).To(BeTrue())
			Expect(f.ValuesGet(versionMapPath).String()).To(MatchJSON(`{"1.33":{"fullVersion":"","revision":"v1x33","imageSuffix":"","isReady":false},"1.88":{"fullVersion":"","revision":"v1x88","imageSuffix":"","isReady":true}}`))
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
			Expect(f.ValuesGet(versionMapPath).String()).To(MatchJSON(`{"1.33":{"fullVersion":"","revision":"v1x33","imageSuffix":"","isReady":true},"1.88":{"fullVersion":"","revision":"v1x88","imageSuffix":"","isReady":true}}`))
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
