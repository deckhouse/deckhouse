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
	"crypto/sha256"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: cluster_autoscaler_deployment_requirements ::", func() {
	f := HookExecutionConfigInit(`
global:
  discovery:
    clusterUUID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
nodeManager:
  internal:
    instancePrefix: "sandbox"
    nodeGroups: []
`, `{}`)

	Context("nodeManager.internal.nodeGroups is absent", func() {
		emptyValuesF := HookExecutionConfigInit(`
global:
  discovery:
    clusterUUID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
nodeManager:
  internal:
    instancePrefix: "sandbox"
`, `{}`)

		BeforeEach(func() {
			emptyValuesF.BindingContexts.Set(emptyValuesF.GenerateBeforeHelmContext())
			emptyValuesF.RunHook()
		})

		It("must not fail", func() {
			Expect(emptyValuesF).To(ExecuteSuccessfully())
			Expect(emptyValuesF.ValuesGet("nodeManager.internal.autoscalerNodes").Exists()).To(BeFalse())
			Expect(emptyValuesF.ValuesGet("nodeManager.internal.autoscalerMCMNodes").Exists()).To(BeFalse())
		})
	})

	Context("NodeGroup CR has no zones but values are enriched by get_crds", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager.internal.nodeGroups", []byte(`
- name: worker
  nodeType: CloudEphemeral
  engine: CAPI
  cloudInstances:
    minPerZone: 0
    maxPerZone: 2
    zones:
    - ru-central1-a
    - ru-central1-b
`))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("must generate autoscaler args from enriched values zones", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.deployAutoscaler").String()).To(Equal("true"))
			Expect(f.ValuesGet("nodeManager.internal.autoscalerNodes").String()).To(MatchJSON(fmt.Sprintf(`[
  "--nodes=0:2:d8-cloud-instance-manager.sandbox-worker-%s",
  "--nodes=0:2:d8-cloud-instance-manager.sandbox-worker-%s"
]`,
				fmt.Sprintf("%x", sha256.Sum256([]byte("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaaru-central1-a")))[:8],
				fmt.Sprintf("%x", sha256.Sum256([]byte("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaaru-central1-b")))[:8],
			)))
		})
	})
})
