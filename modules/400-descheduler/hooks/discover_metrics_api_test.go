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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	apiServiceMetricsV1Beta1 = `
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta1.metrics.k8s.io
spec:
  group: metrics.k8s.io
  version: v1beta1
`
	apiServiceCustomMetrics = `
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  name: v1beta1.custom.metrics.k8s.io
spec:
  group: custom.metrics.k8s.io
  version: v1beta1
`
)

var _ = Describe("Modules :: descheduler :: hooks :: discover_metrics_api ::", func() {
	f := HookExecutionConfigInit(`{"descheduler":{"internal":{}}}`, ``)

	Context("Cluster without metrics.k8s.io APIService", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should set isMetricsServerEnabled to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.isMetricsServerEnabled").Bool()).To(BeFalse())
		})
	})

	Context("Cluster with custom.metrics.k8s.io only", func() {
		BeforeEach(func() {
			f.KubeStateSet(apiServiceCustomMetrics)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should set isMetricsServerEnabled to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.isMetricsServerEnabled").Bool()).To(BeFalse())
		})
	})

	Context("Cluster with metrics.k8s.io APIService", func() {
		BeforeEach(func() {
			f.KubeStateSet(apiServiceMetricsV1Beta1)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should set isMetricsServerEnabled to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("descheduler.internal.isMetricsServerEnabled").Bool()).To(BeTrue())
		})
	})
})
