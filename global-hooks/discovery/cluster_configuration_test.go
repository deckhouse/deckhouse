// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"encoding/base64"
	"fmt"

	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/modules/040-control-plane-manager/hooks"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/clusterConfiguration ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	var (
		stateAClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
cloud:
  provider: OpenStack
  prefix: kube
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.33"
clusterDomain: "test.local"
`
		stateA = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateAClusterConfiguration))

		stateBClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  prefix: lube
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "1.33"
clusterDomain: "test.local"
`
		stateB = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateBClusterConfiguration))

		stateCClusterConfiguration = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: AWS
  prefix: lube
podSubnetCIDR: 10.122.0.0/16
podSubnetNodeCIDRPrefix: "26"
serviceSubnetCIDR: 10.213.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "test.local"
`
		stateC = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateCClusterConfiguration))
	)

	moduleConfigYAML := func(version string) string {
		settings := ""
		if version != "" {
			settings = fmt.Sprintf("\n  settings:\n    kubernetesVersion: %q", version)
		}
		return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  enabled: true
  version: 1%s
`, settings)
	}

	// Set default value for test purposes. Normally this var set to specific kube version on the build stage.
	hooks.DefaultKubernetesVersion = "1.36"

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	Context("Cluster has a d8-cluster-configuration Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA, 1))
			f.RunHook()
		})

		It("Should correctly fill the Values store from it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Static"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("OpenStack"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("kube"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.111.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("24"))
			Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.222.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.33"))

			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.111.0.0/16"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.222.0.0/16"))
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeFalse())

			var maxNodes *float64
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == "d8_max_nodes_amount_by_pod_cidr" {
					maxNodes = m.Value
					break
				}
			}
			Expect(maxNodes).NotTo(BeNil())
			Expect(*maxNodes).To(Equal(float64(256)))
		})

		Context("d8-cluster-configuration Secret has changed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateB, 1))
				f.RunHook()
			})

			It("Should correctly fill the Values store from it", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Cloud"))
				Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("AWS"))
				Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("lube"))
				Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.122.0.0/16"))
				Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("26"))
				Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.213.0.0/16"))
				Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.33"))

				Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.122.0.0/16"))
				Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.213.0.0/16"))
				Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))
				Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
				Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeFalse())

				var maxNodes *float64
				for _, m := range f.MetricsCollector.CollectedMetrics() {
					if m.Name == "d8_max_nodes_amount_by_pod_cidr" {
						maxNodes = m.Value
						break
					}
				}
				Expect(maxNodes).NotTo(BeNil())
				Expect(*maxNodes).To(Equal(float64(1024)))
			})
		})

		Context("d8-cluster-configuration Secret got deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
				f.RunHook()
			})

			It("Should not fail, but should not create any Values", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet("global.clusterConfiguration").Exists()).To(Not(BeTrue()))
				Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
				Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
			})
		})
	})

	Context("Cluster doesn't have a d8-cluster-configuration Secret", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
		f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts("", 0))
			f.RunHook()
		})

		It("Should not fail, but should not create any Values", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("global.clusterConfiguration").Exists()).To(Not(BeTrue()))
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
		})
	})

	Context("Cluster has a d8-cluster-configuration Secret with kubernetesVersion = `Automatic`", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC, 1))
			f.RunHook()
		})

		It("Should correctly fill the Values store from it", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("global.clusterConfiguration.clusterType").String()).To(Equal("Cloud"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.provider").String()).To(Equal("AWS"))
			Expect(f.ValuesGet("global.clusterConfiguration.cloud.prefix").String()).To(Equal("lube"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetCIDR").String()).To(Equal("10.122.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.podSubnetNodeCIDRPrefix").String()).To(Equal("26"))
			Expect(f.ValuesGet("global.clusterConfiguration.serviceSubnetCIDR").String()).To(Equal("10.213.0.0/16"))
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))

			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("10.122.0.0/16"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("10.213.0.0/16"))
			Expect(f.ValuesGet("global.discovery.clusterDomain").String()).To(Equal("test.local"))
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())

			var maxNodes *float64
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == "d8_max_nodes_amount_by_pod_cidr" {
					maxNodes = m.Value
					break
				}
			}
			Expect(maxNodes).NotTo(BeNil())
			Expect(*maxNodes).To(Equal(float64(1024)))
		})
	})

	Context("targetKubernetesVersion priority MC / CC / Default", func() {
		It("pinned ModuleConfig wins over pinned ClusterConfiguration", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.35"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.35"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeFalse())
			// Backward-compat: global.clusterConfiguration still reflects the Secret (CC pin).
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.33"))
		})

		It("ModuleConfig Automatic overrides a pinned ClusterConfiguration", func() {
			// Presence of the ModuleConfig setting decides which document owns the version, so an
			// explicit Automatic means "track the Deckhouse default" and the CC pin is ignored.
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
			// Backward-compat: global.clusterConfiguration still reflects the Secret (CC pin).
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.33"))
		})

		It("ModuleConfig Default overrides a pinned ClusterConfiguration", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("Default"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
			Expect(f.ValuesGet("global.clusterConfiguration.kubernetesVersion").String()).To(Equal("1.33"))
		})

		It("unset ModuleConfig defers to a pinned ClusterConfiguration", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML(""), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeFalse())
		})

		It("unset ModuleConfig and Automatic ClusterConfiguration resolve to Default", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
		})
	})

	Context("ConfigMap d8-cluster-kubernetes data.spec writer", func() {
		It("creates CM with Manual mode for a pinned target", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.35"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(`desiredVersion: "1.35"`))
			Expect(cm.Field("data.spec").String()).To(ContainSubstring("updateMode: Manual"))
			Expect(cm.Field("data.status").Exists()).To(BeFalse())
		})

		It("creates CM with Automatic mode when tracking Default", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(fmt.Sprintf("desiredVersion: %q", hooks.DefaultKubernetesVersion)))
			Expect(cm.Field("data.spec").String()).To(ContainSubstring("updateMode: Automatic"))
		})

		It("updates only data.spec, leaving status and labels untouched", func() {
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    k8s-version: "1.34"
    max-k8s-version: "1.34"
data:
  spec: |
    desiredVersion: "1.34"
    updateMode: Manual
  status: |
    currentVersion: "1.34"
    phase: UpToDate
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.35")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(`desiredVersion: "1.35"`))
			Expect(cm.Field("data.status").String()).To(ContainSubstring(`currentVersion: "1.34"`))
			Expect(cm.Field("metadata.labels.k8s-version").String()).To(Equal("1.34"))
			Expect(cm.Field("metadata.labels.max-k8s-version").String()).To(Equal("1.34"))
		})
	})

	Context("Automatic soft-guard freeze", func() {
		findDriftMetric := func() (float64, bool) {
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == "d8_control_plane_default_version_drift" {
					return *m.Value, true
				}
			}
			return 0, false
		}

		It("freezes desired digit when Default is below maxUsed window", func() {
			// Default is 1.36 in this suite; maxUsed 1.38 means Default is two minors below → freeze.
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    k8s-version: "1.38"
    max-k8s-version: "1.38"
data:
  spec: |
    desiredVersion: "1.38"
    updateMode: Automatic
  status: |
    currentVersion: "1.38"
    phase: UpToDate
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Automatic")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())

			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(`desiredVersion: "1.38"`))
			Expect(cm.Field("data.spec").String()).To(ContainSubstring("updateMode: Automatic"))

			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
		})

		It("publishes Default when it is within the maxUsed window", func() {
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    max-k8s-version: "1.36"
data:
  status: |
    currentVersion: "1.36"
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Automatic")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})

		It("fail-opens when Automatic has no maxUsed baseline", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})

		It("prefers Secret maxUsed over ConfigMap label when both disagree", func() {
			// Label alone (1.36) would keep Default in-window; Secret (1.38) forces freeze.
			// Secret must win so soft-guard matches admission's baseline.
			secretWithMaxUsed := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateCClusterConfiguration)) + `
  "maxUsedControlPlaneKubernetesVersion": ` + base64.StdEncoding.EncodeToString([]byte("1.38"))
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    k8s-version: "1.38"
    max-k8s-version: "1.36"
data:
  spec: |
    desiredVersion: "1.38"
    updateMode: Automatic
  status: |
    currentVersion: "1.38"
    phase: UpToDate
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Automatic")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeTrue())
			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
		})

		It("does not freeze an explicit Manual pin", func() {
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    max-k8s-version: "1.36"
data:
  status: |
    currentVersion: "1.36"
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.33")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsAutomatic").Bool()).To(BeFalse())
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})
	})
})
