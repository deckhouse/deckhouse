/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/benjamintf1/unmarshalledmatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	servicesWithOldLabels = `
---
apiVersion: v1
kind: Service
metadata:
  name: s0
  labels:
    prometheus-target: php-fpm
---
apiVersion: v1
kind: Service
metadata:
  name: s1
  labels:
    prometheus-target: php-fpm
---
apiVersion: v1
kind: Service
metadata:
  name: s2
  labels:
    prometheus-target: winword
`
	servicesWithNewLabels = `
---
apiVersion: v1
kind: Service
metadata:
  name: new
  labels:
    prometheus.deckhouse.io/target: test
---
apiVersion: v1
kind: Service
metadata:
  name: new-nats
  labels:
    prometheus.deckhouse.io/target: nats
`
)

var _ = Describe("Modules :: monitoring-applications :: hooks :: discovery ::", func() {
	f := HookExecutionConfigInit(`{"monitoringApplications":{"internal":{"enabledApplicationsSummary": []}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringApplications.internal.enabledApplicationsSummary").String()).To(MatchJSON(`[]`))
		})
	})

	Context("BeforeHelm — nothing discovered, nothing configured", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("monitoringApplications.internal.enabledApplicationsSummary must be []", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringApplications.internal.enabledApplicationsSummary").String()).To(
				MatchJSON(`[]`))
		})
	})

	Context("Services are in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(servicesWithOldLabels + servicesWithNewLabels))
			f.RunHook()
		})

		It("enabledApplications must contain applications", func() {
			Expect(f).To(ExecuteSuccessfully())
			// null in enabledApplications appears only because fake kubernetes client do not support proper label selection
			Expect(f.ValuesGet("monitoringApplications.internal.enabledApplicationsSummary").String()).To(
				MatchUnorderedJSON(`["php-fpm", "nats"]`))
		})
	})

	Context("BeforeHelm — discovered and configured", func() {
		BeforeEach(func() {
			f.ValuesSet("monitoringApplications.enabledApplications", []string{"nats", "redis"})
			f.KubeStateSet(servicesWithNewLabels)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("monitoringApplications.internal.enabledApplicationsSummary must be unique sum of two lists", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("monitoringApplications.internal.enabledApplicationsSummary").String()).To(
				MatchUnorderedJSON(`["nats","redis"]`))
		})
	})

})
