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

const validCM = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: hubble
    module: cilium-hubble
  name: cilium-agent-hubble-settings
  namespace: d8-cni-cilium
data:
  settings.yaml: |
    extendedMetrics:
      enabled: true
      collectors:
        - name: Drop
          contextOptions: "labelsContext=source_ip,source_namespace,source_pod,destination_ip,destination_namespace,destination_pod"
        - name: Flow
    flowLogs:
      enabled: true
      allowList:
        - '{"verdict":["DROPPED","ERROR"]}'
      denyList:
        - '{"source_pod":["kube-system/"]}'
        - '{"destination_pod":["kube-system/"]}'
      fieldMaskList:
        - time
        - verdict
      fileMaxSizeMb: 30
`

var _ = Describe("Modules :: deckhouse :: hooks :: get_hubble_settings ::", func() {
	f := HookExecutionConfigInit(`
cniCilium:
  internal:
    hubble:
      settings:
        extendedMetrics:
          enabled: false
          collectors: []
        flowLogs:
          enabled: false
          allowList: []
          denyList: []
          fieldMaskList: []
          fileMaxSizeMb: 10
`, `{}`)

	Context("When a valid Hubble ConfigMap exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(validCM))
			f.RunHook()
		})

		It("Hook executes successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Config is populated correctly", func() {
			settings := f.ValuesGet("cniCilium.internal.hubble.settings")
			Expect(settings.Exists()).To(BeTrue())

			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics.enabled").Bool()).To(BeTrue())
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.extendedMetrics.collectors").Array()).To(HaveLen(2))

			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.enabled").Bool()).To(BeTrue())
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.allowList").Array()).To(HaveLen(1))
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.denyList").Array()).To(HaveLen(2))
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.denyList").Array()).To(HaveLen(2))
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.fieldMaskList").Array()).To(HaveLen(2))
			Expect(f.ValuesGet("cniCilium.internal.hubble.settings.flowLogs.fileMaxSizeMb").Int()).To(Equal(int64(30)))
		})
	})
})
