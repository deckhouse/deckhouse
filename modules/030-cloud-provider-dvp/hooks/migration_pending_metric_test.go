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

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: migration_pending_metric ::", func() {
	const (
		migrationMarkerCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-module-is-migrating
  namespace: d8-cloud-provider-dvp
`
		commanderUUIDCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-commander-uuid
  namespace: kube-system
data:
  commander-uuid: "00000000-0000-0000-0000-000000000000"
`
		commanderInfoSupportedCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: commander-info
  namespace: d8-commander-agent
data:
  "data.json": |
    {"flags": {"cloudProviderNoPCCInputFormatSupported": "1"}}
`
		commanderInfoNotSupportedCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: commander-info
  namespace: d8-commander-agent
data:
  "data.json": |
    {"flags": {"cloudProviderNoPCCInputFormatSupported": "0"}}
`
		commanderInfoNoFlagCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: commander-info
  namespace: d8-commander-agent
data:
  "data.json": |
    {"flags": {"commanderHAEnabled": "0"}}
`
		commanderInfoInvalidCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: commander-info
  namespace: d8-commander-agent
data:
  "data.json": "not-a-json"
`
	)

	f := HookExecutionConfigInit(`{"cloudProviderDvp":{"internal":{}}}`, `{}`)

	assertMetricSet := func(f *HookExecutionConfig) {
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(2))
		Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
			Group:  migrationPendingMetricGroup,
			Action: operation.ActionExpireMetrics,
		}))
		Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
			Name:   migrationPendingMetricName,
			Value:  ptr.To(1.0),
			Action: operation.ActionGaugeSet,
			Group:  migrationPendingMetricGroup,
		}))
	}

	assertMetricNotSet := func(f *HookExecutionConfig) {
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(1))
		Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
			Group:  migrationPendingMetricGroup,
			Action: operation.ActionExpireMetrics,
		}))
	}

	Context("No migration marker ConfigMap", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(commanderUUIDCM))
			f.RunHook()
		})

		It("does not set the metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMetricNotSet(f)
		})
	})

	Context("Migration marker present, no Commander (regular cluster)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migrationMarkerCM))
			f.RunHook()
		})

		It("sets the metric (alert fires)", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMetricSet(f)
		})
	})

	Context("Migration marker present, Commander-managed, no commander-info", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migrationMarkerCM + commanderUUIDCM))
			f.RunHook()
		})

		It("does not set the metric (alert suppressed)", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMetricNotSet(f)
		})
	})

	Context("Migration marker present, Commander-managed, support flag = 1", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migrationMarkerCM + commanderUUIDCM + commanderInfoSupportedCM))
			f.RunHook()
		})

		It("sets the metric (Commander supports the new format)", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMetricSet(f)
		})
	})

	Context("Migration marker present, Commander-managed, support flag = 0", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migrationMarkerCM + commanderUUIDCM + commanderInfoNotSupportedCM))
			f.RunHook()
		})

		It("does not set the metric (alert suppressed)", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMetricNotSet(f)
		})
	})

	Context("Migration marker present, Commander-managed, support flag absent", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migrationMarkerCM + commanderUUIDCM + commanderInfoNoFlagCM))
			f.RunHook()
		})

		It("does not set the metric (fail-closed)", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMetricNotSet(f)
		})
	})

	Context("Migration marker present, Commander-managed, invalid data.json", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(migrationMarkerCM + commanderUUIDCM + commanderInfoInvalidCM))
			f.RunHook()
		})

		It("does not set the metric and does not fail (fail-closed)", func() {
			Expect(f).To(ExecuteSuccessfully())
			assertMetricNotSet(f)
		})
	})
})
