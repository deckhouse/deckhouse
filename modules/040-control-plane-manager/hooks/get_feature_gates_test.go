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

func init() {
	FeatureGatesMap = map[string]ComponentFeatures{
		"1.30": {
			Kubelet: []string{
				"CPUManager",
				"MemoryManager",
			},
			APIServer: []string{
				"APIServerIdentity",
				"StorageVersionAPI",
			},
			KubeControllerManager: []string{
				"CronJobsScheduledAnnotation",
			},
			KubeScheduler: []string{
				"SchedulerQueueingHints",
			},
		},
		"1.31": {
			Deprecated: []string{
				"DynamicResourceAllocation",
			},
			Forbidden: []string{
				"SomeProblematicFeature",
			},
			Kubelet: []string{
				"CPUManager",
				"MemoryManager",
			},
			APIServer: []string{
				"APIServerIdentity",
				"StorageVersionAPI",
			},
			KubeControllerManager: []string{
				"CronJobsScheduledAnnotation",
			},
			KubeScheduler: []string{
				"SchedulerQueueingHints",
			},
		},
		"1.32": {
			Deprecated: []string{
				"TestDeprecatedGate",
			},
			Forbidden: []string{
				"SomeProblematicFeature",
			},
			Kubelet: []string{
				"CPUManager",
				"MemoryManager",
			},
			APIServer: []string{
				"APIServerIdentity",
				"StorageVersionAPI",
			},
			KubeControllerManager: []string{
				"CronJobsScheduledAnnotation",
			},
			KubeScheduler: []string{
				"SchedulerQueueingHints",
			},
		},
		"1.33": {
			Forbidden: []string{
				"SomeProblematicFeature",
			},
			Kubelet: []string{
				"CPUManager",
				"MemoryManager",
			},
			APIServer: []string{
				"APIServerIdentity",
				"StorageVersionAPI",
			},
			KubeControllerManager: []string{
				"CronJobsScheduledAnnotation",
			},
			KubeScheduler: []string{
				"SchedulerQueueingHints",
			},
		},
	}
}

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

		It("internal.allowedFeatureGates must have empty arrays for all components", func() {
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates").String()).To(MatchJSON(`{
				"apiserver": [],
				"kubelet": [],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

	Context("Empty feature gates array", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.31.0")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("internal.allowedFeatureGates must have empty arrays for all components", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates").String()).To(MatchJSON(`{
				"apiserver": [],
				"kubelet": [],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

	Context("Feature gates for Kubernetes 1.31", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.31.0")
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
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates").String()).To(MatchJSON(`{
				"apiserver": ["APIServerIdentity", "StorageVersionAPI"],
				"kubelet": ["CPUManager", "MemoryManager"],
				"kubeControllerManager": ["CronJobsScheduledAnnotation"],
				"kubeScheduler": ["SchedulerQueueingHints"]
			}`))
		})
	})

	Context("Forbidden feature gates for Kubernetes 1.33", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.33.0")
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
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates").String()).To(MatchJSON(`{
				"apiserver": ["APIServerIdentity"],
				"kubelet": ["CPUManager"],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

	Context("Non-existent feature gates", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.31.0")
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
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates").String()).To(MatchJSON(`{
			"apiserver": ["APIServerIdentity"],
			"kubelet": [],
			"kubeControllerManager": [],
			"kubeScheduler": []
		}`))
		})
	})

	Context("Feature gates deprecated in future versions", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.30.0")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"DynamicResourceAllocation",
				"TestDeprecatedGate",
				"APIServerIdentity",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Must set metrics for deprecated feature gates", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(2))

			foundDynamicResourceAllocation := false
			foundTestDeprecatedGate := false

			for _, metric := range m {
				switch metric.Labels["feature_gate"] {
				case "DynamicResourceAllocation":
					Expect(metric.Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
					Expect(*metric.Value).To(BeNumerically("==", 1.0))
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", "1.31"))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.30"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "will_be_deprecated"))
					foundDynamicResourceAllocation = true
				case "TestDeprecatedGate":
					Expect(metric.Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
					Expect(*metric.Value).To(BeNumerically("==", 1.0))
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", "1.32"))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.30"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "will_be_deprecated"))
					foundTestDeprecatedGate = true
				}
			}

			Expect(foundDynamicResourceAllocation).To(BeTrue(), "DynamicResourceAllocation metric not found")
			Expect(foundTestDeprecatedGate).To(BeTrue(), "TestDeprecatedGate metric not found")
		})
	})

	Context("Feature gates already deprecated in current version", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.31.0")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"DynamicResourceAllocation",
				"APIServerIdentity",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Must set metrics for currently deprecated feature gates", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(1))

			for _, metric := range m {
				if metric.Labels["feature_gate"] == "DynamicResourceAllocation" {
					Expect(metric.Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
					Expect(*metric.Value).To(BeNumerically("==", 1.0))
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", "1.31"))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.31"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "deprecated"))
				}
			}
		})
	})

	Context("No deprecated feature gates in use", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.32.0")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"APIServerIdentity",
				"CPUManager",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Must set metric to 0 when no problematic feature gates", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(1))
			Expect(m[0].Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
			Expect(*m[0].Value).To(BeNumerically("==", 0.0))
			Expect(m[0].Labels).To(HaveKeyWithValue("feature_gate", ""))
			Expect(m[0].Labels).To(HaveKeyWithValue("deprecated_version", ""))
			Expect(m[0].Labels).To(HaveKeyWithValue("current_version", "1.32"))
			Expect(m[0].Labels).To(HaveKeyWithValue("status", ""))
		})
	})

	Context("Forbidden feature gates in current version", func() {
		BeforeEach(func() {
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion ", "1.31.0")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"SomeProblematicFeature",
				"APIServerIdentity",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Must set metrics for forbidden feature gates", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(1))

			for _, metric := range m {
				if metric.Labels["feature_gate"] == "SomeProblematicFeature" {
					Expect(metric.Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
					Expect(*metric.Value).To(BeNumerically("==", 1.0))
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", "1.31"))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.31"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "forbidden"))
				}
			}
		})
	})

})
