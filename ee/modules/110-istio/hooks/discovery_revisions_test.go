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

			Expect(f.ValuesGet("istio.internal.revisionsToInstall").String()).To(MatchJSON(`["v1x2x3beta45"]`))
			Expect(f.ValuesGet("istio.internal.globalRevision").String()).To(Equal("v1x2x3beta45"))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.2.3-beta.45"))
		})
	})

	Context("No globalVersion in CM and globalVersion was previously discovered", func() {
		BeforeEach(func() {
			f.KubeStateSet("") // to re-init fake api client (reset KubeState)

			values := `
internal:
  supportedVersions: ["1.10.1", "1.3", "1.4"]
  globalVersion: "1.42"
globalVersion: "1.4" # default version "from openapi/values.yaml"
`
			f.ValuesSetFromYaml("istio", []byte(values))
			f.RunHook()
		})
		It("Previously discovered value 1.42 must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.revisionsToInstall").AsStringSlice()).To(Equal([]string{"v1x42"}))
			Expect(f.ValuesGet("istio.internal.globalRevision").String()).To(Equal("v1x42"))
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.42"))
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
		It("Hook must fail with error", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError).To(MatchError("can't find istio.deckhouse.io/global-version annotation for istiod global Service d8-istio/istiod"))
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
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.3"))
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
			Expect(f.ValuesGet("istio.internal.globalVersion").String()).To(Equal("1.2"))
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
})
