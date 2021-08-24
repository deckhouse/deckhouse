/*
Copyright 2021 Flant CJSC

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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: upmeter :: hooks :: disabled_probes ::", func() {
	const initValues = `{"upmeter": { "internal": { "disabledProbes": [] }, "disabledProbes": [] }}`

	Context("smokeMiniDisabled ", func() {
		f := HookExecutionConfigInit(initValues, `{}`)

		It("disables synthetic group when true", func() {
			f.ValuesSetFromYaml("upmeter.smokeMiniDisabled", []byte("true"))

			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())

			value := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()
			Expect(value).To(ContainElement("synthetic/"))
		})

		It("enables synthetic group when false", func() {
			f.ValuesSetFromYaml("upmeter.smokeMiniDisabled", []byte("false"))

			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())

			value := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()
			Expect(value).NotTo(ContainElement("synthetic/"))
		})
	})

	Context("probes depending on modules", func() {
		f := HookExecutionConfigInit(initValues, `{}`)

		DescribeTable("disabled modules",
			func(module, probeRef string) {
				// Module is off, probe is off, the probe ref should be in the disabled list
				f.ValuesSet("global.enabledModules", allModules().delete(module).slice())

				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())

				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()
				Expect(disabledProbes).To(ContainElement(probeRef),
					"we should have probe disabled (in the list) because the module is off")

				// Module is on, probe is on, the probe ref should NOT be in the disabled list
				f.ValuesSet("global.enabledModules", allModules().slice())

				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())

				disabledProbes = f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()
				Expect(disabledProbes).ToNot(ContainElement(probeRef),
					"we should NOT have probe disabled (absent in the list) because the module is on")
			},
			Entry("Monitoring and autoscaling group muted by promehteus module",
				"prometheus",
				"monitoring-and-autoscaling/"),
			Entry("Prometheus metrics adapter probe",
				"prometheus-metrics-adapter",
				"monitoring-and-autoscaling/prometheus-metrics-adapter"),
			Entry("Vertical pod autoscaler probe",
				"vertical-pod-autoscaler",
				"monitoring-and-autoscaling/vertical-pod-autoscaler"),
			Entry("Metrics sources probe",
				"monitoring-kubernetes",
				"monitoring-and-autoscaling/metrics-sources"),
			Entry("Key metrics presence probe",
				"monitoring-kubernetes",
				"monitoring-and-autoscaling/key-metrics-present"),
			Entry("Horizontal pod autoscaler probe",
				"prometheus-metrics-adapter",
				"monitoring-and-autoscaling/horizontal-pod-autoscaler"),
			Entry("Scaling group",
				"node-manager",
				"scaling/"),
		)
	})

	Context("scaling probes depending on deployed apps", func() {
		Context("no apps", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.ValuesSet("global.enabledModules", allModules().slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("all scaling probes disabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).To(ContainElement("scaling/cluster-scaling"))
				Expect(disabledProbes).To(ContainElement("scaling/cluster-autoscaler"))
			})
		})

		Context("with cluster-autoscaler", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(deploymentInCloudInstanceManager("cluster-autoscaler")))
				f.ValuesSet("global.enabledModules", allModules().slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("only cluster-autoscaler enabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).To(ContainElement("scaling/cluster-scaling"))

				Expect(disabledProbes).NotTo(ContainElement("scaling/cluster-autoscaler"))
			})
		})

		Context("with MCM, CCM, and bashible-apiserver", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					deploymentInCloudInstanceManager("machine-controller-manager") +
						deploymentInCloudInstanceManager("bashible-apiserver") +
						deploymentCCM("openstack"),
				))
				f.ValuesSet("global.enabledModules", allModules().slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("only cluster-scaling enabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).NotTo(ContainElement("scaling/cluster-scaling"))

				Expect(disabledProbes).To(ContainElement("scaling/cluster-autoscaler"))
			})
		})

		Context("with everything", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(
					deploymentInCloudInstanceManager("cluster-autoscaler") +
						deploymentInCloudInstanceManager("machine-controller-manager") +
						deploymentInCloudInstanceManager("bashible-apiserver") +
						deploymentCCM("openstack"),
				))
				f.ValuesSet("global.enabledModules", allModules().slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("all enabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).NotTo(ContainElement("scaling/cluster-scaling"))
				Expect(disabledProbes).NotTo(ContainElement("scaling/cluster-autoscaler"))
			})
		})
	})
})

func deploymentInCloudInstanceManager(name string) string {
	const format = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: d8-cloud-instance-manager
`
	return fmt.Sprintf(format, name)
}

func deploymentCCM(provider string) string {
	const format = `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cloud-controller-manager
  namespace: d8-cloud-provider-%s
`
	return fmt.Sprintf(format, provider)
}

// allModules returns the set of all possibly affected modules to test against the filled list
func allModules() set {
	return newSet(
		"monitoring-kubernetes",
		"node-manager",
		"prometheus",
		"prometheus-metrics-adapter",
		"vertical-pod-autoscaler",
	)
}
