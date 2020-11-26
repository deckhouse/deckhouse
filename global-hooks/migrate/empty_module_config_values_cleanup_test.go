package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: migrate/domain ::", func() {
	const (
		deckhouseCM = `
apiVersion: v1
data:
  dashboard: |
    {}
  deckhouse: |
    releaseChannel: Alpha
  testEnabled: "true"
  test: |
    test: true
kind: ConfigMap
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
  namespace: d8-system
`
	)

	f := HookExecutionConfigInit("{}", "{}")

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with deckhouse ConfigMap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(deckhouseCM))
			f.RunHook()
		})

		It("Hook should filter empty config values sections", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("ConfigMap", "d8-system", "deckhouse").ToYaml()).To(MatchYAML(`
apiVersion: v1
data:
  deckhouse: |
    releaseChannel: Alpha
  testEnabled: "true"
  test: |
    test: true
kind: ConfigMap
metadata:
  labels:
    heritage: deckhouse
  name: deckhouse
  namespace: d8-system
`))
		})
	})

})
