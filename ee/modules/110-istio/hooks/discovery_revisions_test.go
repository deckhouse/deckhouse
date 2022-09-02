/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: discovery_revisions ::", func() {
	f := HookExecutionConfigInit(`{"istio":{}}`, "")

	Context("Empty cluster and no settings", func() {
		BeforeEach(func() {
			values := `
internal:
  supportedVersions: ["1.1","1.2.3-beta.45"]
globalVersion: 1.2.3-beta.45 # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.LogrusOutput.Contents()).To(HaveLen(0))

			Expect(f.ConfigValuesGet("istio.globalVersion").String()).To(Equal("1.2.3-beta.45"))
			Expect(f.ValuesGet("istio.internal.revisionsToInstall").String()).To(MatchJSON(`["v1x2x3beta45"]`))
			Expect(f.ValuesGet("istio.internal.globalRevision").String()).To(Equal("v1x2x3beta45"))
		})
	})

	Context("No globalVersion in CM and the global service without annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  supportedVersions: ["1.10.1", "1.3", "1.4"]
globalVersion: "1.4" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))

			var service v1.Service
			var err error
			err = yaml.Unmarshal([]byte(`
---
apiVersion: v1
kind: Service
metadata:
  annotations: {}
  name: istiod
  namespace: d8-istio
spec: {}
`), &service)
			Expect(err).To(BeNil())

			_, err = dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services(service.GetNamespace()).
				Create(context.TODO(), &service, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})
		It("Migration for 1.10.1 should trigger", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.revisionsToInstall").AsStringSlice()).To(Equal([]string{"v1x10x1"}))
			Expect(f.ValuesGet("istio.internal.globalRevision").String()).To(Equal("v1x10x1"))
			Expect(f.ConfigValuesGet("istio.globalVersion").String()).To(Equal("1.10.1"))
		})
	})

	Context("No globalVersion in CM and the global service with annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  supportedVersions: ["1.10.1", "1.3", "1.4"]
globalVersion: "1.4" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))

			var service v1.Service
			var err error
			err = yaml.Unmarshal([]byte(`
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    istio.deckhouse.io/global-version: "1.3"
  name: istiod
  namespace: d8-istio
spec: {}
`), &service)
			Expect(err).To(BeNil())

			_, err = dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services(service.GetNamespace()).
				Create(context.TODO(), &service, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})
		It("globalVersion should be gathered from the Service", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.revisionsToInstall").AsStringSlice()).To(Equal([]string{"v1x3"}))
			Expect(f.ValuesGet("istio.internal.globalRevision").String()).To(Equal("v1x3"))
			Expect(f.ConfigValuesGet("istio.globalVersion").String()).To(Equal("1.3"))
		})
	})

	Context("globalVersion in CM and the global service with annotation", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  supportedVersions: ["1.10.1", "1.2", "1.3", "1.4"]
globalVersion: "1.4" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ConfigValuesSet("istio.globalVersion", "1.2")

			var service v1.Service
			var err error
			err = yaml.Unmarshal([]byte(`
---
apiVersion: v1
kind: Service
metadata:
  annotations:
    istio.deckhouse.io/global-version: "1.3"
  name: istiod
  namespace: d8-istio
spec: {}
`), &service)
			Expect(err).To(BeNil())

			_, err = dependency.TestDC.MustGetK8sClient().
				CoreV1().
				Services(service.GetNamespace()).
				Create(context.TODO(), &service, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			f.RunHook()
		})
		It("globalVersion should be gathered from CM", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.revisionsToInstall").AsStringSlice()).To(Equal([]string{"v1x2"}))
			Expect(f.ValuesGet("istio.internal.globalRevision").String()).To(Equal("v1x2"))
			Expect(f.ConfigValuesGet("istio.globalVersion").String()).To(Equal("1.2"))
		})
	})

	Context("Unsupported versions", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  supportedVersions: ["1.1.0","1.2.3-beta.45","1.3.1"]
globalVersion: "1.3.1" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.ConfigValuesSet("istio.globalVersion", "1.0")
			f.ConfigValuesSet("istio.additionalVersions", []string{"1.7.4", "1.1.0", "1.8.0-alpha.2", "1.3.1", "1.9.0"})
			f.RunHook()
		})
		It("Should return errors", func() {
			Expect(f).ToNot(ExecuteSuccessfully())

			Expect(f.GoHookError).To(MatchError("unsupported versions: [1.0,1.7.4,1.8.0-alpha.2,1.9.0]"))
		})
	})

	Context("There are some deprecated versions exists", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  deprecatedVersions:
  - version: 1.1.0
    severity: 4
  - version: 0.0.2
    severity: 9
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.RunHook()
		})
		It("deprecatedRevisions param should be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(len(f.ValuesGet("istio.internal.deprecatedRevisions").Array())).Should(Equal(2))
			Expect(f.ValuesGet("istio.internal.deprecatedRevisions").String()).Should(Equal(`[{"revision":"v1x1x0","severity":4},{"revision":"v0x0x2","severity":9}]`))
		})
	})

	Context("There are no deprecated versions", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  deprecatedVersions: []
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.RunHook()
		})
		It("deprecatedRevisions param should be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(len(f.ValuesGet("istio.internal.deprecatedRevisions").Array())).Should(Equal(0))
		})
	})
})
