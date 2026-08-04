// Copyright 2026 Flant JSC
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

var _ = Describe("Global hooks :: discovery/targetKubernetesVersion ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	var (
		stateA = clusterConfigurationSecret(ccStateAClusterConfiguration)
		stateC = clusterConfigurationSecret(ccStateCClusterConfiguration)

		stateCClusterConfiguration = ccStateCClusterConfiguration
	)

	// Set default value for test purposes. Normally this var set to specific kube version on the build stage.
	hooks.DefaultKubernetesVersion = "1.36"

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ModuleConfig", false)

	// The Automatic→Default mirror into global.clusterConfiguration.kubernetesVersion belongs to
	// the ClusterConfiguration hook and is asserted in cluster_configuration_test.go: this suite
	// drives only the version hook, so that key is never populated here.
	Context("targetKubernetesVersion priority MC / CC / Default", func() {
		It("pinned ModuleConfig wins over pinned ClusterConfiguration", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.35"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.35"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeFalse())
		})

		It("ModuleConfig Automatic overrides a pinned ClusterConfiguration", func() {
			// Presence of the ModuleConfig setting decides which document owns the version, so an
			// explicit Automatic means "track the Deckhouse default" and the CC pin is ignored.
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
		})

		It("ModuleConfig Default overrides a pinned ClusterConfiguration", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("Default"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
		})

		It("unset ModuleConfig defers to a pinned ClusterConfiguration", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML(""), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeFalse())
		})

		It("unset ModuleConfig and Automatic ClusterConfiguration resolve to Default", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
		})

		// The reason this hook is separate from cluster_configuration.go. That hook returns an
		// error on a ClusterConfiguration missing podSubnetCIDR / serviceSubnetCIDR / clusterDomain,
		// or on a pod-CIDR prefix that fails the node-mask check. While both concerns shared one
		// hook, such a document stopped targetKubernetesVersion from being published at all, and
		// control-plane-manager then failed with "kubernetesVersion required" — a CIDR typo took
		// down the Kubernetes version. The version must survive a broken sibling document.
		It("still publishes the version when ClusterConfiguration is incomplete", func() {
			incomplete := clusterConfigurationSecret(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.33"
`)
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(incomplete, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeFalse())
		})

		It("still publishes the version when ClusterConfiguration is not parsable at all", func() {
			garbage := clusterConfigurationSecret("this: is: not: valid: yaml:\n\t- broken\n")

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(garbage+moduleConfigYAML("1.35"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.35"))
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

		driftMetricFrozenLabel := func() string {
			for _, m := range f.MetricsCollector.CollectedMetrics() {
				if m.Name == "d8_control_plane_default_version_drift" {
					return m.Labels["frozen"]
				}
			}
			return ""
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
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())

			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(`desiredVersion: "1.38"`))
			Expect(cm.Field("data.spec").String()).To(ContainSubstring("updateMode: Automatic"))

			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
			Expect(driftMetricFrozenLabel()).To(Equal("true"))
		})

		// Both freeze-memory slots (data.spec.desiredVersion and data.status.currentVersion) live
		// inside the ConfigMap, so deleting it wipes them together — while maxUsed survives in the
		// Secret. The guard therefore still knows the window is violated; without a third memory
		// source it would nevertheless publish the lower Default and quietly downgrade the digit,
		// all while raising the drift metric as if the freeze had worked.
		It("keeps the frozen digit from the previous run when the ConfigMap is deleted", func() {
			secretWithMaxUsed := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateCClusterConfiguration)) + `
  maxUsedControlPlaneKubernetesVersion: ` + base64.StdEncoding.EncodeToString([]byte("1.38")) + `
`
			// Previous run published 1.38 and left it in Values; the ConfigMap is now gone.
			f.ValuesSet("global.discovery.targetKubernetesVersion", "1.38")

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))

			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(`desiredVersion: "1.38"`))

			Expect(driftMetricFrozenLabel()).To(Equal("true"))
		})

		// Nothing to hold the digit at: no ConfigMap and no previously published value. The version
		// does move down to Default here — that is unavoidable — but the metric must say so instead
		// of looking identical to a successful freeze.
		It("reports frozen=false when the window is violated and there is no memory left", func() {
			secretWithMaxUsed := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateCClusterConfiguration)) + `
  maxUsedControlPlaneKubernetesVersion: ` + base64.StdEncoding.EncodeToString([]byte("1.38")) + `
`
			f.ValuesSet("global.discovery.targetKubernetesVersion", "")

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))

			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
			Expect(driftMetricFrozenLabel()).To(Equal("false"))
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
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})

		It("fail-opens when Automatic has no maxUsed baseline", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})

		// A trailing newline in the Secret value used to make semver.NewVersion fail here, and the
		// caller swallows that error (`err == nil && !inWindow`) — so the soft-guard silently
		// switched itself off on exactly the byte admission has always trimmed and rejected on.
		It("freezes even when maxUsed carries a trailing newline", func() {
			secretWithMaxUsed := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateCClusterConfiguration)) + `
  "maxUsedControlPlaneKubernetesVersion": ` + base64.StdEncoding.EncodeToString([]byte("  1.38  \n"))
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
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

			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
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
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
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
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeFalse())
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})
	})
})
