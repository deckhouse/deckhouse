/*
Copyright 2021 Flant JSC

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

var _ = Describe("Modules :: control-plane-manager :: hooks :: arguments ::", func() {

	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = `{"controlPlaneManager":{"apiserver": {"auditPolicyEnabled": false}}}}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("controlPlaneManager.internal.arguments must be empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.arguments").Exists()).To(BeFalse())
		})

		Context("nodeMonitorGracePeriodSeconds is set to 15 seconds", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.nodeMonitorGracePeriodSeconds", 15)
				f.RunHook()
			})

			It("arguments must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.arguments").String()).To(MatchJSON(`{"nodeMonitorPeriod": 2, "nodeMonitorGracePeriod": 15}`))
			})
		})

		Context("failedNodePodEvictionTimeoutSeconds is set to 15 seconds", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.failedNodePodEvictionTimeoutSeconds", 15)
				f.RunHook()
			})

			It("arguments must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.arguments").String()).To(MatchJSON(`{"podEvictionTimeout": 15, "defaultUnreachableTolerationSeconds": 15}`))
			})
		})

		Context("nodeMonitorGracePeriodSeconds and failedNodePodEvictionTimeoutSeconds both are set to 15 seconds", func() {
			BeforeEach(func() {
				f.ValuesSet("controlPlaneManager.nodeMonitorGracePeriodSeconds", 15)
				f.ValuesSet("controlPlaneManager.failedNodePodEvictionTimeoutSeconds", 15)
				f.RunHook()
			})

			It("arguments must be set", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.arguments").String()).To(MatchJSON(`{"nodeMonitorPeriod": 2, "nodeMonitorGracePeriod": 15, "podEvictionTimeout": 15, "defaultUnreachableTolerationSeconds": 15}`))
			})
		})

	})

})
