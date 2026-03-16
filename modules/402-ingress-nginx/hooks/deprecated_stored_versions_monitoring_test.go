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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: deprecated_stored_versions_monitoring ::", func() {
	f := HookExecutionConfigInit("", "")

	Context("CRD is absent", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("expires metric group and does not set metrics", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  ingressNginxControllerStoredVersionsMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Stored versions contain only v1", func() {
		BeforeEach(func() {
			f.KubeStateSet(crdWithCurrentStoredVersion)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("does not set deprecated stored version metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  ingressNginxControllerStoredVersionsMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}))
		})
	})

	Context("Stored versions still contain v1alpha1", func() {
		BeforeEach(func() {
			f.KubeStateSet(crdWithDeprecatedStoredVersion)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("sets deprecated stored version metric", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  ingressNginxControllerStoredVersionsMetricsGroup,
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   ingressNginxControllerDeprecatedStoredVersionsMetric,
				Group:  ingressNginxControllerStoredVersionsMetricsGroup,
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"crd":            ingressNginxControllerCRDName,
					"stored_version": "v1alpha1",
				},
			}))
		})
	})
})

const crdWithCurrentStoredVersion = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ingressnginxcontrollers.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: IngressNginxController
    plural: ingressnginxcontrollers
    singular: ingressnginxcontroller
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
status:
  storedVersions:
    - v1
`

const crdWithDeprecatedStoredVersion = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ingressnginxcontrollers.deckhouse.io
spec:
  group: deckhouse.io
  names:
    kind: IngressNginxController
    plural: ingressnginxcontrollers
    singular: ingressnginxcontroller
  scope: Cluster
  versions:
    - name: v1alpha1
      served: true
      storage: false
    - name: v1
      served: true
      storage: true
status:
  storedVersions:
    - v1alpha1
    - v1
`
