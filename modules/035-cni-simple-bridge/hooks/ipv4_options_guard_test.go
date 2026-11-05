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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// The decision logic is tested in go_lib/hooks/ipv4_options_guard; here only its inputs: bindings and values.
// Each module uses the agent DaemonSet of another CNI, so all three names are covered.
var _ = Describe("Modules :: cni-simple-bridge :: hooks :: ipv4_options_guard", func() {
	const (
		otherAgentDS = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: flannel
  namespace: d8-cni-flannel
`
		ownCM = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: ipv4-options-guard
  namespace: d8-cni-simple-bridge
data:
  defaultMode: Enforce
`
	)

	f := HookExecutionConfigInit(`{"cniSimpleBridge":{"internal":{}}}`, `{"cniSimpleBridge":{}}`)

	run := func(state string) {
		f.KubeStateSet(state)
		f.BindingContexts.Set(f.GenerateBeforeHelmContext())
		f.RunHook()
		Expect(f).To(ExecuteSuccessfully())
	}
	mode := func() string { return f.ValuesGet("cniSimpleBridge.internal.ipv4OptionsGuardMode").String() }

	It("defaults to Enforce on a new cluster", func() {
		run("")
		Expect(mode()).To(Equal("Enforce"))
		Expect(f.ValuesGet("cniSimpleBridge.internal.ipv4OptionsGuardDefaultMode").String()).To(Equal("Enforce"))
	})

	It("defaults to Audit when a CNI agent is running", func() {
		run(otherAgentDS)
		Expect(mode()).To(Equal("Audit"))
	})

	It("keeps the decision from the ConfigMap of the module", func() {
		run(otherAgentDS + "---" + ownCM)
		Expect(mode()).To(Equal("Enforce"))
	})

	It("keeps the decision not rendered into the ConfigMap yet", func() {
		f.ValuesSet("cniSimpleBridge.internal.ipv4OptionsGuardDefaultMode", "Enforce")
		run(otherAgentDS)
		Expect(mode()).To(Equal("Enforce"))
	})

	It("defaults to Audit on a bootstrapped cluster without a CNI agent", func() {
		f.ValuesSet("global.clusterIsBootstrapped", true)
		run("")
		Expect(mode()).To(Equal("Audit"))
	})

	It("uses the mode from the module settings", func() {
		f.ValuesSet("cniSimpleBridge.ipv4OptionsGuard.mode", "Disabled")
		run("")
		Expect(mode()).To(Equal("Disabled"))
		Expect(f.ValuesGet("cniSimpleBridge.internal.ipv4OptionsGuardDefaultMode").String()).To(Equal("Enforce"))
	})
})
