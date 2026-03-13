/*
Copyright 2025 Flant JSC

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

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: cloud_provider_default_storage_class ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{}},"cloudProviderDvp":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must not be set", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("Module has default storage class in internal values", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudProviderDvp.internal.defaultStorageClass", "replicated")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  discovery-data.json: e30=
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must be set to 'replicated'", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("replicated"))
		})
	})

	Context("Module has no default storage class", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.cloudProviderDefaultStorageClass", "old-value")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  discovery-data.json: e30=
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must be removed", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("Drift detection: no drift", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudProviderDvp.internal.defaultStorageClass", "replicated")
			f.ValuesSet("global.discovery.defaultStorageClass", "replicated")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  discovery-data.json: e30=
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must be set", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("replicated"))
		})

		It("Drift metric must be expired", func() {
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Action).To(Equal(operation.ActionExpireMetrics))
			Expect(metrics[0].Group).To(Equal("d8_cloud_provider_dvp_default_storage_class_drifted"))
		})
	})

	Context("Drift detection: drift detected", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudProviderDvp.internal.defaultStorageClass", "replicated")
			f.ValuesSet("global.discovery.defaultStorageClass", "local")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  discovery-data.json: e30=
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must be set to 'replicated'", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("replicated"))
		})

		It("Drift metric must be set to 1", func() {
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Action).To(Equal(operation.ActionGaugeSet))
			Expect(metrics[0].Name).To(Equal("d8_cloud_provider_dvp_default_storage_class_drifted"))
			Expect(*metrics[0].Value).To(BeNumerically("==", 1.0))
			Expect(metrics[0].Labels).To(HaveKeyWithValue("expected", "replicated"))
			Expect(metrics[0].Labels).To(HaveKeyWithValue("actual", "local"))
		})
	})

	Context("Drift detection: no actual default SC yet", func() {
		BeforeEach(func() {
			f.ValuesSet("cloudProviderDvp.internal.defaultStorageClass", "replicated")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  discovery-data.json: e30=
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must be set", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("replicated"))
		})

		It("Drift metric must be expired", func() {
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(1))
			Expect(metrics[0].Action).To(Equal(operation.ActionExpireMetrics))
			Expect(metrics[0].Group).To(Equal("d8_cloud_provider_dvp_default_storage_class_drifted"))
		})
	})
})
