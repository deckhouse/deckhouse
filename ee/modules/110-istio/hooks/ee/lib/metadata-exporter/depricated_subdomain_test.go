/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package metadataExporter

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func allianceDepricatedSubdomainBaseValues() string {
	return `{
  "global":{
    "discovery":{"clusterUUID":"c1","clusterDomain": "my.cluster"},
    "modules":{"publicDomainTemplate": "%s.example.com"}
  },
  "istio":{
    "federation":{"enabled":false},
    "multicluster":{"enabled":false},
    "internal":{}
  }
}`
}

var _ = Describe("Istio EE hooks :: alliance_metadata_endpoint_depricated_subdomain :: alliance disabled", func() {
	f := HookExecutionConfigInit(allianceDepricatedSubdomainBaseValues(), "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

	BeforeEach(func() {
		f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: mc-depricated
spec:
  metadataEndpoint: "https://istio.example.com/metadata/"
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: fed-depricated
spec:
  trustDomain: remote.test
  metadataEndpoint: "https://istio.example.com/metadata/"
`))
		f.RunHook()
	})

	It("expires metrics without emitting depricated subdomain metric", func() {
		Expect(f).To(ExecuteSuccessfully())
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(1))
		Expect(m[0].Action).To(Equal(operation.ActionExpireMetrics))
	})
})

var _ = Describe("Istio EE hooks :: alliance_metadata_endpoint_depricated_subdomain :: multicluster depricated subdomain", func() {
	f := HookExecutionConfigInit(allianceDepricatedSubdomainBaseValues(), "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

	BeforeEach(func() {
		f.ValuesSet("istio.multicluster.enabled", true)
		f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: mc-depricated
spec:
  metadataEndpoint: "https://istio.example.com/metadata/"
`))
		f.RunHook()
	})

	It("sets depricated subdomain metric", func() {
		Expect(f).To(ExecuteSuccessfully())
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(2))
		Expect(m[0].Action).To(Equal(operation.ActionExpireMetrics))
		Expect(m[1].Name).To(Equal(DepricatedSubdomainMetricName))
		Expect(m[1].Labels).To(Equal(map[string]string{
			"alliance_kind": "IstioMulticluster",
			"name":          "mc-depricated",
		}))
		Expect(m[1].Value).To(Equal(ptr.To(1.0)))
	})
})

var _ = Describe("Istio EE hooks :: alliance_metadata_endpoint_depricated_subdomain :: multicluster new host", func() {
	f := HookExecutionConfigInit(allianceDepricatedSubdomainBaseValues(), "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

	BeforeEach(func() {
		f.ValuesSet("istio.multicluster.enabled", true)
		f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: mc-new
spec:
  metadataEndpoint: "https://istio-metadata.example.com/metadata/"
`))
		f.RunHook()
	})

	It("does not set depricated subdomain metric", func() {
		Expect(f).To(ExecuteSuccessfully())
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(1))
		Expect(m[0].Action).To(Equal(operation.ActionExpireMetrics))
	})
})

var _ = Describe("Istio EE hooks :: alliance_metadata_endpoint_depricated_subdomain :: multicluster depricated subdomain unrelated domain", func() {
	f := HookExecutionConfigInit(allianceDepricatedSubdomainBaseValues(), "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

	BeforeEach(func() {
		f.ValuesSet("istio.multicluster.enabled", true)
		f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: mc-depricated-remote-tld
spec:
  metadataEndpoint: "https://istio.cluster-b.partner.example.net/metadata/"
`))
		f.RunHook()
	})

	It("detects depricated subdomain by first DNS label, not publicDomainTemplate", func() {
		Expect(f).To(ExecuteSuccessfully())
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(2))
		Expect(m[1].Name).To(Equal(DepricatedSubdomainMetricName))
		Expect(m[1].Labels["name"]).To(Equal("mc-depricated-remote-tld"))
	})
})

var _ = Describe("Istio EE hooks :: alliance_metadata_endpoint_depricated_subdomain :: federation depricated subdomain", func() {
	f := HookExecutionConfigInit(allianceDepricatedSubdomainBaseValues(), "")
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioFederation", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "IstioMulticluster", false)

	BeforeEach(func() {
		f.ValuesSet("istio.federation.enabled", true)
		f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: fed-depricated
spec:
  trustDomain: remote.test
  metadataEndpoint: "https://istio.example.com/metadata/"
`))
		f.RunHook()
	})

	It("sets depricated subdomain metric for federation", func() {
		Expect(f).To(ExecuteSuccessfully())
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(2))
		Expect(m[1].Labels).To(Equal(map[string]string{
			"alliance_kind": "IstioFederation",
			"name":          "fed-depricated",
		}))
	})
})
