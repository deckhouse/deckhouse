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

type PodIstiodTemplateParams struct {
	Revision string
	Phase    string
}

func PodIstiodYaml(podParams PodIstiodTemplateParams) string {
	return internal.TemplateToYAML(podIstiodTemplate, podParams)
}

var _ = Describe("Istio hooks :: handle_operator_bootstrap ::", func() {
	f := HookExecutionConfigInit(`{"istio":{"internal":{}}}`, "")

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
			f.ValuesSet("istio.internal.globalRevision", "1x88")
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Bool()).To(BeFalse())
		})
	})

	Context("Istiod pods with `Failed` phase", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalRevision", "v1x88")
			f.BindingContexts.Set(f.KubeStateSet(PodIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Failed",
			})))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Bool()).To(BeFalse())
		})
	})

	Context("Istiod pods with `Running` phase", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalRevision", "v1x88")
			f.BindingContexts.Set(f.KubeStateSet(PodIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Running",
			})))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Bool()).To(BeTrue())
		})
	})

	Context("Istiod pods with `Running` phase but with different revision", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.internal.globalRevision", "v1x33")
			f.BindingContexts.Set(f.KubeStateSet(PodIstiodYaml(PodIstiodTemplateParams{
				Revision: "v1x88",
				Phase:    "Running",
			})))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Exists()).To(BeTrue())
			Expect(f.ValuesGet(globalRevisionIstiodIsReadyPath).Bool()).To(BeFalse())
		})
	})

})
