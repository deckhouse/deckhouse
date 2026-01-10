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
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const validHubbleMonitoringConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: HubbleMonitoringConfig
metadata:
  name: hubble-monitoring-config
spec:
  extendedMetrics:
    enabled: true
    collectors:
      - name: drop
        contextOptions: "labelsContext=source_ip,source_namespace,source_pod,destination_ip,destination_namespace,destination_pod"
      - name: flow
  flowLogs:
    enabled: true
    allowFilter:
      verdict:
        - DROPPED
        - ERROR
    denyFilter:
      source_pod:
        - kube-system/
      destination_pod:
        - kube-system/
    fieldMaskList:
      - time
      - verdict
    fileMaxSizeMB: 30
`

const minimalHubbleMonitoringConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: HubbleMonitoringConfig
metadata:
  name: hubble-monitoring-config
spec: {}
`

const onlyExtendedMetricsHubbleMonitoringConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: HubbleMonitoringConfig
metadata:
  name: hubble-monitoring-config
spec:
  extendedMetrics:
    enabled: true
    collectors:
      - name: dns
`

const onlyFlowLogsHubbleMonitoringConfig = `
---
apiVersion: deckhouse.io/v1alpha1
kind: HubbleMonitoringConfig
metadata:
  name: hubble-monitoring-config
spec:
  flowLogs:
    enabled: true
    allowFilter:
      traffic_direction:
        - EGRESS
    fieldMaskList:
      - time
    fileMaxSizeMB: 15
`

var _ = Describe("Modules :: deckhouse :: hooks :: handle_hubble_monitoring_config ::", func() {
	f := HookExecutionConfigInit(`
cniCilium:
  internal:
    hubble:
      settings: {}
`, `{}`)

	f.RegisterCRD("deckhouse.io", "v1alpha1", "HubbleMonitoringConfig", false)

	Context("When no HubbleMonitoringConfig exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Does not change existing settings", func() {
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings").Exists()).To(BeTrue())
		})
	})

	Context("When a valid HubbleMonitoringConfig exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(validHubbleMonitoringConfig))
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Spec is copied to values", func() {
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings").Exists()).To(BeTrue())

			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics.enabled").Bool()).To(BeTrue())
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics.collectors").Array()).To(HaveLen(2))

			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.enabled").Bool()).To(BeTrue())

			verdicts := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.allowFilter.verdict").Array()
			Expect(verdicts).To(HaveLen(2))
			Expect(verdicts[0].String()).To(Equal("DROPPED"))
			Expect(verdicts[1].String()).To(Equal("ERROR"))

			sourcePods := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.denyFilter.source_pod").Array()
			Expect(sourcePods).To(HaveLen(1))
			Expect(sourcePods[0].String()).To(Equal("kube-system/"))

			destinationPods := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.denyFilter.destination_pod").Array()
			Expect(destinationPods).To(HaveLen(1))
			Expect(destinationPods[0].String()).To(Equal("kube-system/"))

			fieldMaskList := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.fieldMaskList").Array()
			Expect(fieldMaskList).To(HaveLen(2))
			Expect(fieldMaskList[0].String()).To(Equal("time"))
			Expect(fieldMaskList[1].String()).To(Equal("verdict"))

			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.fileMaxSizeMB").Int()).To(Equal(int64(30)))
		})
	})

	Context("When HubbleMonitoringConfig has minimal spec {}", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(minimalHubbleMonitoringConfig))
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Settings contain empty spec (no nested sections)", func() {
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings").Exists()).To(BeTrue())

			extendedMetrics := f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics")
			Expect(extendedMetrics.Exists()).To(BeTrue())
			Expect(extendedMetrics.IsObject()).To(BeTrue())

			flowLogs := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs")
			Expect(flowLogs.Exists()).To(BeTrue())
			Expect(flowLogs.IsObject()).To(BeTrue())
		})
	})

	Context("When only extendedMetrics is set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(onlyExtendedMetricsHubbleMonitoringConfig))
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Writes extendedMetrics and does not write flowLogs", func() {
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics.enabled").Bool()).To(BeTrue())

			collectors := f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics.collectors").Array()
			Expect(collectors).To(HaveLen(1))
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics.collectors.0.name").String()).To(Equal("dns"))

			flowLogs := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs")
			Expect(flowLogs.Exists()).To(BeTrue())
			Expect(flowLogs.IsObject()).To(BeTrue())
		})
	})

	Context("When only flowLogs is set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(onlyFlowLogsHubbleMonitoringConfig))
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Writes flowLogs and does not write extendedMetrics", func() {
			extendedMetrics := f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics")
			Expect(extendedMetrics.Exists()).To(BeTrue())
			Expect(extendedMetrics.IsObject()).To(BeTrue())
			Expect(extendedMetrics.Map()).To(BeEmpty())

			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.enabled").Bool()).To(BeTrue())

			trafficDirection := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.allowFilter.traffic_direction").Array()
			Expect(trafficDirection).To(HaveLen(1))
			Expect(trafficDirection[0].String()).To(Equal("EGRESS"))

			fieldMaskList := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.fieldMaskList").Array()
			Expect(fieldMaskList).To(HaveLen(1))
			Expect(fieldMaskList[0].String()).To(Equal("time"))

			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.fileMaxSizeMB").Int()).To(Equal(int64(15)))
		})
	})

	Context("When HubbleMonitoringConfig is deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(validHubbleMonitoringConfig))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())

			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Resets settings to defaults", func() {
			em := f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics")
			Expect(em.Exists()).To(BeTrue())
			Expect(em.IsObject()).To(BeTrue())
			Expect(em.Map()).To(BeEmpty())

			fl := f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs")
			Expect(fl.Exists()).To(BeTrue())
			Expect(fl.IsObject()).To(BeTrue())
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.allowFilter").Map()).To(BeEmpty())
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.denyFilter").Map()).To(BeEmpty())
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.fileMaxSizeMB").Int()).To(Equal(int64(10)))
		})
	})
})
