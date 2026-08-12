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

	Context("targetKubernetesVersion priority MC / CC / Default", func() {
		It("pinned ModuleConfig wins over pinned ClusterConfiguration", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.35"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.35"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeFalse())
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

		// Handing it onward as a pin would make the control-plane-manager hook call
		// semver.NewVersion("Automatic") on every run.
		It("ignores a ModuleConfig value that is neither Default nor a version", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeFalse())
		})

		// Dropped, not coerced — see applyControlPlaneManagerKubernetesVersionFilter.
		It("ignores a non-string ModuleConfig kubernetesVersion instead of failing the hook", func() {
			numericModuleConfig := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  enabled: true
  version: 1
  settings:
    kubernetesVersion: 1.32
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+numericModuleConfig, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.33"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeFalse())
		})

		It("falls through to Default when both documents carry an unusable value", func() {
			brokenCC := clusterConfigurationSecret(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "not-a-version"
clusterDomain: "test.local"
`)
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(brokenCC+moduleConfigYAML("Automatic"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
		})

		// Why this hook is separate: cluster_configuration.go errors on a malformed CIDR field, and
		// while they shared a hook such a typo took the Kubernetes version down with it.
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

	// update-observer owns every key and receives these values as container environment.
	Context("ConfigMap d8-cluster-kubernetes is not written by this hook", func() {
		It("does not create the ConfigMap when it is missing", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.35"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.35"))
			Expect(f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes").Exists()).To(BeFalse())
		})

		It("leaves an existing ConfigMap untouched", func() {
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    name: d8-cluster-kubernetes
    k8s-version: "1.34"
    max-k8s-version: "1.34"
data:
  spec: |
    desiredVersion: "1.34"
    updateMode: Manual
    maxUsedKubernetesVersion: "1.34"
  status: |
    currentVersion: "1.34"
    phase: UpToDate
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateA+moduleConfigYAML("1.35")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.35"))

			cm := f.KubernetesResource("ConfigMap", "kube-system", "d8-cluster-kubernetes")
			Expect(cm.Field("data.spec").String()).To(ContainSubstring(`desiredVersion: "1.34"`))
			Expect(cm.Field("data.status").String()).To(ContainSubstring(`currentVersion: "1.34"`))
			Expect(cm.Field("metadata.labels.k8s-version").String()).To(Equal("1.34"))
		})
	})

	Context("Default soft-guard freeze", func() {
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
    name: d8-cluster-kubernetes
    k8s-version: "1.38"
    max-k8s-version: "1.38"
data:
  spec: |
    desiredVersion: "1.38"
    updateMode: Automatic
    maxUsedKubernetesVersion: "1.38"
  status: |
    currentVersion: "1.38"
    phase: UpToDate
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Default")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())

			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
			Expect(driftMetricFrozenLabel()).To(Equal("true"))
		})

		// The Secret key stops being written in this release, so it can only be the staler source.
		It("prefers the ConfigMap floor over the Secret one", func() {
			secretWithStaleMaxUsed := `
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  "cluster-configuration.yaml": ` + base64.StdEncoding.EncodeToString([]byte(stateCClusterConfiguration)) + `
  maxUsedControlPlaneKubernetesVersion: ` + base64.StdEncoding.EncodeToString([]byte("1.38")) + `
`
			// The ConfigMap says 1.36, which puts Default (1.36) inside the window — no freeze.
			existing := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    name: d8-cluster-kubernetes
data:
  spec: |
    desiredVersion: "1.36"
    updateMode: Automatic
    maxUsedKubernetesVersion: "1.36"
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithStaleMaxUsed+moduleConfigYAML("Default")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})

		// Regression: desiredVersion is this hook's own previous output routed back through the
		// ConfigMap, so reading it first lets a bad target confirm itself forever.
		It("freezes at currentVersion when desiredVersion was already dragged below it", func() {
			poisoned := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    name: d8-cluster-kubernetes
data:
  spec: |
    desiredVersion: "` + hooks.DefaultKubernetesVersion + `"
    updateMode: Automatic
    maxUsedKubernetesVersion: "1.38"
  status: |
    currentVersion: "1.38"
    phase: ControlPlaneUpdating
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Default")+poisoned, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
			Expect(driftMetricFrozenLabel()).To(Equal("true"))
		})

		// desiredVersion still serves as the second source: a ConfigMap seeded by dhctl at bootstrap
		// carries spec only, so status.currentVersion is not there yet.
		It("falls back to desiredVersion when the ConfigMap has no status yet", func() {
			specOnly := `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
  labels:
    heritage: deckhouse
    name: d8-cluster-kubernetes
data:
  spec: |
    desiredVersion: "1.38"
    updateMode: Automatic
    maxUsedKubernetesVersion: "1.38"
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Default")+specOnly, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))
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

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Default"), 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))
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

			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Default"), 1))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Default")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal(hooks.DefaultKubernetesVersion))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
			_, found := findDriftMetric()
			Expect(found).To(BeFalse())
		})

		It("fail-opens when Automatic has no maxUsed baseline", func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(stateC+moduleConfigYAML("Default"), 1))
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Default")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))

			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
		})

		It("falls back to the Secret when the ConfigMap spec carries no maxUsed", func() {
			// The ConfigMap exists and its spec is readable, but predates the
			// maxUsedKubernetesVersion key — the state of every cluster between a Deckhouse
			// upgrade and the DaemonSet rollout that first writes it. The Secret (1.38) has to
			// carry the floor through that window, or the freeze would not happen at all.
			//
			// The max-k8s-version label below is deliberately lower and deliberately ignored: it
			// used to be a baseline source and is no longer read by anything.
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
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Default")+existing, 1))
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.targetKubernetesVersion").String()).To(Equal("1.38"))
			Expect(f.ValuesGet("global.discovery.kubernetesVersionIsDefault").Bool()).To(BeTrue())
			value, found := findDriftMetric()
			Expect(found).To(BeTrue())
			Expect(value).To(Equal(1.0))
		})

		It("falls through to the Secret when the ConfigMap maxUsed is not a version", func() {
			// data.spec is hand-editable by design — update-observer rewrites edits on its next
			// pass — so an unusable value there is reachable, and in the window before that pass the
			// guard reads it. It must be discarded in favour of the next source, not allowed to
			// shadow it: taking the ConfigMap value first and only then failing to parse it left the
			// guard switched off with a good 1.38 sitting in the Secret.
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
data:
  spec: |
    desiredVersion: "1.38"
    updateMode: Automatic
    maxUsedKubernetesVersion: "latest"
  status: |
    currentVersion: "1.38"
    phase: UpToDate
`
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(secretWithMaxUsed+moduleConfigYAML("Default")+existing, 1))
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
