/*
Copyright 2024 Flant JSC

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

var _ = Describe("Modules :: control-plane-manager :: hooks :: get_feature_gates ::", func() {
	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}}}`
		initConfigValuesString = `{}`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Kubernetes version not set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("internal.enabledFeatureGates must have empty arrays for all components", func() {
			Expect(f.ValuesGet("controlPlaneManager.internal.enabledFeatureGates").String()).To(MatchJSON(`{
				"apiServer": [],
				"kubelet": [],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

	Context("Empty feature gates array", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesVersion", "1.31")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("internal.enabledFeatureGates must have empty arrays for all components", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.enabledFeatureGates").String()).To(MatchJSON(`{
				"apiServer": [],
				"kubelet": [],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

	Context("Feature gates for Kubernetes 1.31", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesVersion", "1.31")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"APIServerIdentity",
				"StorageVersionAPI",
				"CPUManager",
				"MemoryManager",
				"CronJobsScheduledAnnotation",
				"SchedulerQueueingHints",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Feature gates must be distributed correctly by components", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.enabledFeatureGates").String()).To(MatchJSON(`{
				"apiServer": ["APIServerIdentity", "StorageVersionAPI"],
				"kubelet": ["CPUManager", "MemoryManager"],
				"kubeControllerManager": ["CronJobsScheduledAnnotation"],
				"kubeScheduler": ["SchedulerQueueingHints"]
			}`))
		})
	})

	Context("Forbidden feature gates for Kubernetes 1.33", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesVersion", "1.33")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"SomeProblematicFeature",
				"CPUManager",
				"APIServerIdentity",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Forbidden feature gates must be ignored", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.enabledFeatureGates").String()).To(MatchJSON(`{
				"apiServer": ["APIServerIdentity"],
				"kubelet": ["CPUManager"],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

	Context("Non-existent feature gates", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.kubernetesVersion", "1.31")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"NonExistentFeature",
				"AnotherNonExistentFeature",
				"APIServerIdentity",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Non-existent feature gates must be ignored", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.enabledFeatureGates").String()).To(MatchJSON(`{
				"apiServer": ["APIServerIdentity"],
				"kubelet": [],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

})
