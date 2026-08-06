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

/*
1. No nodes in the cluster;
2. Some nodes exist;
3. There are Pods with control-plane.
*/

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type input struct {
	nodeVersions         []string
	configVersion        string
	controlPlaneVersions []string

	// The three interchangeable sources of the maxUsed floor, in descending freshness. An empty
	// string means "this source has nothing", which is a supported state for each of them.
	maxUsedInValues            string
	maxUsedInConfigMap         string
	maxUsedControlPlaneVersion string // the d8-cluster-configuration Secret key (migration seed)

	defaultVersionInSecret string
}

type output struct {
	maxUsedControlPlaneVersion string
	effectiveVersion           string
	minUsedVersion             string
}

func setStateFromTestCase(hec *HookExecutionConfig, caseInput input) {
	const nodeTemplate = `
---
apiVersion: v1
kind: Node
metadata:
  name: test-<<INDEX>>
status:
  nodeInfo:
    kubeletVersion: "<<PLACEHOLDER>>"
`

	const kubeApiserverPodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver-kube-master-<<INDEX>>
  namespace: kube-system
  labels:
    component: kube-apiserver
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: "<<PLACEHOLDER>>"
`

	const kubeControllerManagerPodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager-kube-master-<<INDEX>>
  namespace: kube-system
  labels:
    component: kube-controller-manager
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: "<<PLACEHOLDER>>"
`

	const kubeSchedulerPodTemplate = `
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler-kube-master-<<INDEX>>
  namespace: kube-system
  labels:
    component: kube-scheduler
    tier: control-plane
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: "<<PLACEHOLDER>>"
`

	const d8ConfigurationSecretTemplate = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: kube-system
data:
  <<PLACEHOLDER_MAX_USED>>
  <<PLACEHOLDER_DEFAULT>>
`

	const clusterKubernetesConfigMapTemplate = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cluster-kubernetes
  namespace: kube-system
data:
  spec: |
    desiredVersion: "<<PLACEHOLDER_DESIRED>>"
    updateMode: Manual
    maxUsedKubernetesVersion: "<<PLACEHOLDER_MAX_USED>>"
`

	var b strings.Builder

	for index, nodeVersion := range caseInput.nodeVersions {
		nodeManifest := strings.ReplaceAll(nodeTemplate, "<<PLACEHOLDER>>", nodeVersion)
		nodeManifest = strings.ReplaceAll(nodeManifest, "<<INDEX>>", strconv.Itoa(index))

		b.WriteString(nodeManifest)
	}

	for index, controlPlaneVersion := range caseInput.controlPlaneVersions {
		kubeApiserverManifest := strings.ReplaceAll(kubeApiserverPodTemplate, "<<PLACEHOLDER>>", controlPlaneVersion)
		kubeApiserverManifest = strings.ReplaceAll(kubeApiserverManifest, "<<INDEX>>", strconv.Itoa(index))
		b.WriteString(kubeApiserverManifest)

		kubeControllerManager := strings.ReplaceAll(kubeControllerManagerPodTemplate, "<<PLACEHOLDER>>", controlPlaneVersion)
		kubeControllerManager = strings.ReplaceAll(kubeControllerManager, "<<INDEX>>", strconv.Itoa(index))
		b.WriteString(kubeControllerManager)

		kubeSchedulerManifest := strings.ReplaceAll(kubeSchedulerPodTemplate, "<<PLACEHOLDER>>", controlPlaneVersion)
		kubeSchedulerManifest = strings.ReplaceAll(kubeSchedulerManifest, "<<INDEX>>", strconv.Itoa(index))
		b.WriteString(kubeSchedulerManifest)
	}

	// Both Secret keys are omitted rather than emitted empty when unset: an empty value is not the
	// same state as an absent key, and only the absent one models a cluster that has never
	// recorded the version.
	maxUsedInSecret := ""
	if caseInput.maxUsedControlPlaneVersion != "" {
		maxUsedInSecret = fmt.Sprintf("maxUsedControlPlaneKubernetesVersion: %s", base64.StdEncoding.EncodeToString([]byte(caseInput.maxUsedControlPlaneVersion)))
	}
	secretContent := strings.ReplaceAll(d8ConfigurationSecretTemplate, "<<PLACEHOLDER_MAX_USED>>", maxUsedInSecret)
	deckhouseDefaultKubernetesVersion := ""
	if caseInput.defaultVersionInSecret != "" {
		deckhouseDefaultKubernetesVersion = fmt.Sprintf("deckhouseDefaultKubernetesVersion: %s", base64.StdEncoding.EncodeToString([]byte(caseInput.defaultVersionInSecret)))
	}
	b.WriteString(strings.ReplaceAll(secretContent, "<<PLACEHOLDER_DEFAULT>>", deckhouseDefaultKubernetesVersion))

	if caseInput.maxUsedInConfigMap != "" {
		configMap := strings.ReplaceAll(clusterKubernetesConfigMapTemplate, "<<PLACEHOLDER_MAX_USED>>", caseInput.maxUsedInConfigMap)
		configMap = strings.ReplaceAll(configMap, "<<PLACEHOLDER_DESIRED>>", caseInput.configVersion)
		b.WriteString(configMap)
	}

	clusterConf := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
cloud:
  prefix: sandbox
  provider: OpenStack
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "%s"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`, caseInput.configVersion)
	hec.ValuesSetFromYaml("global.clusterConfiguration", []byte(clusterConf))
	hec.ValuesSet("global.discovery.targetKubernetesVersion", caseInput.configVersion)
	if caseInput.maxUsedInValues != "" {
		hec.ValuesSet("controlPlaneManager.internal.maxUsedKubernetesVersion", caseInput.maxUsedInValues)
	}
	hec.BindingContexts.Set(hec.KubeStateSet(b.String()))
}

var _ = Describe("Modules :: control-plane-manager :: hooks :: effective_kubernetes_version ::", func() {
	// set default value for kubernetes version for testing purposes
	DefaultKubernetesVersion = "1.33"

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(BeEquivalentTo("kubernetesVersion required (global.discovery.targetKubernetesVersion is empty)"))
		})
	})

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		DescribeTable("version change",
			func(in input, out output) {
				setStateFromTestCase(f, in)
				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				// maxUsed now travels through values (and from there into the DaemonSet env and
				// the ConfigMap), not through the Secret.
				Expect(f.ValuesGet("controlPlaneManager.internal.maxUsedKubernetesVersion").String()).To(Equal(out.maxUsedControlPlaneVersion))

				d8ClusterConfigSecret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
				// The Secret key is read as a migration seed and never written back: whatever the
				// fixture put there must survive the hook untouched.
				decodedMaxUsedKubernetesVersion, err := base64.StdEncoding.DecodeString(d8ClusterConfigSecret.Field("data.maxUsedControlPlaneKubernetesVersion").String())
				Expect(err).To(BeNil())
				Expect(string(decodedMaxUsedKubernetesVersion)).To(Equal(in.maxUsedControlPlaneVersion))

				defaultKubernetesVersion, err := base64.StdEncoding.DecodeString(d8ClusterConfigSecret.Field("data.deckhouseDefaultKubernetesVersion").String())
				Expect(err).To(BeNil())
				Expect(string(defaultKubernetesVersion)).To(Equal(DefaultKubernetesVersion))

				Expect(f.ValuesGet("controlPlaneManager.internal.effectiveKubernetesVersion").String()).To(Equal(out.effectiveVersion))

				minVer, ok := requirements.GetValue(minK8sVersionRequirementKey)
				Expect(ok).To(BeTrue())
				Expect(minVer.(string)).To(Equal(out.minUsedVersion))
			},
			Entry("upgrade: Node version lower than control plane, do not allow to bump effective version and max used version",
				input{
					nodeVersions:               []string{"v1.32.3", "v1.32.1", "v1.32.5", "v1.32.2"},
					maxUsedControlPlaneVersion: "1.33",
					configVersion:              "1.34",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
				},
				output{
					maxUsedControlPlaneVersion: "1.33",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.32.1",
				},
			),
			Entry("upgrade: control plane and nodes are on the same version, allow bumping effective version and max used version", input{
				nodeVersions:               []string{"v1.32.10", "v1.32.3", "v1.32.5", "v1.32.2"},
				maxUsedControlPlaneVersion: "1.32",
				configVersion:              "1.33",
				controlPlaneVersions:       []string{"1.32", "1.32", "1.32"},
			},
				output{
					maxUsedControlPlaneVersion: "1.33",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.32.2",
				},
			),
			Entry("upgrade: control plane and nodes are on the same version (but kube-scheduler is on a lower version), do not bump effective version and max used version",
				input{
					nodeVersions:               []string{"v1.33.10", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedControlPlaneVersion: "1.33",
					configVersion:              "1.34",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.32"},
				},
				output{
					maxUsedControlPlaneVersion: "1.33",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.33.2",
				},
			),
			Entry("downgrade: control plane and nodes are on the same version, do not lower effective version",
				input{
					nodeVersions:               []string{"v1.33.10", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedControlPlaneVersion: "1.33",
					configVersion:              "1.32",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
				},
				output{
					maxUsedControlPlaneVersion: "1.33",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.33.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, lower effective version",
				input{
					nodeVersions:               []string{"v1.33.10", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedControlPlaneVersion: "1.33",
					configVersion:              "1.32",
					controlPlaneVersions:       []string{"1.34", "1.34", "1.34"},
				},
				output{
					maxUsedControlPlaneVersion: "1.34",
					effectiveVersion:           "1.34",
					minUsedVersion:             "1.33.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, but configVersion is 2 minor versions lower, lower effective version by one",
				input{
					nodeVersions:               []string{"v1.33.10", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedControlPlaneVersion: "1.34",
					configVersion:              "1.32",
					controlPlaneVersions:       []string{"1.34", "1.34", "1.34"},
				},
				output{
					maxUsedControlPlaneVersion: "1.34",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.33.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, but maxUsedControlPlaneVersion does not allow us to downgrade by more than 1",
				input{
					nodeVersions:               []string{"v1.32.4", "v1.32.3", "v1.32.5", "v1.32.2"},
					maxUsedControlPlaneVersion: "1.34",
					configVersion:              "1.32",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
				},
				output{
					maxUsedControlPlaneVersion: "1.34",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.32.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, maxUsedControlPlaneVersion does not allow us to downgrade by more than 1, but we already violating maxUsedControlPlaneVersion",
				input{
					nodeVersions:               []string{"v1.32.4", "v1.32.3", "v1.32.5", "v1.32.2"},
					maxUsedControlPlaneVersion: "1.35",
					configVersion:              "1.33",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
				},
				output{
					maxUsedControlPlaneVersion: "1.35",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.32.2",
				},
			),
			Entry("deckhouse default version should be changed",
				input{
					nodeVersions:               []string{"v1.32.4", "v1.32.3", "v1.32.5", "v1.32.2"},
					maxUsedControlPlaneVersion: "1.35",
					configVersion:              "1.33",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
					defaultVersionInSecret:     "1.33",
				},
				output{
					maxUsedControlPlaneVersion: "1.35",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.32.2",
				},
			),
			// The floor is never seeded from the declared version. Declaring 1.36 on a 1.32
			// cluster must record 1.33 (the version the cluster is actually moving onto) —
			// recording 1.36 would raise the downgrade floor to 1.35 forever on a typo.
			Entry("no history anywhere: maxUsed follows the effective version, not the declared one",
				input{
					nodeVersions:         []string{"v1.32.4", "v1.32.3", "v1.32.5", "v1.32.2"},
					configVersion:        "1.36",
					controlPlaneVersions: []string{"1.32", "1.32", "1.32"},
				},
				output{
					maxUsedControlPlaneVersion: "1.33",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.32.2",
				},
			),
			// Upgrade from a release that only ever wrote the Secret: the ConfigMap key and values
			// are both empty, and the floor must still come out at the historical maximum instead
			// of collapsing onto the version the cluster happens to sit on today.
			Entry("upgrade from the previous release: the Secret alone carries the floor",
				input{
					nodeVersions:               []string{"v1.33.4", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedControlPlaneVersion: "1.35",
					configVersion:              "1.33",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
				},
				output{
					maxUsedControlPlaneVersion: "1.35",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.33.2",
				},
			),
			Entry("the ConfigMap outranks a stale Secret",
				input{
					nodeVersions:               []string{"v1.33.4", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedControlPlaneVersion: "1.33",
					maxUsedInConfigMap:         "1.35",
					configVersion:              "1.33",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
				},
				output{
					maxUsedControlPlaneVersion: "1.35",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.33.2",
				},
			),
			// values hold what this hook published on its previous run, before the DaemonSet
			// rolled out and update-observer could write it down. Without this source the floor
			// would drop back for the duration of the rollout.
			Entry("values outrank a ConfigMap that has not caught up yet",
				input{
					nodeVersions:               []string{"v1.33.4", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedControlPlaneVersion: "1.33",
					maxUsedInConfigMap:         "1.34",
					maxUsedInValues:            "1.35",
					configVersion:              "1.33",
					controlPlaneVersions:       []string{"1.33", "1.33", "1.33"},
				},
				output{
					maxUsedControlPlaneVersion: "1.35",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.33.2",
				},
			),
			// The floor gates the downgrade step: standing exactly on it means the downgrade has
			// not started yet, so one minor down is allowed.
			Entry("the ConfigMap floor gates the downgrade step",
				input{
					nodeVersions:         []string{"v1.33.10", "v1.33.3", "v1.33.5", "v1.33.2"},
					maxUsedInConfigMap:   "1.34",
					configVersion:        "1.32",
					controlPlaneVersions: []string{"1.34", "1.34", "1.34"},
				},
				output{
					maxUsedControlPlaneVersion: "1.34",
					effectiveVersion:           "1.33",
					minUsedVersion:             "1.33.2",
				},
			),
		)
	})

	Context("ModuleConfig kubernetesVersion override", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		It("MC kubernetesVersion takes precedence over ClusterConfiguration", func() {
			// Global discovery already resolved MC-over-CC into targetKubernetesVersion.
			setStateFromTestCase(f, input{
				nodeVersions:               []string{"v1.34.3", "v1.34.1", "v1.34.5", "v1.34.2"},
				maxUsedControlPlaneVersion: "1.34",
				configVersion:              "1.35",
				controlPlaneVersions:       []string{"1.34", "1.34", "1.34"},
			})
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.effectiveKubernetesVersion").String()).To(Equal("1.35"))
		})

		It("Automatic target follows the resolved ClusterConfiguration version", func() {
			setStateFromTestCase(f, input{
				nodeVersions:               []string{"v1.34.3", "v1.34.1", "v1.34.5", "v1.34.2"},
				maxUsedControlPlaneVersion: "1.34",
				configVersion:              "1.34",
				controlPlaneVersions:       []string{"1.34", "1.34", "1.34"},
			})
			f.RunHook()

			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.effectiveKubernetesVersion").String()).To(Equal("1.34"))
		})
	})
})
