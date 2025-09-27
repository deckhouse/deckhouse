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

var _ = Describe("Modules :: admission-policy-engine :: hooks :: detect pss enforcement actions", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"podSecurityStandards": {"enforcementAction": "Deny"},"internal": {"bootstrapped": true, "podSecurityStandards": {"enforcementActions": []}}}}`,
		`{}`,
	)

	Context("Empty cluster with default enforcement action", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testNamespace))
			f.RunHook()
		})
		It("should have default enforcement action", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").Array()).To(HaveLen(1))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").Array()[0].String()).To(Equal("deny"))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_action",
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(3.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with namespace labeled with default enforcement action", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testNamespaceWithDefaultAction))
			f.RunHook()
		})
		It("should have default enforcement action", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").Array()).To(HaveLen(1))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").Array()[0].String()).To(Equal("deny"))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_action",
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(3.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with namespace labeled with wrong enforcement action", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testNamespaceWithWrongAction))
			f.RunHook()
		})
		It("should have default enforcement action", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").Array()).To(HaveLen(1))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").Array()[0].String()).To(Equal("deny"))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_action",
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(3.0),
				Labels: map[string]string{},
			}))
		})
	})

	Context("Cluster with a bunch of namespaces of all enforcement actions", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(bunchOfNamespaces))
			f.RunHook()
		})
		It("should have all enforcement actions, except for incorrect ones", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").Array()).To(HaveLen(3))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").AsStringSlice()).To(ContainElement("deny"))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").AsStringSlice()).To(ContainElement("dryrun"))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions").AsStringSlice()).To(ContainElement("warn"))
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionExpireMetrics,
			}))
			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_admission_policy_engine_pss_default_action",
				Group:  "d8_admission_policy_engine_pss_default_action",
				Action: operation.ActionGaugeSet,
				Value:  ptr.To(3.0),
				Labels: map[string]string{},
			}))
		})
	})
})

var testNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: foo
`
var testNamespaceWithDefaultAction = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: foo
  labels:
    security.deckhouse.io/pod-policy-action: deny
`
var testNamespaceWithWrongAction = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: foo
  labels:
    security.deckhouse.io/pod-policy-action: permit
`
var bunchOfNamespaces = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: foo
  labels:
    security.deckhouse.io/pod-policy-action: deny
---
apiVersion: v1
kind: Namespace
metadata:
  name: bar
  labels:
    security.deckhouse.io/pod-policy-action: dryrun
---
apiVersion: v1
kind: Namespace
metadata:
  name: foobar
  labels:
    security.deckhouse.io/pod-policy-action: warn
---
apiVersion: v1
kind: Namespace
metadata:
  name: barfoo
  labels:
    security.deckhouse.io/pod-policy-action: permit
---
apiVersion: v1
kind: Namespace
metadata:
  name: foobarfoo
  labels:
    security.deckhouse.io/pod-policy-action: deny
`
