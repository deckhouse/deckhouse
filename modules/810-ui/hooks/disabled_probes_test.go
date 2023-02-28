/*
Copyright 2023 Flant JSC

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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/go_lib/set"
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

	Context("extensions probes depending on deployed apps", func() {
		Context("no apps", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
				f.ValuesSet("global.enabledModules", allModules().Slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("all extensions probes disabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).To(ContainElement("extensions/cluster-scaling"))
				Expect(disabledProbes).To(ContainElement("extensions/cluster-autoscaler"))
				Expect(disabledProbes).To(ContainElement("extensions/prometheus-longterm"))
			})
		})

		Context("with cluster-autoscaler", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(deploymentInCloudInstanceManager("cluster-autoscaler"), 1))
				f.ValuesSet("global.enabledModules", allModules().Slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("extensions/cluster-autoscaler probe is enabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()
				Expect(disabledProbes).NotTo(ContainElement("extensions/cluster-autoscaler"))
			})
		})

		Context("with MCM, CCM, and bashible-apiserver", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
					deploymentInCloudInstanceManager("machine-controller-manager")+
						deploymentInCloudInstanceManager("bashible-apiserver")+
						deploymentCCM("openstack"), 3,
				))
				f.ValuesSet("global.enabledModules", allModules().Slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("extensions/cluster-scaling probe is enabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).NotTo(ContainElement("extensions/cluster-scaling"))
			})
		})

		Context("with prometheus-longterm", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(
					statefulsetInMonitoring("prometheus-longterm"),
					3,
				))
				f.ValuesSet("global.enabledModules", allModules().Slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("extensions/prometheus-longterm probe is enabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).NotTo(ContainElement("extensions/prometheus-longterm"))
			})
		})
	})

	Context("load-balancing probes depending on deployed apps", func() {
		Context("no apps", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(``))
				f.ValuesSet("global.enabledModules", allModules().Slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("load-balancer-configuration is disabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).To(ContainElement("load-balancing/load-balancer-configuration"))
			})
		})

		Context("with CCM", func() {
			f := HookExecutionConfigInit(initValues, `{}`)

			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(deploymentCCM("openstack")))
				f.ValuesSet("global.enabledModules", allModules().Slice())
				f.RunHook()
				Expect(f).To(ExecuteSuccessfully())
			})

			It("load-balancer-configuration is enabled", func() {
				disabledProbes := f.ValuesGet("upmeter.internal.disabledProbes").AsStringSlice()

				Expect(disabledProbes).NotTo(ContainElement("load-balancing/load-balancer-configuration"))
			})
		})
	})
})

func statefulsetInMonitoring(name string) string {
	const format = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: %s
  namespace: d8-monitoring
`
	return fmt.Sprintf(format, name)
}

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

// allModules returns the Set of all possibly affected modules to test against the filled list
func allModules() set.Set {
	return set.New(
		"metallb",
		"monitoring-kubernetes",
		"node-manager",
		"prometheus",
		"prometheus-metrics-adapter",
		"vertical-pod-autoscaler",
	)
}

func Test_calcDisabledProbes(t *testing.T) {
	type args struct {
		presence         appPresence
		manuallyDisabled set.Set
		enabledModules   set.Set
	}
	cases := []struct {
		name              string
		args              args
		expectDisabled    set.Set
		expectNotDisabled set.Set
	}{
		// Prometheus -> MAA group + grafana
		{
			name: "MAA group and Grafana probe are off without Prometheus",
			expectDisabled: set.New(
				"monitoring-and-autoscaling/",
				"extensions/grafana",
			),
		},
		{
			name: "MAA group and Grafana probe are on with Prometheus",
			args: args{
				enabledModules: set.New("prometheus"),
			},
			expectNotDisabled: set.New(
				"monitoring-and-autoscaling/",
				"extensions/grafana",
			),
		},

		// Prometheus, PMA -> MAA/PMA probe
		{
			name: "PMA probe off",
			args: args{
				enabledModules: set.New("prometheus"),
			},
			expectDisabled: set.New("monitoring-and-autoscaling/prometheus-metrics-adapter"),
		},
		{
			name: "PMA probe on",
			args: args{
				enabledModules: set.New(
					"prometheus",
					"prometheus-metrics-adapter",
				),
			},
			expectNotDisabled: set.New("monitoring-and-autoscaling/prometheus-metrics-adapter"),
		},

		// Prometheus, VPA -> MAA/VPA probe
		{
			name: "VPA probe off",
			args: args{
				enabledModules: set.New("prometheus"),
			},
			expectDisabled: set.New("monitoring-and-autoscaling/vertical-pod-autoscaler"),
		},
		{
			name: "VPA probe on",
			args: args{
				enabledModules: set.New(
					"prometheus",
					"vertical-pod-autoscaler",
				),
			},
			expectNotDisabled: set.New("monitoring-and-autoscaling/vertical-pod-autoscaler"),
		},

		// Prometheus, PMA -> MAA/HPA probe
		{
			name: "HPA probe off",
			args: args{
				enabledModules: set.New("prometheus"),
			},
			expectDisabled: set.New("monitoring-and-autoscaling/horizontal-pod-autoscaler"),
		},
		{
			name: "HPA probe on",
			args: args{
				enabledModules: set.New(
					"prometheus",
					"prometheus-metrics-adapter",
				),
			},
			expectNotDisabled: set.New("monitoring-and-autoscaling/horizontal-pod-autoscaler"),
		},
		// Prometheus, monitoring-kubernetes -> MAA/metrics-sources + MAA/key-metrics-present
		{
			name: "MAA/metrics-sources and MAA/key-metrics-present off",
			args: args{
				enabledModules: set.New("prometheus"),
			},
			expectDisabled: set.New(
				"monitoring-and-autoscaling/metrics-sources",
				"monitoring-and-autoscaling/key-metrics-present",
			),
		},
		{
			name: "MAA/metrics-sources and MAA/key-metrics-present on",
			args: args{
				enabledModules: set.New(
					"prometheus",
					"monitoring-kubernetes",
				),
			},
			expectNotDisabled: set.New(
				"monitoring-and-autoscaling/metrics-sources",
				"monitoring-and-autoscaling/key-metrics-present",
			),
		},
		// Metallb -> load-balancing/metallb
		{
			name:           "Metallb off",
			expectDisabled: set.New("load-balancing/metallb"),
		},
		{
			name: "Metallb on",
			args: args{
				enabledModules: set.New("metallb"),
			},
			expectNotDisabled: set.New("load-balancing/metallb"),
		},

		// node-manager, autoscaler -> extensions/cluster-autoscaler
		{
			name:           "extensions/cluster-autoscaler off",
			expectDisabled: set.New("extensions/cluster-autoscaler"),
		},
		{
			name: "extensions/cluster-autoscaler off without autoscaler deployment",
			args: args{
				presence:       appPresence{bashible: true, smokeMini: true, ccm: true, mcm: true},
				enabledModules: set.New("node-manager"),
			},
			expectDisabled: set.New("extensions/cluster-autoscaler"),
		},
		{
			name: "extensions/cluster-autoscaler off without node-manager",
			args: args{
				presence: appPresence{autoscaler: true},
			},
			expectDisabled: set.New("extensions/cluster-autoscaler"),
		},
		{
			name: "extensions/cluster-autoscaler on with node-manager and autoscaler deployment",
			args: args{
				presence:       appPresence{autoscaler: true},
				enabledModules: set.New("node-manager"),
			},
			expectNotDisabled: set.New("extensions/cluster-autoscaler"),
		},

		// node-manager, MCM, CCM, bashible -> extensions/cluster-scaling
		{
			name:           "extensions/cluster-scaling off",
			expectDisabled: set.New("extensions/cluster-scaling"),
		},
		{
			name: "extensions/cluster-scaling off without MCM",
			args: args{
				presence:       appPresence{bashible: true, ccm: true, autoscaler: true},
				enabledModules: set.New("node-manager"),
			},
			expectDisabled: set.New("extensions/cluster-scaling"),
		},
		{
			name: "extensions/cluster-scaling off without CCM",
			args: args{
				presence:       appPresence{bashible: true, mcm: true, autoscaler: true},
				enabledModules: set.New("node-manager"),
			},
			expectDisabled: set.New("extensions/cluster-scaling"),
		},
		{
			name: "extensions/cluster-scaling off without bashible",
			args: args{
				presence:       appPresence{ccm: true, mcm: true, autoscaler: true},
				enabledModules: set.New("node-manager"),
			},
			expectDisabled: set.New("extensions/cluster-scaling"),
		},
		{
			name: "extensions/cluster-scaling off without node-manager",
			args: args{
				presence: appPresence{ccm: true, mcm: true, bashible: true},
			},
			expectDisabled: set.New("extensions/cluster-scaling"),
		},
		{
			name: "extensions/cluster-scaling on with node-manager, MCM, CCM, and bashible deployments",
			args: args{
				presence:       appPresence{ccm: true, mcm: true, bashible: true},
				enabledModules: set.New("node-manager"),
			},
			expectNotDisabled: set.New("extensions/cluster-scaling"),
		},

		// smokeMini -> Synthetic group
		{
			name:           "synthetic group off",
			expectDisabled: set.New("synthetic/"),
		},
		{
			name: "synthetic group on",
			args: args{
				presence: appPresence{smokeMini: true},
			},
			expectNotDisabled: set.New("synthetic/"),
		},

		// OpenVPN -> extensions/openvpn
		{
			name:           "extensions/openvpn off",
			expectDisabled: set.New("extensions/openvpn"),
		},
		{
			name: "extensions/openvpn on",
			args: args{
				enabledModules: set.New("openvpn"),
			},
			expectNotDisabled: set.New("extensions/openvpn"),
		},

		// Dashboard -> extensions/dashboard
		{
			name:           "extensions/dashboard off",
			expectDisabled: set.New("extensions/dashboard"),
		},
		{
			name: "extensions/dashboard on",
			args: args{
				enabledModules: set.New("dashboard"),
			},
			expectNotDisabled: set.New("extensions/dashboard"),
		},

		// Dex in d8-user-authn -> extensions/dex
		{
			name:           "extensions/dex off",
			expectDisabled: set.New("extensions/dex"),
		},
		{
			name: "extensions/dex on",
			args: args{
				enabledModules: set.New("user-authn"),
			},
			expectNotDisabled: set.New("extensions/dex"),
		},

		// prometheus-longterm -> extensions/prometheus-longterm
		{
			name:           "extensions/prometheus-longterm off",
			expectDisabled: set.New("extensions/prometheus-longterm"),
		},
		{
			name: "extensions/prometheus-longterm off when prometheus module is disabled",
			args: args{
				presence: appPresence{prometheusLongterm: true},
			},
			expectDisabled: set.New("extensions/prometheus-longterm"),
		},
		{
			name: "extensions/prometheus-longterm off when longterm is absent",
			args: args{
				presence:       appPresence{prometheusLongterm: false},
				enabledModules: set.New("prometheus"),
			},
			expectDisabled: set.New("extensions/prometheus-longterm"),
		},
		{
			name: "extensions/prometheus-longterm on",
			args: args{
				presence:       appPresence{prometheusLongterm: true},
				enabledModules: set.New("prometheus"),
			},
			expectNotDisabled: set.New("extensions/prometheus-longterm"),
		},

		// certManager -> control-plane/cert-manager
		{
			name:           "control-plane/cert-manager off",
			expectDisabled: set.New("control-plane/cert-manager"),
		},
		{
			name: "control-plane/cert-manager on",
			args: args{
				enabledModules: set.New("cert-manager"),
			},
			expectNotDisabled: set.New("control-plane/cert-manager"),
		},

		// Manually disabled probes are preserved
		{
			name: "manually disabled extensions/prometheus-longterm",
			args: args{
				presence:         appPresence{prometheusLongterm: true},
				enabledModules:   set.New("prometheus"),
				manuallyDisabled: set.New("extensions/prometheus-longterm"),
			},
			expectDisabled: set.New("extensions/prometheus-longterm"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			disabled := calcDisabledProbes(tt.args.presence, tt.args.enabledModules, tt.args.manuallyDisabled)

			if tt.expectDisabled != nil {
				for _, x := range tt.expectDisabled.Slice() {
					assert.True(t, disabled.Has(x), "expected to have disabled %q", x)
				}
			}

			if tt.expectNotDisabled != nil {
				for _, x := range tt.expectNotDisabled.Slice() {
					assert.False(t, disabled.Has(x), "expected to have enabled %q", x)
				}
			}
		})
	}
}
