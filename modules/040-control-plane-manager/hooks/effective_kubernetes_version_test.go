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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

type input struct {
	nodeVersions               []string
	maxUsedControlPlaneVersion string
	configVersion              string
	controlPlaneVersions       []string
	defaultVersionInSecret     string
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
  maxUsedControlPlaneKubernetesVersion: "<<PLACEHOLDER_B64>>"
  <<PLACEHOLDER_DEFAULT>>
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

	secretContent := strings.ReplaceAll(d8ConfigurationSecretTemplate, "<<PLACEHOLDER_B64>>", base64.StdEncoding.EncodeToString([]byte(caseInput.maxUsedControlPlaneVersion)))
	deckhouseDefaultKubernetesVersion := ""
	if caseInput.defaultVersionInSecret != "" {
		deckhouseDefaultKubernetesVersion = fmt.Sprintf("deckhouseDefaultKubernetesVersion: %s", base64.StdEncoding.EncodeToString([]byte(caseInput.defaultVersionInSecret)))
	}
	b.WriteString(strings.ReplaceAll(secretContent, "<<PLACEHOLDER_DEFAULT>>", deckhouseDefaultKubernetesVersion))

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
	hec.BindingContexts.Set(hec.KubeStateSet(b.String()))
}

var _ = Describe("Modules :: control-plane-manager :: hooks :: get_pki_checksum ::", func() {
	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(BeEquivalentTo("global.clusterConfiguration.kubernetesVersion required"))
		})
	})

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		DescribeTable("version change",
			func(in input, out output) {
				setStateFromTestCase(f, in)
				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				d8ClusterConfigSecret := f.KubernetesResource("Secret", "kube-system", "d8-cluster-configuration")
				decodedMaxUsedKubernetesVersion, err := base64.StdEncoding.DecodeString(d8ClusterConfigSecret.Field("data.maxUsedControlPlaneKubernetesVersion").String())
				Expect(err).To(BeNil())
				Expect(string(decodedMaxUsedKubernetesVersion)).To(Equal(out.maxUsedControlPlaneVersion))

				defaultKubernetesVersion, err := base64.StdEncoding.DecodeString(d8ClusterConfigSecret.Field("data.deckhouseDefaultKubernetesVersion").String())
				Expect(err).To(BeNil())
				Expect(string(defaultKubernetesVersion)).To(Equal(config.DefaultKubernetesVersion))

				Expect(f.ValuesGet("controlPlaneManager.internal.effectiveKubernetesVersion").String()).To(Equal(out.effectiveVersion))

				minVer, ok := requirements.GetValue(minK8sVersionRequirementKey)
				Expect(ok).To(BeTrue())
				Expect(minVer.(string)).To(Equal(out.minUsedVersion))
			},
			Entry("upgrade: Node version lower than control plane, do not allow to bump effective version and max used version",
				input{
					nodeVersions:               []string{"v1.26.3", "v1.26.1", "v1.26.5", "v1.27.2"},
					maxUsedControlPlaneVersion: "1.27",
					configVersion:              "1.28",
					controlPlaneVersions:       []string{"1.27", "1.27", "1.27"},
				},
				output{
					maxUsedControlPlaneVersion: "1.27",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.26.1",
				},
			),
			Entry("upgrade: control plane and nodes are on the same version, allow bumping effective version and max used version", input{
				nodeVersions:               []string{"v1.27.10", "v1.27.3", "v1.27.5", "v1.27.2"},
				maxUsedControlPlaneVersion: "1.27",
				configVersion:              "1.28",
				controlPlaneVersions:       []string{"1.27", "1.27", "1.27"},
			},
				output{
					maxUsedControlPlaneVersion: "1.28",
					effectiveVersion:           "1.28",
					minUsedVersion:             "1.27.2",
				},
			),
			Entry("upgrade: control plane and nodes are on the same version (but kube-scheduler is on a lower version), do not bump effective version and max used version",
				input{
					nodeVersions:               []string{"v1.27.10", "v1.27.3", "v1.27.5", "v1.27.2"},
					maxUsedControlPlaneVersion: "1.27",
					configVersion:              "1.28",
					controlPlaneVersions:       []string{"1.27", "1.27", "1.26"},
				},
				output{
					maxUsedControlPlaneVersion: "1.27",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.27.2",
				},
			),
			Entry("downgrade: control plane and nodes are on the same version, do not lower effective version",
				input{
					nodeVersions:               []string{"v1.27.10", "v1.27.3", "v1.27.5", "v1.27.2"},
					maxUsedControlPlaneVersion: "1.27",
					configVersion:              "1.26",
					controlPlaneVersions:       []string{"1.27", "1.27", "1.27"},
				},
				output{
					maxUsedControlPlaneVersion: "1.27",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.27.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, lower effective version",
				input{
					nodeVersions:               []string{"v1.27.10", "v1.27.3", "v1.27.5", "v1.27.2"},
					maxUsedControlPlaneVersion: "1.28",
					configVersion:              "1.26",
					controlPlaneVersions:       []string{"1.28", "1.28", "1.28"},
				},
				output{
					maxUsedControlPlaneVersion: "1.28",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.27.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, but configVersion is 2 minor versions lower, lower effective version by one",
				input{
					nodeVersions:               []string{"v1.27.10", "v1.27.3", "v1.27.5", "v1.27.2"},
					maxUsedControlPlaneVersion: "1.28",
					configVersion:              "1.26",
					controlPlaneVersions:       []string{"1.28", "1.28", "1.28"},
				},
				output{
					maxUsedControlPlaneVersion: "1.28",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.27.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, but maxUsedControlPlaneVersion does not allow us to downgrade by more than 1",
				input{
					nodeVersions:               []string{"v1.26.4", "v1.26.3", "v1.26.5", "v1.26.2"},
					maxUsedControlPlaneVersion: "1.28",
					configVersion:              "1.26",
					controlPlaneVersions:       []string{"1.27", "1.27", "1.27"},
				},
				output{
					maxUsedControlPlaneVersion: "1.28",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.26.2",
				},
			),
			Entry("downgrade: nodes are downgraded already, maxUsedControlPlaneVersion does not allow us to downgrade by more than 1, but we already violating maxUsedControlPlaneVersion",
				input{
					nodeVersions:               []string{"v1.26.4", "v1.26.3", "v1.26.5", "v1.26.2"},
					maxUsedControlPlaneVersion: "1.29",
					configVersion:              "1.27",
					controlPlaneVersions:       []string{"1.27", "1.27", "1.27"},
				},
				output{
					maxUsedControlPlaneVersion: "1.29",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.26.2",
				},
			),
			Entry("deckhouse default version should be changed",
				input{
					nodeVersions:               []string{"v1.26.4", "v1.26.3", "v1.26.5", "v1.26.2"},
					maxUsedControlPlaneVersion: "1.29",
					configVersion:              "1.27",
					controlPlaneVersions:       []string{"1.27", "1.27", "1.27"},
					defaultVersionInSecret:     "1.27",
				},
				output{
					maxUsedControlPlaneVersion: "1.29",
					effectiveVersion:           "1.27",
					minUsedVersion:             "1.26.2",
				},
			),
		)
	})
})
