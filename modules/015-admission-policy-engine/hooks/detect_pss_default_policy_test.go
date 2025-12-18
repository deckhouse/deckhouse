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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	pssRestrictedPolicy = "Restricted"
	pssPrivilegedPolicy = "Privileged"
	pssBaselinePolicy   = "Baseline"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: detect pss default policy", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"podSecurityStandards": {"enforcementAction": "Deny"},"internal": {"bootstrapped": true, "podSecurityStandards": {"enforcementActions": []}}}}`,
		`{"admissionPolicyEngine": {"podSecurityStandards": {}}}`,
	)

	Context("Empty cluster with podSecurityStandards.defaultPolicy preset", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("admissionPolicyEngine.podSecurityStandards.defaultPolicy", pssRestrictedPolicy)
			f.RunHook()
		})
		It("should have the same default policy", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(pssRestrictedPolicy))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_policy",
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(3.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster without install-data configmap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should have the default policy set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(pssPrivilegedPolicy))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_policy",
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with install-data configmap without version field", func() {
		BeforeEach(func() {
			f.KubeStateSet(noFieldConfigMap)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should have the default policy set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(pssPrivilegedPolicy))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_policy",
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with install-data configmap with incorrect semver in version field", func() {
		BeforeEach(func() {
			f.KubeStateSet(wrongSemverConfigMap)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should have the default policy set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(pssPrivilegedPolicy))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_policy",
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with install-data configmap with v1.54 version field", func() {
		BeforeEach(func() {
			f.KubeStateSet(v154ConfigMap)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should have the default policy set to Privileged", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(pssPrivilegedPolicy))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_policy",
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(1.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with install-data configmap with v1.55 version field", func() {
		BeforeEach(func() {
			f.KubeStateSet(v155ConfigMap)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should have the default policy set to Baseline", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(pssBaselinePolicy))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_policy",
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(2.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with install-data configmap with v1.56 version field", func() {
		BeforeEach(func() {
			f.KubeStateSet(v156ConfigMap)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("should have the default policy set to Baseline", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(pssBaselinePolicy))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_policy",
				Group:  "d8_admission_policy_engine_pss_default_policy",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(2.0),
				Labels: map[string]string{},
			}))
		})
	})
})

var noFieldConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  not-version: someversion
`

var wrongSemverConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "1.55"
`

var v154ConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "v1.54.1"
`

var v155ConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "v1.55.1"
`

var v156ConfigMap = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
data:
  version: "v1.56.56"
`
