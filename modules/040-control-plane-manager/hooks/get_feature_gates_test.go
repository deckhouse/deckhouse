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

func featureGatesTestMap() map[string]ComponentFeatures {
	return map[string]ComponentFeatures{
		"1.32": {
			Kubelet: []string{
				"CPUManager",
				"MemoryManager",
			},
			APIServer: []string{
				"APIServerIdentity",
				"StorageVersionAPI",
				"DynamicResourceAllocation",
				"TestDeprecatedGate",
			},
			KubeControllerManager: []string{
				"CronJobsScheduledAnnotation",
			},
			KubeScheduler: []string{
				"SchedulerQueueingHints",
			},
		},
		"1.33": {
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
				"DynamicResourceAllocation",
				"TestDeprecatedGate",
			},
			KubeControllerManager: []string{
				"CronJobsScheduledAnnotation",
			},
			KubeScheduler: []string{
				"SchedulerQueueingHints",
			},
		},
		"1.34": {
			Deprecated: []string{
				"TestDeprecatedGate",
			},
			Forbidden: []string{
				"SomeProblematicFeature",
			},
			Kubelet: []string{
				"CPUManager",
				"MemoryManager",
				"KubeletSkewGate",
			},
			APIServer: []string{
				"APIServerIdentity",
				"StorageVersionAPI",
				"TestDeprecatedGate",
			},
			KubeControllerManager: []string{
				"CronJobsScheduledAnnotation",
			},
			KubeScheduler: []string{
				"SchedulerQueueingHints",
			},
		},
		"1.35": {
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
		"1.37": {
			Kubelet: []string{
				"H2CContainerProbe",
			},
			APIServer: []string{
				"H2CContainerProbe",
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

	var originalFeatureGatesMap map[string]ComponentFeatures
	BeforeEach(func() {
		originalFeatureGatesMap = FeatureGatesMap
		FeatureGatesMap = featureGatesTestMap()
	})
	AfterEach(func() {
		FeatureGatesMap = originalFeatureGatesMap
	})

	setEffectiveKubernetesVersion := func(version string) {
		f.ValuesSet("controlPlaneManager.internal.effectiveKubernetesVersion", version)
		f.ValuesSet("global.discovery.targetKubernetesVersion", version)
		f.ValuesSet("controlPlaneManager.internal.minAPIServerKubernetesVersion", version)
	}

	Context("Effective Kubernetes version not set", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.targetKubernetesVersion", "1.35")
			setEffectiveKubernetesVersion("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("must fail instead of publishing an empty allow-list", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("controlPlaneManager.internal.effectiveKubernetesVersion is empty"))
		})
	})

	Context("Empty feature gates array", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.32")
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

	Context("Feature gates for Kubernetes 1.32", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.32")
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

	Context("Feature gates with digits in their names for Kubernetes 1.37", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.37")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"H2CContainerProbe",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Digits in gate names must survive the values schema", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates").String()).To(MatchJSON(`{
				"apiserver": ["H2CContainerProbe"],
				"kubelet": ["H2CContainerProbe"],
				"kubeControllerManager": [],
				"kubeScheduler": []
			}`))
		})
	})

	Context("Forbidden feature gates for Kubernetes 1.35", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.35")
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
			setEffectiveKubernetesVersion("1.32")
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

		It("Must set metrics for non-existent feature gates", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()

			Expect(m).To(HaveLen(2))

			foundNonExistent := false
			foundAnotherNonExistent := false

			for _, metric := range m {
				switch metric.Labels["feature_gate"] {
				case "NonExistentFeature":
					Expect(metric.Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
					Expect(*metric.Value).To(BeNumerically("==", 1.0))
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", ""))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.32"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "unknown"))
					foundNonExistent = true
				case "AnotherNonExistentFeature":
					Expect(metric.Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
					Expect(*metric.Value).To(BeNumerically("==", 1.0))
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", ""))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.32"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "unknown"))
					foundAnotherNonExistent = true
				}
			}

			Expect(foundNonExistent).To(BeTrue(), "NonExistentFeature metric not found")
			Expect(foundAnotherNonExistent).To(BeTrue(), "AnotherNonExistentFeature metric not found")
		})
	})

	// The control plane is already stepping to 1.34 while every apiserver, and so every kubelet, is still 1.33.
	Context("Kubelet lags behind the effective version", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.34")
			f.ValuesSet("global.discovery.targetKubernetesVersion", "1.35")
			f.ValuesSet("controlPlaneManager.internal.minAPIServerKubernetesVersion", "1.33")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"KubeletSkewGate",
				"CPUManager",
				"NonExistentFeature",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must not pass gates unknown to the running kubelet", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates").String()).To(MatchJSON(`{
			"apiserver": [],
			"kubelet": ["CPUManager"],
			"kubeControllerManager": [],
			"kubeScheduler": []
		}`))
		})

		It("Must not report a gate of the effective version as unknown", func() {
			Expect(f).To(ExecuteSuccessfully())

			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Labels).To(HaveKeyWithValue("feature_gate", "NonExistentFeature"))
			Expect(m[0].Labels).To(HaveKeyWithValue("status", "unknown"))
		})
	})

	Context("Kubelet catches up with the effective version", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.34")
			f.ValuesSet("global.discovery.targetKubernetesVersion", "1.35")
			f.ValuesSet("controlPlaneManager.enabledFeatureGates", []interface{}{
				"KubeletSkewGate",
			})
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Must pass the gate to kubelet", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.allowedFeatureGates.kubelet").String()).To(MatchJSON(`["KubeletSkewGate"]`))
		})
	})

	Context("Feature gates deprecated in future versions", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.32")
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
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", "1.33"))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.32"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "will_be_deprecated"))
					foundDynamicResourceAllocation = true
				case "TestDeprecatedGate":
					Expect(metric.Name).To(Equal("d8_control_plane_manager_problematic_feature_gate"))
					Expect(*metric.Value).To(BeNumerically("==", 1.0))
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", "1.34"))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.32"))
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
			setEffectiveKubernetesVersion("1.33")
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
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", "1.33"))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.33"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "deprecated"))
				}
			}
		})
	})

	Context("No deprecated feature gates in use", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.35")
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
			Expect(m[0].Labels).To(HaveKeyWithValue("current_version", "1.35"))
			Expect(m[0].Labels).To(HaveKeyWithValue("status", ""))
		})
	})

	Context("Forbidden feature gates in current version", func() {
		BeforeEach(func() {
			setEffectiveKubernetesVersion("1.33")
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
					Expect(metric.Labels).To(HaveKeyWithValue("deprecated_version", ""))
					Expect(metric.Labels).To(HaveKeyWithValue("current_version", "1.33"))
					Expect(metric.Labels).To(HaveKeyWithValue("status", "forbidden"))
				}
			}
		})
	})

})
